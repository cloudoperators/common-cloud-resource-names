# How-To: Onboard a New Service to the Permission Concept

This guide provides the step-by-step process for service owners and engineers to integrate a new tool, service, or
application with the SAP Cloud Infrastructure (SCI) Permission Concept.

The entire process is managed as "Permissions as Code." All definitions are created as YAML files (Custom Resource
Definitions) and submitted via a pull request to the central IAM repository:

https://github.wdf.sap.corp/sap-cloud-infrastructure/permission-manager

## Step 1: Define Your Application's Resources (CCRNs)

Before you can create roles, you must define the resources within your application that you want to control access to.
This is done using the **Common Cloud Resource Name (CCRN)** framework.

1. **Identify Resources:** List all the distinct resources in your application that require access control (e.g.,
   projects, secrets, namespaces, configurations).
2. **Create CCRN Definitions:** For each resource, create a Kubernetes Custom Resource Definition (CRD) that specifies
   the structure of its name. This includes required fields (like `cluster`, `namespace`) and optional fields.
3. **Submit for Review:** Add these CCRN definitions to the [IAM repository](https://github.wdf.sap.corp/sap-cloud-infrastructure/permission-manager) for review and approval.
4. **Approval and Merge:** The CCRN definitions must be approved by the IAM process owners Before they can be used in role definitions but they can also be submitted together with the Roles in step 2.

**Example:**
[Here you find an example CCRN for Vault Resources.](https://github.wdf.sap.corp/sap-cloud-infrastructure/permission-manager/blob/main/charts/resources/templates/crds/vault/data.yaml)

*([For a detailed guide on CCRNs, please refer to the CCRN project documentation.](https://github.com/cloudoperators/common-cloud-resource-names))*

## Step 2: Define Application Specific Roles

Next, you will define the granular roles for your application. These map directly to `AccessLevel` resources in CAM.

1. **Define Permission Levels:** Determine the levels of access you need. At a minimum, you should have `AUDIT`,
   `SUPPORT`, and `ADMIN`. You may add `EXTENDED` or `SUPER` for more complex scenarios.
2. **Determine Sub-Classification:** If your application has sub-resources (like namespaces, projects), decide if you
   need to separate access more fine-grained.
3. **Create Role Definitions:** In the IAM repository, create a YAML file for each **Application Specific Role**.
    * **Naming:** The name must follow the pattern: `${APP}_$(REGION)_$(APP_SUB_CLASSIFY)_${PERMISSION_LEVEL}`. The `$(REGION)` is optional and defaults to global. The `$(APP_SUB_CLASSIFY)` is also optional and can be used to denote sub-resources (e.g., namespaces, projects).
    * **Description:** This is the most critical part. You **MUST** define the granted capabilities using the CCRNs you created in Step 1. This makes the permission explicit, transparent and auditable.
    
    **Example YAML:**
    ```yaml
    # file: https://github.wdf.sap.corp/sap-cloud-infrastructure/permission-manager/blob/main/charts/permission-manager/data/application_specific_roles/vault/%20access_level_vault_kv_example-kv_admin.yaml
    
    # THIS IS JUST AN EXAMPLE
    # This example would define the VAULT_PROD_KV_EXAMPLE_ADMIN role.
    # 
    # ${APP} = VAULT
    # $(REGION) = NULL
    # $(APP_SUB_CLASSIFY) = PROD_KV_EXAMPLE
    # ${PERMISSION_LEVEL} = ADMIN
    # 
    # It would link to the approvers and contain the CCRN description.
    name: VAULT_PROD_KV_EXAMPLE_ADMIN
   
    # For each "Application Specific Role" it is recommended to provide a functional_role_name. This is used to create a functional application role that can be used to request exactly this role in CAM. 
    functional_role_name: "Vault - PROD - KV - EXAMPLE - Admin"
    # IF you provide a functional_role_name, you MUST provide at-least one approver using their C/I/D-Number.
    # Normally that should be YOU and/or the Service/Subresrouce Owner.
    approvers: 
      - userID: "I007007" # Replace with actual approver C/I/D-Numbers
    
    # Optional: If this role should be requestable by on-call users, specify the required base role
    on-call-role: "Permit - OnCall - Security" 
   
    # Optional:
    emergency-access: true
   
    # permissions is mandatory and must include the CCRN-based permission definition. This is used to describe the actual access granted by this role on your application.
    permissions:
      - resource: "urn:ccrn:secret.vault.ccrn.cloud.sap/v1/vault.global.cloud.sap/example/*"
        capability: "READ"
      - resource: "urn:ccrn:secret.vault.ccrn.cloud.sap/v1/vault.global.cloud.sap/example/*"
        capability: "WRITE"
      - resource: "urn:ccrn:metadata.vault.ccrn.cloud.sap/v1/vault.global.cloud.sap/example/*"
        capability: "READ"
      - resource: "urn:ccrn:metadata.vault.ccrn.cloud.sap/v1/vault.global.cloud.sap/example/*"
        capability: "WRITE"
        metadata_set: default
        spec_set: default
      ```

## Step 3: Add your Roles to Functional Roles (Profiles)

Now, you do actually not want to get your Functional Application Roles directly requested that we created above. 
Instead you add them to the respective functional Organizational Roles (Profiles) that your team or organization uses.

1.  **Identify WHO needs your Application Specific Role:** What Teams, Support Groups, Organization Memebrs etc. need access to this specific Role?
Lets assume in this case you want to add it to 2 Teams: 
    - Team Security Reliability and Compliance (SRC)
    - Team Observability
2.  **Identify the Functional Role(s):** Find the corresponding Functional Role(s) (CAM Profiles) in the IAM repository under `charts/data/functional_roles/`. 
    - For Team SRC it would be: `charts/data/functional_roles/organizational/team/profile-team-src.yaml`
    - For Team Observability it would be: `charts/data/functional_roles/organizational/team/profile-team-observability.yaml`
    - *NOTE:* Teams might have multiple sub-roles within their Team, consult with the Team Manager to find the right one.
3.  **Add the Application Specific Role to the Functional Role(s):** Edit the YAML
    **Example YAML (`Profile` CRD):**
    ```yaml
    # .....
    applicationSpecificRoles: # HERE: Add your Application Specific Role to the list
      - "..."
      - "VAULT_PROD_KV_EXAMPLE_ADMIN" 
      - "..."
    # .....
      ```

## Step 4: Submit Pull Request and Get Approval

1.  **Submit PR:** Once all your definitions (CCRNs, Application Specific Roles, Functional Roles) are ready, submit them as a single pull request to the IAM repository.
2.  **Approval:** The PR must be reviewed and approved by the IAM process owners and any other designated stakeholders.
3.  **Deployment:** Once merged, the **CAM Operator** will automatically synchronize your new roles to CAM, making them available for users to request. For Functional Roles already requested by individuals, the new Application Specific Roles will be added automatically.

## Step 5: Update your Application 

 Up to this point you are still not using the new Roles, they are populated and usable though

1. **Identify main use-case**: There are 2 supported main use-cases: 
   - **Active Directory Based Groups:**  Your application is using **LDAP** for authentication and/or authorization. In this case you need to ensure that your application can query the LDAP server (Active Directory) and that the users are mapped to the correct LDAP groups. The CAM profiles are then mapped to LDAP groups at runtime.
   - **OIDC/SAML Based Groups:** Your application is using **JWT Tokens** for authentication and authorization. In this case you need to ensure that the JWT tokens contain the `cam_profiles` claim. This is done via IAS configuration. The CAM profiles are then mapped to JWT claims by IAS at runtime.


