# Browser verification

## Ownership and environment

This lane changes only `research/browser.config.ts`, `browser.spec.ts`, this report, and generated evidence under `browser-evidence/`. No product or existing E2E files were changed.

The parent allocated the separate database `collab_source_browser_65139858fd`, API `http://localhost:51953`, and production Web `http://localhost:51954`. The private environment lives in the parent's temporary snapshot and is never copied into evidence. Every browser case checks the actual database name, API health PID against the allocation, and Web readiness before creating fixture data.

The baseline API reports commit `unknown` because it was built from an archive without Git metadata. This is retained as a limitation. Baseline provenance is the recorded source commit `7af9ab9905c98c77bb091223ebe687c639475bb4` and API binary SHA-256 `06439cee6c7a2b1aa6741f54815ef0e6f170999ee3f7b957aa19231b9999e069`, with API PID `16116` and production Web PID `16117`.

## Baseline observations

The initial harness invocation failed before fixtures because the ESM configuration used `__dirname`. That setup failure is retained in `browser-evidence/baseline-run.log`; the config/test now use `fileURLToPath(import.meta.url)`.

The second baseline invocation completed two cases, one per supported locale: **2 passed in 4.6 seconds**. These are measurement cases that explicitly assert the old defect; they are not repair acceptance.

- The native parent select contains the recent task, while the separate textbox has no inline task list.
- Typing the actual displayed complete identifier returns HTTP 200 with no parents, removing the task from the native select.
- Neither typing nor screenshot capture submits creation, and neither page raised a JavaScript error.
- Screenshots use Chromium viewport capture, with dimensions and document overflow checked before saving: English/Chinese at `1280×950` and `390×844`.

Artifacts: `baseline-{en,zh-Hans}-{wide,narrow}.png`, `baseline-{en,zh-Hans}-observations.json`, `baseline-results.json`, and `baseline-run-2.log`.

## Invocation

Load the allocation's private environment using Node `--env-file`, set `SOURCE_PICKER_BROWSER_PHASE=baseline` or `candidate` only for the child process, and execute:

```text
pnpm exec playwright test --config=.trellis/tasks/11-02-collaboration-source-picker/research/browser.config.ts
```

The enclosing task-only Node runner redirects output to a numbered phase log. Playwright runs one worker, no retries, no trace/video/automatic failure screenshot. Browser artifacts contain synthetic fixture data and no tokens or credentials.

## Candidate verification

The parent rebuilt only the owned services after the baseline captures, compared 13 source-file fingerprints, and explicitly signalled candidate readiness. Candidate API health identifies PID `28253` and base commit `7af9ab9905c98c77bb091223ebe687c639475bb4`, with the task patch bound separately by those fingerprints. API binary SHA-256 is `7f30837a74815e9075b3ebfb4fea42e1c947139a59ee445b93b61be63e645cea`; production Web PID is `28254` and build ID is `XEhBjWwU5E7t46A7-oQSM`. The parent also verified the rendered browser API origin. Both candidate cases independently checked the live API PID and observed the matching binary hash. Full recorded source/build provenance is in `browser-evidence/candidate-provenance.json`.

The first candidate invocation functionally passed two cases in 9.2 seconds, but its metadata helper hashed the retained baseline `api_binary` path rather than the separate `candidate_api_binary`. That evidence error was found during the final provenance assertion; original results/captures remain under `browser-evidence/candidate-run-1/` and the original log is `candidate-run-1.log`.

The helper now selects the phase's actual binary and asserts its hash, the live API health commit/PID, the Web build ID, and the recorded 13-source fingerprint inventory. A bounded replay passed **2 cases in 10.2 seconds**, with no retries: English in 4.6 seconds and Chinese in 4.3 seconds. The replay's observed binary SHA matches the candidate allocation. No product edit or rebuild occurred between candidate runs.

| Behavior | Result in both locales |
| --- | --- |
| Open with no search | Both recent tasks displayed immediately |
| Title, bare number, lower-case complete identifier | Intended task displayed; complete identifier excluded the unrelated task |
| No matches, clear and reopen | Empty feedback shown; default list restored |
| Keyboard selection and parent clearing | Chosen title displayed; clearing restored the optional prompt |
| Nested Escape | Popup closed while creation remained open; focus returned to its trigger |
| One 500 source-search response with original CORS headers | Search error displayed; title, description and chosen parent retained |
| Retry | Real response restored the empty-search feedback and input focus |
| Search/selection versus create | Zero automatic creation POSTs; exactly one explicit POST per case |
| Persisted relationship | The database saved the exact selected parent UUID |

Four candidate screenshots cover the same English/Chinese wide and narrow viewports. PNG dimensions, document overflow and narrow popup bounds passed. Saved images were visually inspected: search and readable task rows form one popup, long titles wrap, and the narrow view preserves the optional-parent prompt/help. No visual follow-up edit was needed. Both pages had zero JavaScript errors.

Final artifacts: `candidate-{en,zh-Hans}-{wide,narrow}.png`, `candidate-{en,zh-Hans}-observations.json`, `candidate-results.json`, `candidate-provenance.json`, and `candidate-run-2.log`. The retained run-1 artifacts remain separate.

The actual creation wire contract spreads the client draft's `initialFields` into the body. The parent relation is therefore asserted at top-level `parent_issue_id`, followed by a database check of the saved row, rather than an invented `initial_fields` wrapper.

The browser cases cover empty clicking, title/number/complete identifier, no matches, clear/reopen, keyboard selection, parent clearing, nested Escape/focus, one transient search error/retry, selected parent and draft preservation, and one explicit creation with the same parent UUID. Delayed-obsolete result matrices remain in the canonical shared component tests. Native Electron and physical touch are outside this Chromium Web evidence.

## Final state and limitations

This lane did not change product code, commit, archive, stop/rebuild services, or drop any database. Baseline ordinary test tasks were cleaned up through `TestApiClient`; both candidate runs' fixture rows and their four created collaboration issues remain together in the disposable owned database for evidence. The parent owns the service shutdown and final visual verdict.

This is Chromium Web evidence, with width emulation rather than physical touch. Native Electron, initial no-tasks/truncation fixtures and the full delayed-obsolete response matrix remain outside these browser cases; focused shared/service tests provide those other contracts. No screenshots, traces or logs contain fixture tokens or credentials.
