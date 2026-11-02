# Research: Source-backed knowledge real-model readiness findings

- Query: Identify concrete knowledge experiment seams, route identities, safe finite schedules and minimum future implementation scope.
- Scope: internal; read-only source/contract research with owned research artifacts
- Date: 2026-11-02

## Findings

The next bounded increment is a knowledge-only test framework, not a budget/flag change. design.md gives a six-new-code-file proposal; entry-matrix.json freezes the proposed four/two/eight counts, eight producer/role rows, source-file hashes, explicit fake prefixes and open prototype gates. No implementation or real execution is approved by these files.

| Finding | Evidence and implication |
| --- | --- |
| KR1 — existing framework is present | The 17 rk1_*_test.go files hash to5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9 by read-only bytes. The executable map has runtime E0a/H1/H2/H2b/H3a/H3b/H5/H6 only; both runtime fixture services force knowledge false. Missing work is the knowledge fixture/wrapper/dispatch, not the entire runtime platform. rk1_scenarios_recall_test.go:223; rk1_fixture_test.go:75,86 |
| KR2 — ordinary execution cannot use the existing wrapper unchanged | The existing wrapper resolves a plan only from actual MULTICA_COLLABORATION_RUN_ID. An ordinary task supplies another identity and has a different credential/native-session contract. Add a knowledge-only wrapper that discriminates real task/run IDs; do not forge a run ID or apply Host's environment replacement to ordinary execution. rk1_provider_test.go:63–99 |
| KR3 — eight labels are finite, but do not mean eight APIs/UI journeys | Product eligibility has five applicability surfaces, with comment mapped to issue and all three Host roles mapped to collaboration. The eight proposed entries distinguish actual producer or admitted role. AI quick-create is its own task API; manual quick-create is ordinary issue creation. Matrix row counts must be validated by future fake execution before budgeting N=8. task_knowledge.go:210–256; resolver:250–253 |
| KR4 — the real public root producer is a comment | There is no POST /commands submission route in the current router. RK1 f.submit is a service-only setup seam. A client-faithful root sample posts an authorized explicit target mention with Idempotency-Key, execution_view_version6 and target_versions; the comment transaction invokes AppendCollaborationInputInTx. handler/collaboration_comments_create.go:22–75; service/collaboration_comments_create.go:40–100 |
| KR5 — K2 revocation timing changes the expected result | Before a new supported claim, invalid managed exposure clears the selected native pointer and chooses fresh. After a valid claim/preparation is frozen, current history revalidation refuses authorization/replay. K2 must revoke after that frozen third binding to achieve its registered zero-third-launch refusal. task_knowledge.go:400–415,463–478; task_knowledge_prepare.go:404–418 |
| KR6 — lawful prior outputs can confound K1 | A new cwd alone does not remove prior collaboration handoff/result facts. Four state calls use four distinct consumer issues/runs so pending/withdrawn input cannot answer from previous results. K1 selects withdrawal as real fourth state and null expiry; expiry is fake-only until a separately frozen alternative replaces that slot. Current source policy never erases prior lawful disclosures. knowledge spec:104–107; rk1_run_test.go:78–123 |
| KR7 — the minimal member/continuation rows need honest setup provenance | Canonical RK1 H2/H2b already uses fake root/member prefixes before one measured continuation. Reuse that pattern for independent member/continuation samples:1 fake root seed and2 fake root/member seeds respectively,3 fake starts total and0 paid. Every seed still uses genuine service/Runner process/proof, and its output omits decisive facts. These rows are not wholly real squad journeys. rk1_scenarios_handoff_test.go:96–110 |
| KR8 — private fixtures are not cross-package APIs | Existing ordinary composed fixtures and runTask/reportTaskResult live inside package daemon. A small agentintegration-only driver is the minimum seam for a cmd/server router+daemon composition while keeping default product APIs and the old RK1 files unchanged. The bridge delegates production execution, rather than copying it. daemon/knowledge_server_integration_test.go:281–337; daemon.go:636–688,6856 |
| KR9 — counts and admission remain separate | Proposed knowledge14 is K1four + K2two + K3eight. E0bone is separate, making proposed15 if included. Knowledge currently allocated0/consumed0/runnablefalse/paid source empty; no transfer from original remaining14 or runtime25. run_rk1.py:269–271; knowledge real-model-acceptance-plan.md:16–39 |
| KR10 — source inspection is not execution evidence | The matrix's hashes bind only inspected bytes. Future tagged code, task-token CLI reads, quick-create link/fence behavior, source-role revocation and all process/evidence counters require named fake-first proof. A cmd/server test filter still enters DB TestMain; a guarded test-body skip does not make the whole package DB-free. database-test-isolation.md:5–18 |

