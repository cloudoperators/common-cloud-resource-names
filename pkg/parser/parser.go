// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0
package parser

import (
	"errors"
	"fmt"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"
	"strings"

	"github.com/sirupsen/logrus"
)

const DEFAULT_URN_TEMPLATE string = "urn:ccrn:<ccrn>"

// ResourceParser parses both CCRN and URN formats and converts between them, without backend dependencies
// It requires a URN template to parse a URN.
type ResourceParser struct {
	log     *logrus.Logger
	backend apis.ValidationBackend
}

// NewResourceParser creates a new resource parser
func NewResourceParser(log *logrus.Logger, backend apis.ValidationBackend) *ResourceParser {
	return &ResourceParser{log: log, backend: backend}
}

// Parse parses a CCRN or URN string. For URN, a template must be provided.
func (p *ResourceParser) Parse(input string, urnTemplate string) (*apis.ParsedResource, error) {
	if strings.HasPrefix(input, "ccrn=") {
		parsed, err := parseCCRNFields(input)
		if err != nil {
			return nil, err
		}
		return &apis.ParsedResource{
			Format: "CCRN",
			Fields: parsed,
			Raw:    input,
		}, nil
	} else if strings.HasPrefix(input, "urn:ccrn:") {
		if urnTemplate == "" || urnTemplate == DEFAULT_URN_TEMPLATE {

			parsed, err := parseURNCCRNField(input)

			if err != nil {
				return nil, err
			}

			parsedResource := &apis.ParsedResource{
				Format: "URN",
				Fields: map[string]string{"ccrn": parsed},
				Raw:    input,
			}

			template, err := p.backend.GetURNTemplate(parsedResource.CCRNName(), parsedResource.Version())
			if err != nil {
				return nil, fmt.Errorf("failed to get URN template: %w", err)
			}
			return p.Parse(input, template)
		}

		if !strings.HasPrefix(urnTemplate, "urn:ccrn:") {
			return nil, errors.New("invalid URN template: must start with 'urn:ccrn:'")
		}
		parsed, err := parseURNFields(input, urnTemplate)
		if err != nil {
			return nil, err
		}
		return &apis.ParsedResource{
			Format: "URN",
			Fields: parsed,
			Raw:    input,
		}, nil
	}
	return nil, errors.New("unknown format: must start with 'ccrn=' or 'urn:ccrn:'")
}

// parseCCRNFields parses a CCRN string into fields
func parseCCRNFields(ccrn string) (map[string]string, error) {
	if !strings.HasPrefix(ccrn, "ccrn=") {
		return nil, errors.New("invalid CCRN format: must start with 'ccrn='")
	}
	fieldsPart := strings.TrimSpace(ccrn)
	fields := make(map[string]string)
	fieldEntries := strings.Split(fieldsPart, ",")
	for _, entry := range fieldEntries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid field format: " + entry + " (must be key=value)")
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		fields[key] = value
	}
	if _, exists := fields["ccrn"]; !exists {
		return nil, errors.New("missing required field: ccrn")
	}
	return fields, nil
}

func parseURNCCRNField(urn string) (string, error) {
	// Remove prefix
	body := strings.TrimPrefix(urn, "urn:ccrn:")
	parts := strings.Split(body, "/")
	if len(parts) < 3 {
		return "", errors.New("invalid URN format: must contain at least three segments after 'urn:ccrn:'")
	}
	return parts[0] + "/" + parts[1], nil
}

// parseURNFields parses a URN string into fields using the provided template
func parseURNFields(urn, urnTemplate string) (map[string]string, error) {
	if !strings.HasPrefix(urn, "urn:ccrn:") {
		return nil, errors.New("invalid URN format: must start with 'urn:ccrn:'")
	}
	if !strings.HasPrefix(urnTemplate, "urn:ccrn:") {
		return nil, errors.New("invalid URN template: must start with 'urn:ccrn:'")
	}
	// Remove prefix
	body := strings.TrimPrefix(urn, "urn:ccrn:")
	templateBody := strings.TrimPrefix(urnTemplate, "urn:ccrn:")
	templateParts := strings.Split(templateBody, "/")

	// The first element is the ccrn type/version so we rebuild the parts accordingly, the last part can be an path with slashes
	tmpParts := strings.SplitN(body, "/", len(templateParts))
	parts := make([]string, len(tmpParts)-1)
	parts[0] = tmpParts[0] + "/" + tmpParts[1]
	for i := 2; i < len(tmpParts); i++ {
		if tmpParts[i] != "" {
			parts[i-1] = tmpParts[i]
		}
	}

	if len(parts) < len(templateParts) {
		return nil, errors.New("URN and template do not match in segment count. Expected format " + urnTemplate + " segments, got: " + urn)
	}
	fields := make(map[string]string)
	for i, t := range templateParts {
		if strings.HasPrefix(t, "<") && strings.HasSuffix(t, ">") {
			key := t[1 : len(t)-1]
			fields[key] = parts[i]
		} else if t == "<ccrn>" {
			fields["ccrn"] = parts[i]
		} else if t != parts[i] {
			return nil, fmt.Errorf("URN segment '%s' does not match template '%s'", parts[i], t)
		}
	}
	if _, exists := fields["ccrn"]; !exists {
		return nil, errors.New("missing required field: ccrn")
	}
	return fields, nil
}

// ExtractCCRNKeyFromURN extracts the CCRN key from a URN using the template
func (p *ResourceParser) ExtractCCRNKeyFromURN(urn string) (string, error) {
	ccrn, err := parseURNCCRNField(urn)
	if err != nil {
		return "", err
	}
	return ccrn, nil
}
