# Server handoff source/authority audit — 2026-10-31

Read-only audit of the current main runtime work in `/Volumes/artisan/code/2026/multica-0.4.37`. HEAD: `e3620721efacb820b89709c8db9d326d9cf2b00b` plus the live, uncommitted files identified in `security-source-audit.json`. No product/test/spec file was changed. No database, provider, lint, type-check or test command was run; this report is code-path evidence, not dynamic acceptance.

The initial 17 target-file hashes were rechecked before this report. Changed files: **none**. Supporting contract files have final hashes in the same manifest. Owners may change them after this snapshot; recheck the named regressions on their final tree.

## Findings requiring owner work

### SSA1 — P2: runtime handoff reports a zero event watermark and omits the open information request

**Anchors:** `server/internal/service/collaboration_runtime.go:177`; `collaboration_runtime_handoff.go:107`, `:140–145`; `collaboration.go:110–114`.

`lockCollaborationRuntimeSource` constructs a partial `CollaborationIssueAccess` without `EventSequence` or `InformationRequestID`. The new runtime projection consumes those exact fields. Consequently runtime live/snapshot responses carry `event_seq:"0"` and skip the `information_request` branch, even after issue events and an active information request exist. The human path uses `ResolveIssueAccess`, which does populate both fields, so the two consumers disagree.

Wiring the missing runtime fields is not the whole fix: the information-request projection itself hardcodes `EventSequence:"0"`. A nested information return can restore an older open parent request while the latest handoff is the separate return row; the exact request's correlated event sequence must be used.

**Reproduction to add:** create a managed active run, create an information request without stopping that run, then read human and runtime handoff. Assert the runtime watermark equals the locked session's nonzero `EventSeq` and that request ID/requester/recipient/source are present. Add a nested request→return case that restores the earlier request and retains its actual event sequence.

**Minimal owner fix:** populate `InformationRequestID` from the locked `issue_collaboration.ActiveInformationRequestID` and `EventSequence` from the locked session. Resolve the correlated event for the exact information request (or use an explicit unknown representation if history is unavailable). Do not impersonate a human principal to fill the runtime object, and do not change execution admission merely because the human handler moved.

This is a confirmed projection/provenance defect, not proof that a revoked run can execute. The parent has accepted the primary finding for the runtime owner; its final fix/tests were not part of this audit.

### SSA2 — P2: handoff cursors can omit the required run binding

**Anchor:** `server/internal/service/collaboration_runtime_handoff.go:35–59`.

`parseHandoffCursor` initializes `c.RunID` with the trusted route run ID and then unmarshals caller JSON into it. A cursor such as `{"kind":"inputs","offset":0}` omits `run_id` but passes the equality check because the default survives decoding. Unknown/duplicate keys are also not rejected by this decoder. This repeats the documented recorded-page anti-pattern of decoding untrusted cursors into trusted defaults.

**Reproduction to add:** call the runtime/human handoff paging entry with an otherwise valid current run and base64url-encoded JSON lacking `run_id`; require `invalid_handoff_cursor`. Cover missing, wrong, duplicate and unknown binding fields, plus a valid generated cursor. Apply kind-specific required fields before lookup.

**Minimal owner fix:** decode a fresh value with explicit required fields, then compare its run ID to the trusted route identity. Preserve the deliberate empty-cursor first-page path separately.

This is a confirmed cursor-integrity gap. The route still authorizes the current run, so this audit does **not** claim that omitting the field exposes another issue. Parent has routed the required-binding fix.

### SSA3 — P2, provenance only: handoff source reads echo unvalidated run/version metadata

**Anchors:** `server/internal/service/collaboration_handoff_snapshot.go:121–123`, `:211–240`; `server/internal/handler/collaboration_runtime_handoff.go:101`; `server/pkg/db/queries/collaboration_handoff.sql:2–3`.

For source kind `handoff`, `readHandoffSource` checks the issue and handoff ID, then returns `Note` without checking `Source.RunID` or `Source.Version`. `RuntimeHandoffSource` checks the body hash and copies the caller's source object into its response. A caller with a valid same-issue handoff reference and correct note hash can therefore supply a fabricated `source_run_id`/`version` and receive those values back inside the apparent source provenance. The canonical handoff projection emits empty run/version for this source kind.

**Reproduction to add:** use a legitimate handoff ID/note hash but add a different valid run UUID and arbitrary version in the source-content query. Require rejection or canonical empty run/version. Keep the ordinary handoff content positive and the existing event-backed run/sequence matching controls.

**Minimal owner fix:** reject attributes that do not apply to handoff sources, or derive the canonical source metadata from the row rather than echoing caller metadata. This does not require broader ACLs or a new storage layer.

This finding concerns false attribution, **not** extra body-read authority: the returned note still comes from the currently authorized issue. Parent was sent this distinction for owner prioritization.

## Boundaries verified by inspection; no confirmed bypass found here

