# S1 E2E implementation handoff

Implemented 2026-11-13 in `e2e/collaboration-multi-account-closure.spec.ts` only. The coordinator owns all environment allocation, builds, real test execution, source/binary provenance, spec changes and commits. No shared fixture was changed.

## Boundary and changes

The existing closure proved run authorship but did not require the new public Agent summary or a specific visible author. It also had no focused Electron author check. The canonical scenario now:

- Retains `author_type=agent` and `author=null`; requires exact `author_agent={agent_id,name}` matching the same real controlled run and resource.
- Records comment ID and Agent name with each of the five existing run facts.
- Checks the exact five Agent comments in each Product/Development/Quality session, with visible name and Agent label, no human edit/delete controls, and matching reply hints. Reply is cancelled without posting.
- Opens the same issue as Quality in an isolated Electron renderer profile before final completion, verifies all five authors and reply hints plus the original human author, and records account header isolation and zero daemon starts.
- Captures native author and reply screenshots. Preserves native failure capture, final page errors, app shutdown and profile removal in the existing failure-aware evidence/cleanup flow.

The original three owners, three Agents, 1/2/2 run counts, original-key replay, source isolation, grant withdrawal/explicit Resume, first return and exact second acceptance remain unchanged. No second provider lifecycle, installed Main claim, new dependency or product code was added.

## Static checks actually run

- `pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --list`: exit 0, exactly one Chromium test discovered. This executes no test body.
- `pnpm exec tsc --noEmit --skipLibCheck --strict --esModuleInterop --module ESNext --moduleResolution bundler --target ESNext --jsx react-jsx --resolveJsonModule --allowImportingTsExtensions e2e/collaboration-multi-account-closure.spec.ts`: exit 0. Output in `e2e-typecheck.log` contains only the existing pnpm manifest-location warning.
- `git diff --check -- e2e/collaboration-multi-account-closure.spec.ts`: exit 0.

Actual B05/B06 browser, database, Provider and Electron execution is still NOT_RUN in this handoff. The added assertions require the completed S1 API and shared client builds.

## Long-name visual follow-up

The coordinator approved an additional spec-only visual proof. `createClosureResource` has no name override, so the spec uses the existing owner-authorized `PUT /api/agents/{id}` immediately after creation and before offer/consent. `UpdateAgentRequest.Name` accepts a string, `UpdateAgent` has no name length cap, and the database column is `TEXT` (`001_init.up.sql:39`). The Quality Agent keeps its ID and receives `QualityPaymentCallbackAudit` repeated four times: 104 unbroken ASCII characters. No fixture or product file changed.

The same final Quality comment and reply are measured at Web width 390 and native 900-pixel content width with 2x zoom (450 CSS pixels). Real text Range rectangles must span multiple lines and remain inside their comment/form; both local and document horizontal overflow must be absent. The scenario records geometry and four screenshots (`web-narrow-agent-author.png`, `web-narrow-agent-reply.png`, `desktop-agent-author.png`, `desktop-agent-reply.png`). Web viewport and native size/zoom restore in `finally`. Reviewer corrections to the exact `Edit comment`/`Delete comment` labels remain.

Strict single-entry TypeScript and `git diff --check` passed again after this addition; Playwright still discovers exactly one test. Actual geometry and screenshots remain pending execution.

## Coordinator run requirements

All suites require the recorded task-owned password API/database and matching production Web. For B05/B06, bind `DATABASE_URL`, `COLLABORATION_E2E_API_URL`, `NEXT_PUBLIC_API_URL`, `MULTICA_DEPLOYMENT_ID`, `PLAYWRIGHT_BASE_URL`, `COLLABORATION_ELECTRON_RENDERER_URL`, `E2E_PASSWORD_AUTH=1`; the two API URLs must match. Native assets are the selected checkout's `apps/desktop/out/renderer` and `out/preload`; `COLLABORATION_ELECTRON_SOURCE_ROOT` selects that checkout. The existing host fixture opens/restores the three execution gate flags.

Select explicit native-built fixture binaries:

- `COLLABORATION_HOST_DAEMON`: `go -C server test -c -o <owned path> ./internal/daemon`; the fixture invokes `__multica_collaboration_host_router_poller`.
- `COLLABORATION_HOST_PROVIDER`: native build of `e2e/fixtures/collaboration-multi-account-provider.go`, not the different host/squad Provider.
- `COLLABORATION_HOST_CLI`: current-source production `server/cmd/multica` binary.

Run the canonical spec with one worker and zero retries. Native check is part of this same test, before completion while Reply is permitted.

B08 (`collaboration-offline-result-notifications.spec.ts`) instead needs Linux ELF `COLLABORATION_CROSS_CLIENT_DAEMON` (the same daemon test package cross-compiled for Docker architecture), `COLLABORATION_CROSS_CLIENT_PROVIDER` (the cross-client Provider source), and numeric `COLLABORATION_CROSS_CLIENT_HOST_IPV4`. It creates owned `redis:7-alpine` Docker containers with privileged namespaces and copies only the two binaries. Its API uses `host.docker.internal`, and its Provider callback must reach the selected numeric host IP. Run its two Web/Electron tests separately from the host Provider protocol.

B09 live revoke uses the existing simulated-permit fixture: password auth, task DB, API URLs, deployment ID and matching production Web, with execution enabled. It starts no actual host Provider. The handoff chain is a human-only password fixture. Keep their evidence distinct from the five-run closure.

## Remaining verification

Run B05/B06 and the existing B08/B09 suites in the prepared environments, inspect both native PNGs, and preserve original failures. No actual UI pass, source/build binding or cleanup success is claimed by static checks.
