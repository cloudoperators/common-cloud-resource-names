// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0
package validation

import (
    "bufio"
    "errors"
    "fmt"
    "github.com/cloudoperators/common-cloud-resource-names/pkg/apis"
    "os"
    "path/filepath"
    "strings"
    "sync"

    "github.com/sirupsen/logrus"
    "sigs.k8s.io/yaml"

    "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
    apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
    "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/util/validation/field"
)

const (
    // YAMLDocumentSeparator represents the standard YAML document separator used by Helm
    YAMLDocumentSeparator = "---"

    // CRDKind represents the Kubernetes kind for Custom Resource Definitions
    CRDKind = "CustomResourceDefinition"

    // SupportedFileExtensions defines the file extensions we process
    yamlExtension = ".yaml"
    ymlExtension  = ".yml"

    // URNTemplateAnnotationFormat defines the format for URN template annotations
    URNTemplateAnnotationFormat = "ccrn/%s.urn-template"
)

// CRDLoadingResult contains detailed information about CRD loading operation
type CRDLoadingResult struct {
    ProcessedFiles int      // Number of files processed
    ProcessedCRDs  int      // Number of CRDs successfully loaded
    SkippedCRDs    int      // Number of CRDs skipped (e.g., non-CCRN)
    ErrorCount     int      // Number of errors encountered
    Errors         []error  // Detailed error information
    LoadedCRDKeys  []string // Keys of successfully loaded CRDs
}

// FilesystemBackend implements ValidationBackend using local CRD files
// This backend supports loading CRDs from individual files or directories,
// including multi-document YAML files with Helm-style "---" separators.
type FilesystemBackend struct {
    log         *logrus.Logger
    crds        map[string]*apis.CRDInfo                               // Cache of loaded CRD information
    crdsByFile  map[string][]*apiextensionsv1.CustomResourceDefinition // CRDs organized by source file
    validators  map[string]*validation.SchemaValidator                 // Schema validators for each CRD version
    crdsMutex   sync.RWMutex                                           // Thread-safe access to CRD data
    ccrnGroup   string                                                 // CCRN group for filtering CRDs
    loadedPaths []string                                               // Paths that were loaded (for refresh functionality)
}

// NewOfflineBackend creates a new filesystem-based validation backend
//
// Parameters:
//   - log: Logger instance (will create default if nil)
//   - ccrnGroup: CCRN group name used for filtering relevant CRDs
//
// Returns:
//   - *FilesystemBackend: Configured filesystem backend instance
func NewOfflineBackend(log *logrus.Logger, ccrnGroup string) *FilesystemBackend {
    if log == nil {
        log = logrus.New()
    }

    return &FilesystemBackend{
        log:         log,
        crds:        make(map[string]*apis.CRDInfo),
        crdsByFile:  make(map[string][]*apiextensionsv1.CustomResourceDefinition),
        validators:  make(map[string]*validation.SchemaValidator),
        ccrnGroup:   ccrnGroup,
        loadedPaths: make([]string, 0),
    }
}

// LoadCRDs loads CRD definitions from a glob pattern (files or directories)
// Supports multi-document YAML files separated by "---" (Helm-style)
//
// Parameters:
//   - pattern: File glob pattern (e.g., "/path/to/crds/*.yaml", "/path/to/file.yaml")
//
// Returns:
//   - error: Error if critical failure occurs, nil if at least some CRDs loaded successfully
func (fb *FilesystemBackend) LoadCRDs(pattern string) error {
    fb.log.Infof("Loading CRDs from pattern: %s", pattern)

    // Resolve glob pattern to actual files
    matchedFiles, err := filepath.Glob(pattern)
    if err != nil {
        return fmt.Errorf("failed to resolve glob pattern %s: %w", pattern, err)
    }

    if len(matchedFiles) == 0 {
        return fmt.Errorf("no files found matching pattern: %s", pattern)
    }

    // Process all matched files
    result := &CRDLoadingResult{
        Errors:        make([]error, 0),
        LoadedCRDKeys: make([]string, 0),
    }

    for _, filePath := range matchedFiles {
        if fb.isYAMLFile(filePath) {
            fb.processFile(filePath, result)
        }
    }

    // Store the pattern for potential refresh operations
    fb.loadedPaths = append(fb.loadedPaths, pattern)

    // Log comprehensive results
    fb.logLoadingResults(result)

    // Return error only if no CRDs were loaded at all
    if result.ProcessedCRDs == 0 && len(result.Errors) > 0 {
        return fmt.Errorf("failed to load any CRDs: %w", errors.Join(result.Errors...))
    }

    if len(result.Errors) > 0 {
        fb.log.Warnf("Some errors occurred during CRD loading, but %d CRDs loaded successfully", result.ProcessedCRDs)
    }

    return nil
}

