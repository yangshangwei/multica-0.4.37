# Implementation interfaces

This file fixes the initial interfaces for parallel implementation. Changes require notifying the other lane owners. The main session owns cross-lane agreement; all payload keys use snake_case.

## HTTP

All paths below are under `/api/projects/{id}/materials`; workspace is captured through the standard authenticated workspace header. Project and material IDs are UUIDs. Reads require current project access. Settings/decisions/manual start require a human. Candidates from an Agent require its current task/project scope and cannot choose credentials or claim human approval.

- `GET /settings` → `SettingsEnvelope`
- `PUT /settings` with `SettingsInput` → `SettingsEnvelope`
- `GET /` → `{workspace_id,project_id,materials: Material[]}`
- `POST /` with `{request_id,title}` → `MaterialDetail`
- `GET /{materialID}` → `MaterialDetail`
- `POST /{materialID}/revisions` with `RevisionInput` → `MaterialDetail`
- `POST /{materialID}/decisions` with `DecisionInput` → `MaterialDetail`
- `POST /{materialID}/comments` with `{request_id,revision_id,body}` → `MaterialDetail`
- `POST /{materialID}/retry` with `{request_id,expected_version}` → `MaterialDetail` (resume existing intents; no new revision)
- `POST /{materialID}/start-plan` with `{request_id,expected_version}` → `MaterialDetail`

SettingsEnvelope: `{workspace_id,project_id,can_configure:boolean,settings:Settings|null}`.

Settings: `{version:number,enabled:boolean,provider:"github"|"gitlab",resource_id:string,connection_id:string|null,repository:string,base_branch:string,root_path:string,reviewer_ids:string[],plan_agent_id:string|null,auto_start_plan:boolean}`. `repository` is `owner/repo` or GitLab namespace/path. GitHub connection_id identifies the installation using the existing ID representation; GitLab uses connection UUID. API accepts a string for either. No credential in client payload. Resource and connection must match the repository and current workspace.

SettingsInput: Settings minus version, plus `{request_id,expected_version:number}`; expected 0 for first configuration. First setting omitted auto_start_plan defaults true; explicit false persists. Reviewer IDs refer to human user UUIDs in current workspace. plan_agent_id is an existing invocable project plan-generation Agent; absent keeps handoff blocked with an actionable reason.

Material: `{id,workspace_id,project_id,title,path,version:number,status:string,blocker:string|null,created_at,updated_at}`. `path` is the server-owned stable requirement directory/PRD location. Status values: draft,publishing,in_review,changes_requested,approved,merging,blocked,baseline_ready,planning,plan_review,plan_approved. Clients must display unknown states safely and refuse version-dependent actions unless recognized.

MaterialDetail: `{material:Material,revisions:Revision[],decisions:Decision[],comments:Comment[],baseline:Baseline|null,plan:Plan|null,capabilities:{can_edit:boolean,can_review:boolean,can_start_plan:boolean,can_retry:boolean}}`.

RevisionInput: `{request_id,expected_version:number,kind:"requirement"|"plan",content:string,plan_content?:string,sources:Source[],acceptance_criteria:string[],blocking_questions:string[],ready:boolean,source_baseline_id?:string}`. Requirement content is full PRD. Plan content is full technical design; `plan_content` is task breakdown, required for reviewable plan. The service computes digests, stable paths and readiness; ready is a request to freeze a reviewable candidate, never approval. No manual submit-for-review step: accepted ready candidates enqueue publication automatically. Incomplete saves remain draft.

Source: `{resource_id,commit,path}`; commit must be a full SHA and the resource must be authorized. Candidate text is never interpreted as shell commands or policy.

Revision: `{id,kind,content,plan_content:string|null,content_sha256,sources:Source[],acceptance_criteria:string[],blocking_questions:string[],source_baseline_id:string|null,publication_status:string,review_url:string|null,head_sha:string|null,created_at}`.

DecisionInput: `{request_id,expected_version:number,revision_id,head_sha,decision:"approve"|"request_changes"|"withdraw",reason:string}`.

