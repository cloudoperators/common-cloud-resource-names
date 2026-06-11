// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

// Package lite provides a lightweight CCRN/URN parser with no external dependencies
// beyond the standard library and pkg/ccrn. It never panics on malformed input.
package lite

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/ccrn"
)

// ParseCCRN parses a CCRN-format string into a ParsedResource.
// Expected format: ccrn=<kind>.<group>/<version>, field1=value1, field2=value2
func ParseCCRN(input string) (*ccrn.ParsedResource, error) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "ccrn=") {
		return nil, errors.New("invalid CCRN format: must start with 'ccrn='")
	}

	// Split on commas to get field entries
	entries := strings.Split(trimmed, ",")
	fields := make(map[string]string)
	var ccrnKey string

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid field format: %s (must be key=value)", entry)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Strip surrounding quotes if present
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}

		if key == "ccrn" {
			ccrnKey = value
		} else {
			fields[key] = value
		}
	}

	if ccrnKey == "" {
		return nil, errors.New("invalid CCRN format: ccrn key is empty or missing")
	}

	return &ccrn.ParsedResource{
		Format:   ccrn.FormatCCRN,
		Fields:   fields,
		RawInput: input,
		CCRNKey:  ccrnKey,
	}, nil
}

// ParseURN parses a URN-format string into a ParsedResource using the provided template.
// Expected URN format: urn:ccrn:<kind>.<group>/<version>/<field1>/<field2>/...
// Template format: /{field1}/{field2}/{field3} where {fieldN} are placeholder names.
// The last template field captures remaining path segments (allows slashes in value).
func ParseURN(input string, template string) (*ccrn.ParsedResource, error) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "urn:ccrn:") {
		return nil, errors.New("invalid URN format: must start with 'urn:ccrn:'")
	}

	if template == "" {
		return nil, errors.New("invalid URN template: template must not be empty")
	}

	// Extract the body after the prefix
	body := strings.TrimPrefix(trimmed, "urn:ccrn:")
	if body == "" {
		return nil, errors.New("invalid URN format: empty body after 'urn:ccrn:' prefix")
	}

	// Parse template to get field names
	// Template format: "/{field1}/{field2}/{field3}"
	templateFields := parseTemplateFields(template)
	if len(templateFields) == 0 {
		return nil, errors.New("invalid URN template: no fields defined")
	}

	// The body format is: <kind>.<group>/<version>/<val1>/<val2>/...
	// We need to split carefully: the first two slash-separated segments form the CCRNKey
	// (kind.group/version), and the rest are field values.
	// Use SplitN to limit splits so the last field can contain slashes.
	totalSegments := 2 + len(templateFields) // 2 for ccrn key parts + field count
	parts := strings.SplitN(body, "/", totalSegments)

	if len(parts) < 2 {
		return nil, errors.New("invalid URN format: must contain at least type and version segments")
	}

	ccrnKey := parts[0] + "/" + parts[1]
	valueParts := parts[2:]

	if len(valueParts) < len(templateFields) {
		return nil, fmt.Errorf("URN segment count mismatch: template requires %d field(s), got %d", len(templateFields), len(valueParts))
	}

	fields := make(map[string]string, len(templateFields))
	for i, fieldName := range templateFields {
		fields[fieldName] = valueParts[i]
	}

	return &ccrn.ParsedResource{
		Format:   ccrn.FormatURN,
		Fields:   fields,
		RawInput: input,
		CCRNKey:  ccrnKey,
	}, nil
}

// Parse auto-detects the format of the input and dispatches to ParseCCRN or ParseURN.
// For CCRN format, the template parameter is ignored.
// For URN format, the template parameter is required.
func Parse(input string, template string) (*ccrn.ParsedResource, error) {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "ccrn=") {
		return ParseCCRN(input)
	}
	if strings.HasPrefix(trimmed, "urn:ccrn:") {
		return ParseURN(input, template)
	}
	return nil, errors.New("unknown format: input must start with 'ccrn=' or 'urn:ccrn:'")
}

// parseTemplateFields extracts field names from a template like "/{cluster}/{namespace}/{name}".
// Returns the ordered list of field names.
func parseTemplateFields(template string) []string {
	segments := strings.Split(template, "/")
	var fields []string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			fieldName := seg[1 : len(seg)-1]
			if fieldName != "" {
				fields = append(fields, fieldName)
			}
		}
	}
	return fields
}
