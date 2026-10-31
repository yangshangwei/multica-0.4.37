# Final candidate 3 recheck — 2026-10-31 UTC

**No unresolved defects found in the reviewed scope.** All original findings and the follow-up case-alias P2 are resolved on candidate 3, source tree `5b0262606d143ca44fc2ab19247eece7025087cda2541372f67afc218b76245b`. This verdict is a source review and focused verification result. The fresh full backend gate and rebuilt production Web/native Desktop acceptance remain pending.

## Findings (fixed)

- `server/pkg/protocol/collaboration_host.go`: canonical typed-field validation now runs before decoding. It checks decoded names against the actual JSON tags, rejects unknown/case-variant/duplicate names, recurses through typed struct and array children, and flattens embedded metadata fields. Arbitrary map/interface content and literal strings are preserved. Concrete Inputs/Results/Causes decoders still own their required page facts; embedded page methods cannot consume and hide their siblings.
- The original `"truncated":true,"TRUNCATED":false` probe now rejects, along with missing/null source flags, absent empty-page facts and malformed retention. Valid truncated material still decodes as truncated. A standalone rerun of the retained original protocol probe is captured in `final-review-round3.json`; it accessed no database, server, installed CLI or Provider.
- `collaboration_handoff_decode_test.go` checks raw field ordering, mixed case, escaped aliases/duplicates, optional aliases, flattened page facts and typed children without their own decoders. Its positives preserve escaped canonical names, literal JSON-looking source text and case-distinct keys in arbitrary content maps. These tests verify the boundary rather than merely its helper implementation.
- `host_handoff_wire_test.go` now includes the exact case-alias, escaped-case and escaped-duplicate member responses. It starts from deliberately truncated delegation content and checks zero process starts, zero bindings, one original snapshot request, and unchanged not-started termination/proof identity. The fixture appends the conflicting key after the canonical key, so JSON sorting cannot accidentally make the test a false positive.
- Exact owner evidence: `handoff-decoder-fix/canonical-fields/summary.json` and `owned.patch`. The manifest records the intentional red runs, then **31 protocol tests**, **23 affected daemon tests**, and **33 selected race tests**, all passing with zero skips; package vet, formatting and diff checks also pass. The reported source hashes match candidate 3. These are affected-scope results, not a substitute for the full backend gate.
- Previously reviewed fixes remain intact: the sender's permanent local-staging failure latch; original checkpoint/proof recovery scheduling; required first seal sequence in the E2E fixture; strict required fields and retention validation; the single-kind `wakeup_assigned` exception to preparation start evidence; and retained negative/leak/paging assertions for record projection.
- The revised comment wording continues to match the authoritative PRD: ordinary discussion uses live authorized revision-bound recall/content, while startup material includes accepted inputs, memory, handoff/request notes, immutable delegation and result sources. No unimplemented automatic comment preload or frozen-comment replay guarantee is claimed.

## Findings (not fixed)

None in the reviewed scope. The outstanding full backend and rebuilt browser gates are verification work, not unresolved findings from this review.

## Verification

- Lint / TypeCheck: final coordinated gate status remains with the parent; this recheck did not duplicate them or infer a final pass from earlier snapshots.
- TypeScript tests: `final-frontend/tests-metadata.json` records `pnpm test --force --concurrency=1 -- --maxWorkers=2`, exit 0, 400.779 seconds, zero source changes. `final-frontend/test-summary.json` records **12,982 tests across 1,054 files**, all five tasks freshly executed with zero cache hits.
- Focused protocol recheck: pass for the intended accept/reject matrix; original valid truncated data remains valid and all reproduced malformed variants reject. Full details are in `final-review-round3.json`.
- Source freeze: all **7,973** files match `final-candidate/source-round3.json` at both entry and completion. HEAD remains `e3620721efacb820b89709c8db9d326d9cf2b00b`; zero source differences. Only review report/evidence files changed in this lane.
- Full backend round 2 and rebuilt production Web/native Desktop proof: pending at this review. No claim of real-model understanding, rollout readiness or AC1–AC10 completion is made.

The earlier report and red evidence below are preserved as history. Their unresolved labels describe their original snapshots; the current verdict is the candidate 3 recheck above.

---

# Recheck of integrated fixes — 2026-10-31

Current verdict: the original checkpoint-gap and E2E-seal findings are resolved in source. The original missing/null handoff fields and invalid-retention cases now reject correctly. One related P2 remains: case-variant duplicate fields can override completeness and reach an actual launch. Fresh full gates and rebuilt Web/Desktop acceptance remain pending.

## Findings (fixed)