### Files found

Paths below are repository-relative. Each has a concrete role in the proposed extension.

| File | One-line role |
| --- | --- |
| CLAUDE.md | Authoritative package, migration, fixture and default-safe real-agent test boundaries. |
| .trellis/workflow.md | Research persistence, task/phase and role-context boundaries. |
| .trellis/tasks/11-01-collaboration-knowledge-audit-fixes/{prd,design,implement}.md | #7 audit disposition and existing local verification scope. |
| .trellis/tasks/11-01-collaboration-knowledge-audit-fixes/advance-plan-2026-11-02.md | Shared sole-owner complete check and continuation boundaries. |
| .trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261102/knowledge-closure-readiness.md | Reviewed #6/#7 evidence, precise knowledge/ordinary gaps and independent ledgers. |
| .trellis/tasks/10-30-collaboration-knowledge-promotion/{prd,design,implement}.md | Human authority, execution requirements and retained deterministic G2 boundary. |
| .trellis/tasks/10-30-collaboration-knowledge-promotion/real-model-acceptance-plan.md | Unallocated K1/K2/K3/E0b registration and account/window/source prerequisites. |
| .trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/registration.json | Original scenarios, four/two/per-entry knowledge counts and first-incomplete stop rule. |
| Same runtime directory/runtime-addendum.json | Runtime-only approved25 cap, current ownership/model correction and launch accounting. |
| Same runtime directory/framework-registration.json | Current17-file/launcher digest and closed paid gate/source binding. |
| Same runtime directory/run_rk1.py | Runtime-only scenario/order/budget/source/CLI preflight, never a current knowledge launcher. |
| server/cmd/server/rk1_fixture_test.go | Existing source/target/resource-consent fixture and direct service command setup. |
| server/cmd/server/rk1_scenarios_recall_test.go | Executable eight-runtime-scenario map; no knowledge/E0b entry. |
| server/cmd/server/rk1_scenarios_handoff_test.go | Actual fake-prefix member/continuation construction and handoff source assertions. |
| server/cmd/server/rk1_run_test.go | Real Runner/HostExecutor, unique run ledger/cwd and private logs. |
| server/cmd/server/rk1_provider_test.go | Before-TestMain wrapper/stand-in dispatch, exact input/stdout/argv and finalizer primitives. |
| server/cmd/server/rk1_entry_test.go | Fake/real entry, account gate and old framework hash glob. |
| server/cmd/server/rk1_trial_test.go | Existing sample classification, private/public evidence and measured-versus-seed roles. |
| server/cmd/server/router.go | Exact human, ordinary MDT and Host runtime route producers/consumers. |
| server/cmd/server/collaboration_knowledge_host_composite_test.go | Existing real router/Host proof with test-owned Provider and loss/revocation/persistence controls. |
| server/cmd/server/collaboration_knowledge_correction_composite_test.go | Human v2 pending/approval/correction and next-authorization withdrawal composite. |
| server/internal/service/task_knowledge.go | Server-only ordinary consumer facts, session selection and cumulative exposure history. |
| server/internal/service/task_knowledge_prepare.go | Current disclosure/authorization transactions, exact replay and frozen-history refusal. |
| server/internal/service/collaboration_knowledge_resolve.go | Five surfaces, source/owner/authors/audience eligibility and exact-manifest validation. |
| server/internal/service/collaboration_knowledge_runtime.go | Actual run consumer facts, fresh run namespace, original preparation and current authority. |
| server/internal/service/collaboration_comments_create.go | Public human comment transaction appends admitted command/input with real author. |
| server/internal/service/collaboration_squad.go | Actual runtime member delegation and durable continuation production. |
| server/internal/service/collaboration_knowledge_squad_runtime_test.go | Existing role/namespace/parent/readers/wake/capacity contract, with synthetic service proof explicitly scoped. |
| server/internal/service/task.go and autopilot.go | Genuine ordinary issue, quick-create, Chat and manual/scheduled autopilot originator producers. |
| server/internal/handler/{issue,comment,chat,autopilot}.go | Actual HTTP producer permission, ownership and enqueue boundaries. |
| server/internal/handler/collaboration_comments_create.go | Public root producer body/key/current epoch/target version parsing. |
| server/internal/daemon/knowledge_context.go | Production invocation namespace, MDT-only negotiation and bounded identical-body transport retry. |
| server/internal/daemon/daemon.go | Production StartTask → preparation/input → executeAndDrain/result/fence chain. |
| server/internal/daemon/knowledge_context_test.go | Test-created ordinary executable and production backend setup without installed CLI. |
| server/internal/daemon/knowledge_server_integration_test.go | Existing ordinary HTTP/DB/daemon actual-input/loss/refusal/native reuse composition. |
| server/internal/daemon/collaborationrun/host_knowledge.go | Host private knowledge suffix and per-permit current launch authorization. |
| packages/core/api/client.ts | Client route truth for human comment, create issue, AI quick-create, Chat and autopilot. |
| packages/views/modals/quick-create-issue.tsx | AI generic versus comment-anchored producer and manual mode distinctions. |
| packages/views/chat/components/use-chat-controller.ts | Shared floating/full-page direct Chat send path. |
| server/cmd/multica/cmd_project.go and cmd_issue.go | Actual CLI project-get and inert quick-create issue-create commands; no invented task-show command. |

