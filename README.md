# Common Cloud Resource Names (CCRN)

## About this project

The Common Cloud Resource Names (CCRN) framework provides a standardized, machine-readable convention for naming and validating resources across heterogeneous technology stacks (Kubernetes, OpenStack, Vault, VMware, etc.). CCRN enables organizations to uniquely identify resources, enforce naming policies, and facilitate cross-system integration.

### Key Attributes of the Framework

- **Unified Resource Naming**: Ability to create consistent string-based resource identifiers.
- **K8s Native**: Resource names are defined and validated with Kubernetes Custom Resource Definitions (CRDs).
- **Flexibility**: Use wildcards for broad resource selection/description.
- **Versioning**: Resource type strings include versioning for historical correctness.

### Use Cases

- **IAM Policies and access descriptions**: Consistent resource references for access control use cases
- **Logging & Monitoring**: Unique resource identifiers in logs and metrics.
- **Vulnerability Management**: Precise resource targeting for vulnerability scans and reports.
- **BOM**: Bill of Material Documentations
- **Documentation**: Unambiguous resource references in technical documentation.

### CCRN Formats

CCRNs are submitted as a single string value in either the CCRN format or the URN format and they are defined using k8s
custom resource definitions.

#### Field-Based CCRN Format

A CCRN string consists of a resource type and a set of required and optional fields:

```
ccrn=<kind>.<group>/<version>, <fieldKey>=<field_val>, ...
```

This example assumes you have a CCRN CRD defined that describes a container. The resulting CCRN might look like:

```
ccrn=container.k8s.ccrn.example.com/v1, cluster=st-eu-de-1, namespace=kube-monitoring, pod=log-collect, name=collector
```

The field based format **DOES NOT** enforce **hierarchical fields** but **MUST start with** `ccrn=<resourceIdentifier>` where the `<resourceIdentifier> == <kind>.<group>/<version>`

So the following CCRN are identical to the above:

```
ccrn=container.k8s.ccrn.example.com/v1, cluster=st-eu-de-1, pod=log-collect, name=collector, namespace=kube-monitoring

ccrn=container.k8s.ccrn.example.com/v1, name=collector, cluster=st-eu-de-1, namespace=kube-monitoring, pod=log-collect
```

But for better human readability it is recommended to use the hierarchical order of fields as defined in the CRDs URN
and URL templates.

#### URN Format

A more compact string representation for referencing resources are URN formats.

In the example above the URN format may look like:

```
urn:ccrn:container.k8s.ccrn.example.com/v1/st-eu-de-1/kube-monitoringlog-collect/collector
```

The URN  formats are derived from the field-based CCRN format and the respective CRD Annotations and have a
strict order of fields as defined in the CRD Annotations.

#### The Resource Definition

The above example CCRN is based on the following example CRD definition that describes a k8s container resource:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: containers.k8s-registry.ccrn.example.com
    annotations:
        ccrn/v1.urn-template: "urn:ccrn:<ccrn>/<cluster>/<namespace>/<pod>/<name>"
spec:
    group: k8s-registry.ccrn.example.com
    names:
        kind: container
        listKind: containerList
        plural: containers
        singular: container
    scope: Namespaced
    versions:
        -   name: v1
            served: true
            storage: true
            schema:
                openAPIV3Schema:
                    type: object
                    required: [ "ccrn", "cluster", "namespace", "pod", "name" ]
                    properties:
                        # Required
                        ccrn:
                            type: string
                            enum: [ "container.k8s-registry.ccrn.example.com/v1" ]
                        cluster:
                            type: string
                            description: "The cluster the container is running in"
                            enum: [ "st-eu-de-1", "*" ]
                        namespace:
                            type: string
                            description: "The Namespace the container is running in"
                            pattern: "^([a-z0-9]([a-z0-9-]*[a-z0-9])?|\\*)$"
                            maxLength: 63
                        pod:
                            type: string
                            description: "The name of the Pod the Container is running in"
                            pattern: "^([a-z0-9]([-a-z0-9]*[a-z0-9])?|\\*)$"
                            maxLength: 253
                        name:
                            type: string
                            description: "The name of the Container"
                            pattern: "^([a-z0-9]([-a-z0-9]*[a-z0-9])?|\\*)$"
                            maxLength: 253
                        # Optional
                        id:
                            type: string
                            description: "This is the containerID of the container describing the concrete ephemeral container"
                            pattern: "^([a-zA-Z0-9:.-_]+|\\*)$"
                        labels:
                            type: object
                            additionalProperties:
                                type: string
                            description: "Labels for selecting groups of containers"
