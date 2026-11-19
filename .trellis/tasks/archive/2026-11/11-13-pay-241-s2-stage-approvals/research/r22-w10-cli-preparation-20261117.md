# Research: R22 matching ordinary CLI W10 supplement

- Query: Find the synthetic-creator login recipe and exact ordinary CLI refusals/zero-effect audit for outstanding A09/W10.
- Scope: Internal, read-only retained R20 actor evidence, bound R20 frontend factory, current accepted R22 CLI inputs and owning-layer source. No API/DB/Go/build/process/source actions or unrelated account stores.
- Date: 2026-11-17
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Findings

### Retained creator is identified; inspected metadata does not retain its password

The retained R20 first-case evidence identifies root `01a1e4a0-afa2-72f9-94c2-3c3c9ba256f1`, source workspace `e37632b9-1082-4664-9b83-b4eb74d40898`, and actual Product/creator `1a4a48b5-659d-4e08-ab6f-3084905c9dfb`. Its creation returned201 and the full scenario passed. Exactly one `public synthetic password registration` step matches that creator; its facts contain only `actor_id` and `username`.

Consumed R20 `frontend-source-r01/e2e/fixtures/collaboration-host-runtime.ts` is byte-identical to ROOT, SHA256 `3ea2e6a51cd782f55d2268427739e6749f93c8bf23069f817147f2cff603fc92`. `createHostActor:499` derives the username from label/suffix but generates its password using an independent `randomUUID()` interpolation. The password cannot be reconstructed from username, suffix, actor ID or factory source. The recorded username SHA256 is `fde582e44da560f2fc635eb4efc3fe68f745a139bf746c1dc84e2424012df0d7`; no credential value is copied into this report.

The exact task-owned `r20-source-denial-panel-r01/runtime/four-scenarios-r01/test-results` tree contains zero trace ZIPs. No retained login-body credential witness was found in the scoped evidence inspected. This is not an inventory of every private file/account on the machine.

If main has a separately retained task-owned credential witness for this exact synthetic creator, privately `POST /auth/login`, require the returned user ID and fresh `/api/me` identity to match the recorded creator, then use that JWT. Otherwise use the explicit fresh synthetic-root fallback below. Do not guess a suffix-based password, reset the old creator, impersonate it through admin, or inspect real-account stores.

### Fresh accepted CLI and exact requests

Use current R22 CLI `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r22-runtime-capabilities-read-r01/runtime/build-r01/multica`, SHA256 `817360414a24d19ed760633c8080fe1bfd8201b7dc47f4e214e205cfe5218abb`. Its current `products.json` is SHA256 `b2d1179108dbf50311cb1f367dd1011649d2a7fdedc9f6307e2163816d01f79b`. The old W10 map's R18 CLI is historical.

```text
<R22-cli> --debug --server-url http://127.0.0.1:64922 --workspace-id <source-workspace> issue completion prepare <root> --status done
<R22-cli> --debug --server-url http://127.0.0.1:64922 --workspace-id <source-workspace> issue update <root> --status done --priority <actual-unchanged-before-priority>
```

| Command | Required actual request | Exact expected scope refusal |
| --- | --- | --- |
| `completion prepare`, no `--approval` | `GET /api/issues/<root>/delivery-evidence/candidates?limit=20` | HTTP409 `delivery_completion_consumer_unsupported` |
| `issue update` with status and unchanged priority | `PUT /api/issues/<root>` with exactly those fields | HTTP409 `unsupported_collaboration_action` |

The first command selects the candidate GET at `cmd_issue_completion.go:302–310` and creates no operation file. `lockDeliveryPreviewIssue` calls `completionOrdinary` (`delivery_completion_preview.go:158–160`); that helper checks formal admission and the actual collaboration row (`delivery_completion.go:481–491`) and returns `completionUnsupported` (`delivery_completion_types.go:33–34`). A target lacking formal admission yields an authorization/admission refusal rather than the required scope proof.

Explicit unchanged priority makes `issueUpdateIsStatusOnly=false` (`cmd_issue.go:1615–1625`), bypassing only the advisory hint and reaching actual `PutJSON` (`:1460`). `UpdateIssue` invokes `handleLegacyCollaborationUpdate` before mutation (`handler/issue.go:4188`), which allows only layout position/expected_revision keys and returns the exact scope409 for status/priority (`collaboration_boundary.go:135–138`). Do not guess priority, add a completion grant or operation file, use forced input, or change the original four-case body.

### Actual HTTP status without new instrumentation

Root `--debug` is registered at `server/cmd/multica/main.go:47`. `APIClient.newHTTPError` retains method/path/status/capped response body (`server/internal/cli/client.go:73–102`); `errors.go:591–595` formats `[debug] http: METHOD PATH status=409 body=...`. It never emits request Authorization headers or bearer values. This supplies the actual HTTP status; no proxy, API tracer or source change is needed.

Capture exact argv/CWD/binary hashes, start/end, exit, stdout/stderr to new private files, then parse that debug line and allowlisted error/code JSON. A nonzero exit alone is insufficient. Authentication, local execution-marker, not-found, generic503 or configuration failures do not close W10. Never log login bodies/responses, full environment, passwords, tokens or arbitrary headers.

Create child env from an allowlist with explicit private HOME/TMP/PATH and only the synthetic JWT in `MULTICA_TOKEN`; use explicit server/workspace flags. Strip `MULTICA_AGENT_ID`, `MULTICA_TASK_ID`, `MULTICA_DAEMON_PORT`, `MULTICA_TASK_CONFIG_ROOT`, and all five `MULTICA_COLLABORATION_{TOKEN,SERVER,CONTEXT,ISSUE_ID,RUN_ID}` values. Verify the empty private CWD and its ancestors have no daemon task marker; do not remove an existing user's marker. The actual guards are `cmd_agent.go:254–270`, `:428–445`, and `cli/collaboration.go:37–43`.

