# Final UX acceptance evidence review

**PASS for the scoped five-case UX acceptance.** Independently reviewed the raw Playwright JSON, retained run summaries, configuration inputs, attached business/geometry evidence, source hashes, public screenshot copies and final resource audit on 2026-11-05. No blocking evidence discrepancy remains. This review ran no tests, services, browsers, database queries or builds.

The result is one recipient case from `ux01` plus four remaining cases from `ux02`. The first run remains `interrupted` / `NOT_PASSED`; it must not be described as a successful complete first run. The original frozen core campaign and its outstanding results are separate.

## Exact executed inventory

| Run / spec | Exact test title | Test duration | Result |
| --- | --- | ---: | --- |
| ux01 / recipient | Web recipient picker exposes self/recent/search results and preserves the outer dialog on Escape | 3,196 ms | PASS |
| ux02 / delivery | Web creates one private issue after lost upload/create responses and revokes an open preview through account WS | 5,054 ms | PASS |
| ux02 / delivery | Web explicitly resubmits rejected creation input without losing files or reviving the rejected key | 2,018 ms | PASS |
| ux02 / admin-web | Web administers disabled-creator collaborations with explicit succession and original receipt recovery | 4,031 ms | PASS |
| ux02 / admin-desktop | Desktop administers disabled-creator collaborations in the account shell and recovers the retained receipt | 8,882 ms | PASS |

Each exact identity has one result: `status=passed`, `retry=0`, `expectedStatus=passed`, with no errors or skip annotations. All four raw reports declare one worker, `fullyParallel=false`, Chromium project retries zero, and zero skipped/unexpected/flaky cases or global errors. These are five actual cases in four spec files, not collection-only results. The recipient framework duration is 4.475 seconds; the table uses its test-body duration.

Raw reports are private under `/Volumes/artisan/e2e-private/ux-acceptance-20261105/runs/`:

| Report | SHA-256 |
| --- | --- |
| `ux01/recipient/playwright.json` | `e0800009ba241320408f80479dc75dc26cd87d73df9c5c750cf8e7260c1aeb82` |
| `ux02/delivery/playwright.json` | `d71abbf8bf36fe632bf17a95dae9ad2a879c12b15a04dd7e7c8e998f6f881968` |
| `ux02/admin-web/playwright.json` | `df6ad46cd76658e0f266fda77171f008ebc19c508410b0d1c3dddc4b8f586bd6` |
| `ux02/admin-desktop/playwright.json` | `eed93f2ca3d7f85a58e6e827de6613e51e769ac830a2dd322fbeb9a81b7be8a6` |

## Source, deployment and authentication evidence

Both runs identify candidate `49e99115610fad4c88447d9580cac24498d49a1a`, source fingerprint `7c24721a6c44bbd87926ec50769750149c77b0160ae9829e638efc281a0bb3d3`, artifact fingerprint `2f85e2dd32bef30d8db3a31b0a024048f206b5daffad6cae4622ebf348f7d11a`, and production Web build `3h0wxk4pNc_v3fF1eSijE`. Every completed lane records unchanged source/artifacts; both final audits are true. All four current isolated spec hashes still equal the preflight-reviewed hashes, including the exact attachment locator patch.

The reviewed runner validates its actual API PID/commit and `/api/config` auth mode before fixtures. The retained private input configurations agree with each lane: legacy for recipient/delivery; password with single-process limiter and platform administration enabled for admin Web/Desktop. Device/Cloud authentication and knowledge execution are disabled. Browser/API/WS inputs agree on ports 13681/18681, with the native renderer on 5861. Per-lane HOME/TMPDIR and databases are private; configuration files are mode 0600. No credential values were copied into this report. Raw `/api/config` response bodies are not separately retained; live mode validation is represented by the reviewed runner's mandatory check and the successful real authentication/administration paths.

`runtime-client-proof-resume.json` binds the resumed Web/renderer listener PIDs and checkout directories to the same build/preload/renderer hashes and records rendered API/WS alignment. This reviewer checked the retained proof, without probing listeners after the window was handed to the next owner.