// LoadCRDsFromDirectory loads all CRD YAML files from a directory recursively
// This method searches both the root directory and subdirectories for YAML files
//
// Parameters:
//   - dir: Directory path to search for CRD files
//
// Returns:
//   - error: Error if no CRDs could be loaded from the directory
func (fb *FilesystemBackend) LoadCRDsFromDirectory(dir string) error {
    fb.log.Infof("Loading CRDs from directory: %s", dir)

    // Search patterns for both recursive and non-recursive
    patterns := []string{
        filepath.Join(dir, "*.yaml"),
        filepath.Join(dir, "*.yml"),
        filepath.Join(dir, "**", "*.yaml"),
        filepath.Join(dir, "**", "*.yml"),
    }

    var allErrors []error
    totalLoaded := 0

    // Try each pattern and accumulate results
    for _, pattern := range patterns {
        if err := fb.LoadCRDs(pattern); err != nil {
            allErrors = append(allErrors, fmt.Errorf("pattern %s: %w", pattern, err))
        } else {
            // Count how many CRDs we have now to track progress
            fb.crdsMutex.RLock()
            currentCount := len(fb.crds)
            fb.crdsMutex.RUnlock()
            totalLoaded = currentCount
        }
    }

    // If no CRDs were loaded from any pattern, return combined errors
    if totalLoaded == 0 && len(allErrors) > 0 {
        return fmt.Errorf("failed to load CRDs from directory %s: %w", dir, errors.Join(allErrors...))
    }

    return nil
}

// processFile processes a single file that may contain one or more CRD definitions
// Handles multi-document YAML files with "---" separators
//
// Parameters:
//   - filePath: Path to the file to process
//   - result: Result accumulator for tracking processing statistics
func (fb *FilesystemBackend) processFile(filePath string, result *CRDLoadingResult) {
    fb.log.Debugf("Processing file: %s", filePath)
    result.ProcessedFiles++

    // Read the entire file
    fileContent, err := os.ReadFile(filePath)
    if err != nil {
        err := fmt.Errorf("failed to read file %s: %w", filePath, err)
        fb.log.Error(err.Error())
        result.Errors = append(result.Errors, err)
        result.ErrorCount++
        return
    }

    // Split content into individual YAML documents
    documents, err := fb.splitYAMLDocuments(string(fileContent))
    if err != nil {
        err := fmt.Errorf("failed to split YAML documents in %s: %w", filePath, err)
        fb.log.Error(err.Error())
        result.Errors = append(result.Errors, err)
        result.ErrorCount++
        return
    }

    fb.log.Debugf("Found %d YAML documents in file %s", len(documents), filePath)

    // Process each document in the file
    loadedCRDs := make([]*apiextensionsv1.CustomResourceDefinition, 0)

    for i, document := range documents {
        if fb.isEmptyDocument(document) {
            fb.log.Debugf("Skipping empty document %d in file %s", i, filePath)
            continue
        }

        crd, err := fb.processSingleDocument(document, filePath)
        if err != nil {
            fb.log.Errorf("Failed to process document %d in %s: %v", i, filePath, err)
            result.Errors = append(result.Errors, fmt.Errorf("file %s, document %d: %w", filePath, i, err))
            result.ErrorCount++
            continue
        }

        if crd != nil {
            loadedCRDs = append(loadedCRDs, crd)
            result.ProcessedCRDs++

            // Add CRD keys to result for tracking
            for _, version := range crd.Spec.Versions {
                if version.Served {
                    crdKey := fb.getCRDKey(crd.Spec.Group, version.Name, crd.Spec.Names.Kind)
                    result.LoadedCRDKeys = append(result.LoadedCRDKeys, crdKey)
                }
            }
        } else {
            result.SkippedCRDs++
        }
    }

    // Store all successfully loaded CRDs from this file
    if len(loadedCRDs) > 0 {
        fb.crdsMutex.Lock()
        fb.crdsByFile[filePath] = loadedCRDs
        fb.crdsMutex.Unlock()
    }
}

