# Independent F3 invalidation review — 2026-11-01

Verdict: **changes required**. One P1 credential-freshness finding and one P2 final-send envelope finding remain. This is a bounded review of F3, not a verdict on the unfinished A2/C2 batches or whole-task acceptance.

## Reviewed snapshot and evidence

- Worktree: `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101`.
- HEAD: `bc9c3f10b70119d07e28aa167f38f9d335e39338`.
- All 22 entries in `invalidation-implementation-2026-11-01/owned-files.txt` exactly match that directory's `source-manifest.json`; no missing/extra paths or SHA-256 mismatches were observed.
- Read the PRD, design §5/§8.1–8.2 and F1–F4 acceptance, CLAUDE.md, server security/realtime/source-editing guidance, hook inventory, implementation, relevant original call sites, tests and recorded check results.
- Recorded evidence reports 26 top-level new tests / 32 test-and-subtest passes in both focused and race rounds, 195 related passes, and successful vet/build/diff checks. Inventories show no missing expected tests, failures or skips in those final rounds. These are producer-run results; this reviewer ran **no tests, database operations, verification scripts or Provider calls**.
- B1's 30-second visibility lease / 15-second reread is a missed-hint fallback. It does not establish the server's current credential authority.

## P1 — A revoked legacy PAT can still receive knowledge hints through the cached final-send check

Primary changed references: `server/internal/realtime/collaboration.go:110`; `server/internal/service/collaboration_knowledge_invalidation.go:301`; `server/internal/handler/personal_access_token.go:342`.

`writePump` calls `c.authorized()` before the new predicate (`hub.go:1196`), but legacy PAT authentication reaches `patResolver.ResolveToken`, which returns cached authority without checking PostgreSQL (`router.go:2778`). Its cache can live for ten minutes (`auth/pat_cache.go:19`). `AuthorizeKnowledgeWorkspace` then checks the trusted human classification, active user and current member row, but receives no original PAT identity and cannot tell that this particular credential was revoked. An active account and membership do not restore a revoked credential's authority.

This has a concrete race even when Redis is healthy: a resolver reads the valid PAT at `router.go:2782`; the revoke transaction commits and deletes the cache entry at `personal_access_token.go:342–348`; the earlier resolver then writes its stale result at `router.go:2793`. A queued or later exact three-field hint passes both checks and reaches `WriteMessage` (`hub.go:1202`) despite the already-committed revocation. Failed cache eviction or a successful revoke commit with a lost acknowledgement also leave this path open. The new all-sources scan can itself produce the next hint during that window.

Minimal fix: give the knowledge-only final-send path the original trusted credential identity and validate its current authority directly against the authentication database, including PAT `revoked`/expiry/user binding. Preserve original JWT/password-session checks and existing unrelated realtime behavior. Do not redesign the general PAT cache or treat an invalidation attempt/lease as authorization proof.

Required regression: hold an actual knowledge frame before send, commit PAT revocation, and keep the legacy resolver returning the old cached user (also cover stale cache fill or failed eviction). The knowledge frame must be dropped while the account and workspace member remain active. Include a valid PAT control and current password/JWT checks. The recorded WebSocket test uses a JWT and a fake boolean scope authorizer (`realtime/collaboration_knowledge_test.go:44–102`), so it does not cover this failure.

## P2 — Final-send validation forwards unchecked knowledge envelope fields

Primary changed reference: `server/internal/realtime/collaboration.go:95–111`; forwarding site: `server/internal/realtime/hub.go:1202`.

The predicate unmarshals the envelope into a permissive `{Type, Payload}` struct and validates only its last decoded payload. The original bytes are then sent. For example, an authorized workspace client permits this frame, including the private outer field:

```json
{"type":"collaboration:knowledge_changed","payload":{"target_kind":"workspace","target_id":"b736820a-476b-4e30-bbbf-e8f046c54562","knowledge_revision":"1"},"source_id":"PRIVATE_SOURCE"}
```

The relay's `injectEventID` preserves such extra fields (`redis_relay.go:630–643`). With an existing `event_id`, it preserves original bytes, so duplicate `payload` keys can also hide an earlier private object from the predicate. Envelope decode errors currently return `true` as well (`collaboration.go:100–101`). Thus strict payload decoding does not enforce a content-free final frame.

Minimal fix: for knowledge frames, accept only the sanctioned envelope (`type`, exact validated three-field `payload`, optional valid relay `event_id`) and reject untrusted extra/duplicate fields, or construct the bytes sent from only those validated values. Known malformed knowledge frames must fail closed. Keep ordinary event behavior separate. Extend the existing predicate/socket regression with an outer private field and duplicate payload plus `event_id`; a valid retained deleted-project hint must still pass.

The actual F3 drainer currently constructs a safe typed envelope. This finding concerns the explicitly required final-send malformed/private-frame boundary, not a claim that that producer currently creates private fields.

## Other reviewed contracts

No additional P1/P2 issue was found in these bounded paths:

- Purge/tombstone/participant/member/account hooks append only a source scan in their authoritative transaction; scan insertion has no target/source lookup trigger or foreign key. Ordinary retained comment edits do not invalidate source availability; the tombstone does.
- Source scan routing includes only immutable ever-published history, has no current-source/member join, and freezes its upper bound before target work. New approvals independently create direct hints.
- Worker transactions lock own job → workspace/project `KEY SHARE NOWAIT` → independent counter `UPDATE NOWAIT`; they do not lock source/candidate/task rows. Busy targets retain cursors, missing live counters fail, and cause-target dedupe plus counter/hint/cursor commit together.
- Target delete appends final routing/hints and removes independent counters after parent fences. Existing source collaboration deletion refusals remain; old scans process absent parents without recreating counters.
- Relay uses stable `hint-id:target`, publishes before the delivered marker, and retains retry identity through rollback/restart/unknown commit. Safe payloads have exactly the three approved fields.
- Membership role/removal/grant and account/password hooks retain their transaction ownership; observed membership/PAT broadcast or cache effects occur after commit. No additional source/target lock inversion was identified.
- Account-disabled and membership-loss checks use current database rows; a target-only administrator does not need source Read. Deleted project delivery uses retained trusted workspace routing. Those positive properties do not close P1.

Integration should keep this F3 slice open until both findings have a focused fix and updated hash-bound evidence. No product/generated files or commits were changed by this reviewer; only this report and its JSON companion were written.
