# Codex real execution admission review — preliminary

Date: 2026-11-05 Asia/Shanghai. Reviewer: codex_integration. Read-only artifact and source inspection; no test, database, installed CLI or model execution. Credential content was neither decoded nor printed: the private config was read only as bytes for SHA-256 verification. Root's concurrent-runtime quiet-window pause is respected.

## Decision

**Not yet runnable.** The concrete draft is correctly unauthorized, bound to the reviewed source and binaries, and has consumed no real starts. Two coordinator-launcher corrections and fresh final normal/race evidence plus exclusive-window release remain before positive admission. H1/H2/H4 framework source repairs retain their static pass from `harness-recheck.md`.

## Verified concrete draft

Sixteen direct checks passed: registration hash; authorized=false; cap15; actual test binary hash matches draft, registration and compile record; compile record hash; compile exit0/source_stable; registered source map equals compile map; all source bytes match; current git-visible server file set equals the registered set; CLI hash; native executable hash; private config hash; config mode0600; ledger is exactly cap15/starts[]/stoppedfalse; output path absent; recorded database allocation matches migration764/issues0/tasks0.

- Test binary: `codex-normal-03b.test`, SHA `cc198f11e87566ad6a1bf20270d461ad8f3fd38e95496cd598818f1cd74eb2d1`.
- Compile record SHA: `a81dd7e63b9e424a5edb2ebac4298ad92fc0678c411e646c6a0fb77e05500fbd`.
- Source set: **4,501 files**, including current dependency manifests and embedded resources.
- Multica CLI SHA: `22dd56e7f63decfeee03654fb02602a898bfa927d7c335f9cb8a590a8b7686ab`.
- Actual Codex is the direct native binary inside the installed npm package, SHA `a45099154d7fb4a5ed8a56f40966a95deb938e508bab23e6985035c709e9a85a`, version recorded0.165.0. This resolves the earlier JS-launcher-only ambiguity.
- Model: `gpt-6-astra`; provider: `codex` with the user's selected custom Responses provider. Private minimal config hash `2d8fa41539c89927342ccf16cdf3af13a322fe646615917dc0c7688f3dbad52a`. Global config unchanged and no inherited developer hooks are recorded by root; this reviewer did not inspect those credential-bearing config contents.
- Database: `multica_rk1k_codex_real_20261104_318509c1`; allocation/migration evidence checked, no live DB query by reviewer. Launcher must verify exact identity again before TestMain.
- Ledger empty, cap15, mode0600. Registration mode0600. Private root directory is0755, but config/ledger/registration are0600 and per-run output is created0700.
- Expiry is fixed in the draft; whole-test90m and ordinary/Host5m limits are declared. No automatic retry launcher exists.

## Coordinator launcher corrections

### RA1 — Guard ambient Agent command resolution during real runs

`run-real.py` launches `go tool test2json` directly, whereas the fake runner uses `scripts/go-test-with-agent-cli-guard.sh`. The actual registered real Codex is invoked by absolute native path, so the existing guard can wrap the test command without blocking authorized launches. Without it, a model tool command or unintended fallback can resolve installed codex/claude from ambient PATH outside the wrapper's physical ledger.

Wrap the invocation with the existing guard and include its script/command-name list in the reviewed execution input hashes. Retain the limitation that PATH guards do not prove containment against deliberately supplied absolute executable paths; test samples have narrowly defined CLI actions and the final RPC/command trace must be inspected for unexpected extra Agent execution.

### RA2 — Match accepted fake provenance and exact current file set

The launcher checks accepted/exit0/source_stable booleans on normal/race results, but does not bind each result's compile-record source to the real registration. It checks current bytes of registered files but not additions/removals in the current source set. The current draft passed these checks in this reviewer, yet the launch-time gate must enforce the same invariants after final artifacts are chosen.

