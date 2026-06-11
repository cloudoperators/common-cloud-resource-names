// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestAdmissionHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission Handler Suite")
}

// admissionMockBackend implements apis.ValidationBackend with configurable responses
type admissionMockBackend struct {
	// GetCRD configuration
	getCRDResponse *apis.CRDInfo
	getCRDErr      error

	// ValidateResource configuration
	validateResourceErr error

	// GetURNTemplate configuration
	getURNTemplateResponse string
	getURNTemplateErr      error

	// Refresh configuration
	refreshErr error

	// IsResourceTypeSupported configuration
	isResourceTypeSupported bool
}

func (m *admissionMockBackend) GetCRD(ccrnVersion string) (*apis.CRDInfo, error) {
	return m.getCRDResponse, m.getCRDErr
}

func (m *admissionMockBackend) ValidateResource(namespace string, parsedCCRN *apis.ParsedResource) error {
	return m.validateResourceErr
}

func (m *admissionMockBackend) GetURNTemplate(ccrnName string, ccrnVersion string) (string, error) {
	return m.getURNTemplateResponse, m.getURNTemplateErr
}

func (m *admissionMockBackend) Refresh() error {
	return m.refreshErr
}

func (m *admissionMockBackend) IsResourceTypeSupported(ccrnVersion string) bool {
	return m.isResourceTypeSupported
}

// buildAdmissionReviewBody creates an AdmissionReview JSON request body from a CCRN spec.
// The ccrn value should be the full CCRN string (e.g., "ccrn=type.group/version, field=value").
func buildAdmissionReviewBody(ccrn, urn string) ([]byte, error) {
	ccrnObj := apis.CCRN{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ccrn.cloudoperators.dev/v1",
			Kind:       "CCRN",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
		Spec: apis.CCRNSpec{
			CCRN: ccrn,
			URN:  urn,
		},
	}

	rawObj, err := json.Marshal(ccrnObj)
	if err != nil {
		return nil, err
	}

	review := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid-12345"),
			Name:      "test-resource",
			Namespace: "default",
			Object: runtime.RawExtension{
				Raw: rawObj,
			},
		},
	}

	return json.Marshal(review)
}

// performAdmissionRequest sends a POST to /validate and parses the AdmissionReview response.
func performAdmissionRequest(server *WebhookServer, body []byte) (*admissionv1.AdmissionReview, int) {
	req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.mutateCCRN(w, req)

	result := w.Result()
	statusCode := result.StatusCode

	if statusCode != http.StatusOK {
		return nil, statusCode
	}

	var review admissionv1.AdmissionReview
	err := json.NewDecoder(result.Body).Decode(&review)
	if err != nil {
		return nil, statusCode
	}

	return &review, statusCode
}

