# Research: Independent parent planning review

- Query: Review parent PRD/design/implementation plan against the original acceptance report, defects, repository rules, regression requirements and evidence isolation.
- Scope: internal; independent parent review only; no recursive agents.
- Date: 2026-11-25
- Reviewed at: 2026-11-25T09:46:20Z
- Worktree: `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`
- Original evidence root (read-only): `/Volumes/artisan/code/2026/multica-0.4.37`

## Findings

**Verdict: ACCEPT the parent design and division of responsibility. No parent product-design blocker prevents bounded implementation after each child's own plan review. This is not executable-run clearance, child-plan approval, or product acceptance.** The execution gates below must be made explicit before their dependent runs. The latest coordinator instruction already requires a frozen source snapshot per run, including the dirty overlay, and says fresh owned PostgreSQL/offline dependencies are being prepared; that resolves the intended policy, but it does not itself prove any run's admission.

The plan correctly preserves the historical FAIL and both accounting views, distinguishes machine principal authority from owner role, keeps NOWAIT protection, separates Host and Namespace hypotheses, preserves complete client journeys and original budgets, and prohibits commits/pushes/releases. The original report explicitly says old passing counts cannot establish acceptance after a fix (`report.md:9`).

### Required corrections and run gates

1. **Correct the defect IDs before dispatch/evidence reconciliation.** Parent `design.md:12` calls the old CLI failure D10 and `design.md:13` calls GitHub403 D11. The authoritative original `defects.md:16` assigns **D10 = GitHub403** and `defects.md:17` assigns **D11 = old CLI timeout/cleanup**. Keep the original IDs: security lane owns D03+D10; client lane owns D06–D09+D11. This is a traceability defect, not a reason to redesign either fix.

2. **P1 run gate: move admission ahead of every executable diagnosis/test.** Parent `implement.md:5–7` currently orders parallel diagnosis and full Go regression before the snapshot/allocation step. Amend it so each baseline, diagnostic and regression run first registers its own source, harness, tool and resource identities. Freeze a copy containing the base identity, tracked dirty bytes/modes and relevant untracked files; bind dependency symlink targets/actual exports, tools, configuration and built artifacts. Source may continue moving in the development checkout, but the runner must consume the frozen copy. Any run on changing source is diagnostic only and cannot establish acceptance. Require before/after identity checks plus complete named results and raw exit status; a successful initial hash check alone is insufficient. The final integrated candidate needs a fresh frozen snapshot and complete required regressions after all lane changes. This implements the coordinator's latest instruction and `.trellis/spec/server/verification-environment.md:79–124`.

3. **P1 run gate: allocate and budget resources before parallel execution.** A task-owned PostgreSQL instance is useful, but children still need recorded database identities and explicit command/package permits. Use a fresh database per DB-backed package and separate browser databases; verify `current_database()` against the parent's allocation before migrations/tests. Record aggregate connection headroom and schedule API/browser/test overlap within it; serialize when uncertain. Keep API pool/publisher overrides out of ordinary Go tests, generate a private signing key before first auth use, use package-private HOME and explicit Go caches, and preserve the default agent-CLI guard. Do not treat increasing a pool or reducing concurrent test load as proof of D04's cause. These are existing contracts in `database-test-isolation.md:20–40`, `:47–86` and `verification-environment.md:16–41`. Keep resource allocation under the parent, as already designed.

4. **P1 native run gate: restore the macOS protocol boundary, not only HOME/profile.** Parent `prd.md:16` and `design.md:20` establish general isolation/cleanup but omit the previously observed system-wide LaunchServices effect. Before macOS production Main preparation/launch, capture the exact default `multica://` app URL and existing same-bundle registered URLs; use only the new task's stable app path; restore and independently verify the original default/set after stopping owned processes. Refuse that native run if recovery cannot be established. Original `test-plan.md:87` and `report.md:77` expressly show that private HOME/userData is insufficient and registration can occur during preparation. Keep Linux's selected security policy unchanged, and bind its actual packaged Main/CLI and dependency resolution separately from renderer tests.