- **Current runtime authority precedes replay.** Snapshot create/replay, source reads and binding all enter `lockRuntimeBusiness` (`collaboration_squad.go:84–105`). It locks the current runtime/source aggregate, matches the exact run/capability hash and authorized attempt, rejects revoked capabilities, then checks current group/dependency/grant/author admission and exact reservation. Snapshot or binding IDs alone do not bypass that gate.
- **Binding uses actual current attempt identity.** `BindRuntimeHandoff` checks `AuthorizedAttemptSeq`, the stored attempt nonce, full saved run identity and exact per-nonce binding replay before acceptance. It restores/revalidates the source set before even an identical binding replay is returned.
- **Purge and runtime operations share source locks.** Runtime source locking takes the issue `FOR NO KEY UPDATE`, issue-collaboration and session locks (`collaboration_runtime.go:159–170`; `collaboration.sql:56–60,162–163`). Human memory purge takes the same issue/collaboration/session locks before version mutation (`collaboration_memory_operation.go:111–124`). Admin purge enters its lifecycle lock path, including the same issue/aggregate/session locks (`admin_collaboration.go:316–338`). The version-row lookup alone is not being treated as the entire race fence.
- **Frozen metadata does not copy purgeable source bodies.** Snapshot persistence clears `Envelope.Sources[*].Content`; memory is version/hash metadata and input/result/transfer fields hold IDs/state, not their body text. Restore rejects a purged memory row or mismatched original source hash. This is not a claim that purging memory removes quotations from separate output/checkpoint/provider retention domains.
- **Immutable delegation is deliberately independent of current comment edits.** `immutableHandoffDelegation` reads the original parent-run `comment:<id>` event and checks issue, kind, comment identity and payload hash. Editing a live comment does not rewrite already-admitted work. That is the intended behavior, not a stale-comment bypass.
- **Cross-issue source content stays scoped.** Handoff notes and event sources are selected using the authenticated run's issue ID. Event-backed source reads compare actual source run/sequence to the reference; delegation additionally checks the current member's exact admitted delegation. Legitimate prior-result references may point to another run in the same collaboration issue; rejecting every different source run would break the stated handoff feature.
- **Live comment content has stronger revision/cursor checks.** `RuntimeCommentContent` (`collaboration_comments_recall.go:297–320`) rechecks business authority, issue/workspace/comment identity, deletion and exact current revision. Content cursors (`:323–329`, `collaboration_recall.go:156–166`) require run, issue, comment, revision and positive offset in a fresh decoded value. This is a useful pattern for SSA2.
- **Reader meaning remains conservative.** Handoff metadata starts from `run.PromptSnapshot`; author IDs come from that frozen input. Reader enumeration reuses `collaborationInputReaders`, including the original consumer and conservative legacy readers. The returned IDs mean possible prompt receipt, not model comprehension. Pagination advances through group-run IDs and labels further reader pages; no reader was relabeled as completed by this code.
- **Wake causes retain source keys.** Comment and joined-input producers assign stable keys to the queued continuation; proof producers persist pending metadata and the drain assigns it under aggregate locks. `queueContinuation` reuses only unclaimed `prepared` work, so later producers cannot edit a claimed continuation's cause list via that reuse path. The snapshot stores its captured cause projection; live refresh is distinct. Legacy continuation completeness remains `unknown` rather than claiming all historical causes were reconstructed.
- **Human reads use human authority.** `RunHandoff` uses the existing repeatable-read, read-only `readCollaborationContent` transaction and an issue-scoped recorded-run query. It does not borrow a business token or daemon owner identity. This audit does not promise cancellation of already-authorized in-flight HTTP reads after a concurrent revoke.

## Dynamic verification still needed on final owner code

No tests were executed in this read-only lane. Existing new tests were inspected, including immutable delegation/snapshot replay, Stop refusal, merged wake causes and memory-purge snapshot refusal. They are useful but are not substitutes for the following assertions:

1. SSA1/SSA2/SSA3 regressions above, using real service/handler entrypoints and exact response metadata.
2. Snapshot replay and identical binding replay after purge/Stop/revoke in both controlled transaction orders; ensure refusal carries no old body and no new binding is written after authority loss.
3. Wrong-issue source refs and wrong run/sequence event refs; current comment content after edit/delete/revoke; immutable admitted delegation after a harmless live edit.
4. Reader pagination beyond 32 group runs and claimed-versus-late wake causes with actual group locks; legacy missing provenance must stay explicit.
5. Recheck current source hashes after owner fixes, then use the coordinator's final integrated lint/type-check/DB/native/real-provider gates as applicable.

The combined checkpoint recovery/seal regression belongs to another reviewer and was intentionally not duplicated. The server envelope's worst-case JSON budget issue was already assigned by the parent and was not re-reported as a new finding here.

## Verification record

- Source files modified: none.
- Audit artifacts written: `security-source-audit.md`, `security-source-audit.json`.
- Hash recheck: complete for the initial target set; no drift in that set during this audit.
- Lint/type-check/tests/DB/provider calls: not run, per this bounded read-only review scope.
- Overall: **not an approval of the full runtime**. Two confirmed projection/cursor fixes and one source-metadata normalization issue are routed to the appropriate owner; no confirmed cross-issue content or purged-body resurrection bypass was found in the inspected paths.
