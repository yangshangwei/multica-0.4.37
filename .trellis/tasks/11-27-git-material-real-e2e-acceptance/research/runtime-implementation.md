# Runtime implementation and deterministic evidence — 2026-11-27

Continued existing task and uncommitted runtime implementation; no real account,
model invocation or remote Git write. Contract v1 remains frozen, with the
coordinator-approved additive `MaterialReview.merged_at` field for delayed receipt
recovery against the original merge permit window.

## Changes completed

- Preserved the normal queue claim/start/native backend execution path and the
  local task-token bridge. Only the selected runtime mutates remote state;
  the daemon observer performs bounded independent reads using host gh/glab.
- Material claims now refuse a mismatched local runtime profile and an unresolved
  custom profile instead of falling through to the built-in executable.
- Receipt writes use private temporary files, sync and atomic rename. An
  interrupted pre-publication write cannot leave truncated final JSON that
  blocks recovery. Restart replays complete persisted receipts unchanged and
  ignores temporary remnants; acknowledgment removes the pending receipt.
- Both providers propagate actual remote merge timestamps, reject merged
  responses missing timestamp/actor/commit, and require regular `100644` files.
- Host CLI transport preserves explicit API version/Accept headers. It parses
  status/headers while reading already-decoded stdout directly; original wire
  Content-Length/chunking/compression are not applied a second time to CLI JSON.
- GitLab instances with URL prefixes explicitly return
  `material_instance_prefix_unsupported`. Hostname-only glab invocation cannot
  safely infer that prefix. No wrong-path fallback is retained.
- Updated materialgit README to read-only observer ownership, host auth boundary,
  actual baseline verification and supported CLI constraints.
- Expanded tagged composite to GitHub and GitLab: each performs actual service
  enqueue, daemon claim, native fake executable publication, real human decision,
  native review synchronization/merge, independent hosting-CLI reads, and server
  receipt consumption. Auto-off proves zero planning calls; manual start and
  terminal retry each verify baseline before launch, retain one issue and both
  completed run rows. Ten native fixture invocations total, not live model calls.

## Verification

Passed on current runtime source:

- `go -C server test ./internal/daemon ./cmd/multica -run '^Test(Material|ProjectMaterial)' -race -count=1`
- `go -C server test ./internal/daemon -run '^TestMaterial' -race -count=1` after the CLI decoded-body fix.
- `go -C server test ./internal/materialgit -race -count=1`
- `go -C server vet ./internal/daemon ./internal/materialgit ./cmd/multica ./pkg/protocol`
- `git diff --check`
- `go -C server test -tags=materialintegration ./internal/daemon -run '^TestMaterialCompositeQueueRuntimeObservation$' -race -count=1 -json`

The final composite used freshly created and migrated
`multica_material_runtime_resume_0f56ea65`, verified through current_database().
Private allocation and source hashes are retained under
`/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-material-runtime-resume-uvyjf9o2/`:
`green-allocation.json`, `green-migration.log`, `composite-green.jsonl`,
`green-source.json`. Runtime source bytes remained identical during the final
run. No processes remain writing that database; do not reuse it for a new
complete suite. Allocate another fresh package-owned DB from its private endpoint.

Earlier normal composite passed both providers. The first race composite failed
because every fake host CLI executes the race test binary, whose default one
second exit sleep multiplied across API reads and exhausted the unchanged 20s
runtime deadline. Its failure is retained in `composite-final.jsonl` and its DB;
it is not acceptance evidence. The fixture now sets only
`GORACE=atexit_sleep_ms=0` for subprocesses, retaining race detection and the
original product deadline. Final replay passed on a fresh DB.

## Limits and official references

Normal runtime launch deliberately retains daemon HOME/XDG for hosting tools.
Profiles select executable and fixed args, not a separate daemon-visible host
auth environment. A custom wrapper changing its child HOME must expose the same
Git auth to the independent host observer; arbitrary wrapper-private state is
not inferred from model text. This is a trusted-host boundary, not shell isolation.

CLI flags verified against [GitHub gh api](https://cli.github.com/manual/gh_api)
and [GitLab glab api](https://docs.gitlab.com/cli/api/): explicit host, headers,
method, response headers, and stdin body. Actual installed version/auth and live
provider behavior remain for separate acceptance. GitLab prefixed instances and
GitHub Enterprise are explicitly unsupported by this transport.

Browser/Electron evidence belongs to the frontend lane. Independent full-scope
review and broad ordinary-task regression remain coordinator-owned. No production
deployment, real Codex execution or yangshangwei/test--multica write occurred.
