# Shared initialization implementation

Implemented in the dedicated material-repository-initialization worktree after frozen `contracts.md`/`server/internal/material/repository.go`.

- Added scoped strict initialization schemas, typed API methods, protected polling and monotonic original-command mutations. Unknown protocol/status/file actions cannot authorize review or publication; conflict-only candidates and historical proof metadata remain readable.
- Added shared project initialization panel before ordinary material list/detail: complete file list, inert before/after source, exact head/base/catalog identity, separately scoped owner/admin decisions, explicit current-candidate re-review, check/retry and advertised-version maintenance. Initial configuration never triggers a frontend duplicate check.
- Kept draft editing separate from publication. The ready checkbox requires current server `can_publish`, supported initialization protocol, matching settings version and complete accepted proof. Old/failed endpoint reads leave draft save available, never infer initialization from a past material baseline.
- Preserved mounted review input during transient errors, exact pending intent across remount/explicit retry, and private cleanup on project denial. No second runtime selector, App lookup, arbitrary rules upload or invented bundle version.
- Extended both provider browser fixtures for recursive complete trees, 20-file catalog writes and GitLab raw blobs. Existing ordinary material journey now initializes via the actual queue/native-fake/daemon observation path first, verifies README preservation and then follows the original requirement/manual-plan path.

Verification so far: 22 core materials tests; 38 shared materials/inspector-picker tests; full core/views typecheck; scoped ESLint; both provider fixture tests. The earlier retained focused failure was test notification timing and corrected by awaiting the actual updated query render; implementation preserves exact pending review state.

Fresh private browser allocation: `/private/tmp/multica-material-init-ui-uEHFqY`, container `material-init-ui-90276a8f`, DB `multica_gitmaterial_ui_init_90276a8f`. Initial production Next and Desktop builds passed. Incremental final builds include parser null-history/conflict handling; final independent review and both-provider Web/Desktop acceptance remain pending. No live provider/account/model used.

## Readiness for final integration

Full forced lint/typecheck passed: 17/17 tasks, zero cache hits, existing warnings only (`static-full.log`). Final production frontend builds passed and frontend source hash comparison is stable. Added actual initialization read-failure browser assertions retaining the same focused reason textarea while approval is disabled, with screenshots before and after explicit recovery.

Supported maintenance acceptance now seeds the immutable official 1.0.0 catalog into a unique project root after configuration creates its real repository system ID, before the daemon starts. The real inspection must adopt that existing tree; human selects advertised 1.0.1, reviews exact AGENTS/material.yaml modifications, and the selected fake runtime performs the reviewed merge. The UI invents no higher version. Each provider runs initial setup and real maintenance on both Web/Desktop (four cases per provider), then the ordinary requirement/manual-plan workflow.

Both production builds are finished and their frontend source manifest still matches current source. The eight-case browser matrix is ready: two providers × Web/Desktop × initialization/maintenance. It also verifies current-version no-op maintenance and zero PR on compatible old-bundle adoption, then the existing requirement/manual-plan chain. It has not yet run; waiting for the independent checker's backend/daemon fixes to stabilize avoids certifying stale binaries.

## Final production acceptance — complete

Eight isolated final journeys passed: GitLab Web initialization/maintenance and Desktop initialization/maintenance (1.5m each); GitHub Web initialization 1.7m, maintenance 1.6m, Desktop initialization 1.7m, maintenance 1.6m. Every case uses its own freshly migrated database and fresh local provider tree, selected test executable, normal daemon claim/start, independent readback and human UI decisions. All continue through ordinary requirement publication, manual planning and approved design/task-plan without coding dispatch.

`frontend-final-browser.json` indexes all eight database identities, evidence directories, source/binary/build hashes, durations and cleanup. Each run's start/end source hashes matched. Final source comparison matches all product code and all GitHub evidence; the four earlier successful GitLab runs differ only in `e2e/fixtures/material-github.mjs` and its node test, which are not selected by those GitLab cases. Final frontend build hashes remain valid. The optional runner `record.case` selects one exact Playwright journey with `--grep`; assertions and timeouts were not relaxed.

Two original failed rounds remain retained:

1. `/private/tmp/multica-material-init-ui-uEHFqY/gitlab-final/material-run-3ySVSx`: initial Web/Desktop passed; both maintenance upgrades reached ready, but the later ordinary-plan wait exceeded30s after the shared provider tree accumulated unrelated roots. Failure DOM and DB already showed waiting_manual/baseline_ready. Per-case fresh provider/database isolation removed cumulative cost without changing product or timeout.
2. `/private/tmp/multica-material-init-ui-uEHFqY/github-web-maintenance/material-run-Gzffga`: fixture request-body Buffer chunks were concatenated as strings and split one Chinese UTF-8 character in an existing template. The product correctly blocked the changed bytes. Fixture now buffers bytes and decodes once; a deliberately split-codepoint regression passes. All four GitHub cases then passed on fresh databases. Independent checker reviewed this test-only correction.

Visual comparison passed94, with initialization identity/full diff, maintained before/after rules and retained focused reason through503 inspected in Web/Desktop. See `frontend-visual-verdict.json`. The panel keeps the shared project hierarchy and scroll containment; source text remains inert.

All owned API/Web/daemon/Electron/fixture processes exited and the dedicated container `material-init-ui-90276a8f` is stopped. Databases, credentials and raw reports are retained privately at `/private/tmp/multica-material-init-ui-uEHFqY`; no database or user artifact was deleted. No live provider account, external repository or real model was used, and this does not certify packaged Electron Main/installers or model-generated design quality.
