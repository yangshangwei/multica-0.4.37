# Extras-B R02 run-records locator diagnosis

Date: 2026-11-19. Active task: `11-13-pay-241-s3-recovery`.

Both notification failures are caused by a fixture locator that assumes the
records disclosure is closed. Selecting the original notification opens its
exact run automatically. The real button therefore has the accessible name
`Hide records for <run UUID>`, while the fixture waits only for
`View records for <run UUID>`. The correction matches the two existing state
labels for that exact run and retains the original conditional click and all
record-content assertions. No product markup or translations change.

## Retained R02 result

Evidence root:
`/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly`.

`verification/runtime-extras-b-r02/result.json` records exit 1 with no source,
configuration or tool drift. Its `stdout.log` SHA256 is
`ee1988baa174d05b7dc9999facf68f61de273d132c0148a7d1228dea7c3312aa`.

R02 has **2 PASS / 2 FAIL top-level cases**, retry 0. Both Web/Desktop withdrawal
tops pass their two authored subcases. Each notification top fails its first
`proven_failed_or_incomplete_result` subcase, leaving success and Stop unrun.
The ten-subcase inventory is **4 PASS / 2 FAIL / 4 NOT_RUN**. This remains a
failed complete round; the withdrawal passes are not substituted for a passing
notification composition or full S3 acceptance.

| Canonical case artifact under `runtime/extras-b-r02/results/<case-directory>/` | SHA256 |
| --- | --- |
| `web-runtime-notifications.json` | `c496098cdea828445c12f92d417901cc23274102920c203e4379332f7517dab6` |
| `desktop-runtime-notifications.json` | `7700c1622ce29e2838eb24b2b7c683bae6d8b777bd1872d03a339df898141bdd` |
| `web-runtime-withdrawal.json` | `ec7bc20e311e479579daedbf6cd1dc6f824ab5ed8c6fe3ddbb1d9fc64de91748` |
| `desktop-runtime-withdrawal.json` | `83f07116352d8c970013115f35d8753329875945d6b1af8e993e1fde8ed8e0d3` |

Each notification top reaches its real initial issue Resume, first original
input, controlled failure, held original proof, original-ledger poller restart,
and observer reauthentication. Its exact notification destination is selected
before the locator failure. The notification helper's preceding assertions for
the exact run anchor, focused disclosure and `Managed execution ended` complete;
the timeout occurs at `showResultNotification():688`, reading `aria-expanded`
from the closed-state-only button locator.

The failed run identities are:

- Web: `a30b43d1-ba40-4ead-868e-b3b8c08ff197`; locator timeout 15,000 ms.
- Desktop: `df0ef286-e5ec-4444-9c32-bf396af38cd4`; locator timeout 30,000 ms.

Across the four retained case artifacts, managed Provider observations total
**6 of the intended 10**, final original ledger rows total 6, and original human
input checkpoints total **8 of the intended 12**. Each withdrawal top has two
Provider observations/ledger rows and three inputs; each notification top has
one Provider observation/ledger row and one input. Later notification success,
Stop, complete failed-to-success history comparison and the complete final
notification denominator are unrun. The failed notification subcase does not
return its acceptance result, and post-locator replay/content checks remain
unexecuted. Original R01/R02 failures, source trees, receipts and data are retained.

## Actual UI structure and evidence

Inspected source: `source/client-harness-runtime-r02`. The same run-records UI
and English labels are present in retained accepted client origin
`/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/source-block-ui-run-r05`.
The source files are byte-identical between those trees:

- `packages/views/collaboration/run-records.tsx`: SHA256
  `8453df243b24c976b84a075a11c7a0fff08b3f84e0b99d8bf79c8293333d5ce8`.
- `packages/views/locales/en/collaboration.json`: SHA256
  `e02fd7c2281fe5bee91cca8f0a0c78cddc80cd36694e835259ad258690548ce0`.

`CollaborationRunDisclosure` initializes `open` from
`selectedRunId === run.id`, and its effect opens the same run when the selection
changes (`run-records.tsx:27–28`). The outer element is the focusable
`li#run-<UUID>` (`:43`). Its one disclosure button has `aria-expanded={open}`,
`aria-controls=records-<UUID>` and a state-dependent accessible name (`:52–55`).
The English labels at `collaboration.json:154–159` are exactly:

- Closed: visible `View records`, accessible `View records for {{id}}`.
- Open: visible `Hide records`, accessible `Hide records for {{id}}`.

While open, the run's actual records section is mounted with
`id=records-<UUID>` and accessible region name `Run records`
(`run-records.tsx:58`, `:74`). The label change is expected disclosure behavior.

Retained screenshots independently show the open state: both display
`Hide records` and the rendered records metadata, including failed run/group
status, complete recording and managed execution ended. They are:

- `runtime/extras-b-r02/results/collaboration-recovery-run-50f7a-h-original-receipt-recovery-chromium/web-runtime-notifications-failure-0.png`,
  SHA256 `4b75a32c2a69f41f7bf44eeda23cd2761759adf013730560c3f56aa11e2e2cca`.
- `runtime/extras-b-r02/results/collaboration-recovery-run-e0697-h-original-receipt-recovery-chromium/desktop-runtime-notifications-failure-0.png`,
  SHA256 `b29d82b207a9808ed3d83c0a4badca43f072c0237e72dc842006d97c281fb194`.

The screenshots corroborate visible state; the source establishes the exact
accessible name and UUID binding. No live browser, DOM mutation, API request,
Provider or database operation was used for this diagnosis.

## Minimal fixture correction and verification handoff

Only one runtime-fixture line changes:

```diff
- const view = disclosure.getByRole("button", { name: `View records for ${runId}`, exact: true });
+ const view = disclosure.getByRole("button", { name: new RegExp(`^(?:View|Hide) records for ${runId}$`) });
```

The matcher remains scoped to the exact focused run disclosure and anchored to
its full UUID. It admits only the two source-defined names of that same button.
The unchanged `aria-expanded` check clicks only when the panel is closed,
avoiding an unintended close after notification navigation.

Exact notification kind/recipient/run/event identity, destination and focus,
managed proof labels, `Run records` region, `Direct launches: 1`, host execution
mode and controlled success/failure text remain required. No content assertion,
200-response check, original request/nonce, initial issue Resume, scoped Stop,
reconnect/recovery, late-result rejection, prior failed-run preservation,
withdrawal assertion, planned denominator or budget changes.

The corrected ROOT fixture is
`e2e/fixtures/collaboration-recovery-runtime.ts`, mode `0644`, SHA256
`1669c8356a4cc01720753e5df7280e76fdcf58b19e66da620748fe5ab6c3959c`.
Its retained R02 SHA256 was
`b668eab5c0d00969e67871a66865029ed7a41bef6be8fd59603a4f045b085e7d`.
A source-delta check proves that replacing this one line reproduces the new
file exactly; all other runtime-fixture bytes remain unchanged. Shared helpers,
product/UI source, translations and top-level spec/configuration are untouched.

This lane performed retained source/JSON/hash/screenshot inspection and bounded
delta checks. It ran no Go, TypeScript, DB, API, live browser, Provider, process
control or commit. Main owns a new frozen harness, targeted statics and the
fresh complete four-top runtime run. The locator correction is prepared for
that verification and does not turn retained R02 into a passing round.
