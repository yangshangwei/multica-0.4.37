# Codex harness repair recheck

Date: 2026-11-05 Asia/Shanghai. Reviewer: codex_integration. Scope: read-only H1–H4 repair review; no product edit, test, database or Agent/CLI execution. Original `harness-review.md` remains unchanged.

## Verdict

**Static repair pass, conditional on fresh full normal/race and final execution binding.** The blocking source gaps H1, H2 and H4 have been addressed. H3's manifest algorithm is corrected for this repository; the actual final compile/CLI/native-executable/registration artifacts must still be reviewed before real use. No real execution is authorized by this report and no new test pass is claimed.

## Rechecked repairs

- **H1:** One thread request and one turn request are required; request/response IDs are unique and successful responses are mandatory. Requested resume ID now equals the returned thread and actual turn thread. Scored agent messages and turn notifications must match the exact native thread/turn. Terminal conflict, error-bearing terminal, asynchronous error, or post-terminal item/turn-started cannot produce accepted evidence. Early asynchronous items are interpreted after correlating responses, so legitimate event-before-response ordering does not erase their identity. Ordinary scenario validation independently compares actual fresh/resume and native ID against the server claim, including K2 S1, rather than merely assuming the claim dictates execution.
- **H2:** Measured Host samples now require real successful CLI `mcb` GETs for both memory and handoff. `o.Calls` is already filtered to `/api/collaboration/runtime/runs/<run>/`, so suffix checks retain exact-run scope. Missing reads in a complete execution are behavioral failure, not fabricated infrastructure success. Supervisor MDT snapshot calls cannot satisfy the predicate.
- **H3:** Both compile and fake-run manifests now use all git-visible tracked/untracked `server` files rather than only Go. This covers current repository go.mod/go.sum and the inspected `go:embed` resources (docs, changelog JSON, skills/templates, avatars, reserved slugs, OpenAPI, fixtures). Source/binary equality remains checked before execution and source stability afterward. Root separately selected a direct bundled native Codex executable; actual version/hash and CLI source provenance remain final-package obligations. Explicit compiler/toolchain/flags and embedded-file mutation evidence are not yet in a completed final record.
- **H4:** Ledger now distinguishes ActualStarted, ProcessComplete and Complete. Wrapper finalization only sets ProcessComplete; the next reservation remains refused until `rk1kSaveSample` has durably written the finalized sample and calls outer acceptance. An incomplete sample stops the ledger. The real test's deferred failure handler durably stops on any fatal subtest/setup/K2/seed failure. Thus a failed outer sample write leaves Complete=false even before the failure defer runs. Cap and duplicate identity guards remain unchanged.
- **Fixture correction:** Reuse of the existing registered Codex runtime validates owner, workspace, profile absence, local mode and daemon presence before keeping the row. Capabilities are still registered through the actual endpoint. This fixes the observed duplicate tuple without fabricating a runtime or weakening product uniqueness.

## Controls inspected, not executed

New source controls cover same-thread wrong-turn messages, wrong requested resume, positive exact resume, missing/error/duplicate responses, duplicate terminal, post-terminal item, plus previous invalid JSON/wrong-thread/missing-terminal/duplicate-input controls. Ledger controls now require outer sample acceptance; explicit publication-failure and published-positive cases demonstrate the intended next-reservation gate in source.

Fresh dynamic verification must run these controls together with the unchanged 1/4/2/8 measured and three fake-seed scenarios. Additional missing-Host-read and completed-with-error predicate controls remain useful evidence; the source fixes are direct enough that their absence alone is not a remaining static blocker. Full suites have not yet run on this repair snapshot.

## Original normal01 evidence independently checked

`normal-01-run/result.json` records exit1, accepted=false, source_stable=true, no malformed JSON, no summary. Raw test output shows E0b fixture setup failed with SQLSTATE23505 on `agent_runtime_workspace_id_daemon_id_provider_idx`. There are zero launch.json files in that output tree, confirming this attempt reached no captured fake Provider launch. Its 29 passing control terminals and two failed container/E0b terminals are preserved; they do not cover subsequent repairs. Real calls remain recorded0.

## Remaining before real launch

1. Fresh compiled normal/race full fake suites against separate owned DBs; verify exact top/leaf inventory and 15+3/0 summary, raw artifacts, source and binary binding.
2. Independent final manifest and real registration review, including private minimal account config, exact native Codex executable/version/model, CLI, database identity before TestMain, expiry/exclusive window, physical cap15, empty initial ledger and runnable decision.
3. Account-bearing config/trace trees stay private; no registration secret values enter repo reports. Existing process ledger corruption refuses and must not be manually converted to a successful cursor.

## Reviewed source hashes

```json
{
  "server/cmd/server/rk1k_codex_provider_test.go": "1353eeb3e7036c0d781ef86ccc87aa34519b7be3b4fb0716c56b0d6b1c323e00",
  "server/cmd/server/rk1k_codex_controls_test.go": "19408b1a6133ac894718922a6318d61630c3980cd8d098f3293c4d072bb5fdfb",
  "server/cmd/server/rk1k_codex_registration_test.go": "c8b55f9467f7851ca26627e841433f602b57e203066bb769b4efedd1410049fa",
  "server/cmd/server/rk1k_scenarios_test.go": "a6b6b83c7f259dbea7ea895dbe28661cec811ada8308620849b7e476156e927e",
  "server/cmd/server/rk1k_entry_test.go": "148aafb7157b30cbc2787c5bb9749f122f83e9d13c7fc71756726a22714ec944",
  "server/cmd/server/rk1k_fixture_test.go": "301cc9e8234d16b667adaa35e07406c9825d21c793a4f04c06974cecb1dbc790",
  ".trellis/tasks/10-30-collaboration-knowledge-promotion/research/codex-real-20261104/compile-harness.py": "776fcaa3e41d900e4cfb5cda249264b2df1134a4297b4b4644f172e6c633b347",
  ".trellis/tasks/10-30-collaboration-knowledge-promotion/research/codex-real-20261104/run-fake.py": "e61e2cd30b46c49307c474df50fae49ac232d705f9d43dc3483561898e12998c"
}
```
