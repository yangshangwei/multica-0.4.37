# Runtime command integration handoff

## Observed gap

M08 requires the existing issue comment and squad activity commands to operate
inside the authorized collaboration run. The service has dedicated run-scoped
`context`, `comments`, and `squad-activity` APIs, but `cmd/multica` currently has
no collaboration execution context handling. `newAPIClient` accepts ordinary
task credentials, and both command writers still resolve the ordinary issue
and call `/api/issues/...`. The Linux controller provides the business token
and Server URL to the selected executable; it does not provide an adapted CLI
or a validated CLI run identity.

The current real Linux route test proves the API/Runner/namespace/proof chain.
The separate daemon test proves pre-claim machine-slot ownership and release.
Neither proves the full squad through the unchanged CLI commands and actual
daemon slot polling. Preserve that distinction in acceptance reports.

## Authorized implementation boundary

- Keep the existing user commands. Adapt only the required issue context,
  comment read/write, and squad activity operations to the current run's narrow
  business capability; never substitute ordinary user/task/daemon credentials.
- Detect an incomplete or malformed collaboration environment before profile
  lookup or network access. Unsupported commands must reject explicitly without
  reading a human profile or falling through to ordinary writers.
- Validate the requested issue against the immutable run context. A run token
  cannot select another issue, target, workspace, or another run. A capability
  rejection must not be presented as a request to sign in as the resource owner.
- Preserve comment/event identity across an uncertain response. Do not turn
  an automatic retry into a second comment, delegation, or activity event.
- Supply only the business identity needed by the CLI inside the existing
  Linux namespace. Termination/registration credentials remain private to the
  supervisor. Any executable used by tests must be built or created by the test;
  do not discover or execute a user-installed AI CLI.
- Use the current daemon's actual `MaxConcurrentTasks=1` path for the final
  leader/member/continuation integration evidence. Keep independent run
  identities, permits, nonces, capacity reservations, and sealed proofs.

## Expected ownership and verification

The implementer owns the relevant `server/cmd/multica/` commands, shared CLI
context/client support if necessary, daemon collaboration/controller wiring,
and adjacent behavioral tests. The root coordinates the router/service owners,
isolated Linux environment, generated code, built-in skill documentation,
independent review, and final rollout decision.

First reproduce the ordinary-endpoint or profile-fallback defect with a
test-owned endpoint, then verify the narrow commands, wrong/incomplete identity,
unsupported operations, response-loss replay, and unchanged ordinary commands.
Run the real Linux chain against the current router and PostgreSQL state.
The default creation/execution rollout gates remain closed until their complete
acceptance criteria have evidence; a configured static fixture is not proof of
general provider, Git/attachment, Windows, or macOS runtime support.