### Code patterns to reuse

- Compose the real runtime identity, target roster and owner consent: rk1_fixture_test.go:167–209. New knowledge fixtures grant through startup policy and real services; old knowledge-off constructor and current registration stay unchanged.
- Compose ordinary claims/run/report instead of manufacturing task facts: knowledge_server_integration_test.go:281–337. Its minimal Chi router is a deterministic boundary fixture, not a public UI/router proof; the proposed sibling uses the production router for entry coverage.
- Borrow fake prefixes with measured=false and fake mode, not forged nonlaunch/physical proofs: rk1_scenarios_handoff_test.go:98–110.
- Use original UUID/body replay and current reauthorization: ordinary task_knowledge_prepare.go:512–527; Host collaboration_knowledge_runtime.go:416–458; client knowledge intent/recovery rules in the Core spec.
- Capture raw ordinary native system/result observations before lossy public messages and require canonical ACK: execution spec:157–170; daemon knowledge-context claim tests keep this contract independently of business outcome.
- Report real model visibility and source exposure separately from server/CLI availability: rk1_trial_test.go:36–67 and experiment evidence spec:32–38. Counter/source/hash preservation is not semantic understanding.

### External references and versions

- Repository-retained primary vendor URL: https://code.claude.com/docs/en/cli-reference.md, referenced for --no-session-persistence by .trellis/spec/server/collaboration-knowledge-execution.md:210–218 and the C4 vendor provenance record. No external lookup was performed.
- Historical preregistration names Claude CLI2.1.316, claude-opus-5-5, provider-default thinking/tier and macOS arm64; configured gateway mapping and requested model are not observed model evidence. No account/profile/installed CLI was accessed here.
- Existing runtime fake version is2.1.200 (rk1_provider_test.go:31); fake/stand-in behavior is explicitly not vendor/account acceptance.
- No new SDK/API/library/dependency is selected. Proposed implementation borrows existing Go router/service/daemon/Runner and Python/source-manifest contracts.

### Related specs

Read server/collaboration-knowledge.md, server/collaboration-knowledge-execution.md, server/database-test-isolation.md, server/collaboration-context-experiments.md and the Core/Views knowledge specs, in addition to package indexes and root CLAUDE.md. These own source/target authority, exact original intent, namespace/persistence, private evidence, DB isolation and UI scope; this report proposes no spec changes.

## Caveats / Not Found

- No researcher-readable research.jsonl was present. Role-isolated implement.jsonl/check.jsonl were not opened; the coordinator confirmed that requested Markdown and primary sources are sufficient for this slice.
- The new knowledge framework/ordinary-aware wrapper/tagged daemon bridge/paid launcher does not exist in this proposal. Source-supported product wiring does not count as a new executable or passing framework.
- All new file/test names and commands are future implementation recommendations. No tests, Go metadata, migrations, DB/services, browser, Provider/account/installed CLI, download, git, task/status/registration/budget or launcher mutation occurred.
- Current research only writes design.md, entry-matrix.json and findings.md in its assigned directory. Runtime trials, failed/incomplete evidence, shared complete-check resources and other agents' files remain outside write scope.
- Task-token CLI project reads, the genuine AI quick-create issue-link/fence path and exact external-participant revocation are explicitly fake-first prototype gates. A failing gate must be resolved at its owning boundary; never relabel a manual create, use a human/PAT fallback or add a paid probe.
- Real K1 expiry, same-input later physical retry, original/general/native platform/account/release acceptance, comment-source quick-create, schedule/webhook autopilot, channel output, every claim transport and fully real squad prefixes remain separately scoped.