Require the exact current git-visible source set/map to equal the registration and accepted normal/race compile source manifests. Verify referenced compile and test-binary hashes, and both fake results' CLI hash equals the selected real CLI. Race and normal binary bytes can differ, but their source manifests must match. Include actual command, toolchain and final CLI build provenance in the retained package.

## normal02 dynamic evidence independently checked

Raw test JSON has **45 pass terminals / 8 top-level tests, zero fail/skip/invalid**. Summary contains18 finished/pass samples. Independent launch recount is **15 measured fake + 3 fake seeds + 0 paid**. All **54 recorded artifact hashes** match actual files. Host root/member/continuation and all seeds have actual run-scoped memory/handoff read criteria true. Ledger cap/duplicate/incomplete/write-failure, outer sample publication and exact RPC resume/wrong-turn/postterminal controls all have passing terminals.

This is the valid intermediate normal02 snapshot. Later adapter private-log fixes and final CLI/source changes mean normal02 alone is not the final product gate. Fresh normal03/race results are still required.

## Remaining root-owned admission checks

1. Complete RA1/RA2 and submit the exact launcher/false registration/permission template for final read-only review.
2. Fresh final normal/race acceptance on final source, source hashes before/after, complete named inventory,15+3/0 and54artifact hashes. Any source drift needs honest evidence binding, not relabeling the old pass.
3. Complete independent adapter and integration reviews and release the other runtime coordinator's exclusive quiet window. The knowledge-specific lock protects only this launcher; it does not coordinate another series automatically.
4. Issue authorized registration/permit only after final review; permit binds its review file, launcher, registration and accepted normal/race artifact hashes. Keep initial ledger empty and output absent, perform fresh live DB identity check, run serially exactly once.
5. After real process termination, independently inspect durable start/process/sample states, actual RPC and CLI effects, model verdict, source stability and unconsumed quota. Preserve every failure and stop after incomplete output. The launcher correctly reports review_pending rather than declaring semantic success.

## Reviewed artifact hashes

```json
{
  "/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-knowledge-promotion/research/codex-real-20261104/real-execution-draft.json": "53292bff3ff322f220c40ead5f64aba2bb5c12b1c60fbc289f083646621aa53b",
  "/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/knowledge-codex-20261104/real-registration-draft.json": "70b2ec58bb22708ba3fbafd518ee8c48612ee05fdcb7f07596e05b0c8a2706e7",
  "/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/knowledge-codex-20261104/run-real.py": "42c2ff519545057bc227e8f228ce7d5fe1ec7e8dad6bcd674f78dc4dfb954a1d",
  "/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-knowledge-promotion/research/codex-real-20261104/local-binding.json": "a3ec2bc62c5acd951f10a8fc44f8c5361c6a5785dd2172f5a11255604299a376",
  "/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/.omx/private/knowledge-codex-20261104/normal-02-run/result.json": "b8fb00e0d51f3dba777148996682255019317c23ba6278c838f0c33cd7985cf5",
  "/Volumes/artisan/code/2026/mc-knowledge-codex-20261104/.omx/private/knowledge-codex-20261104/normal-02-run/proof/summary.json": "b30f68be72851b15fdff4af7bd4203ab22a2324f092e6e2d5225d65054a89428"
}
```

## Coordinator launcher recheck

The updated launcher now wraps the fixed test command in the existing Agent CLI guard and checks each accepted normal/race compile source map against the real registration, compile/result binary hash consistency, selected CLI equality and exact current source inventory. RA1 and RA2's principal corrections are present. Current launcher SHA: `42c2ff519545057bc227e8f228ce7d5fe1ec7e8dad6bcd674f78dc4dfb954a1d`.

Before issuing the final permit, include and verify hashes for `scripts/go-test-with-agent-cli-guard.sh` and `scripts/agent-cli-command-names.txt` (outside the server source map), and require the real registration's test binary SHA to equal the accepted normal result's binary SHA. The actual draft already matched the latter in direct review; this adds launch-time enforcement. Final fake results and coordinated quiet-window release remain outstanding.
