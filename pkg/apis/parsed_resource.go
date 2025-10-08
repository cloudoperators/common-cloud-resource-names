package apis

import (
	"fmt"
	"strings"
)

// Methods for ParsedResource
type ParsedResource struct {
	Format      string // "CCRN" or "URN"
	Fields      map[string]string
	Raw         string
	UrnTemplate string // URN template used for parsing, if applicable
}

// CCRN returns the full CCRN string from the parsed resource
func (p *ParsedResource) CCRN() string {
	ccrnString, exists := p.Fields["ccrn"]
	if !exists {
		return ""
	}
	ccrn := "ccrn=" + ccrnString
	for key, value := range p.Fields {
		if key != "ccrn" {
			ccrn += fmt.Sprintf(", %s=%s", key, value)
		}
	}
	return ccrn
}

// URN returns the URN string from the parsed resource using the provided template
func (p *ParsedResource) URN(template string) string {
	if template == "" {
		if p.UrnTemplate != "" {
			template = p.UrnTemplate
		} else {
			return ""
		}
	}

	for key, value := range p.Fields {
		template = strings.Replace(template, "<"+key+">", value, 1)
	}
	return template
}

// Version returns the version from the parsed CCRN or URN
func (p *ParsedResource) Version() string {
	if ccrn, ok := p.Fields["ccrn"]; ok {
		parts := strings.Split(ccrn, "/")
		if len(parts) > 1 {
			return parts[1]
		}
	}
	return ""
}

// CCRNKey returns the CCRN key (type/version)
func (p *ParsedResource) CCRNKey() string {
	if ccrn, ok := p.Fields["ccrn"]; ok {
		return ccrn
	}
	return ""
}

// GetKind returns the kind from the parsed CCRN or URN
func (p *ParsedResource) GetKind() string {
	if ccrn, ok := p.Fields["ccrn"]; ok {
		parts := strings.Split(ccrn, ".")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

// ApiGroup returns the group from the parsed CCRN or URN
func (p *ParsedResource) ApiGroup() string {
	if ccrn, ok := p.Fields["ccrn"]; ok {
		resourceParts := strings.SplitN(strings.SplitN(ccrn, "/", 2)[0], ".", 2)
		if len(resourceParts) < 2 {
			return resourceParts[0]
		}
		return resourceParts[1]
	}
	return ""
}

// ApiGroup returns the group from the parsed CCRN or URN
func (p *ParsedResource) CCRNName() string {
	if ccrn, ok := p.Fields["ccrn"]; ok {
		name := strings.SplitN(ccrn, "/", 2)[0]
		return name
	}
	return ""
}

// GetFieldValue returns a field value by key
func (p *ParsedResource) GetFieldValue(key string) (string, bool) {
	value, exists := p.Fields[key]
	return value, exists
}

// ToResourceMap converts the parsed resource to a map suitable for creating a K8s resource
func (p *ParsedResource) ToResourceMap(namespace, name string) map[string]any {
	resourceObj := map[string]any{
		"ccrn": p.CCRNKey(),
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}
	for key, value := range p.Fields {
		if key == "ccrn" || key == "metadata" {
			continue
		}
		resourceObj[key] = value
	}
	return resourceObj
}
