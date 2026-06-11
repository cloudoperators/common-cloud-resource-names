// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package ccrn_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/ccrn"
)

var _ = Describe("Wildcard Matching", func() {
	Context("Match()", func() {
		It("returns true when pattern is *", func() {
			Expect(ccrn.Match("*", "anything")).To(BeTrue())
		})

		It("returns true when pattern is * and value is empty", func() {
			Expect(ccrn.Match("*", "")).To(BeTrue())
		})

		It("returns true when pattern and value are equal", func() {
			Expect(ccrn.Match("foo", "foo")).To(BeTrue())
		})

		It("returns false when pattern and value differ", func() {
			Expect(ccrn.Match("foo", "bar")).To(BeFalse())
		})

		It("returns true when both are empty", func() {
			Expect(ccrn.Match("", "")).To(BeTrue())
		})

		It("returns false when pattern is non-empty and value is empty", func() {
			Expect(ccrn.Match("foo", "")).To(BeFalse())
		})

		It("returns false when pattern is empty and value is non-empty", func() {
			Expect(ccrn.Match("", "bar")).To(BeFalse())
		})
	})

	Context("MatchAll()", func() {
		It("returns true when all pattern fields match resource fields", func() {
			pattern := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
				},
			}
			resource := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
					"name":      "my-pod",
				},
			}
			Expect(ccrn.MatchAll(pattern, resource)).To(BeTrue())
		})

		It("returns false when a pattern field does not match", func() {
			pattern := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "production",
				},
			}
			resource := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
					"name":      "my-pod",
				},
			}
			Expect(ccrn.MatchAll(pattern, resource)).To(BeFalse())
		})

		It("treats * in pattern as wildcard matching any value", func() {
			pattern := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "*",
					"namespace": "default",
				},
			}
			resource := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "us-west-2",
					"namespace": "default",
					"name":      "my-pod",
				},
			}
			Expect(ccrn.MatchAll(pattern, resource)).To(BeTrue())
		})

		It("treats absent fields in pattern as implicit wildcards", func() {
			pattern := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster": "eu-de-1",
				},
			}
			resource := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
					"name":      "my-pod",
				},
			}
			Expect(ccrn.MatchAll(pattern, resource)).To(BeTrue())
		})

		It("returns true when pattern has no fields (all wildcards)", func() {
			pattern := ccrn.ParsedResource{
				Fields: map[string]string{},
			}
			resource := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
				},
			}
			Expect(ccrn.MatchAll(pattern, resource)).To(BeTrue())
		})

		It("returns false when pattern field references a key not in resource", func() {
			pattern := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"region":    "europe",
				},
			}
			resource := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
				},
			}
			// "region" is in pattern but not in resource - this should NOT match
			Expect(ccrn.MatchAll(pattern, resource)).To(BeFalse())
		})

		It("returns true when pattern field references a key not in resource but pattern value is *", func() {
			pattern := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster": "eu-de-1",
					"region":  "*",
				},
			}
			resource := ccrn.ParsedResource{
				Fields: map[string]string{
					"cluster":   "eu-de-1",
					"namespace": "default",
				},
			}
			// "region" is * in pattern, missing in resource - * matches anything including absent
			Expect(ccrn.MatchAll(pattern, resource)).To(BeTrue())
		})
	})
})