```

### Required vs Optional Fields

Each resource type defines required fields for unique identification and optional fields for grouping/filtering.  
Generally required fields are fields used in the URN and URL templates, while optional fields are used for additional
metadata or grouping are only present in the field-based CCRN format.


### Validation

Validation is performed via Kubernetes Custom Resource Definitions (CRDs) and an admission webhook or directly via
consumption of the CRD Files using the Kubernetes Library.

#### Validation via Admission Webhook

1. Create a CCRN custom resource with either `ccrn` or `urn` field.
2. The webhook parses and validates the CCRN against the corresponding CRD schema.
3. Resource creation succeeds only if validation passes; otherwise, an error is returned.

Example CCRN resource:

```yaml
apiVersion: validate.ccrn.example.com/v1
kind: CCRN
metadata:
    name: test-container-ccrn
spec:
    ccrn: "ccrn=k8s-registry.ccrn.example.com/v1, cluster=eu-de-1, namespace=ccrn-test, pod=somepod-xyz, name=actual-name"
```

#### Validation via Kubernetes Library

You can also validate CCRNs directly using the Kubernetes library in your application code. This allows you to check if
a CCRN conforms to the defined CRD schema before processing it.

```golang 
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/cloudoperators/common-cloud-resource-names/pkg/validation"
    "github.com/cloudoperators/common-cloud-resource-names/pkg/webhook"
)

func main() {
    // Create logger
    logger := logrus.New()
    logger.SetLevel(logrus.DebugLevel)

    // Create offline backend
    backend := validation.NewOfflineBackend(logger, "ccrn.example.com")

    // Load CRDs from directory
    crdDir := "./ccrn_out" // Adjust path as needed
    if len(os.Args) > 1 {
        crdDir = os.Args[1]
    }

    fmt.Printf("Loading CRDs from directory: %s\n", crdDir)

    // Load all YAML files from the directory
    err := backend.LoadCRDsFromDirectory(crdDir)
    if err != nil {
        log.Fatalf("Failed to load CRDs: %v", err)
    }

    // Show loaded CRDs
    loadedCRDs := backend.GetLoadedCRDs()
    fmt.Printf("\nLoaded %d CRDs:\n", len(loadedCRDs))
    for _, crd := range loadedCRDs {
        fmt.Printf("  - %s\n", crd)
    }

    // Create validator
    validator := validation.NewCCRNValidator(backend)

    res, err := validator.ValidateCCRN("ccrn=k8s-registry.ccrn.example.com/v1, cluster=eu-de-1, namespace=ccrn-test, pod=somepod-xyz, name=actual-name")
    if err != nil {
        log.Fatalf("Failed to validate CCRN: %v", err)
    }

    if res.Valid {
        fmt.Println("CCRN is valid")
    } else {
        fmt.Println("CCRN is invalid:")
        for _, err := range res.Errors {
            fmt.Printf("  - %s\n", err)
        }
    }
}
```

## Requirements and Setup

*Insert a short description what is required to get your project running...*

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/cloudoperators/common-cloud-resource-names-ccrn-/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/cloudoperators/common-cloud-resource-names-ccrn-/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2025 SAP SE or an SAP affiliate company and common-cloud-resource-names-ccrn- contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/cloudoperators/common-cloud-resource-names-ccrn-).
