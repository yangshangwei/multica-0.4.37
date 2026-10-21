# AC87 dual-client selection boundary — 2026-10-21

Scope: new `e2e/collaboration-selection-boundary.spec.ts` and its private fixture. Product components/helpers, runtime configuration and authoritative evidence/acceptance remain root-owned and read-only here. No new task, replanning, dependency, provider execution, commit or archive.

The actual shared `CollaborationDetail` body and comment render through restricted `RichContent`, which does not register ordinary subtask controls. Ordinary description uses `ContentEditor.currentIssueId`; ordinary comment actions expose `Create sub-issue from here`. This test drives real pointer text selection on production Web and actual Electron and compares both shared selections with an ordinary issue created first, then attached to the collaboration through the public API. The ordinary body toolbar and comment preview stay available. Merely failing to mount a menu cannot satisfy the positive control.

Root released the exclusive native slot after its refreshed AC33 run5 and supplied the owned password environment launcher. The combined acceptance test is **passed**; root owns independent review and ledger integration.

## Exact current execution

Identity: `e2e/collaboration-selection-boundary.spec.ts:12::AC87 Web and actual Electron hide subtask creation for shared body/comment selections while ordinary children retain it`.

Command from the repository root:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env pnpm exec playwright test e2e/collaboration-selection-boundary.spec.ts --workers=1 --retries=0 --project=chromium --reporter=json --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/selection-boundary-run2-output
```

Exit 0; expected 1, unexpected/skipped/flaky 0; retry 0; test 9638ms, total 10853ms, start `2026-10-21T11:45:15.546Z`. Actual Electron main/profile and ordinary native issue window close in `finally`; the exclusive native slot was released to root after this execution.

| Artifact | SHA-256 |
| --- | --- |
| `selection-boundary-run2.json` | `9d417f306cc7fd0d88fc3639a55c2b5121aa1a3077d0fd068cbdc82975d642bf` |
| `selection-boundary-run2-identities.json` | `8dd6215133adbb18f667e8b7f3b43f6c4fa33efced6fd12318a0e1f1ba5fc17b` |
| `selection-boundary-preflight-run2.json` | `8c1bb70dcfbad14702d80bae80a0ea28adcb40b9974a79eb319636c72f900036` |
| `selection-boundary-run2-command.json` | `5fccb99a2f75089006ba568cf54bf36bb993b800a0387c241049e84ea1f31f73` |
| `selection-boundary-run2.stderr` | `1a585257c58d9c0b02938c759c56c62971c922be22bdf610a77bfbaa9528539f` |

All artifacts above are in this directory. The decoded identities match the JSON reporter's inline attachment. No password/token/capture-token bytes are written into them. Stderr contains only the existing pnpm configuration warning.

## Whole-criterion clauses

- Web and Electron independently select the actual collaboration body and comment with mouse movement through the rendered text bounds. Both selections match the full intended text. Neither surface has a create-sub-issue button/menu text or visible ordinary bubble menu; no sub-issue-preview, sub-issues or source-context request occurs.
- The same synthetic owner first creates an ordinary issue, then attaches it to the collaboration with public `PUT /api/issues/{id}`. The response retains an ordinary marker and exact parent. Thus its positive selection behavior is not inherited from a plain unrelated fixture or a fake response.
- Both actual clients select that ordinary child's body. The real editor toolbar is visible and exposes `Create sub-issue from selection`. Both select its comment, open the observed `Comment actions` menu and see `Create sub-issue from here`; clicking sends one actual preview GET that returns 200 for the ordinary child. No create/capture mutation is issued by this UI test.
- Before/after collaboration detail snapshots are identical, including participants/current handler/lifecycle/execution. No daemon/provider process is started; no account/space privilege or gate is added. Both clients have no page errors, and account native requests contain no ambient workspace headers.
- The existing canonical DB-backed manual/agent/preview refusal and ordinary-child capture/create tests are current completed executions, listed below. They prove no file stream/copy, source-context row or task side effect on refusal, and ordinary capture/create remains viable. The UI test owns selection/affordance wiring; it does not duplicate the storage/fault matrix through DOM.

These combined current identities support **AC87 at P3**, with Web/actual Electron wiring. No unfulfilled clause was identified in this slice; root must independently review before promotion.

## Canonical service/handler evidence

Actual PASS records in the final continuation Go JSONL artifacts:

| Test identity | Source | Raw result / elapsed |
| --- | --- | --- |
| `internal/handler::TestSourceContextCollaborationPreviewIsDenied` | `server/internal/handler/source_context_collaboration_test.go:18` | `go-other-packages.jsonl` / 0.02s |
| `internal/handler::TestSourceContextCollaborationBothModesRejectBeforeCopyOrAgentLookup` | `server/internal/handler/source_context_collaboration_test.go:58` | `go-other-packages.jsonl` / 0.13s |
| `internal/handler::TestSourceContextOrdinaryChildHTTPStillCreatesOwnSubtask` | `server/internal/handler/source_context_collaboration_test.go:113` | `go-other-packages.jsonl` / 0.08s |
| `internal/service::TestSourceContextCollaborationBuildersDenyBeforeProjection` | `server/internal/service/source_context_collaboration_test.go:81` | `go-service-partition-2.jsonl` / 0.23s |
| `internal/service::TestSourceContextCollaborationDirectWritersCannotUsePrebuiltCapture` | `server/internal/service/source_context_collaboration_test.go:127` | `go-service-partition-2.jsonl` / 0.29s |
| `internal/service::TestSourceContextOrdinaryChildRetainsOwnCaptureAndWorkspaceScope` | `server/internal/service/source_context_collaboration_test.go:107` | `go-service-partition-3.jsonl` / 0.13s |

Handler source SHA is `e2e9f092be472a2dd0740755fc803584a92b31966daca621bfd283904d7e421a`; service source SHA is `a146e35b0a06c04f03445215c4a459bfa02cbea2449b9dde2f314261b30adf65`. Raw result hashes match the existing catalog: other `1e71354c2c8743c79c02081dba8450981af0e2b91c699afa1ded78cb470e37f8`, service2 `1a1edb5507cf4a6b4259194d2d8abfe5c1d2ebb9f56672f52353d999ef07d99d`, service3 `876f6ca2aea52576b2d3da19c3da33abe7de67e30969af1d99af13987270623d`. This lane read those passing assertions/results; it did not rerun the full Go suites.

## Source and environment provenance

Owned password API: `localhost:18996`, health PID **68800**, commit label `8ecb98883`; source/configuration fingerprints are preserved in `selection-boundary-preflight-run2.json` from root's post-extraction `ac33-run5-preflight.json`. Production Web: `localhost:13916`, build `tP4r55FoDsuyXfi1EylGw`. Actual Electron renderer: `localhost:6090`; fresh HTTP 200 bytes match built index SHA `337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c` before each run. No rebuild/restart/configuration mutation was performed here.

Fixture and browser sources used by the successful run:

| File | SHA-256 |
| --- | --- |
| `e2e/collaboration-selection-boundary.spec.ts` | `4f7ea2af13cfaba755f40ab6c32b74e0133025d5bb477386a154d261f5c4d9dc` |
| `e2e/fixtures/collaboration-selection-boundary.ts` | `684a7869f9a368036f11989598f15db93f26741ef51a3c0ed037e4632a1fb12a` |

## Retained first attempt

`selection-boundary-run1.json`, `.stderr`, `-command.json`, and `selection-boundary-preflight-run1.json` remain. Run1 passed both Web/Electron collaboration selection negatives, then failed ordinary description selection because the pointer start lay outside `.ProseMirror`; the editor container deliberately calls `preventDefault` on an outer click. The fixture moved start/end 0.5 pixels inside the actual text, and used the observed `Comment actions` name in the ordinary comment region. Product code and assertions were not weakened. Run2 then passed the complete positive/negative flow. This is test-interaction evidence, not a product defect or a concealed retry (`--retries=0` for both runs).

## Static verification

Commands against the final two source files:

```sh
pnpm exec eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true, includeTypes: false}]' e2e/collaboration-selection-boundary.spec.ts e2e/fixtures/collaboration-selection-boundary.ts
pnpm exec tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 --skipLibCheck --strict --esModuleInterop --types node e2e/collaboration-selection-boundary.spec.ts e2e/fixtures/collaboration-selection-boundary.ts
pnpm exec playwright test e2e/collaboration-selection-boundary.spec.ts --list --project=chromium --reporter=list
```

All exit 0; no lint/type diagnostics; discovery is one combined test in one file. Existing pnpm settings warning remains. Task context/JSON/hash/report whitespace validation follows this report write. No new helper test duplicates the actual browser interaction; this E2E is the acceptance evidence.

Changed files: the new spec, its fixture, this report and named selection-boundary raw/preflight/command/decoded evidence files. Simplification: no product code or abstraction/dependency changes. Remaining boundaries: actual Electron uses the established production IPC/preload/renderer harness, not installed full Main/installer; no provider account/OS containment or full unified-check claim. Task/parent remain in progress and creation/execution rollout remains closed.
