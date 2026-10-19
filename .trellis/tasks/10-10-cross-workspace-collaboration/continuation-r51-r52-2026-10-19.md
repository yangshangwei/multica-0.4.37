# R51 children, recorded runs and R52 result notifications

Date: 2026-10-19. The user explicitly requested continuation of the existing
`in_progress` task for ordinary children/legacy entry compatibility and complete
recorded run content, usage, resource provenance and AI-result notifications.
This is an implementation increment under the approved PRD/design; it does not
create a new task or reopen settled product choices.

## Goal and boundaries

The shared account/source detail currently has no ordinary-child section.
Ordinary child relationships already belong to the source issue service. Add an
authorized projection and shared presentation, retaining that relationship owner
and its existing execution isolation. Source members can open ordinary children;
external collaborators receive only id/identifier/title references without an
open action or source directory access. Do not silently cap the child list.

The existing runtime stores final outcome text and source-owned run events, but
does not yet carry typed recorded messages or usage. Complete the supported
recording path from the runtime outcome through immutable source storage and an
object-authorized, paged read to shared views. Preserve old `{outcome, body}`
results; missing historic telemetry means not recorded, never zero usage or
invented success. Show only system-recorded process content and immutable inputs
actually read by that run. Never use a mutable current batch as its old input.

Recorded resource/provider provenance is an allowlisted projection of the frozen
execution identity. Do not disclose credentials, installation secrets, private
directories or unrelated resource catalogs. Engine task/runtime deletion must
not delete the source records. Current issue authority gates every read, including
pagination, native windows and notification links.

AI result notifications use the existing durable account notification/outbox
contract. One terminal group transition produces one logical result event;
ordinary leader/member turns are not whole-squad completion. A stop request is
not stopped: the complete applicable proof barrier must be satisfied. Proof
without a business result remains failed/incomplete. Preserve forced disposition
over late success. System/AI outcomes must not impersonate a resource owner or
input author as the human operator for recipient exclusion. New human actions
retain the existing self-notification exclusion. Replays cannot notify twice or
recompute the original recipient list.

Compatibility fixes include shared account links from source/search actions and
guarding mixed collaboration batch changes before ordinary AI confirmation or
transport. Ordinary issues keep their current links, child actions and execution
behavior. No dependency additions, general provider adapters, runtime Git/file
preparation, new OS containment, P7 administration or release gate changes are
part of this increment. Full P8/97-criterion completion is not claimed.

## Implementation and ownership

1. **Server and supported runtime recording** — one Trellis implementer owns
   `server/` changes, including source child projection, record ingestion/read,
   settlement/result notification, routing, SQL and necessary generated files.
   Reuse the existing issue/record ownership, transaction and replay helpers.
   Add dedicated collaboration record queries/services/handlers when needed;
   do not inflate execution protocol v5 with transcript content. The owner first
   writes `research/r51-r52-api-contract-2026-10-19.md` with exact additive wire
   shapes, limits and anchors for coordination before frontend implementation.
2. **Core contracts and state** — a separate Trellis implementer owns
   `packages/core/` API/schema/query/path work and tests. Implement the agreed
   child and records contract, account/run deep links and protected paging.
   Preserve exact source identities, redacted parsing, access epochs and current
   optional-route behavior. New reads stay in React Query and never persist
   source content in Zustand. No API-client/request-module import cycles.
3. **Shared UI and entry points** — a separate Trellis implementer owns
   `packages/views/`, needed app/platform wiring and E2E tests. Reuse
   `CollaborationDetail` for account/source/panels/windows; render ordinary-child
   references and run record drilldowns with truthful empty/missing/incomplete
   states, resource/usage labels and result-notification anchors. Fix the
   identified source/search copy-link and batch-confirmation entry gaps.
   Shared code uses NavigationAdapter/AppLink, semantic tokens and bilingual
   product terms. Read the relevant component and quality guidelines.
4. **Integration and independent check** — the main session owns the task/spec
   records, test environment, final verification and commits. An independent
   Trellis checker reviews complete changed behavior after integration, with
   mechanical fixes in its explicit scope. No implementer recursively spawns or
   commits. All agents work together without reverting each other's changes.

Each implementation owner writes meaningful failing behavior tests before the
corresponding production edits, verifies the failure, then implements and runs
the narrow suite. Existing coverage is reused instead of mirrored at each layer.
Any interface change is coordinated before dependent edits. A generated sqlc
pass is serialized by the server owner; migrations, if actually required, obey
the repository no-FK/concurrent-index rules and are not mixed with other owners.

## Verification and acceptance for this increment

- R51/AC82: source versus external child projection, no directory/body leakage,
  complete list, ordinary attach/detach/delete/current membership, source links,
  unchanged parent collaboration revision/participants/handler and isolated
  ordinary-child AI behavior. Revalidate the relevant existing R53/R56 boundary
  tests and mixed old-entry actions; do not claim unrelated R58 closure.
- R05/R28 records: exact run/issue matching, current membership/participant ACL,
  machine refusal, invalid and missing optional records, recording size/type
  limits, immutable inputs/resources, no secret projection, paging completeness,
  source retention after engine/resource cleanup, stale response/access cleanup.
- R52/AC83: completion/failure/confirmed-stop timing, member/leader versus group,
  proof/result arrival order, forced outcome, never-launched refusal, replay and
  concurrent terminal settlement, participant eligibility, ordinary inbox
  exclusion, offline account read and exact account record deep link.
- Normal tests use only test-created executables and synthetic runtime records.
  Exercise the supported CLI/daemon recording path with such an executable;
  no installed provider CLI, account or paid AI invocation is authorized.
- Run scoped Go race tests, affected TypeScript suites and lint/typecheck, sqlc
  drift and relevant static checks. Broaden to the required repository quality
  gate once integrated. Verify production Web and actual macOS Electron shared
  child/record/notification journeys, including external permissions, page
  refresh, narrow layout and 200% zoom; keep screenshots and visual verdicts.
- Final report identifies commands/results, changed files, remaining parent
  work and any real environment limits. Do not mark the entire parent complete.

## Evidence and existing workspace changes

Baseline: `a157fd140` on `codex/cross-workspace-collaboration`.
Evidence: `.omx/reports/cwc-r51-r52-20261019T110745Z/`.
Baseline status, diff and task metadata were captured before binding the task.
Existing task/journal/QA archival changes and `apps/web/next-env.d.ts` are retained
and must not be discarded or accidentally included as unrelated product edits.
The test environment uses newly created, labeled PostgreSQL/Redis containers and
isolated databases; it does not use existing application data.

The legacy `make migrate-up` bootstrap tried the shared PostgreSQL entry despite
the task database URL. Its newly started shared container was stopped again with
the data volume retained. Migration now uses explicit isolated database creation
and `go -C server run ./cmd/migrate up`; the failed attempt and recovery remain in
the evidence directory. Do not retry the shared bootstrap for this environment.
