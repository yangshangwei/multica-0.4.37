# Independent F3 correction disposition — 2026-11-01

Verdict: **F3-R1/P1 and F3-R2/P2 resolved at the reviewed hashes**. No residual P1/P2 finding was identified in this correction scope. The original review remains preserved in `invalidation-review-2026-11-01.md/json`.

## Snapshot and scope

- Worktree: `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101`; HEAD `bc9c3f10b70119d07e28aa167f38f9d335e39338`.
- Independently compared current SHA-256 values with `invalidation-implementation-2026-11-01/corrections/source-manifest.json`: all **6 correction-owned files**, **25 full F3 files**, and **4 listed dependencies** match. The correction allowlist matches exactly; the other **19 original F3 files are byte-identical** to the original manifest.
- Reviewed the corrected realtime implementation, original-credential binding, production authentication-database wiring, new/updated tests, and retained red/green/race results. This recheck does not audit A2's human knowledge API, middleware or service authorization work; its private PAT-hash field was inspected only as the stated F3 dependency.

## F3-R1/P1 — resolved

`HandleWebSocket` binds `authorizeKnowledge` to that socket's original authenticated token and subject (`server/internal/realtime/hub.go:997`). No supported inbound message replaces the token; a reconnect constructs a new closure and context. The raw token remains private to the existing connection closures, and the added trusted `PATTokenHash` has `json:"-"`; the correction introduces no token/frame logging.

After current human/account/workspace authorization, `outboundKnowledgeFrame` requires and invokes that callback in a bounded context immediately before returning bytes to `WriteMessage` (`collaboration.go:117`, `:125`, `hub.go:1208`). Missing validation capability or a failed query drops knowledge. `validateKnowledgeCredential` directly queries PostgreSQL by the original token hash in legacy PAT mode and checks subject, revocation and expiry (`collaboration.go:150–164`). It does not consult PATCache or rely on eviction. Production router setup installs the database handle in either authentication mode (`router.go:410–412`). Password mode retains `CheckPasswordToken` and its current version checks; legacy JWT validation retains cryptographic/subject checks (`collaboration.go:140–169`). Existing scope authorization still checks current human identity, account and membership.

The retained socket regression uses real PostgreSQL and Redis PATCache, holds the actual frame after scope authorization, changes the credential, then fills the cache with stale authority while the account/member remain active (`cmd/server/collaboration_knowledge_realtime_test.go:119–228`). Revoked, expired and changed-subject credentials are denied; an ordinary marker still follows the existing path, and a newly connected valid PAT succeeds. JWT/password-JWT/password-PAT controls and password-version denial are covered separately. The retained pre-fix socket run failed the revoke and expiry cases; final focused and race runs pass.

## F3-R2/P2 — resolved

`canonicalKnowledgeFrame` parses all outer keys before selecting the contract, detects a knowledge type even when a duplicate/aliased type would otherwise disguise it, and rejects duplicate or unknown outer fields (`collaboration_knowledge_envelope.go:20–68`). The allowed envelope is exactly:

```json
{"type":"collaboration:knowledge_changed","payload":{"target_kind":"workspace|project","target_id":"canonical UUID","knowledge_revision":"decimal string"},"event_id":"optional canonical nonzero UUID:target"}
```

The existing strict hint decoder validates the three payload fields. Optional `event_id` is checked for canonical nonzero UUID plus `:target`; malformed/trailing JSON and invalid UTF-8 fail with a fixed, content-free error. The function marshals fresh per-send bytes (`:69–93`), and `writePump` sends those bytes rather than the original buffer (`hub.go:1208–1212`). A duplicate payload therefore cannot preserve hidden earlier text, and another subscriber's shared source buffer is not mutated. Valid unrelated object envelopes retain their original bytes and bypass knowledge credential validation.

The correction directly covers the originally reported outer `source_id` and duplicate private `payload` with an existing `event_id`, plus duplicate type, aliases, malformed/trailing JSON, invalid UTF-8 and invalid IDs. Four adversarial cases fail in the retained pre-fix actual-socket run and pass after correction. Positive cases preserve exact public payloads, retained deleted-project routing, stable relay identity and unrelated frames.

## Evidence and limits

Read and cross-checked retained JSONL results against `corrections/test-inventory.json` and `final-checks.json`: **9 affected top-level tests / 32 owned test-and-subtest outcomes passed**, including the corresponding race runs. Full realtime reports **85 passes** and related server checks **23 passes**; final runs contain no failures or skips. The server race run contains 14 total outcomes, of which 13 belong to these corrections; the additional production-route case is not counted as an F3-owned test. Vet, build, formatting and diff checks are recorded as passing. Initial helper/stub compile failures are retained and are not treated as product red evidence.

This reviewer performed only file/artifact inspection and hash/JSON checks; no tests, verification scripts, database operations, Provider calls, product edits, commits or subagents were run. Existing scanner/hook evidence is preserved by the unchanged-file hashes. This closes these two F3 findings only; it does not establish A2/C2 correctness, live Web/Desktop consumption, Provider/session acceptance, or whole-task completion. The Core finite cache lease remains fallback behavior, not authorization evidence.
