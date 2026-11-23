# S1 coordinator verification — 2026-10-21

Scope: independent main-session verification of the S1 runtime scan, deletion
fence, migration inventory, daemon candidate paging, and retained proof
recovery integration. This report does not claim the parent 97-item acceptance,
Linux/provider rollout, or production execution gates.

## Environment distinction

The earlier `service_s1` database clone was intentionally created from the
pre-S4 template (migration 729). Running current collaboration fixtures there
fails before the behavior under test with `column "collaboration_capabilities"
does not exist`. That is a stale test database boundary, not a product result.
The current source was rerun against task-owned `service_s5`, which includes the
full task schema through migration 737.

## Fresh verification

All commands ran from `server/` with the repository agent CLI guard and the
task-owned database wrapper where a database was required:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/service \
  -run '^TestCollaborationRuntimeScan(SerializesWithDeletion|ReauthorizesAfterRuntimeFence|WaitingScanAdvancesPersistentRefusals|CandidateScanAdvancesPersistentRefusals|IsolatesObjectFailure|PropagatesGlobalFailure|UnknownCommitPreservesOriginalRun|CursorCleanup)$' \
  -count=1 -timeout=180s -json

GOTOOLCHAIN=go1.26.6 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/daemon ./internal/daemon/collaborationrun \
  -run 'Candidate|Recovery|Client' -count=1 -timeout=120s -json

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./cmd/migrate \
  -run '^TestEveryConcurrentUpBuildHasCleanup$' -count=1 -timeout=120s -json

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/handler \
  -run '^TestWorkspaceDeletionManifestCoversPublicSchema$' -count=1 -timeout=120s -json

GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon/... ./internal/service ./internal/handler ./cmd/migrate
```

Results: every command exited 0 with zero failed or skipped tests. The raw
JSONL results are retained outside the repository at:

- `/tmp/multica-gap-recovery-20261021-h6phxwo9/root-s1-service-final.jsonl`
- `/tmp/multica-gap-recovery-20261021-h6phxwo9/root-s1-daemon-final.jsonl`
- `/tmp/multica-gap-recovery-20261021-h6phxwo9/root-s1-migrate-final.jsonl`
- `/tmp/multica-gap-recovery-20261021-h6phxwo9/root-s1-manifest-final.jsonl`

The service run covers runtime/workspace deletion ordering, binding re-
authorization, waiting/candidate progress, object-error isolation, global
error propagation, unknown commit preservation, and cursor cleanup. The
daemon run covers single-candidate paging, restart rotation, proof recovery,
and client contract checks. Migration and manifest checks cover concurrent
index cleanup registration and public-schema workspace teardown ownership.

`gofmt -l` over all changed Go files returned no paths, and scoped
`git diff --check` returned 0.

## Limits and handoff

This verifies the current S1 source and its task-owned integration boundary.
It does not establish Linux kernel containment, real provider credentials,
packaged clients, full `make check`, or the complete parent acceptance matrix.
The task remains `in_progress`; S2–S5 integration and S6 evidence gaps remain.
