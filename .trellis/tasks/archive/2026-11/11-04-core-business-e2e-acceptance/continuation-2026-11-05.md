# Continue unresolved acceptance gates

User asked to continue in the same thread. Preserve completed1a32 candidate acceptance and failures; do not switch to newer ROOT product commits.

## Authorized next slice
1. Linux fixture: preserve primary failure separately from cleanup failures; record only command id/name/time/phase/result category without input/output payload. Cover failure+cleanup ordering with focused tests after external H1 window release.
2. Inspect the original restart sequence against current product lifecycle. Use new numbered actual original test only after improved diagnostics, exactsource/artifact binding and newtaskDB. Do not increase timeouts, remove assertions or claim unchanged reruns fix anything.
3. Read existing performance samples/SQL paths to rank hypotheses and design minimal diagnostic; noP95reroll or productoptimization without confirmedcause+regression.

## Ownership
- ROOT e2e/fixtures/collaboration-installed-main.ts, .mjs, .node-test.mjs and e2e/collaboration-installed-main.spec.ts: boundeddiagnostic author. Preserve earlier fixture changes and others'files.
- Frozenruntimecheckout remains unchanged until reviewedpatchready. Productserver/apps/packages unchanged.
- Read-only performance researcher writes only taskresearch/continuation-perf-*.

## Shared window
RuntimeH1 currentlyactive; noheavytest/build/database/browser/Provider until owner01a1a729-99ac-7ab2-a312-6478d946d58a explicitlyreleases. Staticedits/readsallowed. Corecontrolledonly; knowledge retry403stillseparate.