- `server/internal/daemon/collaborationrun/host_checkpoints.go`: the sender-local `stagingFailed` latch stops both later staging and batch growth after an irrecoverable local rejection. HTTP uncertainty still retains exact staged payloads. The new canonical producer/sender test covers invalid first batch, accepted or uncertain prefix before an invalid middle batch, and an entirely valid suffix after a lost ACK. It checks original ordinals, outcome non-reintroduction, honest tail metadata, Stop and proof. The affected source hashes match `recording-fix/followup-no-gap/result.md` and its recorded 20 race / 95 daemon passing tests. Production retained-run scheduling and proof priority are unchanged from the independently traced path in the initial review.
- `e2e/fixtures/collaboration-runtime-handoff.ts`: the seed now supplies `sequence: 1` to its first seal. All other fixture assertions and seeded-history disclaimers are preserved. This resolves the source defect; production Web/native reruns are still required.
- `server/pkg/protocol/collaboration_handoff.go` and `collaboration_checkpoint.go`: missing/null completeness fields are required by concrete page/source decoders, and retention now has shared validation. The embedded `CollaborationHandoffPage` intentionally has no `UnmarshalJSON`, so its concrete Inputs/Results/Causes decoders retain items and cause completeness rather than consuming the outer page through an embedded method. Valid empty/populated round trips and explicit false/zero/nullable values are preserved. Original review probes now reject missing/null truncation, missing empty-page facts and invalid retention. The reported 28 protocol / 23 daemon passes match the fix manifest; actual malformed-member HTTP tests verify no bind/start and the original not-started proof.
- `server/pkg/db/queries/collaboration_preparation.sql` and generated counterpart: the start-evidence query excludes only server-authored `wakeup_assigned` metadata. Termination, attempt, business capability and every other event retain the conservative nonlaunch barrier. Tests retain the original healthy-root progress checks and add exact no-duplicate continuation/refusal/cause assertions, plus unknown/display/binding negatives. The recorded 236 service / 6 handler checks are affected-scope evidence, not a fresh complete gate.
- `server/internal/service/collaboration_records_test.go`: the changed unknown-event expectation matches the approved explicit display allowlist. It still checks identical display records, no private payload/kind leakage, advancing pagination, unchanged not-recorded coverage and absent telemetry. Malformed supported display kinds retain negative coverage. This is an expectation correction, not a production parser relaxation.
- Task design wording: the updated ordinary-comment contract matches PRD R4/AC6's authorized revision-bound recall and AC3's mandatory startup material. It makes no frozen-comment preload/replay/edit-blocking claim. The earlier design conflation is resolved; this review identifies no explicit unmet AC from that engineering scope clarification.

## Findings (not fixed)

### P2 — Case-variant fields still override mandatory completeness

- Files: `server/pkg/protocol/collaboration_handoff.go:289` (source decoder), shared `server/pkg/protocol/collaboration_host.go:78`, and daemon `server/internal/daemon/collaborationrun/ledger.go:520`.
- `encoding/json` matches typed field names without case sensitivity. Both duplicate checks use the raw key string, so a source containing `"truncated":true,"TRUNCATED":false` has its required canonical field present but decodes `Truncated=false`. The noncanonical field is not rejected by `DisallowUnknownFields`.
- The protocol recheck confirms that exact payload is accepted while the original missing/null cases reject. A temporary overlay on `newLaunchHandoffFixture` returned this wire for a deliberately shortened member delegation. The actual Runner reported `starts=1 bindings=1 protocol_error=false`; the required no-launch regression exited 1. Full source, command and output are preserved in `final-review-recheck.json` under `case_alias_launch_probe`. Only the existing test-built fake executable/local HTTP were used; no database, installed CLI or real Provider was accessed.
- This continues the original P2's whole-delegation failure mode. Core's canonical `truncated:true` survives unknown-field stripping, while Go overwrites it through the alias, producing contradictory declared-v1 interpretations.
- Smallest fix: reject case-variant field aliases/duplicates for typed protocol objects before decoding, and add a raw member-response regression. Keep arbitrary user-content maps outside that restriction. Source remains with the coordinator during the freeze.

## Verification

- No product/test source edits or full-suite/DB/browser/real-Provider runs by this recheck.
- Lint and TypeCheck: fresh complete coordinator gates pending; earlier passing gate results remain tied to their own recorded source.
- Focused evidence: original malformed protocol cases now reject; the additional case-alias launch regression fails as reported. Fix-owner focused/race evidence was inspected, not rerun wholesale.
- Review source fingerprint: `fa88791a17a9041cf889b6c31736bebbc41269e2fd2cd0464ab3028666b43587`, covering 7,973 original-manifest plus current untracked product/test files. `final-review-recheck.json` records before/after hashes using its explicit hash definition. All source hashes remained unchanged during this recheck.
- Prior evidence and the original report remain below unchanged as historical findings; the current status is the recheck above.

---

# Final integrated runtime review