5. **Before final regression, publish a defect-to-command/identity matrix.** Parent `implement.md:6–8` has the right categories but no selected commands, canonical identities, configurations or expected inventories. Child plans can supply these; the parent must reconcile them before running acceptance. Keep the existing child-review gate; their plans were still being drafted, so this review does not certify them. Required closure shape is below. Explicitly identify which previous passes are only historical, which affected families receive new complete runs, and which remain unverified for the new candidate. No arithmetic promotion of the old 400/138 or 117/21 to a new release result.

### Minimum regression shape for the child review

| Defect | Required new evidence |
| --- | --- |
| D01 | Real router/auth legacy and password positive/negative credential matrix; machine owner/admin versus genuine human JWT/PAT; author versus other author; body, attachments-only and deletion; spoofed headers, revoked/terminal/scoped credentials and cross-workspace denial; unchanged content/revision/attachments and relevant side effects on denial. Preserve allowed agent self-operation and handler backstops. Generic `comment_id` upload needs an explicit separate contract/disposition, not automatic inclusion in the proven defect. |
| D02 | Capture actual competing backend/lock timeline on a new owned database; retain the original preparation failure and legitimate 409 behavior. Add a regression appropriate to the observed cause, then run the whole owning service package normal/race and the parent's integrated Go checks. Five filtered successes do not close the original full-package failure. |
| D03 | Pin supported Go toolchain and actual dependency versions; reconcile each original finding with fresh scanner results, reachability and target OS. Distinguish source/target scanning from native Windows execution and from the original local Go1.27 scan. No unverified claim that all findings disappear with a compiler update. |
| D04/D05 | Capture sanitized permit transport/context errors and monotonic cancellation/cleanup order; capture the actual Namespace probe error under the admitted configuration. Then replay both original Host journeys and all nine original Namespace browser identities. Seven Linux backend Namespace blockers are a separate inventory, not the same nine browser identities. |
| D06–D09 | Complete macOS and Linux production-Main Stop/restart/Resume journeys; manual-detail and PAY on both clients; resource publishing; full Snooze/mention/assignment/unread chain; full Desktop delayed protected-read/source-loss journey with new pause admission. Preserve business assertions and deadlines; no `.first()`, forced clicks or hidden-content negative assertions to bypass ambiguity. |
| D10 | Fresh public GitHub response status and safe response headers; retain external failure if unresolved. No personal token substitution and no unsupported rate-limit diagnosis. |
| D11 | Exact old v0.6.7 CLI, all ten operations and write/refusal matrix within the original 240-second budget, plus independent owned-container/volume cleanup proof. A later successful cleanup does not erase the historical cleanup FAIL. |

### Boundary observations

- Comment boundaries match source: `server/internal/handler/comment.go:3333` and `:3594` grant the admin alternative by role; `actor_guards.go:110` centrally classifies server-stamped `task_token` and `cloud_pat`. Review real credential provenance before editing; do not replace every comment route with a human-only gate. `file.go:489` has a separate generic upload association path.
- The NOWAIT fence at `server/internal/service/delivery_evidence.go:129` intentionally protects absent/new PR links as well as existing rows. `collaboration_stage_completion_tx_test.go:1272` prepares M1 before `:1293` switches to the wrong consumer. This supports diagnosis of the preparation failure, not an authorization-bypass or Go-data-race claim.
- Current local dependency declarations are `server/go.mod:3` (Go1.26.6), `:17` (gorilla/websocket1.5.3), `:76` (x/net0.57.0); CI/release select Go1.26.x and run govulncheck. This review does not verify external remediation versions.
- UI fixture repair belongs in canonical E2E fixtures; any discovered product UI fix must retain `views -> core + ui`, platform-only routing, Query server state and core-owned client stores. Tests follow their owning package under `CLAUDE.md`.
- The inherited comment task's `implement.md:7` still says to commit. Parent `prd.md:3`, `implement.md:14` and this dispatch override that stale instruction. Reconcile it in the child handoff; no commit/push is authorized here.
- Parent `implement.md:10` should describe iOS/Windows/browser/pg_bigm/Namespace gaps as **unverified or blocked in the available environment**, not product-wide unsupported platforms. Original `defects.md:23` and `report.md:67` expressly limit the Namespace conclusion to the tested policy/configuration.

