# Service interface

`ResourcePublisher{Root, SkillDirectory, McpDirectory string; AllowHTTP bool; OrganizationID func(context.Context)(string,error)}`. Root unset disables publication. OrganizationID is used by consumer adapters; API methods receive the current trusted internal organization explicitly.

- `List(ctx, kind, organizationID string) ([]Resource,error)` includes managed and readonly builtins/manual metadata.
- `Preview(ctx,kind,key,filename string,data []byte,organizationID string) (ResourcePreview,error)`.
- `Publish(ctx, ResourceMutation, ResourceCommitGuard) (ResourceMutationResult,error)`.
- `Withdraw(ctx, ResourceMutation, ResourceCommitGuard) (ResourceMutationResult,error)`.
- `Receipt(ctx,organizationID,actorID,operationID string) (ResourceMutationResult,error)` is actor scoped.
- `ResourcePublishingLimits() ResourceLimits` exposes upload/manifest/file/count limits.

`ResourceMutation` fields: `Kind, Key, Filename string; Data []byte; PreviewDigest, ExpectedVersion, Reason, ActorID, OrganizationID, OperationID string`.

`ResourceCommitGuard` is `func(context.Context, func()(ResourceMutationResult,error))(ResourceMutationResult,error)`. It runs **inside the filesystem lock**, including replay. Handler opens actor DB transaction, locks/reauthorizes, records durable request audit, calls apply, finalizes outcome/DB commit. Never take the filesystem lock while holding DB user locks. Nil guards are rejected for public mutations.

`ResourceMutationResult{Resource Resource, Replayed bool, OperationID string}`. `ResourceError{Code, Message, OperationID string}` provides safe messages and `Error()`. Codes match parent design plus `resource_store_full`. Never wrap raw filesystem errors into API-visible text.

`TaskService.ResourcePublisher *ResourcePublisher`; `McpCatalog.Publisher *ResourcePublisher`. `TaskService.SkillTemplates()` becomes `([]RoleSkillTemplate,error)` and handlers must propagate failures. Managed consumer errors are explicit; feature-unset behavior preserves existing catalogs.