## Minimal fresh synthetic-root fallback

Use the bound existing `closureActor` / `createHostActor` factory with a new W10 label and suffix. It registers through the actual password API and completes onboarding (`collaboration-host-runtime.ts:497–512`). Its `authenticateInstalled` function accepts a callback with the newly generated username/password/userId (`:504–505`); a task-private callback can consume those values in memory and perform the fresh login, without launching a native app. Avoid changing the factory or copying credential values into artifacts.

Alternatively main may create the equivalent explicitly named synthetic account via actual `/auth/register`, holding a fresh random password in private memory, then `/auth/login`. `TestApiClient.loginPassword` and `registerPassword` (`e2e/fixtures.ts:56–83`) validate a real token and returned user identity. Set the exact task API origin before importing the factory; do not allow its environment/default endpoint fallback to select another environment.

Create this actor's own workspace through the existing client and an explicitly manual collaboration root through the ordinary collaboration creation API. Record actual creator/workspace/root IDs and formal admission before the two CLI probes. Use a current authorized handler; if the creation contract requires a distinct human, create and explicitly enroll that additional synthetic human rather than impersonating an existing account. This new target proves the W10 ordinary/collaboration boundary; it does not replace the original managed-stage four scenarios. If acceptance specifically needs a managed-stage root, bind a project policy/configuration through the existing authorized APIs before freezing the W10 baseline.

Start the no-effects baseline **after** explicit account/workspace/root preparation. Those preparation writes are declared setup, not silently included in a zero-write claim. Preserve the new synthetic records and original R20 evidence. Do not delete an actor/workspace with retained collaboration history to manufacture empty counts.

## Before/after audit

Main owns DB admission and actual queries. Reuse the old W10 map's real tables/parameters with fresh actual IDs when using the fallback. Use a read-only repeatable-read transaction with a five-second statement timeout and rollback for each snapshot:

```sql
SELECT id::text,workspace_id::text,creator_id::text,creator_type,
       admission_status,status,priority,revision
FROM issue WHERE id=$1::uuid AND workspace_id=$2::uuid;
SELECT to_jsonb(c) AS snapshot FROM issue_collaboration c WHERE issue_id=$1::uuid;
SELECT to_jsonb(s) AS snapshot FROM collaboration_stage_task s WHERE issue_id=$1::uuid;
SELECT
 (SELECT count(*) FROM collaboration_operation WHERE scope_id=$1::uuid) AS operations,
 (SELECT count(*) FROM collaboration_event WHERE issue_id=$1::uuid) AS events,
 (SELECT count(*) FROM issue_handoff WHERE issue_id=$1::uuid) AS handoffs,
 (SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1::uuid) AS stage_submissions,
 (SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1::uuid) AS authorizations,
 (SELECT count(*) FROM delivery_completion_terminal t
  JOIN delivery_completion_authorization a ON a.id=t.authorization_id
  WHERE a.issue_id=$1::uuid) AS terminals,
 (SELECT count(*) FROM delivery_evidence_evaluation WHERE issue_id=$1::uuid) AS evaluations,
 (SELECT count(*) FROM agent_task_queue WHERE issue_id=$1::uuid) AS queued_tasks;
```

Compare exact status/priority/revision and stable full collaboration/stage JSON hashes; all listed effect counts must remain equal. The queue table is `agent_task_queue`, not an invented `task` table. A stage snapshot absent for an explicitly unmanaged supplemental root stays absent; do not equate that with managed-stage acceptance. Also record the new private CWD's files before/after and require no completion operation/authorization file.

For the retained-root route require fresh `/api/me` to equal the original creator and fresh collaboration GET to confirm current readable creator authority. For the new-root route match the explicitly created synthetic user's ID to the actual `issue.creator_id`. Run the two exact CLI probes once each with the current accepted binary and preserve their full refusals; keep the four-case denominator/result unchanged.

## Files found / code patterns

- `research/r20-final-a01-a12-evidence-map-20261116.*` — original W10 target and zero-effect requirements.
- Retained first-case `stage-approvals-evidence.json` — exact creator registration and creation identities; credential value absent.
- Bound R20 `frontend-source-r01/e2e/fixtures/collaboration-host-runtime.ts`, `collaboration-stage-approvals.spec.ts`, `e2e/fixtures.ts` — actual random-password factory, public registration metadata and fresh login.
- R22 `runtime/build-r01/products.json` — current matching CLI.
- `server/cmd/multica/cmd_issue_completion.go`, `cmd_issue.go`, `cmd_agent.go`, `main.go` — exact requests, unchanged-priority actual PUT, human context and debug flag.
- `server/internal/cli/client.go`, `errors.go` — typed safe HTTP status/error capture.
- `server/internal/handler/collaboration_boundary.go`, `delivery_completion_preview.go`; `server/internal/service/delivery_completion.go`, `delivery_completion_types.go`, `delivery_completion_preview.go` — canonical distinct refusal codes and no-mutation boundaries.

## External references / related specs

No external provider, real account or documentation was accessed. The existing stage and verification-environment specs govern matching inputs, current actor authority, original outcomes and evidence limits. This report prepares the supplement; main retains runtime/resource ownership.

## Caveats / Not Found

- No original creator password or scoped trace witness was found. A synthetic username is not a credential derivation proof.
- No CLI/API/DB/test/build command was executed here; expected409/code and zero-effects remain unobserved.
- Main must decide whether a separately available exact synthetic credential witness permits the preferred original root, or use the explicit fallback. No admin impersonation or credential reset is proposed.
- W10 is one independent negative supplement, not a fifth case or full A09/S2/parent acceptance.