## Files found

All paths below are relative to the new worktree unless marked OLD.

- `AGENTS.md`, `CLAUDE.md` — architecture, state/package boundaries, test ownership and agent-execution constraints.
- `.trellis/workflow.md` — parent/child planning and independent review gates; research persistence contract.
- `.trellis/tasks/11-25-main-post-merge-acceptance-fixes/{prd,design,implement}.md` — reviewed parent requirements, ownership and execution ordering.
- `.trellis/tasks/11-20-comment-admin-human-authority/{prd,design,implement}.md` — reusable permission contract and regression matrix; inherited commit wording needs reconciliation.
- OLD `.trellis/tasks/11-24-main-post-merge-e2e-acceptance/{report,defects,test-plan}.md` — authoritative historical accounting, IDs, complete journeys and native resource constraints.
- OLD same task `research/{backend-continuation-results,host-visit-failure-review}.md` — retained lock and prelaunch evidence, with explicit limits on root-cause attribution.
- OLD `artifacts/qa/main-post-merge-e2e-20261124-080504/publication-manifest.json` — 88 selected published records; explicitly excludes much raw/private evidence.
- `server/internal/handler/{comment,actor_guards,file}.go` — comment authorization, credential classification and separate upload boundary.
- `server/internal/service/{delivery_evidence.go,collaboration_stage_completion_tx_test.go}` — nonwaiting observation fence and original failing preparation.
- `e2e/collaboration-installed-clients.spec.ts:32` — original 240-second deadline; installed-main, recovery-extras, PAY, Snooze and resource-publishing specs/fixtures are present.

## Related specs

- `.trellis/spec/server/security-boundaries.md` — credential provenance and handler backstops.
- `.trellis/spec/server/database-test-isolation.md` — actual DB identity, per-package databases, connection budgets and process environments.
- `.trellis/spec/server/verification-environment.md` — admission, immutable run bindings and final source/dependency/artifact audit.
- `.trellis/spec/web/frontend/e2e-run-environment.md:637` — frozen packaging dependencies, shared controller versions, semantic selectors and complete journey replay.
- Server, core, views, web, desktop and guides indexes were consulted for applicable scope; broad placeholder guideline sections do not override root rules.

## Review input fingerprints

SHA-256 of parent documents immediately before recording this review:

| File | SHA-256 |
| --- | --- |
| `prd.md` | `0ac09b12d62ce451ba6e54ee9bc093836784a302033c6a9a926491c9d98e0040` |
| `design.md` | `7c6b1dd1602268bb760037a63728420eeb89f7fd100883e44b533ddb46b31eab` |
| `implement.md` | `a3cf46071201ec460a8f4bfc1418bb6f51a0f4bdb345b0991789f1c23b87507a` |

## Caveats / Not Found

- Child plans require their own subsequent review. In-progress drafts/placeholders are not treated as defects in the accepted parent architecture.
- No services, product edits, test execution, package installation, Git commands, commits or recursive agents were used. Only this research report was written; original evidence remained read-only.
- No `implement.jsonl` or `check.jsonl` was loaded; manifest readiness is outside this role's review.
- The manifest structure/count and original reports were read. This review did not independently rehash all 88 publication records or inspect all private raw streams; the parent's claimed full hash check remains separately attributable to the parent. Each executing lane still must inspect/hash its actual original logs.
- External references: none fetched. Dependency versions above are repository declarations, not a current advisory assessment; the security lane must verify official remediation references before selecting upgrades.
- Planning acceptance does not certify that the original P1 failures are fixed or that the candidate is releasable.