Decision: `{id,revision_id,actor_id,decision,reason,head_sha,created_at}`. Comment: `{id,revision_id,actor_id,body,created_at}`. Server obtains actor; caller-supplied actor is rejected/ignored by strict request decoding.

Baseline: `{id,revision_id,commit,path,content_sha256,review_url,verified_at}`. Plan: `{issue_id:string|null,status:string,blocker:string|null,baseline_id:string,revision_id:string|null}`. Plan states waiting_manual,blocked,queued,running,awaiting_review,approved. Queued/running must come from actual execution facts.

All mutation bodies carry stable request IDs, CAS failures return 409; reuse with different bodies returns idempotency conflict. Reloaded detail provides durable operation states and blockers. Reads may poll; GET never performs remote writes. Native checkout sources and private repository reads remain credential-free in UI.

`start-plan` also provides explicit human recovery for a terminal planning attempt that produced no plan revision. It reuses the issue and preserves prior execution rows, creates at most one new active attempt after current authority/baseline checks, and returns the original result for an identical request replay. An active attempt or a submitted/approved plan cannot be restarted through this command. Automatic jobs, reconnects and configuration re-enablement never restart a terminal attempt.

## Go provider boundary

Provider owner creates `server/internal/materialgit` (only provider HTTP, no DB/services).

```go
type Connection struct { Provider, BaseURL, Repository, Token string }
type File struct { Path, Content string }
type Change struct { Path, PreviousPath, Status, Mode string }
type PublishRequest struct { Key, Branch, BaseBranch, Title, Body string; Files []File }
type Review struct {
    Number int64
    URL, HeadSHA, BaseBranch, Branch, State, MergeSHA, MergeActor, BlockReason string
    Changes []Change
    GatesPassed bool
}
type DecisionNote struct { Key, ActorID, RevisionID, HeadSHA, Decision, Reason, URL string }
type MergeExpectation struct {
    Number int64
    HeadSHA, Branch, BaseBranch string
    Changes []Change
}
type Provider interface {
    Publish(context.Context, PublishRequest) (Review, error)
    Observe(context.Context, int64) (Review, error)
    SyncDecision(context.Context, int64, DecisionNote) error
    Merge(context.Context, MergeExpectation) (Review, error)
    ReadFile(context.Context, string, string) (string, error)
}
func New(Connection) (Provider, error)
```

`Publish` uses a deterministic unique branch per immutable revision; recovers an existing branch/review only after exact content/target checks. Never update a foreign/mutated branch or force-push. No branch update is necessary for frozen revisions. Both providers create a PR/MR for each candidate revision; historical reviews remain traceable. `Observe` returns the complete changed-file list or error and does not interpret missing/unknown native gate facts as passed. `Merge` checks the frozen review number, source/target branches, head and complete change set before its write and against the final receipt, never bypasses native policy, and returns observed state after API completion. These checks do not make the provider's target-branch update atomic. `ReadFile` verifies regular-file mode (not symlink/submodule), full commit and bounded complete content. Service rechecks expected paths and content before merging and after merge. HTTP errors are sanitized and bounded; transient/uncertain outcomes remain retryable, unknown states fail closed.

Provider handles token HTTP only. Backend owner resolves existing GitHub App installation or encrypted GitLab connection into Connection; credentials never enter material JSON, logs or Agent input. Official documentation references are in design.md.

## Ownership

- Backend: service/handler/routes/lifecycle, migrations/SQL/generated, execution join and internal material domain. Owns all Go files except `internal/materialgit`.
- Provider: `server/internal/materialgit/**` and tests only; publish interface immediately so backend can compile.
- Frontend: `packages/core/materials/**`, narrow ApiClient wiring/package exports, `packages/views/projects/**`, locales and tests. Preserve existing APIs and shared project layout. No Go changes.
- Main: task artifacts, environment/test coordination, SDK/skill documentation and integration checks after lanes settle.

No lane changes another lane's files without coordination. No new dependency, no destructive Git, no remote Git mutations during development verification.
