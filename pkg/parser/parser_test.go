// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/parser"
)

func TestParser(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Parser Suite")
}

var _ = Describe("ResourceParser", func() {
	Context("nil logger handling", func() {
		It("should not panic when created with nil logger", func() {
			// A nil logger should not cause a panic when the parser is used
			Expect(func() {
				p := parser.NewResourceParser(nil, nil)
				Expect(p).ToNot(BeNil())
			}).ToNot(Panic())
		})
	})

	Context("URN parsing bounds checking", func() {
		It("should not panic on empty URN after prefix", func() {
			p := parser.NewResourceParser(nil, nil)
			Expect(func() {
				_, err := p.Parse("urn:ccrn:", parser.DefaultURNTemplate)
				Expect(err).To(HaveOccurred())
			}).ToNot(Panic())
		})

		It("should not panic on URN with only one segment", func() {
			p := parser.NewResourceParser(nil, nil)
			Expect(func() {
				_, err := p.Parse("urn:ccrn:foo", parser.DefaultURNTemplate)
				Expect(err).To(HaveOccurred())
			}).ToNot(Panic())
		})

		It("should not panic on URN with two segments", func() {
			p := parser.NewResourceParser(nil, nil)
			Expect(func() {
				_, err := p.Parse("urn:ccrn:foo/bar", parser.DefaultURNTemplate)
				Expect(err).To(HaveOccurred())
			}).ToNot(Panic())
		})

		It("should not panic on malformed URN with template", func() {
			p := parser.NewResourceParser(nil, nil)
			Expect(func() {
				_, err := p.Parse("urn:ccrn:x", "urn:ccrn:<ccrn>/<field1>/<field2>")
				Expect(err).To(HaveOccurred())
			}).ToNot(Panic())
		})

		It("should not panic on URN with insufficient segments for template", func() {
			p := parser.NewResourceParser(nil, nil)
			Expect(func() {
				_, err := p.Parse("urn:ccrn:a/b", "urn:ccrn:<ccrn>/<field1>/<field2>")
				Expect(err).To(HaveOccurred())
			}).ToNot(Panic())
		})

		It("should return error for ExtractCCRNKeyFromURN with malformed input", func() {
			p := parser.NewResourceParser(nil, nil)
			_, err := p.ExtractCCRNKeyFromURN("urn:ccrn:")
			Expect(err).To(HaveOccurred())
		})

		It("should return error for ExtractCCRNKeyFromURN with only one segment", func() {
			p := parser.NewResourceParser(nil, nil)
			_, err := p.ExtractCCRNKeyFromURN("urn:ccrn:foo")
			Expect(err).To(HaveOccurred())
		})
	})
})
