package apis

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ValidationBackend defines the interface for different validation implementations
type ValidationBackend interface {
	// GetCRD retrieves CRD information for a given apiVersion and kind
	GetCRD(ccrnVersion string) (*CRDInfo, error)

	// ValidateResource validates a resource against its schema
	// For KubernetesBackend, this creates an actual resource
	// For FilesystemBackend, this validates against OpenAPI schema
	ValidateResource(namespace string, parsedCCRN *ParsedResource) error

	// GetURNTemplate retrieves the URN template from CRD annotations
	GetURNTemplate(ccrnName string, ccrnVersion string) (string, error)

	// Refresh reloads CRD information
	Refresh() error

	// IsResourceTypeSupported checks if a resource type is supported
	IsResourceTypeSupported(ccrnVersion string) bool
}

// CRDInfo contains information about a Custom Resource Definition
type CRDInfo struct {
	Name      string              // CRD name (e.g., "pod.k8s-registry.ccrn.example.com")
	Plural    string              // Plural resource name (e.g., "pods")
	Singular  string              // Singular resource name (e.g., "pod")
	Group     string              // API group (e.g., "k8s-registry.ccrn.example.com")
	Kind      string              // Resource kind (e.g., "pod")
	Version   string              // API version (e.g., "v1")
	Schema    *v1.JSONSchemaProps // OpenAPI schema (for offline validation)
	URNFormat string              // URN template from annotations
}

// ValidationResult contains the result of a CCRN validation
type ValidationResult struct {
	Valid      bool            // Whether the CCRN is valid
	ParsedCCRN *ParsedResource // The parsed CCRN
	Errors     []string        // Validation errors
	Warnings   []string        // Validation warnings
}