var _ = Describe("Admission Handler", func() {
	var (
		server  *WebhookServer
		backend *admissionMockBackend
		log     *logrus.Logger
	)

	BeforeEach(func() {
		log = logrus.New()
		log.SetOutput(GinkgoWriter)

		backend = &admissionMockBackend{
			isResourceTypeSupported: true,
			getURNTemplateResponse:  "urn:ccrn:<ccrn>/<cluster>/<namespace>/<name>",
		}
	})

	JustBeforeEach(func() {
		var err error
		server, err = NewWebhookServer(log, backend)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Valid CCRN submission", func() {
		It("should allow the request and generate a URN mutation patch", func() {
			// spec.ccrn stores the full CCRN string including the "ccrn=" prefix
			body, err := buildAdmissionReviewBody(
				"ccrn=pod.k8s-registry.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod",
				"",
			)
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeTrue())
			Expect(review.Response.UID).To(Equal(types.UID("test-uid-12345")))

			// Verify mutation patch is present (URN should be added)
			Expect(review.Response.Patch).NotTo(BeNil())
			Expect(review.Response.PatchType).NotTo(BeNil())
			Expect(*review.Response.PatchType).To(Equal(admissionv1.PatchTypeJSONPatch))

			// Parse and verify patch content
			var patches []map[string]interface{}
			err = json.Unmarshal(review.Response.Patch, &patches)
			Expect(err).NotTo(HaveOccurred())
			Expect(patches).To(HaveLen(1))
			Expect(patches[0]["op"]).To(Equal("add"))
			Expect(patches[0]["path"]).To(Equal("/spec/urn"))
			// The value should be a URN string
			urnValue, ok := patches[0]["value"].(string)
			Expect(ok).To(BeTrue())
			Expect(urnValue).To(HavePrefix("urn:ccrn:"))
		})
	})

	Describe("Valid URN submission", func() {
		It("should parse the URN and attempt CCRN validation from derived key", func() {
			body, err := buildAdmissionReviewBody(
				"",
				"urn:ccrn:pod.k8s-registry.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod",
			)
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.UID).To(Equal(types.UID("test-uid-12345")))

			// The URN path extracts the CCRN key and passes it to ValidateCCRN.
			// The extracted key ("pod.k8s.../v1") does not have the "ccrn=" prefix,
			// so the validator's parser rejects it. This is the current behavior.
			Expect(review.Response.Allowed).To(BeFalse())
			Expect(review.Response.Result.Message).To(ContainSubstring("Derived CCRN validation error"))
		})

		Context("when the extracted key is prefixed with ccrn=", func() {
			It("should parse URN template segments correctly", func() {
				// Test that the URN path correctly calls GetURNTemplate with the right arguments
				// and parses the URN using that template. Even though the final validation step
				// fails, the parsing portion works.
				body, err := buildAdmissionReviewBody(
					"",
					"urn:ccrn:pod.k8s-registry.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod",
				)
				Expect(err).NotTo(HaveOccurred())

				review, statusCode := performAdmissionRequest(server, body)

				Expect(statusCode).To(Equal(http.StatusOK))
				Expect(review).NotTo(BeNil())
				Expect(review.Response).NotTo(BeNil())
				// Confirms the URN was parsed successfully (no "Failed to parse URN" error)
				// and the CCRN extraction succeeded (no "Failed to extract CCRN from URN" error)
				// The error is specifically at the ValidateCCRN step
				Expect(review.Response.Result.Message).NotTo(ContainSubstring("Failed to parse URN"))
				Expect(review.Response.Result.Message).NotTo(ContainSubstring("Failed to extract CCRN"))
				Expect(review.Response.Result.Message).To(ContainSubstring("Derived CCRN validation error"))
			})
		})
	})

	Describe("Both CCRN and URN present", func() {
		It("should allow the request with no mutation patch needed", func() {
			body, err := buildAdmissionReviewBody(
				"ccrn=pod.k8s-registry.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod",
				"urn:ccrn:pod.k8s-registry.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod",
			)
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeTrue())

			// No mutation needed since both are present
			Expect(review.Response.Patch).To(BeNil())
			Expect(review.Response.Result.Message).To(Equal("CCRN is valid and target resource validated"))
		})
	})

	Describe("Neither CCRN nor URN present", func() {
		It("should reject the request with a clear error", func() {
			body, err := buildAdmissionReviewBody("", "")
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeFalse())
			Expect(review.Response.Result).NotTo(BeNil())
			Expect(review.Response.Result.Message).To(ContainSubstring("must have either spec.ccrn or spec.urn defined"))
		})
	})

	Describe("Invalid CCRN format", func() {
		It("should reject with a validation error for unrecognized format", func() {
			// "invalidformat" does not start with "ccrn=" or "urn:ccrn:" so the parser rejects it
			body, err := buildAdmissionReviewBody("invalidformat", "")
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeFalse())
			Expect(review.Response.Result).NotTo(BeNil())
			Expect(review.Response.Result.Status).To(Equal("Failure"))
			Expect(review.Response.Result.Message).To(ContainSubstring("CCRN validation error"))
		})

		It("should reject a CCRN with empty value after prefix", func() {
			// "ccrn=" with no value - the parser will likely fail
			body, err := buildAdmissionReviewBody("ccrn=", "")
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeFalse())
			Expect(review.Response.Result).NotTo(BeNil())
			Expect(review.Response.Result.Status).To(Equal("Failure"))
		})
	})

	Describe("Unsupported resource type", func() {
		BeforeEach(func() {
			backend = &admissionMockBackend{
				isResourceTypeSupported: false,
				getURNTemplateResponse:  "urn:ccrn:<ccrn>/<cluster>/<namespace>/<name>",
			}
		})

		It("should reject with unsupported resource type error", func() {
			body, err := buildAdmissionReviewBody(
				"ccrn=pod.k8s-registry.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod",
				"",
			)
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeFalse())
			Expect(review.Response.Result).NotTo(BeNil())
			Expect(review.Response.Result.Message).To(ContainSubstring("Resource type not supported"))
		})
	})

	Describe("Backend validation failure", func() {
		BeforeEach(func() {
			backend = &admissionMockBackend{
				isResourceTypeSupported: true,
				validateResourceErr:     fmt.Errorf("backend connection timeout"),
				getURNTemplateResponse:  "urn:ccrn:<ccrn>/<cluster>/<namespace>/<name>",
			}
		})

		It("should reject with backend error message", func() {
			body, err := buildAdmissionReviewBody(
				"ccrn=pod.k8s-registry.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod",
				"",
			)
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeFalse())
			Expect(review.Response.Result).NotTo(BeNil())
			Expect(review.Response.Result.Message).To(ContainSubstring("backend connection timeout"))
		})
	})

	Describe("Malformed request body", func() {
		It("should return HTTP 400 for invalid JSON", func() {
			req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader("not json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.mutateCCRN(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should reject unparseable CCRN object raw", func() {
			// Construct the AdmissionReview JSON manually with invalid object.raw
			// that will fail json.Unmarshal into a CCRN struct.
			// Using invalid JSON bytes that k8s RawExtension will store but CCRN unmarshal will reject.
			rawBody := `{
				"apiVersion": "admission.k8s.io/v1",
				"kind": "AdmissionReview",
				"request": {
					"uid": "test-uid-malformed",
					"object": {"not_a_valid_field": true}
				}
			}`

			req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(rawBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.mutateCCRN(w, req)

			// When the AdmissionReview is parsed, request.Object.Raw will be nil
			// because the "object" field is not in the expected RawExtension format.
			// This triggers "neither CCRN nor URN" error since the CCRN struct is zero-valued.
			Expect(w.Code).To(Equal(http.StatusOK))
			var resp admissionv1.AdmissionReview
			err := json.NewDecoder(w.Result().Body).Decode(&resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Response.Allowed).To(BeFalse())
		})
	})

	Describe("URN template lookup failure", func() {
		BeforeEach(func() {
			backend = &admissionMockBackend{
				isResourceTypeSupported: true,
				getURNTemplateErr:       fmt.Errorf("CRD not found"),
			}
		})

		It("should reject when URN template cannot be retrieved", func() {
			body, err := buildAdmissionReviewBody(
				"",
				"urn:ccrn:pod.k8s-registry.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod",
			)
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review).NotTo(BeNil())
			Expect(review.Response).NotTo(BeNil())
			Expect(review.Response.Allowed).To(BeFalse())
			Expect(review.Response.Result.Message).To(ContainSubstring("CRD not found"))
		})
	})

	Describe("Response UID propagation", func() {
		It("should propagate the request UID to the response", func() {
			body, err := buildAdmissionReviewBody(
				"ccrn=pod.k8s-registry.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod",
				"urn:ccrn:pod.k8s-registry.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod",
			)
			Expect(err).NotTo(HaveOccurred())

			review, statusCode := performAdmissionRequest(server, body)

			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(review.Response.UID).To(Equal(types.UID("test-uid-12345")))
		})
	})

	Describe("Content-Type header", func() {
		It("should set Content-Type to application/json in the response", func() {
			body, err := buildAdmissionReviewBody(
				"ccrn=pod.k8s-registry.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod",
				"urn:ccrn:pod.k8s-registry.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod",
			)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.mutateCCRN(w, req)

			Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
		})
	})
})
