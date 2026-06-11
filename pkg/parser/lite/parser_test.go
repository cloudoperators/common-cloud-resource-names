// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package lite_test

import (
	"github.com/cloudoperators/common-cloud-resource-names/pkg/ccrn"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/parser/lite"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseCCRN", func() {
	Context("valid inputs", func() {
		It("parses a simple CCRN with multiple fields", func() {
			input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod"
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Format).To(Equal(ccrn.FormatCCRN))
			Expect(result.CCRNKey).To(Equal("pod.k8s.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(HaveKeyWithValue("cluster", "eu-de-1"))
			Expect(result.Fields).To(HaveKeyWithValue("namespace", "default"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "my-pod"))
			Expect(result.RawInput).To(Equal(input))
		})

		It("parses a CCRN with a single field", func() {
			input := "ccrn=bucket.s3.ccrn.cloudoperators.dev/v1, name=my-bucket"
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.CCRNKey).To(Equal("bucket.s3.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(HaveLen(1))
			Expect(result.Fields).To(HaveKeyWithValue("name", "my-bucket"))
		})

		It("parses a CCRN with no extra fields (only ccrn key)", func() {
			input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1"
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.CCRNKey).To(Equal("pod.k8s.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(BeEmpty())
		})

		It("handles embedded = in values", func() {
			input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, label=key=value, name=test"
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Fields).To(HaveKeyWithValue("label", "key=value"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "test"))
		})

		It("handles extra commas gracefully", func() {
			input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, name=test,,"
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.CCRNKey).To(Equal("pod.k8s.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "test"))
		})

		It("handles extra spaces gracefully", func() {
			input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1 ,  name = test ,  cluster = eu "
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.CCRNKey).To(Equal("pod.k8s.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "test"))
			Expect(result.Fields).To(HaveKeyWithValue("cluster", "eu"))
		})

		It("handles quoted values by stripping quotes", func() {
			input := `ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, name="my-pod"`
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Fields).To(HaveKeyWithValue("name", "my-pod"))
		})

		It("handles wildcard values", func() {
			input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, cluster=*, namespace=*, name=*"
			result, err := lite.ParseCCRN(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Fields).To(HaveKeyWithValue("cluster", "*"))
			Expect(result.Fields).To(HaveKeyWithValue("namespace", "*"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "*"))
		})
	})

	Context("invalid inputs", func() {
		It("returns error for empty string", func() {
			_, err := lite.ParseCCRN("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must start with 'ccrn='"))
		})

		It("returns error for missing ccrn= prefix", func() {
			_, err := lite.ParseCCRN("urn:ccrn:something")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must start with 'ccrn='"))
		})

		It("returns error for whitespace-only input", func() {
			_, err := lite.ParseCCRN("   ")
			Expect(err).To(HaveOccurred())
		})

		It("returns error for ccrn= with no value", func() {
			_, err := lite.ParseCCRN("ccrn=")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ccrn key"))
		})

		It("returns error for field without value", func() {
			_, err := lite.ParseCCRN("ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, badfield")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key=value"))
		})

		It("never panics on malformed input", func() {
			inputs := []string{
				"",
				"ccrn=",
				"ccrn",
				"ccrn=,,,",
				"ccrn=x, =y",
				string(make([]byte, 10000)),
			}
			for _, input := range inputs {
				Expect(func() {
					lite.ParseCCRN(input) //nolint:errcheck
				}).ToNot(Panic())
			}
		})
	})
})

var _ = Describe("ParseURN", func() {
	Context("valid inputs", func() {
		It("parses a URN with explicit template", func() {
			input := "urn:ccrn:pod.k8s.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod"
			template := "/{cluster}/{namespace}/{name}"
			result, err := lite.ParseURN(input, template)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Format).To(Equal(ccrn.FormatURN))
			Expect(result.CCRNKey).To(Equal("pod.k8s.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(HaveKeyWithValue("cluster", "eu-de-1"))
			Expect(result.Fields).To(HaveKeyWithValue("namespace", "default"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "my-pod"))
			Expect(result.RawInput).To(Equal(input))
		})

		It("parses a URN with a single-field template", func() {
			input := "urn:ccrn:bucket.s3.ccrn.cloudoperators.dev/v1/my-bucket"
			template := "/{name}"
			result, err := lite.ParseURN(input, template)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.CCRNKey).To(Equal("bucket.s3.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "my-bucket"))
		})

		It("handles last field with slashes (path-like values)", func() {
			input := "urn:ccrn:object.s3.ccrn.cloudoperators.dev/v1/my-bucket/path/to/file.txt"
			template := "/{bucket}/{path}"
			result, err := lite.ParseURN(input, template)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Fields).To(HaveKeyWithValue("bucket", "my-bucket"))
			Expect(result.Fields).To(HaveKeyWithValue("path", "path/to/file.txt"))
		})
	})

	Context("invalid inputs", func() {
		It("returns error for empty string", func() {
			_, err := lite.ParseURN("", "/{name}")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must start with 'urn:ccrn:'"))
		})

		It("returns error for wrong prefix", func() {
			_, err := lite.ParseURN("ccrn=something", "/{name}")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must start with 'urn:ccrn:'"))
		})

		It("returns error for empty template", func() {
			_, err := lite.ParseURN("urn:ccrn:pod.k8s.ccrn.cloudoperators.dev/v1/foo", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("template"))
		})

		It("returns error for URN with insufficient segments", func() {
			input := "urn:ccrn:pod.k8s.ccrn.cloudoperators.dev/v1"
			template := "/{cluster}/{namespace}/{name}"
			_, err := lite.ParseURN(input, template)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("segment"))
		})

		It("returns error for URN with missing type/version", func() {
			_, err := lite.ParseURN("urn:ccrn:podonly", "/{name}")
			Expect(err).To(HaveOccurred())
		})

		It("never panics on malformed input", func() {
			inputs := []string{
				"",
				"urn:ccrn:",
				"urn:ccrn:a",
				"urn:ccrn:a/",
				"urn:ccrn:///",
				string(make([]byte, 10000)),
			}
			templates := []string{"", "/{name}", "/{a}/{b}/{c}"}
			for _, input := range inputs {
				for _, tmpl := range templates {
					Expect(func() {
						lite.ParseURN(input, tmpl) //nolint:errcheck
					}).ToNot(Panic())
				}
			}
		})
	})
})

var _ = Describe("Parse", func() {
	Context("auto-detection", func() {
		It("dispatches CCRN format correctly", func() {
			input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, name=test"
			result, err := lite.Parse(input, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Format).To(Equal(ccrn.FormatCCRN))
			Expect(result.CCRNKey).To(Equal("pod.k8s.ccrn.cloudoperators.dev/v1"))
			Expect(result.Fields).To(HaveKeyWithValue("name", "test"))
		})

		It("dispatches URN format correctly with template", func() {
			input := "urn:ccrn:pod.k8s.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod"
			template := "/{cluster}/{namespace}/{name}"
			result, err := lite.Parse(input, template)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Format).To(Equal(ccrn.FormatURN))
			Expect(result.Fields).To(HaveKeyWithValue("cluster", "eu-de-1"))
		})

		It("returns error for unknown format", func() {
			_, err := lite.Parse("unknown:format", "/{name}")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown format"))
		})

		It("returns error for empty string", func() {
			_, err := lite.Parse("", "")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("Round-trip fidelity", func() {
	It("CCRN: parse -> format -> re-parse produces same result", func() {
		input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, name=my-pod, namespace=default"
		result1, err := lite.ParseCCRN(input)
		Expect(err).ToNot(HaveOccurred())

		// Format back to CCRN string
		formatted := result1.CCRN()
		Expect(formatted).ToNot(BeEmpty())

		// Re-parse
		result2, err := lite.ParseCCRN(formatted)
		Expect(err).ToNot(HaveOccurred())

		// Compare
		Expect(result2.CCRNKey).To(Equal(result1.CCRNKey))
		Expect(result2.Fields).To(Equal(result1.Fields))
		Expect(result2.Format).To(Equal(result1.Format))
	})

	It("URN: parse -> format -> re-parse produces same result", func() {
		input := "urn:ccrn:pod.k8s.ccrn.cloudoperators.dev/v1/eu-de-1/default/my-pod"
		template := "/{cluster}/{namespace}/{name}"
		result1, err := lite.ParseURN(input, template)
		Expect(err).ToNot(HaveOccurred())

		// Format back to URN string
		formatted := result1.URN(template)
		Expect(formatted).ToNot(BeEmpty())

		// Re-parse
		result2, err := lite.ParseURN(formatted, template)
		Expect(err).ToNot(HaveOccurred())

		// Compare
		Expect(result2.CCRNKey).To(Equal(result1.CCRNKey))
		Expect(result2.Fields).To(Equal(result1.Fields))
		Expect(result2.Format).To(Equal(result1.Format))
	})

	It("cross-format round-trip: CCRN -> URN -> re-parse preserves fields", func() {
		input := "ccrn=pod.k8s.ccrn.cloudoperators.dev/v1, cluster=eu-de-1, namespace=default, name=my-pod"
		template := "/{cluster}/{namespace}/{name}"

		// Parse as CCRN
		parsedCCRN, err := lite.ParseCCRN(input)
		Expect(err).ToNot(HaveOccurred())

		// Format as URN
		urnStr := parsedCCRN.URN(template)
		Expect(urnStr).ToNot(BeEmpty())

		// Re-parse as URN
		parsedURN, err := lite.ParseURN(urnStr, template)
		Expect(err).ToNot(HaveOccurred())

		// Fields should match
		Expect(parsedURN.CCRNKey).To(Equal(parsedCCRN.CCRNKey))
		Expect(parsedURN.Fields).To(Equal(parsedCCRN.Fields))
	})
})
