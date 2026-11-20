# Existing maintenance verification — 2026-11-20

**PASS for the bounded maintenance controls.** The existing leader scratch correction, read-only product-analyst policy, protected-file checks and contained Git controls agree with the preserved maintenance evidence. All **50 Python tests** and **11 named Go results** passed. The latter count includes four top-level tests and seven subtests, including their parent; it is not eleven independent scenarios.

This review does not admit or continue C1/D1. The prior C1 continuation implementation was rejected by cybersecurity tool screening. No retry, rewritten request, alternate agent route, continuation feature, real smoke, native model request, supplier/account access or database operation occurred in this audit. Historical C1 remains incomplete and D1 remains without real-model acceptance. Parent AC9/AC10 are unchanged by these results.

## Scope and identities

- Reviewed checkout: `/Volumes/artisan/code/2026/mc-secrem-v2-audit-20261120`, HEAD `4c78c8b98958cb8a76f1c9a448b56e94285c36b3` (the complete observed identity is also retained in the private comparison JSON).
- Assigned repository write: this report only. Production files, task scripts, tests and fixtures were not edited. Other task-document edits belong to the main session.
- Private output root, abbreviated `AUDIT` below: `/Volumes/artisan/code/2026/secrem-v2-resume-audit-20261120T065505Z/maintenance`, mode `0700`.
- Historical input root, abbreviated `HISTORY`: `/Volumes/artisan/code/2026/secrem-v2-cli-evidence-20261120`.
- Read the task PRD/design/implementation, `check.jsonl` sources, current CLI review/maintenance/checkpoint sections, root AGENTS/CLAUDE, and server task-capability, database/test-state isolation, context-experiment and built-in-template specs. Applied the Trellis check workflow without recursive delegation.

The main session explicitly extended temporary write ownership for the twelve existing `CodePolicy`/`ContainedGit` tests: their unchanged fixtures hardcode unique `code-policy-test-*` and `contained-git-test-*` directories under `HISTORY`. Only the directories created by those tests were eligible for their normal cleanup; no pre-existing historical file was writable scope. The root's top-level names were identical before and after, and no new fixture directory remains. The historical fixture parent was additionally confirmed outside Git after execution; that check is recorded as post-run evidence, not a pre-run assertion.

## Maintenance code-path review

| Contract | Source and observed result | Verdict |
|---|---|---|
| Leader does not claim to verify unmounted tests | `codex_exec_code_container.py:188` hashes actual `AGENTS.md`, then returns `protected_tests_status="not_mounted"` and `protected_tests_unchanged=null` for `leader_scratch`. It does not access the project test paths. The original protected map remains in metadata. The nonempty-unmounted-map regression passed. | PASS |
| Prepared scratch does not rewrite actual claim counts | `leader_prepared_manifest` returns a new dictionary with zero prepared resource counts; the original manifest is retained for metadata. Preconditions require actual leader identity, ignored-project declaration, zero prepared repos and no repository. Existing regression passed. | PASS |
| Ignored project paths cannot alias or overlap scratch | The wrapper first canonicalizes actual cwd through `common.rooted`. Each ignored source must be an existing absolute canonical directory; equality, ancestor and descendant overlap are rejected at `codex_exec_code_container.py:171`. Existing missing/relative/same/ancestor/descendant negative cases passed; canonical alias refusal was also checked in source. | PASS |
| Analyst has the actual role and read-only filesystem | `code_policy` binds `analysis` to `product-analyst`, rejects a leader and validates the exact full clone/HEAD/tests. `prepare` requires `read_only_code=true`, mounts cwd read-only and adds no writable tests or Git mount for this mode. Only existing comments/transport scratch and session state are writable. Post-check compares HEAD with the baseline. | PASS |
| Protected tests remain unchanged | Before execution, protected paths must be canonical files under `tests` with registered SHA-256. Implementer mounts the test subtree read-only; verifier protects each prior test while permitting new test files. Nonleader post-checks use bounded regular-file reads and preserve `checked/true` only on equality. Existing changed-test/role/path/bundle tests passed. | PASS |
| Model-owned Git never executes on host | Wrapper `git_read` and code-harness `secRemGit` call `contained_git_role_handoff.py`. Inspection/export/import run with `--pull=never`, `--network=none`, read-only rootfs, dropped capabilities, nonroot user, exact clone and explicit artifact mounts. Initial host Git is limited to newly constructed synthetic fixtures. Post-run HEAD uses nofollow/nonblocking bounded reads, not host Git. Symlink/FIFO/shared-clone/write-command and fsmonitor/include controls passed. | PASS |
| No new product/template contract | These are task-local transport/evidence policies. No production source, template text, registry/version, authority grant or `.trellis/spec/` content changed. This audit supplies no new product convention requiring spec synchronization. | PASS |