Every completed lane began with 752 migrations, zero users/issues/admin roles and gate values `creation_enabled=true`, `enabled=false`, `host_execution_enabled=false`; those gate values and epoch remain unchanged afterward. The two admin lanes use separate fresh databases and synthetic administrators.

## Business and native evidence

- The passed recipient source contains the reviewed held real search response, pending Enter/no accidental create assertions, exact second-account UUID selection, explicit single creation and independent owner/recipient reads. Its unchanged hash binds those assertions to this execution.
- Delivery evidence contains one creation POST, a real account WebSocket and issue access-change events, with no ambient workspace headers. The final attachment records two upload POSTs because the scenario separately uploads the creation file and a later comment file; the earlier creation recovery assertion still requires exactly one initial upload. Byte/range/full-download, retained reference and revoked capability assertions remain intact. The correction attachment confirms distinct rejected/corrected request keys and retention of the original refusal.
- Both admin attachments contain four exact lifecycle receipts and four persisted ledger operations, explicit succession, ordinary-content reads refused with 404, and a new lifecycle write refused with 409 after creator restoration. Web additionally records a deliberately dropped committed response recovered through the original receipt. Desktop records original receipt recovery after remount without claiming the same injected loss. These are real API/admin operations, not mocked admin projections; both report zero provider runs.
- The Desktop case launches actual Electron using the production renderer, preload and selected production routing/IPC through the reviewed test Main harness. Its passed assertions include zero daemon starts and no ordinary collaboration detail reads by the administrator. It does not establish installed or packaged full Main behavior.
- All 20 public PNG files are byte-identical to their private originals and match `evidence/screenshots.json`: one recipient, one revoked-preview, eight Web admin and ten native admin captures. All six native geometry records retain stable layout, unchanged geometry during capture and full target coverage, including real native zoom factor 2. This is artifact/geometry verification, not a fresh visual-design or accessibility audit.

## First interruption and cleanup

`ux01` retains the original `OSError: [Errno 48] Address already in use` after recipient PASS and before delivery API/test startup. Its delivery row is `setup`, with no API log or Playwright report. The parent diagnosed TIME_WAIT with no listener and retained the successful reuse-bind control plus refusal of a real listener in `port-rebind-diagnosis.json`.

The current runner hash is `588fd01246e07a39f378219c36879f75bd55526df53311b76818d61cd9ff0122`. Removing only the added `SO_REUSEADDR` call and its formatting reconstructs the previously reviewed hash `96d9e32e9c9ad943b08f62d7d35c84c76ddf05a5bcd86c9aabc5db3195980998`. No test assertion or product behavior changed for the continuation. `ux02` used new databases and did not repeat the passed recipient case.

Each completed lane and both final runner records report zero remaining browser processes and database connections. `final-resource-audit.json` records all seven owned client PIDs stopped, zero listeners on 18681/13681/5861, and zero connections for all six owned databases: the template, recipient clone, unused ux01 delivery clone, and three ux02 clones. Data is retained for evidence. The latest window record shows the explicit release to runtime H1; this review started no competing work.

## Findings (fixed)

The preflight-approved isolated delivery locator fix was exercised successfully. No further source repair was needed during the five actual cases. The parent corrected only the private port preflight between ux01 and ux02, preserving the first interruption.

## Findings (not fixed) and scope

No unresolved blocking finding for this assigned increment. Installed/full Main, commercial models/providers, Windows/iOS, physical touch, screen-reader behavior, new bilingual real-admin status assertions and the separate OtherTarget directory-failure branch remain outside these five cases. Prior 312-test/54-capture evidence keeps its own identity. This result does not close the frozen core campaign's Linux/performance failures or establish whole-repository acceptance.

## Verification

- Recorded tests: PASS, five exact executions, zero skips, zero automatic retries.
- Raw report/summary/source/runner-delta/screenshot/geometry consistency: PASS.
- Recorded final resource cleanup: PASS, six databases at zero connections, three ports without listeners, zero remaining owned browsers.
- Lint and TypeCheck: NOT_RUN in this evidence-review pass; prior checks were not repeated or relabeled.
- Reviewer changes: this report only. No products, tests, credentials, other-owner records or runtime resources were changed in this final-review pass.
