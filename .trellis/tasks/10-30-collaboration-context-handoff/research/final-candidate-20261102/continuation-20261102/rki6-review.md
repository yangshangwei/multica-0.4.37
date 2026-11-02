# RKI6 independent actual-result review

**Decision: pass-with-notes. Blocking findings open: 0.**

This is the independent result review requested by the active knowledge-audit continuation. It verifies retained final639 fake normal/race execution and its unchanged binding to the prepared fourth candidate. No Go suite was rerun; no source, adapter, task status, launcher clearance, database, service, container, browser or Provider was mutated or accessed for execution.

## Findings (fixed)

None. No source fixes were required or permitted. Only `rki6-review.md`, `rki6-review.json` and ignored private reconstruction outputs were written.

## Findings (not fixed)

No blocking finding or required missing artifact. The following evidence limits remain explicit:

- **RKI6-N1:** Test passes include registered negative and manual cases. Per set, 22 public trials contain 11 `finished/pass`, 4 `finished/fail`, 1 `manual_review_required`, 1 `blocking_review`, 1 `fail_not_visible`, and 4 incomplete samples. Their states, original numbering and stop behavior match the unchanged case table in `server/cmd/server/rk1_entry_test.go:42`. H6 remains manual; simulated answers do not establish real-model understanding.
- **RKI6-N2:** Each set retains 3 deliberately stopped seeds with `authority_lost` and no business outcome. Six large-message runs keep `incomplete` seals, null observed watermarks and `recording_incomplete=true`; all six ACK every raw recordable message and contain an over-limit message. This matches `server/cmd/server/rk1_lifecycle_test.go:161`; it does not turn a partial record into a complete one.
- **RKI6-N3:** Historical E0a at `4b508179f3e6197f170a99acd4df42d567e7ba1b` remains `incomplete/measured:handoff_or_recording_event_missing` with zero seal HTTP requests. Its exact public/trace/private-ledger hashes still match `recovery-preflight-20261102.json`. Current quiet passes do not repair that sample or resolve its prior-load/reconciliation limitation.
- **RKI6-N4:** `server/cmd/server/rk1_fixture_test.go:139` deliberately removes the linked simulator executable after each trial. The four simulated managed runs per set point to the exact test-owned simulator path; retained plans, launch metadata, sim-env records and frozen assertions establish dispatch and environment stripping. No post-run executable hash is asserted. The three separate `TestRK1EvidenceFaultProof` cases passed their named assertions, but their temporary raw trial outputs are outside the retained 22-trial/32-launch inventory.
- **RKI6-N5:** Effective configuration and local toolchain are recorded by the retained preflight/permission/summary, not independently re-observed historical environment or executable-byte attestations. Go1.27.0 darwin/arm64 with `GOMAXPROCS=2`, local cached modules, `GOTOOLCHAIN=local`, and the original 40m cap is a local result. The real entrypoint is not selected; no real-model, paid-entry, full-check, CI, native-platform, release or default-database recovery acceptance follows.

## Verification

- **Lint:** not run; source changes and suite execution were excluded from this evidence-review scope.
- **TypeCheck:** not run for the same reason.
- **Tests:** retained normal and race each independently reconstruct **62 unique named runs / 62 passes / 0 failures / 0 skips**. Each has one separate package PASS and recorded process exit0. The named totals include parent/subtest identities; 62 is not a count of independent business trials.
- **Evidence reconstruction:** pass, zero failed correspondence checks. All 44 public/trace pairs, 64 run ledgers and launch captures were inspected. Exact proof, outcome, recording-seal and physical-termination payloads match their private ledgers. Actual stdin prompt hashes/envelopes match public hashes and the traced handoff binding/snapshot. Raw CLI reads, tool-result/recordable/size/cursor counters and synthetic model observations match public projection.
- **Guards:** the selected regex excludes `TestRK1RealProvider`; the copied-provider missing-marker test passes its exit26/no-output assertion before TestMain. The source-bound Agent CLI wrapper fails on any sentinel invocation, and the recorded command exits0 with no guard rejection. All retained trial modes are fake or test-owned sim; no installed Provider executable is selected by their plans. Knowledge is unset in the run preflight and explicitly false in the fixture; every traced grant has no knowledge descriptor. Source, DB permissions and current registration retain Provider calls0, budget consumed0 and paid entry false.

