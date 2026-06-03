# CCRN Extensions: K8s Namespace & GitHub Repository Resource Types

## Work Type: Feature

## Summary

Add two new CCRN resource types to enable Permission Manager ASRs to express K8s namespace-level RBAC targets and GitHub repository access targets. These types are consumed by the pm-controller operator (Phase 3/4) to generate K8s Roles/RoleBindings and GitHub team permissions directly from the permission spec.

---

## Requirements

### REQ-1: K8s Namespace CCRN Resource Type
A new CCRN resource type `namespace.k8s.ccrn.cloud.sap/v1` must be defined as a Helm-templated CRD in `ccrn-chart/templates/crds/k8s/namespace.yaml`. It represents a Kubernetes namespace as an RBAC access target.

**Fields:**
- `ccrn` (required, enum): `"namespace.k8s.{{ .Values.ccrn.apiGroup }}/v1"`
- `cluster` (required, pattern): Any valid K8s cluster name + `*` wildcard. Pattern: `^([a-z0-9]([a-z0-9-]*[a-z0-9])?|\*)$`
- `namespace` (required, pattern): K8s namespace name pattern + `*` for cluster-wide

**URN format:** `urn:ccrn:namespace.k8s.ccrn.cloud.sap/v1/<cluster>/<namespace>`

### REQ-2: GitHub Repository CCRN Resource Type
A CCRN resource type `repository.github.ccrn.cloud.sap/v1` must be defined in `ccrn-chart/templates/crds/github/repository.yaml`. It represents a GitHub repository as an access target.

**Fields:**
- `ccrn` (required, enum): `"repository.github.{{ .Values.ccrn.apiGroup }}/v1"`
- `instance` (required, enum): GHE instance hostnames
- `owner` (required, enum): GitHub organization names + `*` wildcard
- `repo` (required, pattern): Repository name pattern + `*` wildcard

**URN format:** `urn:ccrn:repository.github.ccrn.cloud.sap/v1/<instance>/<owner>/<repo>`

### REQ-3: CRD Structure Follows Existing Patterns
Both CRDs must follow the established CCRN CRD pattern:
- SPDX license header
- `ccrn/v1.urn-template` and `ccrn/v1.url-template` annotations
- `ccrn/v1.examples` annotation with both URN and CCRN format examples
- `ccrn/resource-criticality` annotation
- `additionalPrinterColumns` for `kubectl get` output
- `scope: Namespaced`

### REQ-4: Sample K8s ASRs
Create at least 2 sample ASR YAML files in `permission-manager/charts/permission-manager/data/application_specific_roles/k8s/`:
- One namespace-scoped (e.g., `kube-monitoring` on `eu-de-1`, capability `ADMIN`)
- One cluster-wide (e.g., `*` on `eu-de-1`, capability `READ`)

### REQ-5: Sample GitHub ASRs
Create at least 1 sample ASR YAML file in `permission-manager/charts/permission-manager/data/application_specific_roles/github/`:
- Targeting all repos in an org (wildcard), capability `ADMIN`

### REQ-6: ConnectorBinding Definitions
Create ConnectorBinding YAML files for `sci-k8s` (provider: kubernetes) and `sci-github-cc` (provider: github) in the appropriate data directory.

### REQ-7: Validator Compatibility
The new CCRN types must be included in the `full.yaml` CRD bundle used by `permission-manager-validator`. The validator test suite (`ccrn_test.go`) must have test cases for valid and invalid URNs of both new types.

---

## Success Criteria

### SC-1: Helm Template Renders
`helm template ccrn-chart/` renders both new CRDs without errors, with correct metadata names and group names.

### SC-2: Chart Validation Passes
`make check` in `common-cloud-resource-names/` passes with the new CRDs.

### SC-3: ASR Validation Passes
`make validate` in `permission-manager/` passes with sample ASRs referencing the new CCRN types.

### SC-4: Validator Tests Pass
`go test -v ./...` in `permission-manager-validator/` passes with new CCRN test cases (valid URNs accepted, invalid URNs rejected).

### SC-5: URN Format Correctness
- K8s: `urn:ccrn:namespace.k8s.ccrn.cloud.sap/v1/eu-de-1/kube-monitoring` is valid
- K8s: `urn:ccrn:namespace.k8s.ccrn.cloud.sap/v1/eu-de-1/*` is valid (cluster-wide)
- GitHub: `urn:ccrn:repository.github.ccrn.cloud.sap/v1/github.wdf.cloud.sap/cc/*` is valid
- GitHub: `urn:ccrn:repository.github.ccrn.cloud.sap/v1/github.com/cc/test` is INVALID (unknown instance)

---

## Constraints

- API group for K8s RBAC targets is `k8s.` (NOT `k8s-registry.`) — different concern
- No `name` field on namespace CCRN — RBAC is namespace-scoped, not per-resource
- Capability mapping (READ→get,list,watch etc.) is NOT defined in CCRN — it's operator logic
- Legacy GitHub ASRs (CC_GITHUB_* using advancedConfig) remain untouched
- `scope` must be `Namespaced` (not `Namespace`)
- Both URN and CCRN format must be parseable (existing parser in `pkg/parser/`)
- `cluster` field uses pattern (not enum) for flexibility across environments

## Grill Decisions (Phase 2)

- **D1:** `cluster` field uses regex pattern, not static enum. Reason: avoids CRD redeployment when new clusters are added; test environments need arbitrary cluster names.
- **D2:** BDD skipped — acceptance tests are helm template assertions + Go validator test cases.
- **D3:** Wildcard `*` matches exactly one segment per existing convention (see `feedback_ccrn_wildcard` memory).
- **D4:** Multi-permission ASRs (multiple namespaces in one ASR) are valid and will generate multiple Role/RoleBindings. This is a Phase 3 operator concern, not Phase 0.
