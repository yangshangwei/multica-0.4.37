# S2 server 2a follow-up — independent recheck

- Reviewer: `/root/s2_server_check`, 2026-11-14.
- Verdict: **follow-up remains open**. C01 and V01 are resolved in the reviewed code. C02's digest projection and sequential denial are fixed, but its new A5 source-membership requirement lacks the live membership fence. The initially incomplete test-fixture manifest has been repaired with a separately verified supplement.
- Reviewed inputs: the coordinator-frozen 11-file follow-up, six log/exit pairs, the original 4,864-file server snapshot, and the separately frozen 826–828 migrations needed to check the disclosed 827 registry entry. All supplied hashes matched. No live 2b files were borrowed to repair an incomplete reconstruction.
- Reviewer writes: these two reports and private verification artifacts only. No product, test, contract, database, runtime, git index or commit writes.

## Findings (fixed in the implementation)

### S2A-C01 — S2 migration retry hooks

All 16 original S2 concurrent indexes now have the correct `concurrentIndexCleanups` entries. The new scoped invariant reads actual migration files and verifies the existing up-hook map. It does not replace or weaken the original global test.

Independent checks on the assembled frozen source passed both `TestStageConcurrentIndexCleanupInventory` and `TestConcurrentIndexCleanupsMatchTheirMigrations`. `TestEveryConcurrentUpBuildHasCleanup` still fails with exactly the eight previously known `report_snapshot` omissions, and no S2 omission. The 827 hook matches the separately frozen 2b migration; checking that name does not constitute 2b acceptance. Actual interrupted-index execution was not rerun by this reviewer.

### V01 — original authority expiry at final A9 consumption

The original verification concern is now backed by genuine red evidence: after the initial signer/dependency checks, the DBTX clock seam advances time; both `original_approval` and `original_issuer` previously advanced successfully (`err=nil`, two late clock hits). The caller-expiry branch already refused and serves as a useful control.

`checkStageFinalAuthority` now runs after artifact observation. It rechecks original decision lifecycle/role/policy, the original signer and source member identity, and each retained credential; it independently checks the caller. Its last database-clock result bounds the decision, original credential, caller and CI immediately before the stage update. No intervening evidence observer remains after that final time check. The deterministic regression verifies all three refusals and that the stage stays at `code_review`.

The retained normal/race logs show the new regression passing. This is a deterministic production-DBTX clock seam, not a wall-clock stress result or a final-completion gate proof.

### S2A-C02 — protected R1 digest and sequential cached selection

The source-denied R1 child now serializes `content_digest:null` explicitly while retaining title/identifier. Source readers retain the existing hex64 digest. The new A5 check rejects a nonempty cached selection when the transaction's access read already reports source membership lost. The first red log demonstrates both the original digest disclosure and the previously accepted cached selection; the green regression covers their sequential correction.

The remaining transaction race below prevents closing the full C02 write boundary.

## Findings (not fixed)

### S2A-F01 — A5 does not fence its newly required source membership (P1)

**Frozen locations:** `server/internal/service/collaboration_stage.go:45`, `collaboration_stage_actions.go:656`, and the original `collaboration_workflow.go:483`.

The follow-up makes current source membership necessary for a nonempty A5 selection, but `collaborationStageSourceMemberKind` still excludes `stageKindConfirmScope`. Therefore the workflow never takes `LockArtifactMembers` for A5. The new check reads `t.access.SourceMember` from an earlier `GetCollaborationAccess` snapshot; later credential checks validate JWT/PAT/password authority, not source membership.

The production remover `handler/workspace_revoke.go` can delete that member independently: it holds a compatible workspace key-share lock, does not lock the user's row, and ends with the canonical `DeleteMember` plus a knowledge-scan insert. An A5 transaction can consequently retain its earlier positive membership observation while revocation commits before its scope decision. Existing root/user/child locks do not fence this member row.

**Required fix and verification:** include the source-gated A5 action in the existing live member-lock path, retaining NOWAIT/retry and the current read after locks. Add the second-connection regression with membership deletion between observation and scope recording, covering both commit orders and preserving the cached-selection negative case. The analogous new 2b subject-creation gate must use its own correct current-authority fence, but its implementation is outside this review.

**State:** open/routed to the same server owner. This is a concrete missing-lock/code-path finding; no database race was executed by this reviewer, and no later repair is claimed here.

## Repaired verification provenance

### E01 — three required fixture changes were omitted from the original manifest

The 11 supplied files do not reconstruct the code that produced the green 90-test log. Applying exactly those files to the verified original snapshot makes independent `go vet` fail to compile test code because the newly nullable `ContentDigest` is still consumed as `string` in:

- `server/internal/service/collaboration_stage_admission_test.go`: three uses;
- `server/internal/service/collaboration_stage_enqueue_test.go`: one use;
- `server/internal/service/collaboration_stage_test.go`: one use.

These files were not included in the original follow-up manifest. This did **not** establish that the active workspace was broken. The owner subsequently supplied `server-increment-2a-followup-fixture-supplement.json` and frozen copies. All three hashes match. Comparison with the verified full original snapshot shows only five substitutions of `stageReadableDigest(t, child)` for direct string use; no assertion or behavior was weakened. The owner's original partial-snapshot hash omissions remain explicit; the reviewer separately recorded the available full-snapshot hashes.

After applying only this certified supplement, independent `go vet` passed for service, handler and migrate. The initial compile failure remains at `vet.log`; the successful repeat is `vet-supplement-1.log`. **E01 is resolved by the supplemental provenance and successful reconstruction**, without changing the original report or pretending its 11-file list was complete.

## Verification

| Check | Result |
| --- | --- |
| Supplied input hashes | PASS: 11 original follow-up files, three certified fixture supplements, six logs and direct exit sidecars, 4,864 original server files, and six supplemental migration files. |
| Formatting | PASS: all 11 reported Go files; empty `gofmt -l` output. |
| Independent migration invariants | Two named PASS, one named FAIL, zero SKIP. The sole failure retains the eight unrelated F2 omissions; zero S2 omissions. |
| Independent production build | PASS on the frozen service, handler and migrate packages; empty output. |
| Independent Go vet/type analysis | Initial reconstruction FAIL retained for E01; PASS with empty output after applying its three verified fixture supplements. |
| Implementation normal/race/HTTP evidence | Hashes and named inventories verified: service 90 PASS, targeted service race 5 PASS, handler 10 PASS, scoped cleanup 1 PASS; zero skips in these green runs. These are the implementation's executions, not independent DB reruns. |
| Red evidence | Preserved: cleanup one failure; source disclosure/cached selection failure; original approval and issuer expiry failures; caller-expiry passing control. |

Private verification lives in `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-2a-followup-check/`. The JSON companion records exact commands, inputs, failures and hashes. The supplemented verification tree has no live API or database configuration. No database-backed tests, migrations, browser, Desktop, model calls or external integration were executed by this reviewer.

C01/V01/E01 closure is limited to this follow-up. C02/F01 requires subsequent source/transaction evidence. Policy activation, full 2b–2d and A01–A12 remain outside this report.
