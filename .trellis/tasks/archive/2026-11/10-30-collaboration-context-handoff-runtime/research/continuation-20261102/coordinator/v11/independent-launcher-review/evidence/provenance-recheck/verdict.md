# Corrected V11 provenance recheck

**Pass with notes for the exact corrected labels and static artifact/source binding. The prior label/RK1 discrepancy is closed.** Corrected binding SHA `6db6435e476b8f448a412d4b097e16e6131ddbf7458450ced1cf69a8f281d80b` and launcher SHA `dd0f5903b8b604f409fd10505645b9cc3920028ea4af56ad0adab8ee5a96063e` match. This result does not clear resource ownership, current-source runtime, SSR, full/password7, paid, wholecheck or task completion.

## Findings (fixed)

- File: Owner-created `owned-bindings-v11-provenance.json` and `run-make-check-v11-provenance.py`.
- Issue: Original preparation conflated standard product SourceID with the dev-environment HEAD/overlay identity.
- Fix: The owner corrected the labels. This recheck verified canonical product `4622182a7e129f1712f883eaea36a4756b183f539b5733eb818b7f8df9195fcb` / 7,962 files separately from dev HEAD/overlay `3f97…` and preserved all-tracked fingerprint `26e3…` / 17,182 files. Actual corrected canonical records and preserved independent records have identical sets, zero hash differences and the same recomputed canonical digest. Only `.provenance_revision`, `.source.checkout_source_id_method`, `.source.identity_purposes` and `.source.product_source` changed in the binding. No product, artifact or original provenance file was edited by this reviewer.

- File: This recheck's `collector.md`.
- Issue: Initial discovery assumed generic artifact field names and old normalized record groups. The preserved observation instead contains `cli_actual` and `desktop_actual` root/count observations.
- Fix: Before its single execution, used the exact corrected binding `desktop[role][installed|staged]` records. Derived renderer `index.html` and preload/Main `index.js` directly from each root. The frozen executed collector and result remain unchanged.

## Findings (not fixed)

No remaining label or static artifact-source blocker was observed for these exact files. The following remain outside this clearance:

- Current canonical `4622…` / Go 1.26.6 / CLI `c008…` fake normal/race `62 + 62`, CLI/runtime and real knowledge/CCG acceptance are pending. Old source `639…` / `9f72…`, framework `5eaea44f…`, Go 1.27 and CLI `b1d54…` results keep their historical attribution. Exact unchanged code/protocol/guard or retained helper/log bytes may support review only within their proven scope.
- Resource/current-process ownership, new SSR/full/password7 and paid gates are pending. Disk equality and recorded offline cases do not establish those product behaviors.
- This narrow pass compared corrected and preserved canonical records; it did not repeat the full source/tracked-snapshot filesystem scan. The owner reported source/dependencies unchanged, and the current artifact files were actually rehashed. The original large collector, source discrepancy and first blocked verdict remain preserved.

## Verification

The owner executed the frozen standard-library collector once at 01:34 UTC, command `python3 evidence/provenance-recheck/collector-executed.py`, exit 0, with no collector errors. Corrected binding/launcher, correction summary and canonical manifest pins passed. Current HEAD remains `304f74dd3ba4ddea862c94917c5bb0770355a351`. Both canonical manifests contain 7,962 identical records and recompute `4622…`.

Actual CLI SHA remains `c008c02377c4c52665000a0620a5a7b3f90f4d10d54d53e1d74bce97dccc6197`. Both installed/staged renderer index files match `9d92714285456d587bb8bcd3ecf2897ad015cf7f63d76f75fea9a541536ae0eb`; preload and Main indices match `68a9036e…` and `99129d62…`. Each of the six installed/staged roots matches every expected relative path, SHA, byte count and mode. Counts are renderer 493 each, preload 1 each and Main 1 each; reads were stable and roots/counts agree with the preserved observation.

| Component | Actual and expected normalized record digest |
| --- | --- |
| Renderer, each side | `b3e36d9985069535a82a01070336956fe8a3e007428f8204a4473377c3b3c4b3` |
| Preload, each side | `2053c77c6cb88a75f537948adc9dd6328286f94bb8bcf2771fc7b2d6e0af10ba` |
| Main, each side | `f83541ac3b0f789c32aba9adf2e574ad14ad21217bf5df32bc2641d46a58c3c8` |

These digests encode sorted path/SHA/bytes/mode tuples. They differ in serialization scope from the declared renderer file-list manifest `bc467544…`; this is expected, not an artifact mismatch.

The separate code recheck has zero blocking findings, report SHA `a859c3615b24d7b747afd28edc4f49d6a50783a2d7fd7b9332df028397555f19`. It verified the recorded 45 distinct offline pass terminals and their source/log hashes without replay. Those cases are metadata/control proof, not repository, runtime, DB, PW or Provider acceptance.

Frozen collector Markdown SHA: `e3c6e7746da687e8ddff0aba8b55f1e0f566f45ab50f390a628ab0e65fc83faf`. Executed code SHA: `55f1c1525d2635f3824d8fb548deddc9f928d041e1a5ca1a45c9422e57369934`. The 37,532-byte result SHA is `03fec7ccf268e49be2e2ab60283f67d399b7525851f22df6a7734e07728ad384`. Original binding `ccc11e4d…` and all preserved reports/observations remained unchanged. No original 60 MB result was read or rerun.

- Lint: not run; read-only scope.
- TypeCheck: not run; read-only scope.
- Repository/runtime tests: not run.
- Product/binary/SDK, Node, DB, PW, Provider and network execution: not performed by this review. The one allowed process was the owner-run standard-library file collector.

The new sole owner of SSR/full/password7 and original14 execution is thread `01a19ee4-40e8-7b70-b15c-5b5a13bd050e`; creator/original provenance was not rewritten. The original series retains remaining cap 14 and has consumed 0 new launches here. Its exact next cursor is **E3 after-arm trial 2**. The separate runtime cap is 25, consumed 0: E0a 1; H1/H2/H2b/H3a/H3b/H5 each 3; H6 three trials × two launches = 6. H2b is its own scene. This recheck changes no budget or gate state and supplies no new paid instruction.

Only this recheck directory received reviewer-authored files: `collector.md`, `verdict.json` and `verdict.md`. The owner-created executed collector/result were preserved. Earlier blocked reports remain the first-round historical record; this new report closes their label finding for the exact corrected files.