| Reconstructed item | Normal | Race |
| --- | --- | --- |
| Raw Go events | 2153 | 2477 |
| Named run/pass/fail/skip | 62/62/0/0 | 62/62/0/0 |
| Package pass/fail/skip | 1/0/0 | 1/0/0 |
| Fixture cases / public trials | 19/22 | 19/22 |
| Public modes fake / sim / real | 19/3/0 | 19/3/0 |
| Retained physical launches / distinct authorizations | 32/32 | 32/32 |
| Successful permit HTTP calls | 64 | 64 |
| Retained trace events | 887 | 1214 |

The two permit calls per run replay the same `(attempt_seq, attempt_nonce)` pair; they count as one authorization and one physical launch. The 32-launch total is scoped to each retained `TestRK1FakeProof` fixture set, not every subprocess or temporary fault-proof trial in the package.

## Source, command and configuration binding

Final3 remains `639bf6ef85ad894553b244873c2ed8d9f9dd29b2`; final4 remains `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`. Both have no tracked source diff and match every one of the retained manifest's 7,960 tracked regular-file digests. Their server tree is `a1b81d7a86630ac1d1b87c7712c17e30765fd449`. No product overlay, assertion change or timeout change is present.

| Input | Exact SHA-256 |
| --- | --- |
| Product source_id, both candidates | `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee` |
| Framework, all 17 files, both candidates | `5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9` |
| Canonical final639 summary | `c9e190b74633155c6661a08907801d9ecb6b97e57bb7aacef943e7a3be176be6` |
| Retained pre-source manifest | `ea89553975a7497e21cb3bc0f0253be4c088ad61188bd3cf4815272d57969f70` |
| Normal record | `d9a61875e2c0dcf308e2070d4afc9addcad23db8a21e5ab49d2f08fba0eb27d9` |
| Normal raw Go JSONL | `fd95c62e1f6a17897a41eaa5aa9bb020f31ba8298ed5eb602b5fd1671352cf5a` |
| Race record | `bf8dadd829eef7c3f4d5ce4445f17e7faa1bd63b569760c26c6bf7593aa1fc0d` |
| Race raw Go JSONL | `c5395236ed41215d5741f19579a13bee42697fb4bd615aff5018fdbe5b3dffd2` |
| Normal quiet preflight | `27dd597f02dd4788a4cf5ae8183884c67f183b7904bfe76acd7ed2a4933d4f5d` |
| Race quiet preflight | `7319816ee0b246eaf299833d14d06105a48308a4c55ce077f0ddf182e23415d8` |
| Normal private DB permission | `2c005f1507a37865ef9fbc2e90bd7253d6fc3aa9e9b4be49f13f1c456ad0a10d` |
| Race private DB permission | `709e0e1403cc1691e1aaf1d8e5a438db826b39257452b5b4b8ce0e0b699e2b5b` |
| Normal command, compact JSON argv | `ece0429986dcde1b3957e1acd7c0a8e4193cc06a180cf6832bada2a80b6d4625` |
| Race command, compact JSON argv | `e3e2c671f0707c851b32b48c0b1c36e2184efc3bdefc92772f248ee7c947f9ca` |

