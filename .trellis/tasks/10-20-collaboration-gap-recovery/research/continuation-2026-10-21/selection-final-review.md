# AC87 selection boundary — independent final review

Reviewed 2026-10-21. **Approve full AC87 at its primary P3 content boundary.** No defect or missing clause was found in the current evidence. The coordinator's promotion to passed is supported. This reviewer wrote only this report; source, environment, database, native slot, matrix and other agents' files remained read-only.

## Actual Web/Electron selection evidence

`e2e/collaboration-selection-boundary.spec.ts:12` is one combined test using production Web and an actual Electron BrowserWindow/production renderer through the existing native IPC harness. The final raw `selection-boundary-run2.json` reports **1 passed, 0 failed/skipped/flaky, retry 0**, test 9.638 s, total 10.853 s, start `2026-10-21T11:45:15.546Z`.

Raw report SHA-256 `9d417f306cc7fd0d88fc3639a55c2b5121aa1a3077d0fd068cbdc82975d642bf` matches the documented run. The decoded inline `selection-boundary-identities` attachment is structurally identical to the safe identity file, SHA-256 `8dd6215133adbb18f667e8b7f3b43f6c4fa33efced6fd12318a0e1f1ba5fc17b`. Preflight SHA-256 `8c1bb70dcfbad14702d80bae80a0ea28adcb40b9974a79eb319636c72f900036`, command SHA-256 `5fccb99a2f75089006ba568cf54bf36bb993b800a0387c241049e84ea1f31f73` and stderr SHA-256 also match [selection-boundary.md](selection-boundary.md). Stderr contains only the existing pnpm ignored package-field warning.

The reviewer independently checked all **eight actual pointer selections**, two shared and two ordinary surfaces per client. `selectText` measures the actual visible text range, then drives `page.mouse` down/move/up; it does not set a DOM selection programmatically. All retained `selected` strings equal their intended markers. Both shared body/comment selections expose zero subtask-create entries; native shared requests remain empty. The spec additionally checks Web's request list immediately after shared selection and rechecks the native shared list after the ordinary control.

The positive control is the same ordinary child created first via `/api/issues`, then attached via actual `PUT /api/issues/{id}` with a confirmed parent and absent collaboration marker. Web and Electron both select its body and see the actual selection toolbar, then select its comment and open the observed Comment actions menu. Each displays two ordinary subtask affordances and sends exactly one real preview GET returning 200 for the ordinary child's exact issue ID. The preview token itself is omitted from evidence. No capture/create mutation is made by this browser test.

Actual issue `01a15e47-a91a-7af0-bdbd-838369138b4a`, shared comment `b403eb5b-6281-4f6c-a9f9-168cd02d2699`, ordinary child `01a15e47-a94e-7ac9-8eb1-d71f5c641233`, ordinary comment `01a15e47-a96a-72ab-88c0-5d3ae297eeaa`, actor `42a2ab3a-60ae-48d4-8244-d0c66ae31eed` are consistent across the safe/raw attachment. Full collaboration detail snapshots compare equal before/after both shared and ordinary flows. The passing assertions require zero page errors, nonempty native account requests with no ambient workspace headers and zero native daemon starts.

Source inspection confirms shared body/comment use restricted `RichContent` without ordinary editor subtask registration. Ordinary issue `ContentEditor` receives `currentIssueId`; comment actions retain their existing ordinary behavior. The positive controls therefore detect failure to mount the normal ordinary menu, rather than accepting any absent menu as proof of correct suppression.

## Canonical backend clauses

All six canonical identities have actual pass events in the current continuation Go artifacts. Their source hashes still match: handler `e2e9f092be472a2dd0740755fc803584a92b31966daca621bfd283904d7e421a`, service `a146e35b0a06c04f03445215c4a459bfa02cbea2449b9dde2f314261b30adf65`. The reviewer parsed raw events and reviewed the assertions directly.

