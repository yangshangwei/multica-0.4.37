# RKI-3 / RKI-4 implementation status

Owner: `/root/rk1_evidence_fixes`. The live NEW_TASK payload is blank; the exact assignment was recovered from the visible parent spawn record. Worktree `mc-rk1-framework-20261101`, branch `codex/rk1-framework-20261101`, base4b508179f. Root owns launcher/regression/registration changes; those dirty files are preserved.

## Boundary

RKI-3 lives in the Provider wrapper finalization and in observation/lifecycle classification. Extract the existing sync/close/final-record sequence into a test-only I/O seam; preserve execution exit separately from evidence publication failure. A failed or unfinalized successful launch must become incomplete before answer scoring and keep its launch in accounting. An intentionally killed H1 seed may retain started/partial raw evidence without inventing a successful final publication; explicit observed I/O failure never passes. Public summaries expose only stage enums and execution codes, never raw I/O text. Add deterministic sync, close and metadata-publication error cases through the same helper and prove the existing incomplete stop predicate holds.

RKI-4 lives in H1/H2 exact-answer matchers. Move their canonical parsing into a small test-only contract file, keeping semantics and frozen field meanings; reject extra lines, prose and cross-line whitespace. Prompts and scenario count/meaning do not change.

Files expected: `rk1_provider_test.go`, `rk1_observe_test.go`, `rk1_lifecycle_test.go`, `rk1_trial_test.go`, `rk1_scenarios_handoff_test.go`, new `rk1_evidence*_test.go`; only RKI3/RKI4 status/evidence under this task. No runtime product code, launcher or registration edits. No dependencies, database commands, migration, Provider or installed Agent CLI execution.

## Verification plan

Use explicit-file Go test selections for the new stdlib-only contract and unit files so package TestMain is excluded. The first extracted seam retains the old ignored-error behavior for a meaningful deterministic red run; then propagate publication faults and rerun normal/race. Format tests first exercise the current multiline regex behavior before exact parsing is fixed. Compile/vet the complete tagged cmd/server package without TestMain via go build/vet/test-c. A final full-path fake proof requires a root-allocated explicit database and remains a separate gate. Existing paid25, consumed0, runnablefalse and final source binding remain untouched.

## Live implementation and root test request

The deterministic contract red run reproduced three ignored I/O errors and H1/H2 extra-line acceptance. Complete-looking final metadata cache bytes after a failed publish cannot be the only guard; the wrapper now emits the fixed nonsecret RK1_EVIDENCE_PUBLICATION_FAILED diagnostic on an evidence error. The existing production host logger captures stderr separately; observation rejects that marker even if the metadata file looks complete. Sync/close failures also retain failed stage enums in launch metadata. Actual Provider execution exit is kept separately in the public summary, and failed successful results remain incomplete before semantic scoring. No runtime product or logger behavior changed.

A new TestRK1EvidenceFaultProof drives the same seam through the actual fake Provider/router/Runner/Host path and requires one successful raw Provider result/one launch, incomplete verdict and no trial2 for each sync/close/final-write fault. It has NOT been executed; cmd/server TestMain requires an explicit coordinator database permit. Please allocate a new migrated multica_rk1_* cmd/server-only DB and permit the following guarded selection:

`go -C server test -tags=agentintegration ./cmd/server -parallel=2 -count=1 -json -timeout=30m -run '^(TestRK1Evidence.*|TestRK1HandoffAnswersRequireTheirExactLineCounts)$'`

Real gate unset. All Providers are local test-created fake executables. RK1_EVIDENCE_FAULT_PROOF_OUT may name a new private absolute directory (create parent only; subdirectories are exclusive); raw proof/ledgers stay private. Normal/race focused contracts can run without TestMain as an explicit Go file list. Existing normal/fake/runtime seal failures remain unchanged and are not reclassified by these new evidence guards.

## Current non-DB verification

The explicit-file contract suite passed10/10 after error propagation, and20/20 under race,count2. A subsequent independent-backend-Wait control was added: even if final record cache bytes look complete and the explicit failure marker could not be written, the wrapper's separately logged process exit must match the recorded Provider execution exit. A normal measured result with no backend Wait summary is incomplete; stopped raw-prefix seeds remain explicitly partial. Public summaries now separate execution exit codes, observed wrapper exit and derived evidence states. H1/H2 format validation consumes raw outcome body rather than TrimSpace, allows a single terminal line ending, and rejects blank/additional prose lines. Tagged full-package vet passed; final updated unit normal/race runs are being collected.

Root allocation is still required for TestRK1EvidenceFaultProof. No DB or Provider command has been run by this child. A new private allocation permission file may be placed at research/real-model-rk1/RKI3-db-permission.md pointing to an exact private DB permit, or root can run the named selection and send its result file path. No migration/creation command will be executed by this child.

## Allocated full-path verification started

Root permit `evidence-fix-db-permission.md` explicitly allocated cmd/server database multica_rk1_evidence_root_20261102_0125 through760. Its actual identity was read-only verified before the guarded test command. Normal full-path selection is now running with a new private output tree; only TestRK1Evidence.* and the exact-format canonical test were selected, real-agent/knowledge gatesunset. No migration or database creation was run by this child. Result records: RKI3-RKI4-full-fake-normal.json/jsonl; later race uses a separate private output directory and log.

## Normal full-path result

The root-permitted normal cmd/server selection completed18/18 named tests/subtests,0fail/skip. Each fake sync/close/final-record fault retained a successful raw Provider result and executionexit0, one physical wrapper launch, an incomplete evidence-failure verdict and no secondtrial. Normal and race output trees are separate; the focused race selection is now running on the same expressly allocated cmd/server package database after identity verification. Tagged full-package vet and the final stdlib-only contracts passed (11normal;22race repetitions). No default database, migration, real Provider or installed Agent CLI was used.

## Completed bounded slice

RKI3/RKI4 implementation and focused verification are complete, pending independent delta review. Final contract11/11, contract race22/22; root-permitted actual fake Host/router/Runner proof18/18 normal and18/18 race,0fail/skip. Both old ignored-completion and multiline-format standalone mutants are killed by the final canonical tests (4named failures each). Tagged vet and compile-only test binary succeed; scoped whitespace check succeeds. Own Go files match the recorded normal/race source hashes. Raw private capture/ledger directories were not staged; public evidence contains only named tests, sanitized summaries and hashes.

The full-path proofs preserve the actual Provider raw success and executionexit0, separately observe wrapperexit43, and derive evidence failed/incomplete. Final metadata failure is modeled after complete-looking bytes were written, so this specifically proves the observer does not accept cache readability as durable publication. The normal backend Wait summary is required for a completed measured launch; the intentionally killed H1 seed may retain only started/partial evidence.

RKI4 consumes the raw recorded outcome body, uses whole-answer anchors and horizontal rather than cross-line spacing, permits one terminal LF/CRLF and refuses extra prose or blank lines. Semantic criteria and scenario trial counts do not change.

Remaining: coordinator independent recheck of this delta; complete final candidate fake/full matrix; root updates all framework/launcher/registration hashes and documentation. Old full fake/race failures remain unchanged and were not rerun or converted into pass. Paid budget25 consumed0, original series ledger and runnablefalse untouched. Source/commands/results/hashes are in RKI3-RKI4-verification.json.

Product/test commit: `4229e2d186c49e73656b508cd5c3b29cca0a966e`. All recorded Go source hashes match that commit's files. Current remaining untracked files are coordinator-owned database permit, historical intermediate race records and independent review reports; this slice leaves them untouched. No registration, launcher, default database or runtime product behavior was modified. Ready for coordinator-independent delta review and final integration.
