// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0
package validation_test

import (
	"fmt"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/validation"
)

func TestFilesystemBackend(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FilesystemBackend Suite")
}

var _ = Describe("FilesystemBackend", func() {
	var tempDir string
	var backend *validation.FilesystemBackend

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "crdtest")
		Expect(err).ToNot(HaveOccurred())
		backend = validation.NewOfflineBackend(logrus.New(), "ccrn.example.com")
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			fmt.Printf("Warning: failed to remove temp directory %s: %v\n", tempDir, err)
		}
	})

	Context("LoadCRDsFromDirectory", func() {
		It("loads valid CRDs from a directory", func() {
			// Arrange
			crdPath := filepath.Join("testdata", "minimal_crd.yaml")
			// Act
			err := backend.LoadCRDsFromDirectory(filepath.Dir(crdPath))
			// Assert
			Expect(err).ToNot(HaveOccurred())
			Expect(backend.GetLoadedCRDs()).To(ContainElement("testresource.tr.ccrn.example.com/v1"))
		})

		It("returns error for non-existent directory", func() {
			// Act
			err := backend.LoadCRDsFromDirectory("/non/existent/path")
			// Assert
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to load CRDs from directory"))
		})
	})

	Context("LoadCRDs", func() {
		It("returns error for invalid YAML", func() {
			// Arrange
			invalidPath := filepath.Join(tempDir, "invalid.yaml")
			os.WriteFile(invalidPath, []byte("not: valid: yaml"), 0644)
			// Act
			err := backend.LoadCRDs(invalidPath)
			// Assert
			Expect(err).To(HaveOccurred())
		})

		It("loads a valid CRD file", func() {
			// Arrange
			crdPath := filepath.Join("testdata", "minimal_crd.yaml")
			// Act
			err := backend.LoadCRDs(crdPath)
			// Assert
			Expect(err).ToNot(HaveOccurred())
			Expect(backend.GetLoadedCRDs()).To(ContainElement("testresource.tr.ccrn.example.com/v1"))
		})

		It("returns error if no files match the path", func() {
			// Act
			err := backend.LoadCRDs(filepath.Join("testdata", "nonexistent_*.yaml"))
			// Assert
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no files found matching pattern"))
		})

		It("returns error if all CRDs fail to load (invalid YAML)", func() {
			// Arrange
			dir := filepath.Join(os.TempDir(), "allinvalid")
			os.MkdirAll(dir, 0755)
			defer os.RemoveAll(dir)
			invalid1 := filepath.Join(dir, "bad1.yaml")
			invalid2 := filepath.Join(dir, "bad2.yaml")
			os.WriteFile(invalid1, []byte("not: valid: yaml"), 0644)
			os.WriteFile(invalid2, []byte("also: bad: yaml"), 0644)
			// Act
			err := backend.LoadCRDs(filepath.Join(dir, "*.yaml"))
			// Assert
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse YAML"))
		})

		It("returns error if both directory patterns fail in LoadCRDsFromDirectory", func() {
			// Arrange
			dir := filepath.Join(os.TempDir(), "emptydir")
			os.MkdirAll(dir, 0755)
			defer os.RemoveAll(dir)
			// Act
			err := backend.LoadCRDsFromDirectory(dir)
			// Assert
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no files found matching pattern"))
		})
	})

	Context("ValidateCCRN", func() {
		var validator *validation.CCRNValidator
		var crdPath string

		BeforeEach(func() {
			crdPath = filepath.Join("testdata", "testpod_crd.yaml")
			backend = validation.NewOfflineBackend(logrus.New(), "tr.ccrn.example.com")
			err := backend.LoadCRDs(crdPath)
			Expect(err).ToNot(HaveOccurred())
			validator = validation.NewCCRNValidator(backend)
		})

		DescribeTable("validates CCRNs correctly",
			func(ccrn string, expected bool) {
				// Act
				result, err := validator.ValidateCCRN(ccrn)
				// Assert
				if expected {
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Valid).To(BeTrue(), "Expected CCRN to be valid, got errors: %v", result.Errors)
				} else {
					Expect(result.Valid).To(BeFalse())
				}
			},
			Entry("with valid testpod CCRN", "ccrn=pod.k8s-registry.tr.ccrn.example.com/v1, cluster=eu-de-1, namespace=default, name=my-pod", true),
			Entry("with valid testpod CCRN with wildcards", "ccrn=pod.k8s-registry.tr.ccrn.example.com/v1, cluster=*, namespace=*, name=*", true),
			Entry("with invalid testpod CCRN - missing required field", "ccrn=pod.k8s-registry.tr.ccrn.example.com/v1, cluster=eu-de-1", false),
			Entry("with invalid testpod CCRN - wrong API version", "ccrn=invalid.tr.ccrn.example.com/v1, cluster=eu-de-1, namespace=default, name=my-pod", false),
			Entry("with invalid testpod CCRN - invalid cluster pattern", "ccrn=pod.k8s-registry.tr.ccrn.example.com/v1, cluster=INVALID!, namespace=default, name=my-pod", false),
		)
	})

	Context("GetCRD", func() {
		It("gets CRD info if present", func() {
			// Arrange
			crdPath := filepath.Join("testdata", "minimal_crd.yaml")
			backend.LoadCRDs(crdPath)
			// Act
			crd, err := backend.GetCRD("testresource.tr.ccrn.example.com/v1")
			// Assert
			Expect(err).ToNot(HaveOccurred())
			Expect(crd.Kind).To(Equal("TestResource"))
		})

		It("returns error if CRD info is not found", func() {
			// Act
			_, err := backend.GetCRD("DoesNotExist.ccrn.example.com/v1")
			// Assert
			Expect(err).To(HaveOccurred())
		})
	})

	Context("GetURNTemplate", func() {
		It("gets URN template if annotation exists", func() {
			// Arrange
			crdPath := filepath.Join("testdata", "testurn_crd.yaml")
			backend.LoadCRDs(crdPath)
			// Act
			val, err := backend.GetURNTemplate("testurn.tr.ccrn.example.com", "v1")
			// Assert
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal("urn:ccrn:testurn.tr.ccrn.example.com/v1/<name>"))
		})

		It("returns error if URN template annotation is missing", func() {
			// Arrange
			crdPath := filepath.Join("testdata", "testurn2_crd.yaml")
			backend.LoadCRDs(crdPath)
			// Act
			_, err := backend.GetURNTemplate("testurn2.tr.ccrn.example.com", "v1")
			// Assert
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("URN template"))
		})

		It("returns error if CRD is not found in GetURNTemplate", func() {
			// Act
			_, err := backend.GetURNTemplate("doesnotexist", "v1")
			// Assert
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("CRD doesnotexist not found"))
		})
	})

	Context("IsResourceTypeSupported", func() {
		It("returns true for IsResourceTypeSupported if present", func() {
			// Arrange
			crdPath := filepath.Join("testdata", "minimal_crd.yaml")
			backend.LoadCRDs(crdPath)
			// Act & Assert
			Expect(backend.IsResourceTypeSupported("testresource.tr.ccrn.example.com/v1")).To(BeTrue())
		})

		It("returns false for IsResourceTypeSupported if not present", func() {
			// Act & Assert
			Expect(backend.IsResourceTypeSupported("tr.ccrn.example.com/v1")).To(BeFalse())
		})
	})

	Context("ValidateResource", func() {
		It("validates resource successfully", func() {
			// Arrange
			crdPath := filepath.Join("testdata", "minimal_crd.yaml")
			backend.LoadCRDs(crdPath)
			parsed := &apis.ParsedResource{Fields: map[string]string{"ccrn": "testresource.tr.ccrn.example.com/v1", "name": "foo"}}
			// Act
			err := backend.ValidateResource("default", parsed)
			// Assert
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if resource type is not found in ValidateResource", func() {
			// Arrange

			parsed := &apis.ParsedResource{Fields: map[string]string{"ccrn": "DoesNotExist.tr.ccrn.example.com/v1", "name": "foo"}}
			// Act
			err := backend.ValidateResource("default", parsed)
			// Assert
			Expect(err).To(HaveOccurred())
		})
	})
})
