// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/parser"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/validation"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// WebhookServer implements the admission webhook for CCRN validation
type WebhookServer struct {
	log        *logrus.Logger
	validator  *validation.CCRNValidator
	backend    apis.ValidationBackend
	parser     *parser.ResourceParser
	httpServer *http.Server
	ready      atomic.Bool
}

// NewWebhookServer creates a new webhook server using the provided validation backend
func NewWebhookServer(log *logrus.Logger, backend apis.ValidationBackend) (*WebhookServer, error) {
	if log == nil {
		log = logrus.New()
		log.SetOutput(io.Discard)
	}

	server := &WebhookServer{
		log:       log,
		validator: validation.NewCCRNValidator(backend),
		backend:   backend,
		parser:    parser.NewResourceParser(log, backend),
	}

	// Initialize the HTTP mux and server
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", server.mutateCCRN)
	mux.HandleFunc("/healthz", server.healthz)
	server.httpServer = &http.Server{
		Handler: mux,
	}

	return server, nil
}

// NewWebhookServerFromConfig creates a new webhook server with Kubernetes backend (backward compatibility)
func NewWebhookServerFromConfig(log *logrus.Logger, ccrnGroup string) (*WebhookServer, error) {
	// Get in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	// Create Kubernetes backend
	backend, err := validation.NewKubernetesBackend(config, log, ccrnGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes backend: %w", err)
	}

	// Start the refresh loop
	backend.StartRefreshLoop(5 * time.Minute)

	return NewWebhookServer(log, backend)
}

// Serve starts the webhook server with TLS
func (s *WebhookServer) Serve(port int, certFile, keyFile string) error {
	s.httpServer.Addr = fmt.Sprintf(":%d", port)
	s.log.Infof("Starting webhook server on port %d", port)
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// ServeHTTP starts the webhook server without TLS (for testing).
// It accepts a net.Listener so callers can bind to port 0 for ephemeral ports.
func (s *WebhookServer) ServeHTTP(ln net.Listener) error {
	s.log.Infof("Starting webhook server (HTTP) on %s", ln.Addr())
	return s.httpServer.Serve(ln)
}

// Shutdown gracefully shuts down the webhook server, allowing in-flight requests to complete.
func (s *WebhookServer) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// SetReady marks the server as ready to serve traffic
func (s *WebhookServer) SetReady() {
	s.ready.Store(true)
}

// IsReady returns whether the server is ready to serve traffic
func (s *WebhookServer) IsReady() bool {
	return s.ready.Load()
}

// mutateCCRN is the HTTP handler for webhook mutation requests
func (s *WebhookServer) mutateCCRN(w http.ResponseWriter, r *http.Request) {
	// Read the AdmissionReview from the request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.log.Errorf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Decode the AdmissionReview
	admissionReview := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		s.log.Errorf("Failed to parse AdmissionReview: %v", err)
		http.Error(w, "Failed to parse AdmissionReview", http.StatusBadRequest)
		return
	}

	// Process the AdmissionRequest
	admissionResponse := s.handleCombinedRequest(admissionReview.Request)

	admissionResponse.UID = admissionReview.Request.UID
	admissionReview.Response = admissionResponse

	respBytes, err := json.Marshal(admissionReview)
	if err != nil {
		s.log.Errorf("Failed to marshal response: %v", err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(respBytes)
	if err != nil {
		s.log.Errorf("Failed to write response: %v", err)
	}
}

// handleCombinedRequest orchestrates the validation, mutation, and resource creation
func (s *WebhookServer) handleCombinedRequest(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	s.log.Infof("Handling combined request for %s/%s", request.Namespace, request.Name)

	// Parse the CCRN resource
	ccrn := &apis.CCRN{}
	if err := json.Unmarshal(request.Object.Raw, ccrn); err != nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("Failed to parse CCRN resource: %v", err),
			},
		}
	}

	// 1. Basic Validation
	parsedCCRN, validationResponse := s.validateFormats(ccrn)
	if validationResponse != nil {
		return validationResponse
	}

	// 2. Mutation (if needed)
	patches, mutated := s.generateMutationPatches(ccrn, parsedCCRN)

	// 3. Target Resource Creation/Validation
	if err := s.backend.ValidateResource(request.Namespace, parsedCCRN); err != nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("Resource validation failed: %v", err),
			},
		}
	}

	// Build the final success response with any patches for mutation
	response := &admissionv1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Status:  "Success",
			Message: "CCRN is valid and target resource validated",
		},
	}

	if mutated {
		patchBytes, err := json.Marshal(patches)
		if err != nil {
			s.log.Errorf("Failed to marshal patches: %v", err)
		} else {
			pt := admissionv1.PatchTypeJSONPatch
			response.Patch = patchBytes
			response.PatchType = &pt
			response.Result.Message = "CCRN is valid, missing format added, and target resource validated"
		}
	}

	return response
}

