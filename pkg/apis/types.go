// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CCRN defines the Common Cloud Resource Name resource
type CCRN struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   CCRNSpec   `json:"spec"`
	Status CCRNStatus `json:"status"`
}

// CCRNSpec defines the desired state of CCRN
type CCRNSpec struct {
	// Value is the CCRN string value that is field-based, following format:
	CCRN string `json:"ccrn"`
	URN  string `json:"urn"`
}

// CCRNStatus defines the observed state of CCRN
type CCRNStatus struct {
	// Valid indicates whether the CCRN is valid
	Valid bool `json:"valid"`
	// Message provides additional information about the validation result
	Message string `json:"message,omitempty"`
	// ValidatedAt is the timestamp when the CCRN was last validated
	ValidatedAt metav1.Time `json:"validatedAt"`
}

// GenericResource is a dynamic resource that can represent any custom resource
type GenericResource struct {
	metav1.TypeMeta `json:",inline"`
	// Fields contains the dynamic fields of the resource
	// The webhook doesn't need to know the specific structure
	// of each resource type, as it will be validated by the K8s API
	Fields map[string]any `json:"fields,omitempty"`
}

// CCRNList contains a list of CCRN resources
type CCRNList struct {
	Items []CCRN `json:"items"`
}
