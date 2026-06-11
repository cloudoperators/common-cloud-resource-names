// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/ccrn"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/parser/lite"
	"github.com/sirupsen/logrus"
)

const DefaultURNTemplate string = "urn:ccrn:<ccrn>"

// ResourceParser parses both CCRN and URN formats and converts between them.
// It delegates string parsing to pkg/parser/lite and adds URN template lookup
// from the ValidationBackend.
type ResourceParser struct {
	log     *logrus.Logger
	backend apis.ValidationBackend
}

// NewResourceParser creates a new resource parser
func NewResourceParser(log *logrus.Logger, backend apis.ValidationBackend) *ResourceParser {
	if log == nil {
		log = logrus.New()
		log.SetOutput(io.Discard)
	}
	return &ResourceParser{log: log, backend: backend}
}

// Parse parses a CCRN or URN string. For URN, a template must be provided.
// If the template is empty or the default placeholder, the backend is consulted
// to look up the real template.
func (p *ResourceParser) Parse(input string, urnTemplate string) (*apis.ParsedResource, error) {
	if strings.HasPrefix(input, "ccrn=") {
		parsed, err := lite.ParseCCRN(input)
		if err != nil {
			return nil, err
		}
		return liteToAPIs(parsed), nil
	} else if strings.HasPrefix(input, "urn:ccrn:") {
		if urnTemplate == "" || urnTemplate == DefaultURNTemplate {
			// Need a backend to look up the URN template
			if p.backend == nil {
				return nil, errors.New("cannot parse URN without a template: no backend configured for template lookup")
			}
			// Extract the CCRN key to look up the real template from the backend
			ccrnKey, err := extractCCRNKeyFromURN(input)
			if err != nil {
				return nil, err
			}

			// Split ccrnKey into name and version for the backend lookup
			parts := strings.SplitN(ccrnKey, "/", 2)
			if len(parts) < 2 {
				return nil, errors.New("invalid URN format: CCRN key must contain type and version separated by /")
			}
			ccrnName := parts[0]
			ccrnVersion := parts[1]

			template, err := p.backend.GetURNTemplate(ccrnName, ccrnVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to get URN template: %w", err)
			}
			return p.Parse(input, template)
		}

		if !strings.HasPrefix(urnTemplate, "urn:ccrn:") {
			return nil, errors.New("invalid URN template: must start with 'urn:ccrn:'")
		}

		// Convert the backend template format (urn:ccrn:<ccrn>/<field1>/<field2>)
		// to the lite parser template format (/{field1}/{field2})
		liteTemplate := convertToLiteTemplate(urnTemplate)

		parsed, err := lite.ParseURN(input, liteTemplate)
		if err != nil {
			return nil, err
		}
		return liteToAPIs(parsed), nil
	}
	return nil, errors.New("unknown format: must start with 'ccrn=' or 'urn:ccrn:'")
}

// ExtractCCRNKeyFromURN extracts the CCRN key from a URN string.
func (p *ResourceParser) ExtractCCRNKeyFromURN(urn string) (string, error) {
	return extractCCRNKeyFromURN(urn)
}

// extractCCRNKeyFromURN extracts the CCRN key (type/version) from a URN string.
func extractCCRNKeyFromURN(urn string) (string, error) {
	body := strings.TrimPrefix(urn, "urn:ccrn:")
	if body == "" {
		return "", errors.New("invalid URN format: empty body after 'urn:ccrn:' prefix")
	}
	parts := strings.Split(body, "/")
	if len(parts) < 3 {
		return "", errors.New("invalid URN format: must contain at least three segments after 'urn:ccrn:'")
	}
	return parts[0] + "/" + parts[1], nil
}

// convertToLiteTemplate converts a backend URN template
// (e.g., "urn:ccrn:<ccrn>/<cluster>/<namespace>/<name>")
// to the lite parser template format (e.g., "/{cluster}/{namespace}/{name}").
// The <ccrn> placeholder occupies one segment in the template string, even though
// it expands to "type.group/version" (two slash-separated parts) at runtime.
func convertToLiteTemplate(backendTemplate string) string {
	// Strip the urn:ccrn: prefix
	body := strings.TrimPrefix(backendTemplate, "urn:ccrn:")

	// Split by /
	segments := strings.Split(body, "/")

	// Skip the first segment which is the <ccrn> placeholder
	var fieldSegments []string
	if len(segments) > 1 {
		fieldSegments = segments[1:]
	} else {
		return ""
	}

	// Convert <field> to {field}
	var result strings.Builder
	for _, seg := range fieldSegments {
		if seg == "" {
			continue
		}
		result.WriteString("/")
		if strings.HasPrefix(seg, "<") && strings.HasSuffix(seg, ">") {
			fieldName := seg[1 : len(seg)-1]
			result.WriteString("{")
			result.WriteString(fieldName)
			result.WriteString("}")
		} else {
			result.WriteString(seg)
		}
	}
	return result.String()
}

// liteToAPIs converts a ccrn.ParsedResource from the lite parser into an apis.ParsedResource.
// The CCRNKey is stored in the fields map under the "ccrn" key for backward compatibility.
func liteToAPIs(parsed *ccrn.ParsedResource) *apis.ParsedResource {
	fields := make(map[string]string, len(parsed.Fields)+1)
	fields["ccrn"] = parsed.CCRNKey
	for k, v := range parsed.Fields {
		fields[k] = v
	}
	return &apis.ParsedResource{
		Format: parsed.Format,
		Fields: fields,
		Raw:    parsed.RawInput,
	}
}