// validateFormats performs basic validation of the CCRN and URN formats
func (s *WebhookServer) validateFormats(ccrn *apis.CCRN) (*apis.ParsedResource, *admissionv1.AdmissionResponse) {
	if ccrn.Spec.CCRN == "" && ccrn.Spec.URN == "" {
		return nil, &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: "Resource must have either spec.ccrn or spec.urn defined",
			},
		}
	}

	var parsed *apis.ParsedResource

	if ccrn.Spec.CCRN != "" {
		result, err := s.validator.ValidateCCRN(ccrn.Spec.CCRN)
		if err != nil {
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf("CCRN validation error: %v", err),
				},
			}
		}
		if !result.Valid {
			errorMsg := "Invalid CCRN format"
			if len(result.Errors) > 0 {
				errorMsg = result.Errors[0]
			}
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: errorMsg,
				},
			}
		}
		parsed = result.ParsedCCRN
	} else {
		// URN path: get URN template from backend, parse URN, extract CCRN, validate
		// We need the CRD name and version to get the template. Assume URN is in the form urn:ccrn:<crd>/<version>/...
		// We'll extract the CRD name and version from the URN string.
		parts := strings.Split(strings.TrimPrefix(ccrn.Spec.URN, "urn:ccrn:"), "/")
		if len(parts) < 2 {
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: "URN does not contain enough segments to determine CRD and version",
				},
			}
		}
		crdName := parts[0]
		version := parts[1]
		urnTemplate, err := s.backend.GetURNTemplate(crdName, version)
		if err != nil {
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf("Failed to get URN template: %v", err),
				},
			}
		}
		parsed, err = s.parser.Parse(ccrn.Spec.URN, urnTemplate)
		if err != nil {
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf("Failed to parse URN: %v", err),
				},
			}
		}

		ccrnValue, err := s.parser.ExtractCCRNKeyFromURN(ccrn.Spec.URN)
		if err != nil {
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf("Failed to extract CCRN from URN: %v", err),
				},
			}
		}
		result, err := s.validator.ValidateCCRN(ccrnValue)
		if err != nil {
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf("Derived CCRN validation error: %v", err),
				},
			}
		}
		if !result.Valid {
			errorMsg := "Derived CCRN is invalid"
			if len(result.Errors) > 0 {
				errorMsg += ": " + result.Errors[0]
			}
			return nil, &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: errorMsg,
				},
			}
		}
		parsed = result.ParsedCCRN
	}
	return parsed, nil
}

// generateMutationPatches creates mutation patches if a format is missing
func (s *WebhookServer) generateMutationPatches(ccrn *apis.CCRN, parsedCCRN *apis.ParsedResource) ([]map[string]any, bool) {
	patches := []map[string]any{}

	// Case A: Has CCRN, need to potentially add URN
	if ccrn.Spec.CCRN != "" && ccrn.Spec.URN == "" {
		s.log.Infof("CCRN is present, generating URN from CCRN")
		template, err := s.backend.GetURNTemplate(parsedCCRN.CCRNName(), parsedCCRN.Version())
		if err != nil {
			s.log.Errorf("Failed to get URN template for %s/%s: %v", parsedCCRN.APIGroup(), parsedCCRN.Version(), err)
			return nil, false
		}
		urn := parsedCCRN.URN(template)
		if urn == "" {
			s.log.Errorf("Failed to generate URN from CCRN.")
			return nil, false
		}
		s.log.Infof("URN generated: %s", urn)
		patches = append(patches, map[string]any{
			"op":    "add",
			"path":  "/spec/urn",
			"value": urn,
		})

		// Case B: Has URN but no CCRN, add CCRN
	} else if ccrn.Spec.URN != "" && ccrn.Spec.CCRN == "" {
		s.log.Infof("URN is present, generating CCRN from URN")
		// Validate URN and derive CCRN
		parsedURN, err := s.parser.Parse(ccrn.Spec.URN, parser.DefaultURNTemplate) // Use default template to get the ccrn field
		if err != nil {
			s.log.Errorf("Failed to parse URN using default template: %v", err)
			return nil, false
		}
		ccrnValue := parsedURN.CCRN()

		s.log.Infof("CCRN generated: %s", ccrnValue)
		patches = append(patches, map[string]any{
			"op":    "add",
			"path":  "/spec/ccrn",
			"value": ccrnValue,
		})
	}

	return patches, len(patches) > 0
}

// healthz is the health check endpoint. Returns 503 if the server is not ready,
// 200 otherwise.
func (s *WebhookServer) healthz(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("not ready")); err != nil {
			s.log.Errorf("Failed to write response: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		s.log.Errorf("Failed to write response: %v", err)
	}
}
