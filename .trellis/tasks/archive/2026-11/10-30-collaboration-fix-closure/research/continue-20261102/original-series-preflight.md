# Original frozen-series continuation preflight

Verdict: **changes required before the complete live continuation**. Frozen sources, framework bytes, explicit Claude executable and historical formal E0 provenance are available. No Provider call, installed CLI/version execution, account/config inspection, Go suite, DB command or server startup ran.

## Verified readiness

- Baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f` and after `835e7b046c6841e0945a59326bc4ddcbae4a0df7` match registration. Both have exactly the three permitted framework overlay files and aggregate hash `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`; no other dirty source paths.
- `/Users/artisan/.local/bin/claude` exists, is executable and resolves to the installed `versions/2.1.316` file. SHA-256 `64c327e92b2c5e0f78c0a980c5a518121e1f0d140ca771ca5dd4488d1a2dcb3f`. This is a file-only check, not a version or account probe.
- Formal `TestHostProviderAccountSmoke` originally passed at `4c833e737` with Claude 2.1.316 and observed model `claude-opus-5-5`. Its retained raw log matches metadata and its test bytes match the frozen after arm. The baseline is intentionally pre-fix; this does not prove current Provider health or a new RK1 candidate.
- All 29 current continuation evidence hashes match. Raw consumed slots agree with the ledger: E1/E2a/E2b/E3 baseline 1–3 used; E3 after 1 incomplete,2/3 unrun. The next cursor is **E3 after trial 2**, with **14 calls**: 2 E3 + 3 E4 baseline + 3 E5 baseline + 6 E6 baseline. E6 has 3 pairs, 2 physical launches per pair.

## Findings requiring action

### OEP-1: new append-only registration/run directory

The old driver/prerun registers E3 after start_trial 1. Actual `--check-only` for start2 exits 2 (`no single registered plan entry`); start1 exits 2 because its evidence directory already exists. E4/E5/E6 baseline start1 check-only exit 0 without executing Go or Provider. They must still wait for E3 and its stop rule.

Create a new run directory and remaining-plan registration, copy the runner bytes unchanged, link/hash the previous summary, register E3after2 and caps 2/3/3/6, and retain every old slot and raw record. Merely restarting or editing the original directory is invalid.

### OEP-2: frozen E5 does not stop immediately on canary

The preregistration requires E5 canary output to enter blocking review and stop subsequent execution. The actual frozen `host_context_experiment_evidence_test.go:242–249` returns `finished/fail` for the canary. `host_context_experiment_test.go:435–438` adds an error only for non-finished records; `runSeries:208–229` therefore proceeds to later trials. The outer test reports fail only after the complete runSeries returns (`:180–188`). A canary at trial 1 can be followed by trials 2/3.

This is a specific code/registration mismatch. A new directory cannot correct it, and post-process checking after Go exits is too late. Keep E5 unstarted until the main session resolves the stop contract; a fix needs separately registered framework hashes and independent fake stopping evidence without relabeling or silently mixing old provenance. Sources are left unchanged by this reviewer.

### OEP-3: current exclusive window remains unverified

The original run retained cross-session overlap. Sibling runtime metadata still names the prior coordinator/old running-check phase, so it is not proof of a current exclusive window. The main session owns deterministic/full-check work and scheduling. Confirm current ownership and quiescence immediately before live calls. This reviewer did not send peer messages, interrupt processes or probe account health.

## Semantics and reporting limits

E3 requires context exposure and BETA-only posts, with repeated BETA allowed; baseline 3/3 already contradicts the original before-fix prediction. E4 requires manual verification of stale-version merge, completed step 2 and both existing/new pitfalls. E5 requires real exposure and immediate canary blocking. E6 requires separate pairs, memory-only handoff and manual evaluation of current advisory locks, rejected alternatives/reasons and CONCURRENTLY pitfall. E1/E4/E6 Go exit 0 is not semantic pass; E2a's single valid sample cannot validate a 2/3 prediction.

Root can complete the new registration, review and deterministic work without quota use. Sources/CLI are not currently missing. Live Provider/model availability and exclusive window are untested prerequisites, not proven present or absent. New Provider calls 0; original 14 and RK1 25 budgets unchanged.

Detailed checked file hashes, raw slot records, exact check-only commands/results and recommendations are in [original-series-preflight.json](original-series-preflight.json). Lint/typecheck/tests were not run because only these two evidence files were written.
