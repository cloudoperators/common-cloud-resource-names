// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package ccrn_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/ccrn"
)

var _ = Describe("ParsedResource", func() {
	Context("struct fields", func() {
		It("has Format, Fields, RawInput, and CCRNKey fields", func() {
			pr := ccrn.ParsedResource{
				Format:   ccrn.FormatCCRN,
				Fields:   map[string]string{"cluster": "eu-de-1", "namespace": "default"},
				RawInput: "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default",
				CCRNKey:  "pod.k8s.ccrn.cloudoperators.dev/v1",
			}
			Expect(pr.Format).To(Equal(ccrn.FormatCCRN))
			Expect(pr.Fields).To(HaveKeyWithValue("cluster", "eu-de-1"))
			Expect(pr.RawInput).To(ContainSubstring("ccrn="))
			Expect(pr.CCRNKey).To(Equal("pod.k8s.ccrn.cloudoperators.dev/v1"))
		})
	})

	Context("Format constants", func() {
		It("defines FormatCCRN and FormatURN", func() {
			Expect(ccrn.FormatCCRN).To(Equal("CCRN"))
			Expect(ccrn.FormatURN).To(Equal("URN"))
		})
	})

	Context("DefaultDomain constant", func() {
		It("equals ccrn.cloudoperators.dev", func() {
			Expect(ccrn.DefaultDomain).To(Equal("ccrn.cloudoperators.dev"))
		})
	})

	Context("APIGroup constant", func() {
		It("equals ccrn.cloudoperators.dev", func() {
			Expect(ccrn.APIGroup).To(Equal("ccrn.cloudoperators.dev"))
		})
	})

	Context("CCRN()", func() {
		It("returns deterministic alphabetically-sorted field string", func() {
			pr := ccrn.ParsedResource{
				Format:  ccrn.FormatCCRN,
				CCRNKey: "pod.k8s.ccrn.cloudoperators.dev/v1",
				Fields: map[string]string{
					"namespace": "default",
					"cluster":   "eu-de-1",
					"name":      "my-pod",
				},
			}
			result := pr.CCRN()
			Expect(result).To(Equal("ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, name=my-pod, namespace=default"))
		})

		It("returns just the ccrn key when no fields are present", func() {
			pr := ccrn.ParsedResource{
				Format:  ccrn.FormatCCRN,
				CCRNKey: "pod.k8s.ccrn.cloudoperators.dev/v1",
				Fields:  map[string]string{},
			}
			result := pr.CCRN()
			Expect(result).To(Equal("ccrn=pod.k8s.ccrn.cloudoperators.dev/v1"))
		})

		It("returns empty string when CCRNKey is empty", func() {
			pr := ccrn.ParsedResource{
				Format: ccrn.FormatCCRN,
				Fields: map[string]string{"cluster": "eu-de-1"},
			}
			result := pr.CCRN()
			Expect(result).To(Equal(""))
		})
	})

	Context("URN()", func() {
		It("returns correctly formatted URN string from template", func() {
			pr := ccrn.ParsedResource{
				Format:  ccrn.FormatURN,
				CCRNKey: "pod.k8s.ccrn.cloudoperators.dev/v1",
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
					"name":      "my-pod",
				},
			}
			template := "/{cluster}/{namespace}/{name}"
			result := pr.URN(template)
			Expect(result).To(Equal("urn:ccrn:pod.k8s.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod"))
		})

		It("returns empty string when template is empty", func() {
			pr := ccrn.ParsedResource{
				Format:  ccrn.FormatURN,
				CCRNKey: "pod.k8s.ccrn.cloudoperators.dev/v1",
				Fields:  map[string]string{"cluster": "eu-de-1"},
			}
			result := pr.URN("")
			Expect(result).To(Equal(""))
		})

		It("returns empty string when CCRNKey is empty", func() {
			pr := ccrn.ParsedResource{
				Format: ccrn.FormatURN,
				Fields: map[string]string{"cluster": "eu-de-1"},
			}
			result := pr.URN("/{cluster}")
			Expect(result).To(Equal(""))
		})

		It("handles template with missing field values gracefully", func() {
			pr := ccrn.ParsedResource{
				Format:  ccrn.FormatURN,
				CCRNKey: "pod.k8s.ccrn.cloudoperators.dev/v1",
				Fields:  map[string]string{"cluster": "eu-de-1"},
			}
			// namespace not in Fields, so it won't be substituted
			template := "/{cluster}/{namespace}"
			result := pr.URN(template)
			// Should still build the URN but leave the placeholder as-is or empty
			Expect(result).To(Equal("urn:ccrn:pod.k8s.ccrn.cloudoperators.dev/v1/eu-de-1/{namespace}"))
		})
	})

	Context("IsCCRNGroup()", func() {
		It("returns true when group ends with domain", func() {
			Expect(ccrn.IsCCRNGroup("k8s.ccrn.cloudoperators.dev", "ccrn.cloudoperators.dev")).To(BeTrue())
		})

		It("returns true when group equals domain", func() {
			Expect(ccrn.IsCCRNGroup("ccrn.cloudoperators.dev", "ccrn.cloudoperators.dev")).To(BeTrue())
		})

		It("returns false when group does not match domain", func() {
			Expect(ccrn.IsCCRNGroup("apps.example.com", "ccrn.cloudoperators.dev")).To(BeFalse())
		})

		It("returns false for partial match without dot separator", func() {
			Expect(ccrn.IsCCRNGroup("notccrn.cloudoperators.dev", "ccrn.cloudoperators.dev")).To(BeFalse())
		})
	})
})
