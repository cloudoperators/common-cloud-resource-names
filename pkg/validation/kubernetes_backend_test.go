// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestGetCRDKeyFromCRD(t *testing.T) {
	kb := &KubernetesBackend{}

	tests := []struct {
		name     string
		singular string
		kind     string
		group    string
		version  string
		want     string
	}{
		{
			name:     "uses singular when set and differs from lowercased kind",
			singular: "hyphen-resource",
			kind:     "HyphenResource",
			group:    "tr.ccrn.example.com",
			version:  "v1",
			want:     "hyphen-resource.tr.ccrn.example.com/v1",
		},
		{
			name:     "uses singular when it matches lowercased kind",
			singular: "testresource",
			kind:     "TestResource",
			group:    "tr.ccrn.example.com",
			version:  "v1",
			want:     "testresource.tr.ccrn.example.com/v1",
		},
		{
			name:     "falls back to kind when singular is empty",
			singular: "",
			kind:     "TestResource",
			group:    "tr.ccrn.example.com",
			version:  "v1",
			want:     "testresource.tr.ccrn.example.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			crd.Spec.Names.Singular = tt.singular
			crd.Spec.Names.Kind = tt.kind
			crd.Spec.Group = tt.group

			got := kb.getCRDKeyFromCRD(crd, tt.version)
			if got != tt.want {
				t.Errorf("getCRDKeyFromCRD() = %q, want %q", got, tt.want)
			}
		})
	}
}