Review target: `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`, HEAD `e3620721efacb820b89709c8db9d326d9cf2b00b`, final-candidate source tree `e18295d0c892811870fb2f8d42572751288e26061f311ec157942553e4b6736b`. All 7,969 source-manifest entries matched during review. Product code and tests were frozen; this reviewer changed only this report and `final-review-probes.json`.

Read the check manifest, PRD, design, implementation plan, CLAUDE and applicable contracts, prior integration/fix evidence, tracked diff from the stated HEAD, and untracked runtime/protocol/service/handler/CLI/Core/Views/E2E implementation. This is a code/spec review, not a substitute for the coordinator's full gates or real-model acceptance.

## Findings (fixed)

None. Source edits were explicitly frozen during concurrent gates. The three actionable defects below were sent to the coordinator as they were confirmed.

## Findings (not fixed)

### P1 — A locally rejected checkpoint batch lets later messages become a false retained prefix

- File: `server/internal/daemon/collaborationrun/host_checkpoints.go:125` (failure branch at 129; dequeue/flush loop at 152).
- Trigger: one Provider tool event has a nonempty tool name longer than the protocol's 128-byte limit, and later messages are already queued. `hostRecording.message` admits this event; `stageHostCheckpoint` rejects its batch before persistence. `flush` clears that batch and sets `failed`/`closed`, but subsequent queued batches still call `stageHostCheckpoint` before the `failed` check. Their offsets are calculated from the last persisted chunk, omitting the rejected batch.
- Proof: a temporary Go overlay exercised the real `hostRecording.message`, sender, and existing test-owned ledger/permit fixtures. With the invalid event in the first 16-message batch and another 16 messages already queued, it persisted `sequence=1 offset=0 count=16 first=original-ordinal-16`. The regression exited 1. Exact test source, command, output and scope are in `final-review-probes.json` under `checkpoint_gap`. The later payload remains eligible for ordinary recovery, so this is more than an inaccurate local counter.
- Contract: design §2 requires closing append at the first irrecoverable gap and retaining only the continuous prefix. Rebasing later messages hides the missing prefix and gives them false stable ordinals.
- Smallest fix: distinguish an unrecoverable local staging failure from an uncertain HTTP receipt, and prevent further staging after the former while preserving the already staged prefix and independent proof. Alternatively, validate a typed message before offering it and close the stream on an invalid message. Add the producer/sender regression beside the checkpoint tests, including an already accepted prefix; keep transport-replay behavior intact.
- Left unchanged because the coordinator owns source during final gates.

### P1 — Both new browser journeys seed an invalid recording seal

- File: `e2e/fixtures/collaboration-runtime-handoff.ts:92`.
- Issue: `attempt` at line 64 contains no `sequence`. The checkpoint explicitly supplies `sequence: 1`, but the seal only spreads `attempt` and adds observation/ACK/reason fields. Its decoded seal sequence is therefore zero.
- Proof: `ValidateCollaborationRecordingSeal` requires a sequence in 1–32, and `readCollaborationRecording` also requires the first seal to have sequence 1. Both `seedRuntimeHandoff` journeys immediately GET handoff/records at lines 103–104, whose builders call that reader. They fail during setup before browser assertions. This is directly established by the final fixture and decoder code; no DB/browser run was duplicated.
- Smallest fix: add `sequence: 1` to the seeded seal, retain the explicit seeded-history disclaimer, then run both production Web and native Desktop journeys on the corrected candidate. Do not weaken the seal validator.
- Left unchanged because test source was included in the freeze.

### P2 — Declared v1 handoff data can omit required completeness facts and still launch

- File: `server/pkg/protocol/collaboration_handoff.go:183`, including `ValidateCollaborationHandoff`; consumer: `server/internal/daemon/collaborationrun/host_handoff.go:108`.
- Issue: the decoder disallows unknown fields but does not require nested zero/false fields. Missing or null source `truncated` becomes false; an empty page can omit `total`, `returned` and `truncated` and become a complete empty page. The optional `recording` object is never validated at all.
- Proof: a temporary `go run` importing the production protocol returned nil for (a) omitted or null source `truncated`, (b) omitted empty-page `total/returned/truncated`, and (c) a recording with version 999, a non-UUID stream, negative retained/observed values and arbitrary reason. A second captured probe used a truncated member delegation: removing/nulling its flag changed `source_truncated=true` to `false` without a decode error. Exact source/output is in `final-review-probes.json` under `handoff_decoder`. The member startup gate specifically accepts a source when `!source.Truncated`; absent completeness evidence thus satisfies its whole-delegation condition. The daemon's duplicate/unknown-key pass does not enforce required fields. Core already rejects missing source/page flags, so the declared v1 consumers disagree.
- Contract: R5/AC8 and startup-handoff validation require malformed supported responses to fail before launch, without fabricated completeness. This finding does not allege a bypass of the server's current source authority locks.
- Smallest fix: reuse the existing required-field decoding pattern for the v1 envelope/nested objects (preserving intentionally optional fields), validate retention through a shared protocol helper, and add missing/null completeness plus invalid-retention tests. Include a daemon test showing a missing delegation flag prevents binding and process start.
- Left unchanged because the public protocol/source owner must integrate the fix and rerun its affected gates.

