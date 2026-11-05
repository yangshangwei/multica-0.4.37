# M1-A1 implementation verification (independently accepted)

## Behavior actually implemented
New role versions (reporter/code/security v2, migration v3) require a server-minted capability binding one task claim generation, current issue/project, runtime and originator. Reporter reads its frozen project; reviewers and no-project reporters read only their issue. Only current-issue comments are writable. The default-deny guard runs after both ordinary Auth router groups; legacy and password authentication share verified task-token context. Actual handler reads use a repeatable-read transaction; comments validate and lock authority/resource inside the write transaction. Ordinary human and legacy role controls remain unchanged.

Current membership, disabled users, invocation target access, runtime owner, task status, generation and project are revalidated. Claim preparation and finalization compare scope. Extra broker credentials and configured external tools/non-issue claims are refused. Restricted results persist existing dispatch_eligible=false, skip thread reopening/escalation cancellation, and remain inert in completion/reconciliation/plugin event handling. Restricted failed tasks do not auto-wake coordinators. Native resume and cross-issue sibling summaries are withheld. Restricted requests cannot mint wildcard CloudFront cookies.

## Red/green evidence
Private raw logs and summaries live under `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-rnd-m1-9l4yjl2l/`; hashes and named inventories are retained in verification-runs.json.

- `capability-router-red01`: real NewRouter reviewer v2 attempted title edit; expected403, got200 with title actually changed. 1 named failure,0skip. `capability-router-green01` (root replay): same test passes.
- `capability-cdn-red01.log`: restricted authenticated request received wildcard CloudFront cookies; failed. Fixed middleware withholding those cookies; `capability-pure-green01.log` focused auth/middleware packages pass.
- `capability-handler-green01`:17 named pass,0fail/skip, initial claim/resource/auth/replay/legacy cases.
- `capability-handler-green02`:31pass/1fail/0skip. New deferred-escalation fixture omitted its mandatory runtime_id; failure occurred during setup, not capability assertion. Corrected that fixture only.
- `capability-handler-green03`:37pass/0fail/skip. Four role profiles, both claim paths, real Auth, allowed UUID/identifier/project reads, denied mutations/list aliases, revocation/expiry/terminal/claim generations, current originator membership/disabled/invoke, attachments/thread/escalation controls, blocked tool/non-issue claims, scope/broker finalization rejection, actual snapshot/move check, password path, old role and fresh template adoption.
- `capability-handler-race01`:9 named pass/0fail/skip for scope snapshot, current authority, replay and finalization.
- `capability-handler-extra01`:2 named pass/0fail/skip with race: actual daemon plugin tool/credential backstops; concurrent project move held across comment attempt rejects before write, then stale scope403.
- `capability-service-green01`:3 compatibility controls pass; command regex excluded the new capability test, so this is not evidence for it. `capability-service-green02` explicitly selects and passes4 tests: restricted failure does not wake coordinator, legacy failure recovery, plugin suppression, atomic claim-failure rollback.
- `capability-migrations-green01` (root):1 named pass; real migration765 populated-refusal, concurrent-mint NOWAIT, empty rollback and re-up in a private schema.
- Scoped build auth/service/handler/cmd/server and go vet auth/middleware/service/handler/cmd/server pass. git diff --check clean. sqlc v1.31.1 regeneration leaves all130 generated Go files byte-identical (`capability-sqlc-check.json`).

Counts include subtests, overlap between runs, and are not a total repository pass count. Failed fixtures, build attempts and baseline failures are retained; new run IDs preserve original logs.

## Scope and remaining verification
Independent reviewer is checking live-source projections and whole enforcement path; no M1-A1 completion claim until findings close. Local tests use synthetic tokens and a task-owned database; no real model/provider, production, external credentials, push, hosted CI or release. No complete repository frontend/backend or browser smoke was run by this lane. Go1.26 hosted CI remains unmeasured. This is API/platform-injected-tool authority, not host shell isolation. User-authored captured context and ambient host inputs remain outside the HTTP scope guarantee. Existing per-resource signed attachment URLs retain their own expiry and are not revoked by deleting a task token.

## Independent source-context finding and final handoff
Reviewer added `task_capability_review_test.go` (separate ownership). `check-a1-source-context-red02` proved3 live out-of-scope projection failures with one same-project positive. API source context was returning newly added source comments/attachment filenames through the allowed target issue. The implementer now checks both stored source issue and current anchor issue against the capability before any live source/thread/author projection, preserving the captured user-provided snapshot and returning current state unavailable outside scope. Same-project reporter behavior remains intended; reviewer owns the replay and final verdict.

Product-owned paths and exact hashes are in implementation-source-manifest.json. The role/doc helper has its separate manifest. Final build/vet logs are `capability-static-final-*.log` with `capability-static-final.json`. Independent-review evidence after this handoff remains authoritative for subsequent source changes. No full milestone acceptance is claimed here.

## Independent final acceptance
Independent check fixed a live source-context projection leak (new outside-scope attachment names), retaining the failing regression and frozen-snapshot/same-project controls. Final independent checks: handler race38, production router10, auth/middleware8, service4 named passes,0fail/skip; build/vet/gofmt/diff pass. See independent-review.md and review-evidence/. This accepts M1-A1 only, not complete M1.

The retained source-context red patch deliberately preserves unified-diff context bytes (space+tab and blank context lines). Product/code/spec whitespace checks pass; raw patch evidence is exempt from source-format cleanup.
