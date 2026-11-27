# Shared frontend implementation — 2026-11-27

Implemented runtime-only settings and strict v1 writes, one explicit inspector RuntimePicker, protected runtime inventory and retained editor recovery, old pending-request refusal with deliberate reload and preserved original request history, optional operation status/runtime display and English/Chinese copy. Removed App/VCS and Agent queries from material setup. No new dependencies or platform routes.

Evidence: initial schema regression 5 failures / 7 passes before implementation. Final core materials: 15 tests passed; shared material UI and existing inspector picker: 34 tests passed. Full core and views typecheck passed. ESLint on changed core/materials and four shared material components/tests passed. Checks run through pnpm package scripts.

Existing schema/approval, auto=false/manual plan, exact-request retry, private-data clearing and retained-focus tests remain. Added both providers without integrations, explicit real picker selection, removed/offline runtime, inventory 503/403, old-server protocol and old pending intent recovery coverage.

Remaining integration: production browser/Electron visual evidence and real queue-to-fake-runtime-to-daemon observation are not established by these component tests. Live model/provider execution not performed.

## Continuation verification — 2026-11-27

Revalidated the existing frontend implementation without additional product UI edits: core material suites 15 passed; shared material and real inspector picker suites 34 passed; core/views typecheck and scoped ESLint passed. Root `pnpm lint` and `pnpm typecheck` passed, followed by `pnpm exec turbo lint typecheck --force --concurrency=2`: 17 successful tasks, zero cached, existing warnings only. Fresh isolated Next production build and Electron renderer/preload build passed.

Expanded deterministic browser acceptance to both providers. `material-github.mjs` implements a local GitHub-shaped immutable commit/tree/blob, PR, review-note and current-head merge store. The selected fake native executable uses that provider's write requests; the separate fake gh/glab transport reads resulting state. Both fixtures retain the actual merge timestamp. No final receipt or workflow database state is manufactured. The GitHub resource keeps its github.com identity while all test executable traffic maps exclusively to loopback.

Fresh per-provider database results:

- GitLab: `multica_gitmaterial_ui_20261127_gitlab_dbf6d4ae`; Web and native Electron both passed, 2.2m. Evidence: `/private/tmp/multica-material-runtime-ui-Y7BDfo/gitlab-replay/material-run-KVTnYc`.
- GitHub: `multica_gitmaterial_ui_20261127_github_74fc47de`; Web and native Electron both passed, 2.1m. Evidence: `/private/tmp/multica-material-runtime-ui-Y7BDfo/github-replay/material-run-kaHcOL`.
- Two standalone provider fixture tests passed, covering immutable UTF-8 files, exact-head merge refusal and retained decisions; the GitLab case also covers uncertain mutation replies.
- Visual comparison with the previous accepted material screenshot scored 94/pass. See `frontend-visual-verdict.json`; native and Web screenshots remain in each browser report.

The first GitLab run passed Web but failed Desktop before selecting a runtime: the native shell fetched an empty inventory before the fixture registered its runtime directly. Reloading the native page after setup ensures a fresh real API inventory; no runtime auto-selection or product access behavior changed. Original failure retained at `/private/tmp/multica-material-runtime-ui-Y7BDfo/material-run-RqvyJj`.

These browser passes precede the independent final code review and are intermediate evidence. The runner now records start/end source hashes, actual API/daemon binary hashes, Web build ID and Desktop artifact hashes, and fails source drift. A final frozen-source replay will follow any checker fixes. Live accounts, model output quality, external repositories, packaged Electron Main/installers and actual provider deployments remain untested.

Backend recovery integration added a `no_effect` operation status. Both locales now explain that a remote change was not found and require an explicit resume. A red/green shared-view regression verifies no false "Operation verified" label, no background write and the exact version-bound human retry. Final focused view/picker count is 35 passed; views typecheck and test-file ESLint passed afterward. The existing native-check blocker matcher already covers `material_native_checks_pending`.

Fresh Web and Desktop production builds including this copy passed. Frontend source hashes recorded before building and verified unchanged afterward (`frontend-build-source.json`, `frontend-build-stability.json`, stable true) under the private evidence root. No layout edit followed the passing visual verdict.

Two attempted final GitLab replays also passed both Web/Desktop scenarios, but were deliberately rejected by the source guard because independent review was still changing backend recovery/disable behavior and its regression tests. They are not final acceptance evidence: `gitlab-final/material-run-skEFya` (2.1m, service + test drift) and `gitlab-final2/material-run-yTl9M5` (2.6m, test-source drift). Both retained manifests and successful owned-process cleanup. The queued GitHub run did not start after either source-guard rejection; its fresh database contains migrations only. Waiting for the completed independent check before the final run avoids blessing a moving source snapshot.

## Final post-review acceptance

Independent review finished before these runs. Both provider/client pairs passed with zero source drift during each run and a final comparison against current source:

| Provider | Database | Production Web / native Desktop | Evidence |
| --- | --- | --- | --- |
| GitLab | `multica_gitmaterial_ui_20261127_gitlab_6bdfbe18` | 2 passed, 2.0m | `/private/tmp/multica-material-runtime-ui-Y7BDfo/gitlab-final3/material-run-9UmYiA` |
| GitHub | `multica_gitmaterial_ui_20261127_github_55d59404` | 2 passed, 2.0m | `/private/tmp/multica-material-runtime-ui-Y7BDfo/github-final/material-run-npcJe1` |

Each report includes source-before/source-after/source-stability, real built API and daemon SHA-256, Next build ID, Desktop renderer/preload hashes, screenshots and owned-process exit receipts. Frontend artifacts match the separately retained frontend build-source/stability manifest. `frontend-final-browser.json` contains the concise public evidence index without credentials.

Both final settings screenshots were inspected: a single explicit runtime, Git platform/resource, human reviewers and auto-off setting remain legible and consistent across Web/Desktop. Visual verdict stays 94/pass. The actual daemon fixture uses a test-created native executable; the inherited fixture device subtitle says "Test fixture; no daemon process" and is static fixture metadata, not execution evidence. Native process/receipt logs and immutable remote snapshots establish execution.

All owned API/Web/daemon/Electron/fixture processes have stopped. The dedicated PostgreSQL container `material-runtime-ui-2e6c1f97` is stopped with its databases and logs retained. No live model/account/repository was used. Planning content in this browser journey is entered by the human fixture after a verified native planning launch; real model-generated design quality remains a separate acceptance item.
