// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

// Package ccrn provides zero-dependency types and utilities for Common Cloud Resource Names.
// This package only imports from the Go standard library (production code).
package ccrn

import (
	"sort"
	"strings"
)

// Format constants for resource representations.
const (
	FormatCCRN = "CCRN"
	FormatURN  = "URN"
)

// DefaultDomain is the default API group domain for CCRN resources.
const DefaultDomain = "ccrn.cloudoperators.dev"

// APIGroup is the Kubernetes API group for CCRN resources.
const APIGroup = "ccrn.cloudoperators.dev"

// ParsedResource represents a parsed CCRN or URN resource with its fields.
type ParsedResource struct {
	// Format indicates the source format: FormatCCRN or FormatURN.
	Format string

	// Fields contains the key-value pairs of the resource (excluding the ccrn key itself).
	Fields map[string]string

	// RawInput is the original input string that was parsed.
	RawInput string

	// CCRNKey is the CCRN type key (e.g., "pod.k8s.ccrn.cloudoperators.dev/v1").
	CCRNKey string
}

// CCRN returns the deterministic CCRN string representation with fields sorted alphabetically.
// Returns empty string if CCRNKey is empty.
func (p ParsedResource) CCRN() string {
	if p.CCRNKey == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("ccrn=")
	b.WriteString(p.CCRNKey)

	if len(p.Fields) == 0 {
		return b.String()
	}

	// Sort field keys alphabetically for deterministic output
	keys := make([]string, 0, len(p.Fields))
	for k := range p.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		b.WriteString(", ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(p.Fields[k])
	}

	return b.String()
}

// URN returns the URN string representation using the given template.
// Template format: "/{field1}/{field2}/..." where {fieldN} are placeholders for field values.
// Returns empty string if template or CCRNKey is empty.
func (p ParsedResource) URN(template string) string {
	if template == "" || p.CCRNKey == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("urn:ccrn:")
	b.WriteString(p.CCRNKey)

	// Parse template segments - template starts with /
	// e.g., "/{cluster}/{namespace}/{name}" -> ["", "{cluster}", "{namespace}", "{name}"]
	segments := strings.Split(template, "/")

	for _, seg := range segments {
		if seg == "" {
			continue
		}

		b.WriteString("/")

		// Check if segment is a placeholder like {fieldName}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			fieldName := seg[1 : len(seg)-1]
			if val, ok := p.Fields[fieldName]; ok {
				b.WriteString(val)
			} else {
				// Leave placeholder as-is if field not found
				b.WriteString(seg)
			}
		} else {
			b.WriteString(seg)
		}
	}

	return b.String()
}

// IsCCRNGroup checks if a group belongs to the given CCRN domain.
// A group belongs to the domain if it equals the domain or ends with ".<domain>".
func IsCCRNGroup(group, domain string) bool {
	if group == domain {
		return true
	}
	return strings.HasSuffix(group, "."+domain)
}

// Match performs wildcard matching where "*" matches any single segment value.
// Returns true if pattern equals "*" or pattern equals value exactly.
func Match(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	return pattern == value
}

// MatchAll matches all fields of a pattern against a resource.
// Fields absent from the pattern are treated as implicit wildcards (always match).
// Fields present in the pattern with value "*" also match any value.
// If a pattern field key is not present in the resource, it does not match
// (unless the pattern value is "*").
func MatchAll(pattern, resource ParsedResource) bool {
	for key, patternValue := range pattern.Fields {
		resourceValue, exists := resource.Fields[key]
		if !exists {
			// Key not in resource: only matches if pattern is wildcard
			if patternValue != "*" {
				return false
			}
			continue
		}
		if !Match(patternValue, resourceValue) {
			return false
		}
	}
	return true
}
