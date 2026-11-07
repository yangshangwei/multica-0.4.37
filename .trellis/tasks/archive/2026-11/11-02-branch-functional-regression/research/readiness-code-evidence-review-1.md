# Independent code and raw evidence review

**PASS WITH NOTES for phase1 of this scoped review.** The new Webhook diagnostic passed1top+4sub with race enabled, and offline updater preparation is consistent with current source and exact bytes. Original QA-PERF-001 remainsFAIL at2171ms>2000ms; required acceptance gates remain open. This is independent of both implement-child self-checks and coordinator verification. Final current-document/history/cleanup review is deferred to a separate coordinator followup.

## Findings (fixed)

No implementation, test, raw-evidence or spec issue required a fix. The reviewer changed no completed private diagnostic/template, product/source/test or coordinator document. One read-only reviewer source lookup assumed the wrong receiver spelling in a regex; the actual Copy/Config symbols were found with rg and inspected. The lookup error and transient warning are preserved in evidence/next-readiness-review-1/lookup-observation.json; they are not product or vet failures.

## Findings (not fixed)

- Original performanceFAIL and native platform/authentication/updater/trust/Provider gates remain open. Original6Go skips and10376PASS/1FAIL inventory are unchanged; the new private diagnostic is a separate identity. CLI unknown-field loss is explicitly documented in current config.go/config_test.go and cannot be turned into a passing preservation contract.
- The server fault is real pgx PrepareError wrapping PgError42P01. It establishes server prepare/parse failure and the handler500branch; it does not prove SELECT executor work or server-side parameter binding. The closed-pool fault is client acquisition failure. Forwarding20typedScan destinations does not establish a successfully materialized row/decode/signing/delivery/worker/Provider journey. Three observed fingerprint tables were initially empty; no broader persistence claim is supported.
- Updater material is offline preparation. Templates have unresolved owned-endpoint/profile placeholders, no HTTP feed or Mac updater ran, and actual native signing/install/restart/authentication compatibility is unproved. Node HOME/cache and native proxy/phase requirements are source-derived; initial launch does not prove native relaunch environment propagation.

These are disclosed scope limits, not blocking evidence defects.

## Code and evidence checks

The private DBTX wrapper forwards the exact generated GetWebhookTriggerByToken SQL/pgtype.Text argument and delegates all generated Scan destinations to the actual owned pool. It does not manufacture errors. testPool.Config returns a copy through pgxpool.Config.Copy, ConnConfig.Copy and pgconn.Config.Copy; RuntimeParams is deep-copied. Separate MaxConns1/MinConns0/session search_path therefore does not mutate the global pool config. Cleanup ordering closes only owned pools before recording their final closed witness. The actual handler500/404/absolute429order, standard error JSON, body-read0, downstreamExec/Query/SendBatch0, debt split and positiveRetry-After match the cases and code. Healthy globaltestPool/search_path and three table fingerprints remain unchanged.

All100distinct bound raw/source/dependency hash comparisons matched; no missing artifact was found. The reviewer reparsed exact Go terminals:1top+4subPASS, noFAIL/SKIP, one guardedracecompile and one selected execution. Canonical SQL matches the current generated constant. Source/frozen/originalskip identities match. The original failed performance stream hash matches without rerunning or reanalysing the P95/cache campaign.

Updater validation uses the actual installed electron-updater6.8.3 parser/GenericProvider/file selection and the repository reference parser, with a network executor that refuses calls. Original generatedlatest-mac.yml is byte-identical; the two exact ZIP/DMG bytes independently match recorded SHA256/SHA512/sizes. MacARM64 channel and ZIP selection are correct, and stable-only rejection versus explicit offline RC opt-in is correctly scoped. Both original ASAR-version/IPC observations are bound by the saved artifact hashes; no application launch was repeated. Actual AppAdapter/ElectronAppAdapter/AppUpdater/MacUpdater source confirms Nodeos.homedir cache selection and the dynamically allocated Squirrel proxy with wrapper notification preceding native readiness. The two new seven-section spec additions match these contracts without changing product behavior.

## Verification

- Go static lint/type analysis: one guarded `go -C server vet -p2 -overlay <private overlay> ./internal/handler`, exit0, stdout/stderr empty. It does not execute TestMain. Source/frozen/privateoverlay identities remained unchanged; the owned vet process group is empty afterward.
- Formatting/syntax: actual gofmt-l, Node--check of the offline validator, PythonAST, JSON/rawhash and gitdiff--check PASS.
- Product TypeScript lint/typecheck/full suites: NOT_RERUN; no product/TypeScript/dependency changes and the dispatch forbids completed-suite repeats.
- Tests: reviewer reruns0. Saved actual new diagnostic1top+4subracePASS verified; original tests/failures unchanged.
- Database/native/account/remote actions by reviewer:0. Fresh whole-task cleanup and final coordinator-document reconciliation belong to the next phase.

Exact command/time/exit/hash and process evidence is in evidence/next-readiness-review-1/run.json, raw-verification.json and this report's JSON companion. The parent may update coordination links/status later; completed raw results remain immutable.
