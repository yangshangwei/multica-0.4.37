# Backend verification result

Candidate: `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`. All82 selected packages have complete acceptance evidence;73 packages contain named tests and9 have no named tests. Final selected inventory: **10,393 top-level PASS /0FAIL /20SKIP**. Including subcases:20,964PASS /0FAIL /21SKIP. Every one of10,413 expected selected top-level names is accounted for. `go vet -p2 ./...` passed. Backend source/cross-package contract inputs and Go/Node/Bash tool hashes match the original reviewed candidate.

This is **not a single clean full execution**. Results come from four preserved segments. Service andSlack have complete Go package terminalPASS plus independent observations because their paused parent was interrupted before receipt writing. Other selected packages have recorded process exit0. Original run3 execenv had655PASS/5FAIL due the runner's Git-containedHOME/TMP; the unchanged660-case full package passed in run4 on a fresh allocated database after movingHOME/TMP outsideGit. Those first failures remain in evidence.

## Fixed testing-environment defect

Per-package directory separation was insufficient while directories were nested under the original repository: git/cursor parent traversal reached that repository. Root reviewed the minimalv4 runner, authorized `/Volumes/artisan/e2e-private/core1a32-20261105/backend`, and allocated a newexecenvDB. PositiveGit-free and negativeGit-contained boundary probes passed. Originalroot `.cursor/mcp.json` was observed after the failed run; no pre-run content snapshot exists, so no claim is made about its earlier contents. Its SHA256remained `fb40aae928dc3a6ec111763e3d52cd76ce83c03dd78c792d7285ccb21eb7a606` throughv4; no speculative deletion/restoration occurred.

## Remaining boundaries

20top-level skips and1subcase remain explicitly recorded in backend-skips.json. These include Linux/Windows kernel behavior, missingpg_bigm, opt-in performance, Go→TSwire export, externalGitHub import, missingHTTP MCP recipe, and intentionally unimplemented assertions. Default package successes do not close these conditions. knowledgeintegration,performance,wire andLinux follow-ups are in backend-conditional-plan.json and remain the coordinator's separate execution lanes. Real-provider tests are explicitly deferred by user.

LocalGo1.27.0/darwinarm64 differs from CI Go1.26. Existing shared PostgreSQL17 is x86_64 on the ARMhost; no system defaults or unowned services changed.

## Evidence

- `research/backend-result.json`: exact package selection, named totals, source/tool identity, original failure and final sidecarhash.
- `research/backend-skips.json`: actual names/reasons and evidence paths.
- `evidence/backend/normal-race-1..4/`: immutable rawJSON and process receipts.
- `evidence/backend/resume-4-launch/runner.json`: detached supervisor complete exit0.
- `evidence/backend/normal-race-4/vet/`: finalvet exit0.

No product source, production behavior, test assertion, dependency or global setting changed. Only private runner/permit/evidence/task documentation changed. Frontend lint/typecheck were already completed by the frontend lane and were not redundantly rerun here; Go static analysis passed.
