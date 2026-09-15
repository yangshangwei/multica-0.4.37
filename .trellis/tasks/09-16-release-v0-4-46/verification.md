# V0.4.46 verification and publication evidence

Source validation is complete. Native Windows packaging, final tagged release,
and offline upgrade validation are still pending; this file is not a publication
claim.

## Current source evidence

Base source is `b9b10a10f888fffdcff1d640b4f9cfd36f7ded55` plus the reviewed task
diff. Exact command, process, source and exit records are retained under
`.omx/reports/release-v0.4.46/`. The final commit will record these verified files.

- Full production Web/API/Electron Playwright suite: **91 passed, zero failed,
  zero skipped, zero flaky**, 198.7 seconds (`e2e-final.json`).
- TypeScript tests: **8,210 passed** with fresh Turbo execution: docs 60, core
  1,956, web 260, desktop 636, views 5,298 (`ts-final`).
- Production builds plus typechecks: 12/12 task executions passed. Lint: 6/6
  package tasks passed, retaining existing advisory warnings.
- Complete guarded Go race wrapper passed (`go-final`). Dedicated Redis-backed
  handler/service/docs/middleware verification also passed; explicit external
  integration and intentionally unavailable DB-error cases remain skipped in Go.
- `go vet ./...`, `go tool govulncheck ./...`, and UI wildcard-export validation
  passed. No Go vulnerability was found.
- Changelog/generator/packaging tests: 52 passed plus four opt-in skips, followed
  by the real Docker lane with all 19 tests passed and zero skips.
- Installer, entrypoint, build naming, self-host environment, worktree cleanup
  and UI export guard shell tests passed. The unchanged dev-env registry suite
  passed in an isolated source-only checkout; source hashes are recorded.
- Windows ia32 preflight: 75 focused tests passed after 23 expected red failures,
  desktop typechecks and PowerShell parser checks passed, and a real Windows/386
  CLI compiled successfully. This local Go 1.27 preflight is not a release binary.
- Independent application and Windows packaging reviews cleared the final fixes.
  Screenshot inspection passed at 96/100 against the existing component style.

## Repairs and coverage

- Template squad creation now preserves the successful squad identity: the
  server returns empty arrays, while the client accepts null/missing optional
  lists from older servers. New browser tests verify first creation, all-reuse,
  Chinese/English names, preserved customizations and conflict rollback.
- The desktop version action uses the actual menu primitive. Real-menu unit
  tests and Electron verify keyboard selection, exact Updates navigation and
  menu dismissal, followed by the Chinese grouped daemon settings page.
- Floating-chat hints now match its enabled default and dedicated Chat page;
  generated embedded agent/conventions docs match their current source.
- The onboarding test now checks the approved fourth display-only project row
  and completes the real skip-runtime path to Projects.
- The live changelog test protects its entire fixture lifetime. A controlled
  setup-429 probe verified that failed auth leaves the seed unchanged and no
  temporary repositories behind; publication assertions were not weakened.
- Genuine Windows ia32 packaging includes a 386 CLI, independent update metadata
  and native installation checks. x64 and arm64 mappings remain available.

Changed areas: `packages/core/api/{schemas.ts,agent-template-schemas.test.ts}`;
`server/internal/handler/squad_template*`; desktop packaging, updater and version
menu files; `packages/views/layout/help-launcher*` and four settings locale files;
two generated docs; five E2E specs; release docs/workflow and focused spec notes.
No dependency was added. Standard menu behavior replaces custom button behavior;
the existing release pipeline is reused.

## Environment findings and limits

- The first browser attempt exhausted Redis's five-auth-requests/minute loopback
  quota. Only the task API uses higher documented auth limits; product defaults
  remain unchanged. The final suite uses real auth/API/database requests.
- The task services use API 18446, production Web 13446, renderer 13447 and
  dedicated PostgreSQL databases/Redis. Existing development services were kept.
- Simultaneous builds caused two unchanged five-second unit tests to time out.
  The complete suite passed with no timeout changes at Turbo concurrency two.
- Knip retains the existing nine unused files and two dependency declarations;
  CI explicitly treats this audit as advisory. No new finding was introduced.
- Electron acceptance uses real preload/renderer/router with native daemon and
  updater IPC isolated. No actual model execution, external SaaS integration or
  real daemon start is claimed. Mailbox delivery is replaced by the generated
  local verification code; the real login UI was also exercised manually.
- Final Windows smoke must prove the delivered bytes on native Windows. ia32
  runs under WOW64 on the x64 runner, not a pure 32-bit Windows OS; no Windows
  GUI, ARM64 native install or side-by-side x64/ia32 coexistence is claimed.
- Mobile has an independent release cadence and is excluded by the repository's
  root test/build scripts. This release targets Web/server and Desktop.

## Remaining publication gates

1. Commit/push source and verify the Windows candidate build/install.
2. Create the immutable v0.4.46 tag; complete Release and tagged Windows jobs.
3. Build the exact cumulative-feed Linux amd64 intranet archive.
4. Run the prepared real V0.4.45-to-V0.4.46 upgrade, with original task, attachment,
   SQL sentinel, JWT, secrets, ports, backup and ordinary recreation checks.
5. Upload validated assets, verify remote hashes, and provide delivery links.