The analyst transport policy is not a completed analyst-to-leader continuation. The existing code-flow harness was inspected for maintenance call sites only and was neither executed nor extended.

## Preserved no-supplier evidence

Compared current sources with `research/cli-code-maintenance-20261120.json`, each original `result.json`, its separately retained transport `metadata.json`, and the listed source hashes. All comparisons passed. Old checkout paths were mapped to the same repository-relative files in the current checkout; evidence-local synthetic responder files were hashed in place.

| Preserved result | Exact SHA-256 | Findings |
|---|---|---|
| `HISTORY/code-leader-r2/result.json` | `167a3fc7ca061a4e7497b838c5a29548a8b5754a658f4672d077b6e945927cfe` | Completed synthetic CLI transport; nine synthetic requests, zero supplier requests; nonempty protected map retained, status `not_mounted`, value `null`; actual resource count 1 and prepared repo count 0; AGENTS unchanged. |
| `HISTORY/code-analysis-r1/analysis/result.json` | `a67433109878aa204733679de22f90b8e64780fc2accc318ec6058e58208208f` | Completed synthetic CLI transport; six synthetic requests, zero supplier requests; code/new-test/baseline-test/AGENTS writes denied; Git commit attempt failed on read-only storage and HEAD unchanged; protected tests `checked/true`. |

The historical tool outputs show a live synthetic peer control, denied shell network access and rejection of permission escalation. The analyst record preserves the actual task/workspace read identity. These are retained historical observations; this audit did not rerun the native CLI probes. They are not supplier-backed model behavior evidence.

Current wrapper SHA-256: `141b7154fe6aea9ee0660a45fed102052751866de036d70297995f9c0c60dec5`. Contained Git launcher SHA-256: `8f938ed796a43093da177e5f657234a9ef666af79ca0c6334eab8f7ab269de66`.

All 409 file/link identity records beneath the two assigned historical probe directories were unchanged before/after, including regular-file bytes and modes. Their matching inventory SHA-256 is `08e1ebbad1d8d31d483ed51f74959323faa0a5633744acc58aee8aa21410788d`. This is a bounded comparison of these probes, not a claim to have audited every historical database or real-model artifact.

## Executed checks and environment

Commands below ran from the reviewed checkout, with an explicit allowlisted child environment rather than inherited account settings. Exact argv, cwd, timestamps, exit status and raw-log hashes are stored in `AUDIT/logs/*.command.json`.

`AUDIT/synthetic-environment.json` sets private `HOME`, `CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `XDG_*`, `TMPDIR/TMP/TEMP`, `GOTMPDIR`, `GOPATH`, `GOCACHE`, `GOMODCACHE` and Docker config under `AUDIT/state`. Git identity and Codex config are synthetic. `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, `GOENV=off`, `GOFLAGS=-mod=readonly` and `GOTELEMETRY=off`; no database URL, account token or real execution opt-in is inherited. Tests that exercise opt-in validation use their existing synthetic private fixtures and restore those test-local values.

Before execution, canonical `AUDIT`, state, HOME, temp and compiler-temp roots each returned nonzero/empty stdout from `git -C <path> rev-parse --show-toplevel`; the same boundary passed afterward. The ordinary test guard remained active. Pure Go tests create explicit shell fakes; no installed agent executable was invoked.

Toolchain: Python 3.14.3, Go 1.27.0 darwin/arm64, Git 2.52.0. Docker image inspection confirmed the existing cached Linux/arm64 image `sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f`. Existing contained tests use its immutable ID with `--pull=never`; no image pull/build or shared Docker restart occurred.

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- python3 -B -m unittest discover \
  -s .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts \
  -p 'test_*.py' -v

python3 -B .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/prepare_direct_cli_overlay.py \
  --repo /Volumes/artisan/code/2026/mc-secrem-v2-audit-20261120 \
  --out /Volumes/artisan/code/2026/secrem-v2-resume-audit-20261120T065505Z/maintenance/overlay/generated

bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test \
  -overlay /Volumes/artisan/code/2026/secrem-v2-resume-audit-20261120T065505Z/maintenance/overlay/generated/overlay.json \
  -tags=agentintegration ./pkg/agent -run '^TestSecurityDirectCLI' -count=1 -json -timeout=2m

bash scripts/go-test-with-agent-cli-guard.sh -- go -C server vet \
  -overlay /Volumes/artisan/code/2026/secrem-v2-resume-audit-20261120T065505Z/maintenance/overlay/generated/overlay.json \
  -tags=agentintegration ./pkg/agent

gofmt -l .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/direct_cli_backend.go \
  .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/direct_cli_backend_test.go
