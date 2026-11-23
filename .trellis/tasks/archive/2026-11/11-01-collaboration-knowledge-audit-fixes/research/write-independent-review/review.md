# Independent write lane review

Verdict: **pass-with-notes**, no blocking defect introduced by the L1–L5 repair batch or the non-expiry NoRows follow-up. Reviewed product `d4bc37a86e36f7db517c451a4db1eba5b56973b4` against `4e3b61500ed0afbc12cf4c79b1c6aea866778d0e`. Reviewer `/root/knowledge_web_review` is independent of the implementers, including the root-authored NoRows correction. Product files, other writers' evidence and all databases stayed untouched. Only this directory was written.

## Assessment

- **L1 and d4bc37a86:** The database clock decides approval expiry before advancing the target or superseding a prior contribution. If expiry occurs during the final UPDATE, only a second database-time check proving expiry produces the retryable conflict. The outer transaction rolls back every target/correction write before retry. An unexpired or expiry-free NoRows retains404 and is not repeatedly retried. The two injected NoRows cases and the real mid-transaction expiry test match these paths. A remaining retry can seal409; exhaustion remains503 knowledge_busy, with no partial commit. Current spec wording accurately preserves that bound.
- **L2:** Only SQLSTATE23514 with the candidate table and the exact pinned expiry CHECK name maps to400 invalid_knowledge_request. Unrelated CHECKs stay server errors. The canonical DB test verifies that the named constraint actually expresses expiry after proposed_at. A refused candidate insertion cannot seal a successful operation or retain target changes.
- **L3:** All queued-frame credential/scope/knowledge authorization precedes the physical write deadline. Normal text, close, ping and the direct access-changed hint each set a fresh deadline immediately before WriteMessage. Uncached final-send authorization was preserved.
- **L4:** Failed selected hints roll back first, record their attempt through a fresh bounded parent context when the worker remains alive, and are excluded for the rest of that pass. Later hints can progress; three consecutive failures bound relay-outage work. Stable hint `<UUID>:target` identity and marker-after-publication preserve retry/deduplication behavior. Shutdown does not extend work beyond the parent context.
- **L5:** FOR SHARE is retained because it conflicts with revocation's non-key UPDATE. The short250ms wait handles asynchronous last-used updates without permitting revocation to pass concurrently. The previous lock_timeout is restored after a live or absent row; transaction errors roll back the local setting. The retained tests cover both revocation orders, bounded waiting and setting restoration.

## Independent checks

Current-source guarded fake-authorizer loopback realtime regressions under race,count2: **8 named passes,0 failures/skips**. Current service/realtime/generated packages **build and vet exit0**. `git diff --check` passes. No DB-backed test or migration was run by this reviewer.

The source-bound coordinator service run reports3262 passes,18 explicit skips,0 failures; targeted DB race16 passes; migration package536 passes. The completed log hashes and the L1-NoRows/L5 red-green records match their manifests. These are retained coordinator/author results, not independent DB reruns. See `author-evidence-check.json`, `source-binding.json`, per-command records and `review.json` for exact commands, source/file hashes and evidence boundaries.

## Nonblocking known note

**IW-1 — active-target budget still uses application time.** The approval-expiry decision is now on DB time, but the active-target budget query still receives `knowledgeNow()`/application time at collaboration_knowledge.go:599. This matches preexisting behavior rather than a new regression. An application clock ahead of the database can exclude still-active expiring contributions from the count; a lagging one can count already-expired rows. No DB skew reproduction or budget fix is claimed here. Retain the existing owner decision separately; do not describe the L1 expiry tests as proving every budget-clock boundary.

Final combined-source and platform/model acceptance remain coordinator gates. This review applied no product fixes and does not close those gates.
