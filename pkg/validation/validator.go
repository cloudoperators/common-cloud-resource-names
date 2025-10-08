// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package validation

import "C"
import (
	"github.com/cloudoperators/common-cloud-resource-names/pkg/apis"
	"github.com/cloudoperators/common-cloud-resource-names/pkg/parser"
)

// CCRNValidator provides CCRN validation using a pluggable backend
type CCRNValidator struct {
	backend apis.ValidationBackend
	parser  *parser.ResourceParser
}

// NewCCRNValidator creates a new CCRN validator with the specified backend
func NewCCRNValidator(backend apis.ValidationBackend) *CCRNValidator {
	return &CCRNValidator{
		backend: backend,
		parser:  parser.NewResourceParser(nil, backend),
	}
}

// ValidateCCRN validates a CCRN string
func (v *CCRNValidator) ValidateCCRN(ccrnStr string) (*apis.ValidationResult, error) {
	parsed, err := v.parser.Parse(ccrnStr, parser.DEFAULT_URN_TEMPLATE)
	if err != nil {
		return &apis.ValidationResult{
			Valid:  false,
			Errors: []string{err.Error()},
		}, err
	}

	if parsed.Format == "URN" {
		info, err := v.backend.GetCRD(parsed.CCRNKey())
		if err != nil {
			return &apis.ValidationResult{
				Valid:      false,
				ParsedCCRN: parsed,
				Errors:     []string{"A CCRN definition for %s could not be retrieved: %s", parsed.CCRNKey(), err.Error()},
			}, err
		}
		parsed, err = v.parser.Parse(ccrnStr, info.URNFormat)
	}

	if parsed != nil && !v.backend.IsResourceTypeSupported(parsed.CCRNKey()) {
		return &apis.ValidationResult{
			Valid:      false,
			ParsedCCRN: parsed,
			Errors:     []string{"Resource type not supported: " + parsed.CCRNKey()},
		}, nil
	}

	err = v.backend.ValidateResource("", parsed)
	if err != nil {
		return &apis.ValidationResult{
			Valid:      false,
			ParsedCCRN: parsed,
			Errors:     []string{err.Error()},
		}, err
	}

	return &apis.ValidationResult{
		Valid:      true,
		ParsedCCRN: parsed,
	}, nil
}