// splitYAMLDocuments splits a multi-document YAML string into individual documents
// using the standard "---" separator commonly used by Helm and Kubernetes manifests
//
// Parameters:
//   - content: Multi-document YAML content as string
//
// Returns:
//   - []string: Slice of individual YAML documents
//   - error: Error if processing fails
func (fb *FilesystemBackend) splitYAMLDocuments(content string) ([]string, error) {
    if strings.TrimSpace(content) == "" {
        return []string{}, nil
    }

    var documents []string
    var currentDocument strings.Builder

    scanner := bufio.NewScanner(strings.NewReader(content))

    // Configure scanner for potentially large documents
    const maxScanTokenSize = 1024 * 1024 // 1MB
    scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

    for scanner.Scan() {
        line := scanner.Text()

        // Check for document separator
        if strings.TrimSpace(line) == YAMLDocumentSeparator {
            // Save current document if it has content
            if currentDocument.Len() > 0 {
                documents = append(documents, strings.TrimSpace(currentDocument.String()))
                currentDocument.Reset()
            }
            continue
        }

        // Add line to current document
        currentDocument.WriteString(line)
        currentDocument.WriteString("\n")
    }

    // Handle the last document (no trailing separator case)
    if currentDocument.Len() > 0 {
        documents = append(documents, strings.TrimSpace(currentDocument.String()))
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("error scanning YAML content: %w", err)
    }

    return documents, nil
}

// processSingleDocument processes an individual YAML document and converts it to a CRD
//
// Parameters:
//   - document: Single YAML document content
//   - filePath: Source file path (for error reporting)
//   - docIndex: Document index within the file (for error reporting)
//
// Returns:
//   - *apiextensionsv1.CustomResourceDefinition: Parsed CRD if successful
//   - error: Error if processing fails
func (fb *FilesystemBackend) processSingleDocument(document, filePath string) (*apiextensionsv1.CustomResourceDefinition, error) {
    // Parse YAML document
    crd := &apiextensionsv1.CustomResourceDefinition{}
    if err := yaml.Unmarshal([]byte(document), crd); err != nil {
        return nil, fmt.Errorf("failed to parse YAML: %w", err)
    }

    // Validate this is actually a CRD
    if err := fb.validateCRDStructure(crd); err != nil {
        return nil, fmt.Errorf("invalid CRD structure: %w", err)
    }

    // Check if this CRD is relevant to our CCRN group
    if !fb.isCCRNRelevant(crd) {
        fb.log.Debugf("Skipping non-CCRN CRD: %s (group: %s)", crd.Name, crd.Spec.Group)
        return nil, nil // Not an error, just not relevant
    }

    fb.log.Infof("Loading CCRN CRD: %s from %s", crd.Name, filePath)

    // Process and store the CRD
    if err := fb.storeCRD(crd); err != nil {
        return nil, fmt.Errorf("failed to store CRD: %w", err)
    }

    return crd, nil
}