| Identity | Result source / elapsed | Clause |
| --- | --- | --- |
| `TestSourceContextCollaborationPreviewIsDenied` | `go-other-packages.jsonl` / 0.02 s | Actual preview returns content-free 409 `unsupported_collaboration_action`. |
| `TestSourceContextCollaborationBothModesRejectBeforeCopyOrAgentLookup` | same / 0.13 s | Manual, agent and preview reject; injected relation failure returns503. Zero stream/copy, attachment/agent lookup, source-context/object-intent/task rows or extra issue/attachment rows. |
| `TestSourceContextOrdinaryChildHTTPStillCreatesOwnSubtask` | same / 0.08 s | Ordinary attached child preview200 and actual manual creation201 record its own source context, with no parent-private content or shared/task inheritance. |
| `TestSourceContextCollaborationBuildersDenyBeforeProjection` | `go-service-partition-2.jsonl` / 0.23 s | Both builders reject the resolved shared source before workspace/thread/attachment projection; fault fails closed. |
| `TestSourceContextCollaborationDirectWritersCannotUsePrebuiltCapture` | same / 0.29 s | Previously valid captures cannot bypass manual/agent writer backstops; no child/context/copied attachment/task rows. |
| `TestSourceContextOrdinaryChildRetainsOwnCaptureAndWorkspaceScope` | `go-service-partition-3.jsonl` / 0.13 s | Normal child captures only its own content; wrong-workspace request refuses before projection. |

Raw hashes independently match: other packages `1e71354c2c8743c79c02081dba8450981af0e2b91c699afa1ded78cb470e37f8`, service partition2 `1a1edb5507cf4a6b4259194d2d8abfe5c1d2ebb9f56672f52353d999ef07d99d`, partition3 `876f6ca2aea52576b2d3da19c3da33abe7de67e30969af1d99af13987270623d`. None of the six identities is a skip or failure in those final runs. Counts for unrelated skipped platform tests are not used to support this criterion.

## Provenance and fresh verification

Final preflight records owned API PID68800, source ID `3315c0bc363e1fa5a6b62b8316fabaaa494e9dbb179fa981dd467d1562c19265`, configuration ID `805fc993af4daf895e4de11a22cd372244d7845eed807b7eaa56a50f30b39cd3`, password mode. Production Web build `tP4r55FoDsuyXfi1EylGw` and renderer HTTP200/index SHA-256 `337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c` identify the same task-owned environment. The historical API commit label is complemented by source fingerprint; no rollout/provider-account changes occurred.

Final browser/fixture hashes match the run and current source: spec `4f7ea2af13cfaba755f40ab6c32b74e0133025d5bb477386a154d261f5c4d9dc`, fixture `684a7869f9a368036f11989598f15db93f26741ef51a3c0ed037e4632a1fb12a`.

Fresh read-only reviewer checks:

```sh
pnpm exec eslint --config packages/core/eslint.config.mjs \
  --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true, includeTypes: false}]' \
  e2e/collaboration-selection-boundary.spec.ts e2e/fixtures/collaboration-selection-boundary.ts
pnpm exec tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 \
  --skipLibCheck --strict --esModuleInterop --types node \
  e2e/collaboration-selection-boundary.spec.ts e2e/fixtures/collaboration-selection-boundary.ts
pnpm exec playwright test e2e/collaboration-selection-boundary.spec.ts --list --project=chromium --reporter=list
```

All exit0, lint/type diagnostics empty; discovery is one combined test. Only the existing pnpm warning appears. Source/raw-attachment/hash/selection/request invariant checks also pass. No DB/native/browser/full-suite rerun was performed.

Run1 remains failed test-interaction evidence: Web/Electron shared negatives passed, but ordinary editor pointer origin was outside `.ProseMirror`; the fixture moved 0.5px inside text and used the actual Comment actions name. Production/assertions stayed intact, and run2 completed both positive controls with retries disabled. Do not count run1 as passing or a product defect.

## Findings and limits

No fixes or unresolved clause gaps. Existing root [selection-boundary-independent-review.json](selection-boundary-independent-review.json) reaches the same promotion verdict; this report independently confirms its eight selections, exact preview/request facts and canonical backend support.

Native evidence uses the actual Electron IPC/preload/renderer acceptance harness, not an installed full Main/installer. The test requires neither provider execution nor other-OS containment. Those scopes, external-storage faults and full 97-item acceptance remain separate; production creation/execution rollout stays closed. No speculative warning or additional suite is required for AC87.

Changed file: `research/continuation-2026-10-21/selection-final-review.md` only. No production simplification or dependency was introduced.
