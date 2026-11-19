# Blocking backend R03 source review and deferred R04 handoff

Reviewer: `/root/s3_check_batch_block`. Active task: `11-13-pay-241-s3-recovery`. Decision: **CHANGES_REQUESTED_SOURCE_REVIEW; full backend/S3 not accepted**. Existing coordinator R03 package evidence is retained; this review does not replace its final database/process audit or matching API/Web/Desktop proof.

## Findings (fixed)

No server production fix was made in this review. Three client contract defects were independently fixed and verified in separate immutable increments: R02 adds the actual inactive reason and exact sealed semantic-503 recovery; R03 fixes explicit repair entry/readiness while generic server capabilities stay false. Their complete reports, REDs, source manifests, Core/Views tests and static checks remain unchanged.

## Findings (not fixed)

### BK-R03-01 — zoned ETA can leave the four-digit UTC wire range

`collaboration_information_state.go:37–53` checks parsed local year before UTC normalization but does not restrict normalized UTC to year1..9999. For example, `9999-12-31T23:59:59-00:01` normalizes to year10000. It can pass future admission; retained `informationBlock:87–101` marks it complete and `informationTime:75–76` emits a five-digit year that the frozen Core ISO timestamp parser rejects. The lower offset crossing can likewise produce UTC year0. This is source evidence; no API/database reproduction was performed.

Main accepted a narrow R04 correction: normalize/check new UTC year1..9999; project retained out-of-range metadata unavailable without backfill, history/hash changes or metadata prose. Exact finite year1 midnight is Go's zero time, so a future implementation must preserve the legitimate lower bound while explicitly denying pgtype positive/negative Infinity. Canonical pure tests should cover both offset crossings, exact limits, retained unavailable variants and unchanged old omission/precision behavior.

Not fixed because the latest user scope transferred backend coordination and requested preserving the current work. R04 production remains byte-identical to R03. An unexecuted canonical metadata test draft is retained in the admitted private R04 source; no RED, GREEN, compile, vet, database, API or build result is claimed for it.

### BK-R03-02 — malformed raw query pairs can bypass closed query validation

`internal/handler/collaboration_information.go:160–181` validates `r.URL.Query()` and exact detail at `:208` tests its length. Go's URL.Query discards malformed percent-encoded pairs. Therefore raw `?limit=%ZZ` or `?cursor=%ZZ` can become a default first page; `?ignored=%ZZ` can appear to be no query on exact detail. The authority-first object check is preserved, but the canonical/closed query contract is not fully enforced.

Recommend checking `url.ParseQuery(r.URL.RawQuery)` errors after current object authority and before collection/affected/exact-detail decoding, with a canonical handler regression. Creation GET uses the same URL.Query pattern and should be considered if its closed-query scope is included. No such fix or body execution was made: this is outside the two-file R04 UTC admission and belongs to the new coordinator's bounded ownership decision.

## Verified source paths

- Human authority is resolved before private query parsing; PAT validity, password version, active account and actual source/participant read basis are rechecked. Source-only nullable epochs remain genuine; membership identity/role and verified credential facts are fingerprinted.
- New history readers use a five-second deadline, an authorized RR snapshot and a fresh final current-authority/revision/lifecycle fence. This is source compliance, not real slow-storage or runtime-reader acceptance.
- History cursors use the actual cached validated JWT key, a private HMAC domain, canonical payload/MAC/base64url/tuple checks and viewer/root/projection/revision/lifecycle/authority/repair correlation. A real restarted key-rotation journey still needs execution evidence.
- Complete JSON paging reserves the actual next cursor, counts metadata/escaping/newline against65536bytes, refuses an oversized first item and preserves all legacy/unavailable IDs and exact decimal repair denominator. SQL orders actual microsecond(created_at,id), protects upper/last tuples and does not filter corrupted affected rows from counts.
- Repair is one existing workflow transaction: original key before fresh admission; exact pointer/context/basis/status/creator/pending checks; explicit handler/status/iteration fact, immutable superseded links/count, event and receipt; no automatic B2, Resume, command/run or approval consumption. Required dependency/commit failures propagate; a semantic unavailable pending refusal can seal503, which the corrected client now recovers exactly.
- Metadata pair/raw presence, original ETA string digest, UTC microsecond persistence and final database-clock savepoint checks are present. Old omitted parameters remain omitted. Creation checks the actual inherited effective project under its existing project fence before counters/issue effects, with current policy observation restricted to GET/hash-independent data.
- Runtime-v1 live/new snapshots may mark unavailable structured metadata. Recovered stored snapshots return through restoreStoredHandoff before rebuilding; original note/source/hash/JSONB bytes remain unchanged. The actual runtime test uses a synthetic persisted specimen and service permit; it does not prove a real model, installed Main, output-upload support or kernel containment.

## Verification

Go reviewer lint/typecheck/tests: **NOT_RUN** in this server source-only lane. Coordinator's R03 functional summary records102 named normal/race executions (service43+handler3+migration5 per mode) and8 retained pure names; full raw reconciliation/final acceptance remains with coordination. All5077 frozen R03 file hashes/modes were independently verified unchanged during R04 initialization and final partial audit.

Client verification remains independently green in the separate reports: R02 Core171; R03 Core178/Views94; both package typechecks and exact scoped lint0, with all REDs preserved. No matrix, task status, archive, ROOT, Git, database or runtime action was changed by this backend review.

## Handoff

The latest user update restricts the current execution lane to the existing Extras-A R05 owner and transfers broader coordination to session `01a1ee9f-bdfc-7f31-ac53-9d7e072a20d7`. R04 admission was written before the source directory existed. Before that latest steering, this reviewer initialized the designated private R04 path from the exact5077-file admitted R03 manifest, then changed only metadata_test.go with a pending pure regression draft. Production state.go remains unchanged. A temporary parent message failed with `agent thread limit reached`; it and initialization/read-lookup errors are retained. No R04 execution took place, and work stopped at the new scope update.

Reuse the partial R04 source and `independent-check/block-server-r04/handoff-partial-r04.json` after verifying its binding; do not overwrite the pending draft with another baseline copy. Review/run its tests only after new-coordinator source/ownership delivery. Backend full acceptance, real HMAC restart/read-fence journeys, current compiled clients, Extras-A/B, S4 and S5 remain open. Full S3 must not be archived from this report.
