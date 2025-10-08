// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"context"
	"fmt"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"

	"k8s.io/apimachinery/pkg/util/rand"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesBackend implements ValidationBackend using a live Kubernetes cluster
type KubernetesBackend struct {
	log           *logrus.Logger
	kubeClient    kubernetes.Interface
	apiextClient  apiextensionsclientset.Interface
	dynamicClient dynamic.Interface
	ccrns         map[string]*apis.CRDInfo
	crdsMutex     sync.RWMutex
	ccrnGroup     string // CCRN group for filtering CRDs
}

// NewKubernetesBackend creates a new Kubernetes validation backend
func NewKubernetesBackend(config *rest.Config, log *logrus.Logger, ccrnGroup string) (*KubernetesBackend, error) {
	// Create Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create Dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create apiextensions client
	apiextClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	backend := &KubernetesBackend{
		log:           log,
		kubeClient:    kubeClient,
		apiextClient:  apiextClient,
		dynamicClient: dynamicClient,
		ccrns:         make(map[string]*apis.CRDInfo),
		ccrnGroup:     ccrnGroup,
	}

	// Initial load of CRDs
	if err := backend.Refresh(); err != nil {
		log.Warnf("Failed to load CRDs initially: %v", err)
	}

	return backend, nil
}

// GetCRD retrieves CRD information for a given apiVersion and kind
func (kb *KubernetesBackend) GetCRD(crdVersion string) (*apis.CRDInfo, error) {
	kb.crdsMutex.RLock()
	crdInfo, exists := kb.ccrns[crdVersion]
	kb.crdsMutex.RUnlock()

	if !exists {
		// Try to refresh CRDs to see if it was added recently
		if err := kb.Refresh(); err != nil {
			kb.log.Errorf("Failed to refresh CRDs: %v", err)
		}

		// Check again after refresh
		kb.crdsMutex.RLock()
		crdInfo, exists = kb.ccrns[crdVersion]
		kb.crdsMutex.RUnlock()

		if !exists {
			return nil, fmt.Errorf("CRD for resource type %s not found", crdVersion)
		}
	}

	return crdInfo, nil
}

// ValidateResource validates a resource by creating it in the Kubernetes cluster
func (kb *KubernetesBackend) ValidateResource(namespace string, parsedCCRN *apis.ParsedResource) error {

	// Get CRD info
	group := parsedCCRN.ApiGroup()
	version := parsedCCRN.Version()
	kind := parsedCCRN.GetKind()

	crdInfo, err := kb.GetCRD(parsedCCRN.CCRNKey())
	if err != nil {
		return err
	}

	// Generate a resource name based on the kind and timestamp
	resourceName := fmt.Sprintf("%s-%s-%d", strings.ToLower(kind), rand.String(4), time.Now().Unix())

	// Convert parsed CCRN to a resource map
	resourceObj := parsedCCRN.ToResourceMap(namespace, resourceName)

	// Get the resource API
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: crdInfo.Plural,
	}

	// Create the resource
	kb.log.WithField("resource", resourceObj).Infof("Creating resource %s/%s", namespace, resourceName)
	resourceClient := kb.dynamicClient.Resource(gvr).Namespace(namespace)
	_, err = resourceClient.Create(context.TODO(), &unstructured.Unstructured{Object: resourceObj}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	return nil
}

// GetURNTemplate retrieves the URN template from CRD annotations
func (kb *KubernetesBackend) GetURNTemplate(crdName, version string) (string, error) {
	// Get the CRD
	apiextClient := kb.apiextClient.ApiextensionsV1().CustomResourceDefinitions()
	crd, err := apiextClient.Get(context.TODO(), crdName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get CRD %s: %w", crdName, err)
	}

	annotationKey := fmt.Sprintf("ccrn/%s.urn-template", version)
	if urnFormat, exists := crd.Annotations[annotationKey]; exists && urnFormat != "" {
		return urnFormat, nil
	}

	return "", fmt.Errorf("URN Template %s not found in CRD %s", annotationKey, crdName)
}

// Refresh reloads CRD information from the cluster
func (kb *KubernetesBackend) Refresh() error {
	kb.log.Info("Refreshing CRDs cache")

	// Get all CRDs
	apiextClient := kb.apiextClient.ApiextensionsV1().CustomResourceDefinitions()
	crdList, err := apiextClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	// Clear the current cache
	kb.crdsMutex.Lock()
	defer kb.crdsMutex.Unlock()

	kb.ccrns = make(map[string]*apis.CRDInfo)

	// Add relevant CRDs to the cache
	for _, crd := range crdList.Items {
		if strings.Contains(crd.Spec.Group, kb.ccrnGroup) {
			for _, version := range crd.Spec.Versions {
				if version.Served {
					crdKey := kb.getCRDKeyFromCRD(&crd, version.Name)
					kb.log.Infof("Found CCRN related CRD: %s", crdKey)

					// Extract URN format if available
					urnFormat := ""
					annotationKey := fmt.Sprintf("ccrn/%s.urn-template", version.Name)
					if format, exists := crd.Annotations[annotationKey]; exists {
						urnFormat = format
					}

					// Store CRD info
					kb.ccrns[crdKey] = &apis.CRDInfo{
						Name:      crd.Name,
						Plural:    crd.Spec.Names.Plural,
						Singular:  crd.Spec.Names.Singular,
						Group:     crd.Spec.Group,
						Kind:      crd.Spec.Names.Kind,
						Version:   version.Name,
						Schema:    version.Schema.OpenAPIV3Schema,
						URNFormat: urnFormat,
					}
				}
			}
		}
	}

	kb.log.Infof("Refreshed CRDs cache, found %d relevant CRDs", len(kb.ccrns))
	return nil
}

// IsResourceTypeSupported checks if a resource type is supported
func (kb *KubernetesBackend) IsResourceTypeSupported(ccrnVersion string) bool {
	kb.crdsMutex.RLock()
	defer kb.crdsMutex.RUnlock()

	_, exists := kb.ccrns[ccrnVersion]

	return exists
}

// StartRefreshLoop starts a background goroutine to refresh CRDs periodically
func (kb *KubernetesBackend) StartRefreshLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := kb.Refresh(); err != nil {
				kb.log.Errorf("Failed to refresh CRDs: %v", err)
			}
		}
	}()
}

// getCRDKey generates a cache key for a CRD based on apiVersion and kind
func (kb *KubernetesBackend) getCRDKey(apiVersion, kind string) string {
	return strings.ToLower(fmt.Sprintf("%s.%s", kind, apiVersion))
}

// getCRDKeyFromCRD generates a cache key from a CRD object
func (kb *KubernetesBackend) getCRDKeyFromCRD(crd *apiextensionsv1.CustomResourceDefinition, version string) string {
	return strings.ToLower(fmt.Sprintf("%s.%s/%s", crd.Spec.Names.Kind, crd.Spec.Group, version))
}