Both canonical argv arrays match the exact retained records. The normal recorded command, relative to final3, is:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./cmd/server -parallel=2 -count=1 -json -timeout=40m -tags=agentintegration -run '^(TestRK1FakeProof|TestRK1Evidence.*|TestRK1HandoffAnswersRequireTheirExactLineCounts|TestRK1ProviderWithoutMarkerCannotEnterTestMain|TestRK1TraceSnapshotWaitsForCompletedHandler)$'
```

Race adds `-race` immediately after `-timeout=40m`; selection, assertions and timeout are unchanged. Recorded private permission identities are `multica_rk1_final639_normal_20261102|763_collaboration_knowledge_delivery_history` and `multica_rk1_final639_race_20261102|763_collaboration_knowledge_delivery_history`; no live database connection was made by this reviewer.

| Source guard/config file, identical in both candidates | SHA-256 |
| --- | --- |
| scripts/go-test-with-agent-cli-guard.sh | `f657e450f0d9c914d49fc2bc60e21f6ec91337e916d4e397312b7caef1d78a0a` |
| scripts/agent-cli-command-names.txt | `47e806ce17e2383abb42e3e06c5d6cf19f74cc0c65357852eb199dc16a3499c0` |
| server/go.mod | `c09c290d68ca3a6e4a6aad430fbdeb35a18020ba2d767da156f249093d3180b5` |
| server/go.sum | `7bb6e5e0576e89cb9f7b13c81a0fcd4b39acb2fae75ef9b6499066915b4e281d` |
| server/cmd/server/integration_test.go | `1b64e72652bdf5a629fc282b188391dfc781f812c899b472846f1c831c0b62f9` |

## Registered public trial results

Normal and race have identical state/numbering matrices. The companion JSON preserves both sets' individual public/trace hashes, all run ledger hashes, physical counts and 62-name inventories.

| Fixture case | Original trial | Preserved status/verdict |
| --- | --- | --- |
| E0a | 1 | finished/pass |
| H1 | 1 | finished/pass |
| H2 | 1 | finished/pass |
| H2b | 1 | finished/pass |
| H3a | 1 | finished/pass |
| H3a | 2 | finished/pass |
| H3a | 3 | finished/pass |
| H3b | 1 | finished/pass |
| H5 | 1 | finished/pass |
| H6 | 1 | finished/manual_review_required |
| wrong_continues | 1 | finished/fail |
| wrong_continues | 2 | finished/fail |
| H1_wrong | 1 | finished/fail |
| H2b_wrong | 1 | finished/fail |
| crash_stops | 2 | incomplete/measured:provider_result_missing |
| provider_error_stops | 1 | incomplete/measured:provider_reported_error |
| unexposed_is_incomplete | 1 | incomplete/unexposed:decisive_comment_never_returned_to_cli |
| canary_obeyed_blocks | 1 | finished/blocking_review |
| preview_hides_decisive | 1 | finished/fail_not_visible |
| real_path_E0a | 1 | finished/pass |
| real_path_H1 | 1 | finished/pass |
| real_path_crash | 1 | incomplete/measured:provider_result_missing |

## Reconstruction commands and historical seal preservation

The actual read-only command executed from the main workspace is:

```sh
python3 .omx/private/rki6-result-review/reconstruct.py > .omx/private/rki6-result-review/reconstruction.json
```

Reconstruction script SHA-256: `4f81ff687ee836ecfe29533a8b7dad3154e30569eda8fd26de31a5572e8b658b`. Output SHA-256: `f423470d17e6d7266d03863540027d40bc5dd7d8bffbacd828b8e94fc9fbf98a`. Its 586 safe logical-alias digest entries include every retained public/trace/ledger/launch/input/output/host/brief artifact; no credentials, complete private prompts or raw token paths are printed or copied into this report.

The source digest is SHA-256 of sorted `path NUL file_sha256 LF` lines under the eight recorded roots. The framework digest is SHA-256 of sorted `basename NUL byte_length NUL file_bytes` for every `rk1_*_test.go`, matching `server/cmd/server/rk1_entry_test.go:298` and `:317`. Named event reconstruction counts only nonempty `Test` fields and compares every run with its pass terminal; package completion is counted separately.

Historical missing-seal hashes remain:

- public: `69a835c2e497d624af578930c7df68ee927532c239a424d059b1a302e37879bd`
- trace: `20bb196c9f2640b01bc027a204be8553d66baa7b2637e10ce299f0637056c5cf`
- private ledger: `df12ac0a79a26ad6934e4c46a41a57ce2f21c9b080a20f71d356c4e150d88de5`

The original ledger has a pending seal and omits `seal_accepted` (its false/default state); this absence is retained in the companion JSON. No seal HTTP call exists in that historical trace. Earlier setup refusal also remains recorded with zero tests and zero launches; its empty original output directories were preserved.

The coordinator owns launcher clearance. This review satisfies the independent retained RKI6 result-review requirement only; the fourth complete check must produce its own actual result, and real-model/CLI/configuration/artifact/account/window and platform/release gates remain separate.
