# Blocking ETA UTC range — private R04 checkpoint, 2026-11-18

**BK-R03-01: corrected and verified in pure Go; not integrated or accepted end to end.** New ETAs must still satisfy the original local timestamp grammar and microsecond precision; after applying the offset, the UTC year must also be 1–9999 inclusive. Retained finite year-one midnight is a real instant, not missing data. A partial pair, either pg infinity, or a finite instant outside the UTC range projects `unavailable` with `information_metadata_invalid` and no condition or ETA. Both fields absent still projects `legacy`. The projection does not backfill rows. No digest, history, serializer, schema or final database-clock logic changed.

## Canonical RED → GREEN

Used the handed-off test draft, extending its controls before running it. RED used byte-identical R03 production in the private R04 tree; GREEN used the same test bytes, command and environment. Go 1.27.0 on darwin/arm64 ran `go test -json -count=1 -timeout=90s` with 222 build-selected production service files and **only** `collaboration_information_metadata_test.go`; no package `TestMain` or DB fixture. The runner denied network access, used private scratch and read existing dependencies offline.

| Top-level test | Subtests | RED failures (excluding top) |
| --- | ---: | ---: |
| `TestCollaborationInformationMetadataPresenceAndPrecision` | 5 | 0 |
| `TestCollaborationInformationTextUsesDecodedUTF8Bytes` | 0 | 0 |
| `TestCollaborationInformationEstimateUTCWireRange` | 7 | 2 |
| `TestCollaborationInformationBlockRetainedUTCRange` | 15 | 8 |

RED exited 1: **19 passed, 12 failed** (10 failing subtests plus their two parents). The estimate failures were `offset_below_min` and `offset_above_max`. The retained failures were `utc_min`, `offset_to_min`, `below_min`, `above_max`, `offset_below_min`, `offset_above_max`, `positive_infinity_with_time`, and `negative_infinity_with_time`. GREEN exited 0: **31/31 named passes** (4 tops + 27 subtests), no skips, duplicates or missing names. Exact first/last UTC microseconds, offset crossings in both directions, legacy omission, unset ETA, infinities and precision controls passed. Scoped `go vet` exited 0; `gofmt -l` returned no paths.

## Source and evidence

Evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep`. No newer blocking owner record was found before writing. All 5,077 R03 paths, bytes and modes matched the frozen manifest before and after. R04 retains 5,075 unchanged files; these are the only two changes under `source-block-work-r04/server/internal/service/` (no adds, deletions or mode changes):

| File | R04 SHA-256 |
| --- | --- |
| `collaboration_information_state.go` | `c0a833283691d75c6913cb83db090f38172c5f7e3881771cc962e4d4dff756fb` |
| `collaboration_information_metadata_test.go` | `bbf589ca15dff29665ab0dd3d28dce55fbf91a4c197d60d565a2c2e05de8588f` |

Under `verification/block-utc-range-r04/`, `inventory.json` lists every full test name and terminal event with hashes of raw outputs and commands. `source-seal.json` binds the original and final manifests, the two-file delta and patch. Full manifest SHA-256: R03 `538e4f92de2707285d285a98238260edf2acc3930512056b2f560ba01db6ad36`; R04 `4ce478ca64019a8e1169a0c199c53c4638e43d4b23d5a876b2150b3c6d2fbb68`.

**Limits:** No full package or race suite, database, API/service, application build, Web/Desktop/browser or provider execution. The lower-bound tests establish representability, not permission to write an ETA in the past. BK-R03-02 (raw query handling) was not changed. No production ROOT, Git, shared task/plan or matrix updates. Integration, independent review and full backend/S3 acceptance remain with the parent.
