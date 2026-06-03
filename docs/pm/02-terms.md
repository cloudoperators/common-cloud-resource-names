# Glossary: Understanding the Terminology

This document defines the key terms used within the SAP Cloud Infrastructure (SCI) Permission Concept.

### **Access Level**

* This is the technical term for the resource defined in **CAM (Cloud Access Manager)**. In our concept, each `AccessLevel` resource in CAM represents one **Application Specific Role**. End-users do not interact with `AccessLevels` directly.

### **Access Request**

* An official request made by a user in CAM to be granted a **Profile** (Functional Role).

### **Application**

* A general term for any service, system, tool, or process that requires access control (e.g., Vault, NetBox, Kubernetes).

### **Application Specific Role**

* A granular role that defines a collection of **Capabilities** on the **Resources** of a specific Application. These are managed as `AccessLevel` resources in CAM.

* **Example:** `VAULT_EU-DE-1_TEAM-SRC-KV_SUPPORT` (for a regional tool with sub-resource separation) or `NETBOX_ADMIN` (for a global tool).

### **Base Role**

* A special type of **Profile** in CAM that does not grant any direct permissions. Instead, having a Base Role is a prerequisite that allows you to request certain Functional Roles. They act as a meta-role to control who can request sensitive permissions and can be used to enable pre-approved, rapid access elevation in certain scenarios.

* **Example:** `Permit - OnCall - Observability`

### **CAM (Cloud Access Manager)**

* The central SAP tool used to manage identity and access. All **Access Requests** for roles are made through CAM.

### **CAM Operator**

* An automated service that synchronizes role definitions from a central Git repository to CAM. This is the engine behind our "Permissions as Code" approach.

### **CCRN (Common Cloud Resource Name)**

* A standardized, machine-readable naming convention for identifying resources across different systems. We use CCRNs in the description of **Application Specific Roles** to precisely document what resources a role grants access to.

* **Example:** `ccrn: apiVersion=k8s_registry.ccrn.sap.cloud/v1, kind=*, cluster=*, namespace=keppel`

### **Functional Application Role**

* A type of **Profile** in CAM that an end-user can request. It typically maps to a single **Application Specific Role** and is used to grant fine-grained access to a specific application.

* **Example:** `Keppel - EU-DE-1 - Admin`

### **Functional Organizational Role**

* A type of **Profile** in CAM that an end-user can request. This role maps to an organizational unit (like a team or department) and is a collection of multiple **Application Specific Roles** that members of that unit typically need.

* **Example:** `Team - Security Reliability and Compliance`

### **Profile**

* This is the term for a resource in CAM that maps to our **Functional Roles**. Profiles are the user-facing roles that you can see and request in the CAM user interface.
