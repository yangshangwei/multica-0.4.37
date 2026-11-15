# v9 private adapter preflight review

Verdict: **pass-with-notes**, **0 blocking findings**. The exact v9 hashes and machine-readable clearance fields are already available in final-v9-review.json.

## Confirmed repair

The only behavior change from frozen v6 is removal of inherited MULTICA_RESOURCE_PUBLISH_DIR from each ordinary mode1 Go package test child. Explicit recursive test configuration, compile/non-test calls and API/source configuration retain it. There is no product/assertion change or default internal-organization seeding. pnpm and launcher changes select separate v9 report, home, registry, profile and workspace paths so the failed v6 run remains preserved.

## Verification

Independent deny-network fake suites passed **20 base/HOME/pool/publisher +6 global separator +38 package/parser matrix +22 launcher =86**, zero failures. Six Python sources pass AST parsing. The candidate is clean, source env remains mode0600 with matching19845 renderer and API publisher configuration, and before/after hashes stayed unchanged.

Root's actual unchanged-test causal replay is recorded in [catalog-actual-redgreen.json](catalog-actual-redgreen.json): v6 7run/4pass/3fail/0skip, v9 7run/7pass/0fail/0skip. It covers the two original Skill Template catalog/copy scenarios on separate fresh package clones; only the mode1 publisher environment differs. This reviewer read that proof and ran no database test or TestMain.

All earlier closed invariants remain green: pool/root/HOME/cache isolation, Make-safe canonical dotenv and mode0600, actual hash binding, canonical package/override guard, compiler flag/value/alias boundaries, global -- dispatch and the original Agent CLI guard/selections.

| Binding | SHA-256 |
|---|---|
| Go | `70ca24bb88b2a8df93ae4694e9fd9739c523e8f35755a10bb799b7561b934fce` |
| pnpm | `acb022a829423fd10cde20c30b00ceb15c9ce4c38a1dd3327e8e0338bbce7ca2` |
| launcher | `68dfc4c48a42e7ea276eee815b7f0985e96cf950ec3e2e623187e8ab583bf475` |
| source env | `99475d2e36917761e88bab9687073ddb182c131241d8d815b7eb18850572dfe2` |

Candidate: `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`. The predecessor Go/launcher and failed whole-check record/log retain their original hashes. No source, metadata, resource, service, account, Provider or clearance action was performed by this reviewer.

Root can issue exact v9 clearance and run a separately numbered complete check. This preflight and targeted seven-pass replay do not replace the required full-run result. Repository lint/TypeCheck remain part of that upcoming complete check.

