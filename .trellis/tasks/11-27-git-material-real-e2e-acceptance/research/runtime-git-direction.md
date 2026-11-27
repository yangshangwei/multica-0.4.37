# Runtime Git execution: corrected direction — 2026-11-27

## User intent and previous mismatch

The user clarified: “真实的场景是用运行时比如codex操作吧，为什么需要github app呢？” Treat runtime-based Git operation as the intended scenario. Requiring the user to create or install a GitHub App was an implementation assumption, not a requirement established by the user's D1–D10 decisions. Stop planning App setup as the next prerequisite.

The first delivery implemented server-owned provider writes while the Agent only authored/submitted candidate text. Its tests remain valid evidence for that implementation, but they do not demonstrate the intended runtime-based Git path. The next work therefore includes design correction and runtime integration before live acceptance; it cannot be completed by merely replacing test credentials.

## Repository evidence

- `server/internal/daemon/repocache/cache.go:24`: remote Git subprocesses inherit daemon environment so credential helpers such as gh can locate their config. This supports host-side credentials without a custom GitHub App, but does not prove the exact Agent child environment or write/API scopes.
- `server/internal/daemon/execenv/runtime_config_sections.go:437`: project Git resources can be checked out through multica repo checkout, including pinned revisions. The same file defines runtime handling of CI waits; a material operation should have its own bounded reconcile intent rather than relying on a model waiting forever.
- Parent `research/product-authoring-workflow.md`, “多仓库与提交链路的核查”, already distinguishes host Git credentials from App integration. The earlier research did not require App credentials for runtime operations.
- `server/internal/handler/material_provider.go:22`: current material resolution instead requires a workspace GitHub installation and App credentials. `server/internal/service/material_worker.go` calls Publish/Observe/SyncDecision/Merge/ReadFile directly on that server-side provider.
- `server/internal/handler/material_capability.go`: current narrow task API only permits candidate reads/submission; no runtime material-operation claim/receipt protocol is implemented by that guard. Ordinary shell capability alone is not a verified material publishing workflow.

## Intended responsibilities

- Codex or another configured runtime authors and modifies PRD/design/plan files and performs authorized Git publication in the chosen execution environment.
- Multica keeps material identity, current revision, human decisions, permission checks, durable intent/retry state and scheduling. Runtime reports real Git commit/review/merge facts; model prose alone cannot establish a baseline.
- Credentials remain available through the authorized execution environment. Git transport authentication and hosting API authentication are separate: an SSH key may allow push but is not itself an API credential for PR creation. GitHub CLI supports browser login/PAT/environment tokens without a custom GitHub App.
- Human approval remains a recorded Multica decision; approving a document does not let an authoring model approve itself. Current HEAD, allowed file set, native gates and approved bytes remain enforced conditions.

## Confirmed execution ownership

User subsequently specified: “不需要github app 批准执行 都有选择的运行时来执行”. Publication, PR/MR operations and execution/merge after human approval all belong to the selected runtime. The earlier deterministic merge executor suggestion was not selected and is no longer an open question. Multica retains the human approval, current version/authority conditions and durable scheduling/receipt state; the authoring runtime cannot manufacture human approval.

Runtime selection, staged authorization, credential availability/isolation, offline recovery, claim/lease and verified remote receipts still require a concrete technical design. These are integration details to research, not grounds for asking the user to choose App versus runtime again. Do not expose a reusable unrestricted merge credential to an earlier stage and describe a prompt as an enforced approval fence. Do not introduce a server App read requirement as a hidden prerequisite for accepting runtime observations.

## Exact current hard dependencies

- `server/internal/material/types.go:199`: ValidateSettings refuses absent/empty ConnectionID.
- `server/internal/service/material.go:379`: checkMaterialBinding rejects absent connection_id; at401 the GitHub branch requires a workspace-owned github_installation row.
- `server/internal/handler/material_provider.go:44`: GitHub resolves that installation, requires App JWT at49 and requests an installation token at60. No selected-runtime credential path exists here.
- `packages/views/projects/components/project-material-settings.tsx:40`: fetches GitHub installations; at52 derives the selectable connections from them; at62 disables valid saving without a connection.
- `server/internal/service/material_worker.go:209`: server directly resolves provider; at217 publishes and at312 merges. `server/cmd/server/main.go:706` starts this server worker.

Therefore the new GitHub material feature currently requires App integration end to end. This is not a global requirement that every Multica task or every Git operation use an App: ordinary runtime Git uses its host environment, and the GitLab material branch uses its own server-held connection token. Correcting the intended workflow requires UI/configuration, service authority binding and runtime dispatch/results changes together, not bypassing one validation check.

## Official CLI references checked

- https://cli.github.com/manual/gh_auth_login — browser/PAT/environment token authentication, HTTPS/SSH transport.
- https://cli.github.com/manual/gh_pr_merge — exact-head matching and native merge behavior. Do not use administrator bypass to manufacture acceptance; successful queue submission still is not a completed merge.

No product code, real runtime, repository mutation or App setup was performed for this correction.
