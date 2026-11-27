# Engineering material Git observations

`NewObserver(provider, instance, repository, transport)` exposes only reads of
GitHub/GitLab reviews, decisions and immutable files. The selected runtime owns
publication, review-note writes and merging. The Multica server never resolves
material Git credentials or constructs a network observer.

The daemon supplies a host transport using `gh api` or `glab api`. Credentials
stay in the selected runtime host's existing hosting-CLI environment. The normal
runtime launcher preserves the daemon's HOME/XDG configuration for these tools;
a custom runtime wrapper that changes its own child environment must make the
same Git authentication available to the host observer. Such private wrapper
state cannot be inferred or imported from model output.

## Contract

- Remote reads use a frozen repository, instance, branches and exact commit.
  No request follows a model-supplied URL. HTTPS and same-host requests are
  required; redirects are refused. GitHub Enterprise and GitLab installations below a URL prefix are explicitly
  unsupported by the host-CLI transport.
- The observer API has no publication, comment-writing or merge methods.
  The transport admits GET and only the two fixed read-only GitHub GraphQL
  queries used for native policy verification. Credential headers are removed
  before the host CLI adds its own authentication.
- `FindReview` searches all states by the fixed source branch, rejects ambiguous
  results, and leaves target validation to `Observe`. An empty list proves no
  effect only after repository access and exact source-branch absence are read.
  A leftover branch is uncertain and never authorizes blind publication.
- `Observe` returns the complete changed-file set or fails. Truncation, missing
  diff metadata, unknown modes and incomplete pagination cannot authorize a
  baseline. The service independently compares these facts to approved paths.
- `ReadFile` walks immutable trees, refuses executable files, symlinks and
  submodules, and requires complete UTF-8 bytes of at most 1 MiB. Tree/file/blob
  identity must match the exact commit. HTTP 404 is not an empty tree.
- Native gates remain distinct from a conflict-free diff. GitLab checks current
  merge permission, detailed merge status and any visible pipeline at the exact
  source HEAD. GitHub checks active rules, classic protection and exact-head
  GraphQL approval/status rollups. Missing policy access and unsupported rules
  remain blockers. Neither observer changes policy or upgrades credentials.
- A classic protection 404 is insufficient when GitHub reports a protected
  branch: ruleset-only support also requires affirmative scoped GraphQL proof
  that no classic rule exists. Configured bypass actors, merge queues and unknown
  native policy facts remain unsupported.
- An already-merged review records a remote fact, not human approval. The server
  requires a current authorized merge operation and the actual merge-commit
  bytes before accepting a baseline. The observed provider merge timestamp must also fall within the original
  permit window; a later readback does not renew that window.
  Source-head conditional provider APIs do
  not atomically fence every target-branch fact.
- `ObserveDecision` requires exactly one note with the frozen marker and exact
  attributed body, plus a real remote actor. It never creates native approvals.
- Errors contain bounded safe codes and status, without provider response bodies,
  token-bearing URLs or raw transport diagnostics.

The `Provider` read interface and `New(Connection)` are retained for isolated
HTTP parser tests; production material flow uses the daemon-only observer.
Repositories must already contain their configured base branch. Repository
initialization and policy administration are separate operations.

## Verification

`provider_test.go` exercises both providers through isolated HTTP fixtures:
fixed-commit bytes, complete trees/diffs, rejected modes, native checks, ruleset
proof, pagination and redacted errors. Daemon tests exercise fake host CLI
subprocesses, independent readback, secret isolation and durable receipt replay.
The `materialintegration` composite uses real queue claims and a test-created
native runtime executable before observing fixture effects and consuming the
receipt on the server. These tests never use a real account or model.

```sh
go -C server test ./internal/materialgit -count=1
go -C server test ./internal/materialgit -race -count=1
go -C server vet ./internal/materialgit
```

Current reference API behavior is implemented by the existing parsers. Local
fixtures do not attest to a deployed repository's rules, permissions, hosting
CLI version, connectivity or actual remote merge.

CLI argument contracts were checked against the official
[gh api manual](https://cli.github.com/manual/gh_api) and
[glab api manual](https://docs.gitlab.com/cli/api/). Both document the explicit
host/method/header switches and stdin request body support used here. The actual
installed versions and authentication still require live acceptance.