git diff --check
```

The fresh overlay was generated from this checkout; no historical absolute overlay mapping was reused. Only `./pkg/agent` was executed. Although the existing generator also lists server-harness virtual files, neither `./cmd/server` nor any handler/database package was selected. All virtual `zz_secrem_*` paths remain absent on disk. The generated factory copy, mapping and provenance are retained as evidence outside Git.

| Check | Result |
|---|---|
| Python inventory | PASS: 50/50, 0 FAIL, 0 SKIP, 15.747s test time. Exact names match both source inventory and `retained-final-checkpoint-controls.txt`. Files contribute 6 context-copy + 8 code-policy + 6 exec + 4 contained-Git + 6 bundle + 8 spool + 12 runtime tests. |
| Direct-backend Go contracts | PASS: 11 named RUN/PASS, 0 FAIL, 0 SKIP, no invalid JSON; four top-level tests plus seven native-contract subtests. Entire command 8.908s. |
| Go type/compile checking | PASS for selected tagged `pkg/agent` package and fresh overlay as part of the test/vet commands. |
| Static/lint checks | PASS: explicit `go vet`, clean `gofmt -l`, AST parse of all 29 task Python files without bytecode writes, and `git diff --check`. |
| Full repository lint/TS typecheck/`make check` | SKIP: report-only audit, no TS or product implementation change, no installed `node_modules` in this checkout, and database/full-platform checks are outside the assigned permit. No full-repository green claim. |
| Source identity | PASS: 65 script/fixture/guard/module input files unchanged in bytes and modes; private full byte copies retained. Before/after inventory SHA-256 `fa7f459b46e848590a1d5740de86ca9554a2a78941949b5ed4e900a0f9af5584`. |

Private cache preparation initially failed: `cp -cR` could not clone across volumes (`EXDEV`), then ordinary copying encountered `EACCES` on readonly directories left in the partial owned module cache. These were setup failures, not test failures. Their command receipts/raw outputs remain. Only that newly created private cache received owner-write permission; ordinary `cp -R` then succeeded. The shared source caches were never changed, and no dependency fetch was enabled. The actual test/vet commands ran once each and passed.

## Evidence hashes

Paths are relative to `AUDIT`.

| Artifact | SHA-256 |
|---|---|
| `logs/python-controls.log` | `2c4ff34eb4254469979e4e29de5586285cb9af20408ff9a8fa9f49a3c16eb3db` |
| `logs/direct-backend-contracts.jsonl` | `b60e1ccc65b50a07c78cb1ded8793d73d55d0112ca474c18e9fb0c198c6cdf0b` |
| `logs/go-vet.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty successful output) |
| `historical-maintenance-comparison.json` | `8da01ab3ade2963f25714e80e1bae8a09c64e8fb9a9e9774a16b11eeb5488aa3` |
| `python-inventory-comparison.json` | `8eb9dbcb34c382b2e1eb0d496c0194849082bbed38bced93eb0697e44cdd8183` |
| `go-contract-inventory.json` | `defd29b1da927d36054590f72d7cb057aa2444032ed1f8eb8fd92edbe75ad695` |
| `synthetic-environment.json` | `30f0b379486987bb50da1603aa5cf6b91ba280b5d2bb7bb0444d8ab29e812b7d` |
| `overlay/generated/overlay.json` | `879d4094eaac7073056d8f88633a4987693d420c91e7701567ebfb7d389b4223` |
| `overlay/generated/overlay-provenance.json` | `23e0b77d25957f6e4f77d2f57d7e650a6df405fee677ce45ab9c5e2fd69c2823` |
| `residue-check.json` | `5a689de0a50c30e80ea8836c87edeb0a602c1eb6d238807e50f768120862fb8a` |

## Findings (fixed)

No implementation defect was found or fixed. No simplification, template update, test rewrite or boundary relaxation was made. The only recovery was the private cache-copy setup described above.

## Findings (not fixed)

No new maintenance defect requires a code change. The existing tests' fixed historical fixture parent limits portability; the main session authorized their exact temporary writes for this run, and changing those tests is outside this audit's ownership. Complete C1 continuation, real D1 execution, new admission and the historical source-retention gaps remain outside this review. None is resolved by passing maintenance controls.

## Cleanup

The test-created directories under `HISTORY` were removed by their own fixture cleanup; the historical root inventory is unchanged. Both before/after task-label Docker inventories are empty. No shared container, database, network, image or history file was deleted.

The main session requested removal of newly created private execution state after preserving source/config bytes, commands and raw results. The exact owned paths, sizes and final removal status are recorded in `AUDIT/cleanup.json`. The generated overlay source/mapping/provenance and all raw logs, source snapshots, synthetic config snapshots and inventory/hash reports remain as evidence. The audit report itself remains the only file written by this reviewer in the checkout.

Cleanup completed: **3,333,867,595 bytes** of newly created state across **11 exact child paths** removed; `AUDIT/state` is absent and residue is empty. Cleanup record SHA-256: `b84e88f1eadaeb41f1313e17569d5b67cfb7004cbaf1aa5fee3d3cdd1ff7d26f`.