## Reviewed boundaries

- Production restart scheduling reaches recording work: `runBatchPoller` calls `recoverProofs`; `BeginProofRecovery` consumes the fully validated `Retained` inventory; acknowledged host runs remain retained when recording metadata/chunks need recovery; the no-Host recovery runner routes the persisted v2 record into `recoverHost`. It does not need a new claim or Provider instance.
- Proof recovery has priority for an already sealed proof. Original checkpoint replay gets one bounded telemetry context, and errors are joined only after the independent termination/outcome/proof path. Seal staging/replay retains exact issued attempt/nonce and a shared monotonic protocol transition. The rejected-batch defect above is separate from that repaired scheduler/proof wiring.
- New content goes through current business admission and reservation checks under the existing run/group locks. Checkpoint exact-key ACK is content-free; seal uses the termination credential and issued-attempt validation. Reader retention is derived from committed messages, not the sender ACK. Public record kinds stay unchanged and stream/ordinal IDs stay stable; internal events are filtered from the public record iterator.
- Snapshot storage strips source bodies. Replay/binding reconstructs source text using issue/run/event identity and original hash, checks immutable admitted delegation, and rejects a purged memory base. Authorization precedes binding; binding precedes nonce burn/process start. Provider retry and outer fallback retain the same logical snapshot, while each actual Provider launch binds its own attempt. The Codex continuity notice is included in the prehashed application text.
- SSA1 uses the locked session event sequence and current request ID. The original request event is selected by that request's handoff ID, preserving nested return provenance; legacy missing links remain explicitly unknown. SSA3 rejects handoff sources carrying noncanonical run/version attribution before loading text.
- Handoff cursors require a nonempty matching run and reject duplicate/noncanonical field names. The material/wire split counts 256 KiB of metadata/framing plus decoded source text and separately caps raw JSON at 2 MiB. Required delegation remains whole; optional source prefixes retain hashes and truncation markers. Core uses original, pre-normalization metadata for this accounting.
- Comment recall retains the legacy newest-200 array path. The new page path counts serialized metadata/escaping/newline, advances past an oversized first placeholder and leaves an unselected later row for the next page. Runtime content cursors bind run/issue/comment/revision and UTF-8 offsets; every read rechecks authority and refuses deletion/stale revision. Current author capability remains `data_only`.
- Core reads remain under Server/viewer/access fencing, budget is in comment query identity, denial removes private caches, and record chains reject changed retention snapshots. Shared views restore same-revision placeholder text before editing. Handler display uses the issue projection only when stable user IDs match.
- Web fragment changes use the platform History API only for the same origin and rendered pathname/search; changed path/query/origin still use the router. Shared AppLink retains modifier/target handling. The E2E assertions distinguish seeded record projection from ingestion and keep Web/native evidence separate.

## Spec alignment note

The current startup envelope contains transfer/request notes, immutable delegation, memory references, run results and wakeup metadata. Ordinary mutable comments remain explicit runtime recall; they are not frozen startup sources. The task design still discusses a comments first page in handoff and rejecting launch after a snapshotted ordinary comment changes. The coordinator was asked to reconcile this description with the accepted scope; this reviewer did not silently widen the protocol or infer a new product decision. This observation is distinct from the three reproducible defects above.

## Verification

- Lint: not rerun by this review; the coordinator owns the final repository lint gate. Earlier `frontend-fix/verification.json` and `full-static-initial/summary.json` record successful runs on their own snapshots and are not substituted for final-candidate evidence.
- TypeCheck: same coordination/limitation as lint.
- Tests: targeted protocol acceptance probe completed; the new checkpoint-gap regression failed as described above. No long suite, DB, browser or Provider call was duplicated. `final-review-probes.json` preserves the focused evidence; full backend/frontend gates remain separate.
- The first checkpoint-gap run's useful nonzero output was hidden by a tool-wrapper diagnostic. It was not treated as an environment failure. The next identical run captured output to the report-owned JSON file, establishing the actual assertion failure in 0.421 seconds of package time.
- No runtime reconfiguration, commits, product/test edits or simplification pass were performed. Remaining risks are the listed defects and still-outstanding coordinated gates; this review does not mark AC1–AC10 complete.
- Final source audit: HEAD unchanged; every one of the 7,969 manifest files still matches `final-candidate/source.json`; zero differences. The exact audit timestamp and result are stored in `final-review-probes.json` under `source_verification`.