// validateCRDStructure performs basic validation of CRD structure
//
// Parameters:
//   - crd: CRD to validate
//
// Returns:
//   - error: Validation error if CRD structure is invalid
func (fb *FilesystemBackend) validateCRDStructure(crd *apiextensionsv1.CustomResourceDefinition) error {
    if crd.Kind != CRDKind {
        return fmt.Errorf("expected kind '%s', got '%s'", CRDKind, crd.Kind)
    }

    // Validate required fields
    if crd.Spec.Group == "" {
        return fmt.Errorf("CRD spec.group cannot be empty")
    }

    if crd.Spec.Names.Kind == "" {
        return fmt.Errorf("CRD spec.names.kind cannot be empty")
    }

    if len(crd.Spec.Versions) == 0 {
        return fmt.Errorf("CRD must have at least one version")
    }

    // Validate that at least one version has a valid schema
    hasValidVersion := false
    for _, version := range crd.Spec.Versions {
        if version.Name != "" && version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
            hasValidVersion = true
            break
        }
    }

    if !hasValidVersion {
        return fmt.Errorf("CRD must have at least one version with a valid OpenAPI schema")
    }

    return nil
}

// isCCRNRelevant checks if a CRD is relevant to the configured CCRN group
//
// Parameters:
//   - crd: CRD to check
//
// Returns:
//   - bool: true if CRD is relevant to CCRN group
func (fb *FilesystemBackend) isCCRNRelevant(crd *apiextensionsv1.CustomResourceDefinition) bool {
    return strings.Contains(crd.Spec.Group, fb.ccrnGroup)
}

// storeCRD stores a validated CRD and creates necessary validators
//
// Parameters:
//   - crd: CRD to store
//
// Returns:
//   - error: Error if storage fails
func (fb *FilesystemBackend) storeCRD(crd *apiextensionsv1.CustomResourceDefinition) error {
    fb.crdsMutex.Lock()
    defer fb.crdsMutex.Unlock()

    // Process each version of the CRD
    for _, version := range crd.Spec.Versions {
        if !version.Served {
            fb.log.Debugf("Skipping non-served version %s of CRD %s", version.Name, crd.Name)
            continue
        }

        crdKey := fb.getCRDKey(crd.Spec.Group, version.Name, crd.Spec.Names.Kind)

        // Extract URN template from annotations
        urnFormat := fb.extractURNTemplate(crd, version.Name)

        // Create CRD info structure
        crdInfo := &apis.CRDInfo{
            Name:      crd.Name,
            Plural:    crd.Spec.Names.Plural,
            Singular:  crd.Spec.Names.Singular,
            Group:     crd.Spec.Group,
            Kind:      crd.Spec.Names.Kind,
            Version:   version.Name,
            Schema:    version.Schema.OpenAPIV3Schema,
            URNFormat: urnFormat,
        }

        fb.crds[crdKey] = crdInfo

        // Create schema validator for this version
        if err := fb.createSchemaValidator(crdKey, version); err != nil {
            fb.log.Warnf("Failed to create schema validator for %s: %v", crdKey, err)
            // Don't fail the entire operation for validator creation issues
        }

        fb.log.Debugf("Successfully stored CRD version: %s", crdKey)
    }

    return nil
}

// extractURNTemplate extracts the URN template from CRD annotations for a specific version
//
// Parameters:
//   - crd: CRD containing annotations
//   - version: Version name to look for
//
// Returns:
//   - string: URN template if found, empty string otherwise
func (fb *FilesystemBackend) extractURNTemplate(crd *apiextensionsv1.CustomResourceDefinition, version string) string {
    if crd.Annotations == nil {
        return ""
    }

    annotationKey := fmt.Sprintf(URNTemplateAnnotationFormat, version)
    if urnFormat, exists := crd.Annotations[annotationKey]; exists {
        return urnFormat
    }

    return ""
}

// createSchemaValidator creates and stores a schema validator for a CRD version
//
// Parameters:
//   - crdKey: Key to store the validator under
//   - version: CRD version spec containing the schema
//
// Returns:
//   - error: Error if validator creation fails
func (fb *FilesystemBackend) createSchemaValidator(crdKey string, version apiextensionsv1.CustomResourceDefinitionVersion) error {
    if version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
        return fmt.Errorf("no schema available for version")
    }

    // Convert v1 schema to internal schema format
    jsonSchemaProps := apiextensions.JSONSchemaProps{}
    err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
        version.Schema.OpenAPIV3Schema,
        &jsonSchemaProps,
        nil,
    )
    if err != nil {
        return fmt.Errorf("failed to convert OpenAPI schema: %w", err)
    }

    // Create the validator
    validator, _, err := validation.NewSchemaValidator(&jsonSchemaProps)
    if err != nil {
        return fmt.Errorf("failed to create schema validator: %w", err)
    }

    fb.validators[crdKey] = &validator
    return nil
}

// isYAMLFile checks if a file has a YAML extension
//
// Parameters:
//   - filePath: Path to check
//
// Returns:
//   - bool: true if file has .yaml or .yml extension
func (fb *FilesystemBackend) isYAMLFile(filePath string) bool {
    ext := strings.ToLower(filepath.Ext(filePath))
    return ext == yamlExtension || ext == ymlExtension
}

// isEmptyDocument checks if a YAML document contains only whitespace or comments
//
// Parameters:
//   - document: YAML document content to check
//
// Returns:
//   - bool: true if document is effectively empty
func (fb *FilesystemBackend) isEmptyDocument(document string) bool {
    trimmed := strings.TrimSpace(document)
    if trimmed == "" {
        return true
    }

    // Check if document contains only comments
    lines := strings.Split(trimmed, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line != "" && !strings.HasPrefix(line, "#") {
            return false
        }
    }

    return true
}

// logLoadingResults logs comprehensive information about CRD loading results
//
// Parameters:
//   - result: Loading result to log
func (fb *FilesystemBackend) logLoadingResults(result *CRDLoadingResult) {
    fb.log.Infof(
        "CRD loading completed - Files: %d, CRDs: %d, Skipped: %d, Errors: %d",
        result.ProcessedFiles,
        result.ProcessedCRDs,
        result.SkippedCRDs,
        result.ErrorCount,
    )

    if len(result.LoadedCRDKeys) > 0 {
        fb.log.Debugf("Loaded CRD keys: %v", result.LoadedCRDKeys)
    }

    if len(result.Errors) > 0 {
        fb.log.Debugf("Errors encountered during loading:")
        for i, err := range result.Errors {
            fb.log.Debugf("  %d: %v", i+1, err)
        }
    }
}

// Implementation of ValidationBackend interface methods

// GetCRD retrieves CRD information for a given ccrnVersion
func (fb *FilesystemBackend) GetCRD(ccrnVersion string) (*apis.CRDInfo, error) {
    fb.crdsMutex.RLock()
    defer fb.crdsMutex.RUnlock()

    crdInfo, exists := fb.crds[ccrnVersion]
    if !exists {
        return nil, fmt.Errorf("CRD for resource type %s not found", ccrnVersion)
    }

    return crdInfo, nil
}

// ValidateResource validates a resource against its OpenAPI schema
func (fb *FilesystemBackend) ValidateResource(namespace string, parsedCCRN *apis.ParsedResource) error {
    ccrnVersion := parsedCCRN.CCRNKey()
    kind := parsedCCRN.GetKind()

    fb.crdsMutex.RLock()
    validator, exists := fb.validators[ccrnVersion]
    fb.crdsMutex.RUnlock()

    if !exists || validator == nil {
        return fmt.Errorf("no schema validator available for %s", ccrnVersion)
    }

    // Convert parsed CCRN to unstructured object for validation
    resourceName := strings.ToLower(kind) + "-validation"
    resourceObj := parsedCCRN.ToResourceMap(namespace, resourceName)

    // Convert to unstructured for validation
    unstructuredObj := &unstructured.Unstructured{Object: resourceObj}

    // Validate against schema using the custom resource validation
    if errs := validation.ValidateCustomResource(field.NewPath(""), unstructuredObj, *validator); len(errs) > 0 {
        var errorMessages []string
        for _, err := range errs {
            errorMessages = append(errorMessages, err.Error())
        }
        return fmt.Errorf("validation failed for %s: %s", ccrnVersion, strings.Join(errorMessages, "; "))
    }

    fb.log.Debugf("Resource %s validated successfully against schema", ccrnVersion)
    return nil
}

// GetURNTemplate retrieves the URN template from CRD annotations
func (fb *FilesystemBackend) GetURNTemplate(crdName, version string) (string, error) {
    fb.crdsMutex.RLock()
    defer fb.crdsMutex.RUnlock()

    // Search through all loaded CRDs to find the specified one
    for _, crds := range fb.crdsByFile {
        for _, crd := range crds {
            if crd.Name == crdName {
                annotationKey := fmt.Sprintf(URNTemplateAnnotationFormat, version)
                if crd.Annotations != nil {
                    if urnFormat, exists := crd.Annotations[annotationKey]; exists && urnFormat != "" {
                        return urnFormat, nil
                    }
                }
                return "", fmt.Errorf("URN template annotation %s not found in CRD %s", annotationKey, crdName)
            }
        }
    }

    return "", fmt.Errorf("CRD %s not found in loaded CRDs", crdName)
}

// Refresh reloads CRD information from previously loaded paths
func (fb *FilesystemBackend) Refresh() error {
    if len(fb.loadedPaths) == 0 {
        fb.log.Debug("No paths to refresh - no previous LoadCRDs calls")
        return nil
    }

    fb.log.Info("Refreshing CRD information from previously loaded paths")

    // Clear current state
    fb.crdsMutex.Lock()
    fb.crds = make(map[string]*apis.CRDInfo)
    fb.crdsByFile = make(map[string][]*apiextensionsv1.CustomResourceDefinition)
    fb.validators = make(map[string]*validation.SchemaValidator)
    fb.crdsMutex.Unlock()

    // Reload from all previously loaded paths
    var allErrors []error
    for _, path := range fb.loadedPaths {
        if err := fb.LoadCRDs(path); err != nil {
            allErrors = append(allErrors, fmt.Errorf("failed to refresh path %s: %w", path, err))
        }
    }

    if len(allErrors) > 0 {
        return fmt.Errorf("refresh completed with errors: %w", errors.Join(allErrors...))
    }

    fb.log.Info("CRD refresh completed successfully")
    return nil
}

// IsResourceTypeSupported checks if a resource type is supported
func (fb *FilesystemBackend) IsResourceTypeSupported(ccrnVersion string) bool {
    fb.crdsMutex.RLock()
    defer fb.crdsMutex.RUnlock()

    _, exists := fb.crds[ccrnVersion]
    return exists
}

// getCRDKey generates a consistent cache key for a CRD version
//
// Parameters:
//   - group: API group
//   - version: API version
//   - kind: Resource kind
//
// Returns:
//   - string: Formatted cache key
func (fb *FilesystemBackend) getCRDKey(group, version, kind string) string {
    return strings.ToLower(fmt.Sprintf("%s.%s/%s", kind, group, version))
}

// GetLoadedCRDs returns a list of loaded CRD keys (useful for debugging and monitoring)
//
// Returns:
//   - []string: List of all loaded CRD keys
func (fb *FilesystemBackend) GetLoadedCRDs() []string {
    fb.crdsMutex.RLock()
    defer fb.crdsMutex.RUnlock()

    keys := make([]string, 0, len(fb.crds))
    for k := range fb.crds {
        keys = append(keys, k)
    }
    return keys
}

// GetLoadingStatistics returns detailed statistics about loaded CRDs
//
// Returns:
//   - map[string]interface{}: Statistics including counts and file information
func (fb *FilesystemBackend) GetLoadingStatistics() map[string]interface{} {
    fb.crdsMutex.RLock()
    defer fb.crdsMutex.RUnlock()

    stats := map[string]interface{}{
        "total_crds":        len(fb.crds),
        "total_files":       len(fb.crdsByFile),
        "total_validators":  len(fb.validators),
        "loaded_paths":      fb.loadedPaths,
        "ccrn_group_filter": fb.ccrnGroup,
    }

    // Add per-file statistics
    fileStats := make(map[string]int)
    for filePath, crds := range fb.crdsByFile {
        fileStats[filePath] = len(crds)
    }
    stats["crds_per_file"] = fileStats

    return stats
}
