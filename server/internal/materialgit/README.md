# Engineering material Git adapters

`New(Connection)` returns a GitHub or GitLab `Provider`. Connection credentials
are resolved by the host, never by candidate text. The constructor accepts
github.com's API or an explicitly configured HTTPS GitLab instance (including
an installation below a URL prefix). It never disables certificate checks or
follows redirects. GitHub Enterprise and HTTP instances are not supported.

The package owns bounded provider HTTP operations, not business authorization,
database intentions, worker ownership, or an Agent's execution permissions.

## Contract

- Each immutable candidate has one stable operation key and unique branch.
  Only regular Markdown files at `requirements/<id>/{prd,design,plan}.md`
  below an optional configured root are writable; the host must additionally
  verify that this root and material ID are the authorized ones.
- Publication first looks for existing effects. An existing candidate branch
  requires the exact operation/content marker and exact frozen file contents.
  Existing PR/MR recovery also checks the repository, branches and full changed
  file set. Discovery searches by source branch across all review states and
  targets before creating any objects. A retargeted review or a deleted source
  branch on an existing review blocks replay; neither is silently recreated.
  Human branch edits are never force-pushed away.
- GitHub creates the commit before creating its reference. A colliding ref is
  only read, never updated. GitLab first uses the create-only branch endpoint.
  A conflicting or ambiguous branch creation never authorizes a commit. A
  created branch with no candidate marker is `candidate_branch_ownership_unknown`
  on recovery and needs explicit operator resolution; the adapter cannot prove
  that it was created by the interrupted operation.
- GitLab lacks branch-head CAS in its commits API. After a confirmed branch
  creation, the adapter rechecks the head, pins existing files with
  `last_commit_id`, and uses create actions for absent files. Concurrent unrelated
  changes are preserved; full diff validation refuses a review containing them.
  This is not an atomic branch-head compare-and-swap guarantee.
- `Observe` returns all changed paths or fails, including when a provider
  truncates/limits the diff. `ReadFile` walks immutable trees, rejects symlinks
  and submodules, and requires a complete UTF-8 blob of at most 1 MiB. Unknown
  tree, file, pagination or response shapes fail closed. Only a complete tree
  listing can establish that an entry is absent; an HTTP 404 cannot.
- `GatesPassed` requires native checks, rather than only a conflict-free diff.
  GitLab waits for new MR diff metadata, then checks `detailed_merge_status`,
  current merge permission and any visible pipeline against the exact source
  HEAD. Pipelines on synthetic merged-result commits are not verified.
  GitHub checks active rules, classic protection and exact-head GraphQL approval
  and status-check rollups. Classic protection must enforce administrators;
  configured bypass actors/allowances are refused. A ruleset-only branch may
  return 404 for classic protection: accept that only when a scoped GraphQL
  ref explicitly reports no classic rule, with no response errors. Missing policy access,
  unknown rules, merge queues and unsupported policies yield explicit blockers.
- GitHub policy reads can require App `Administration: read`, in addition to
  Contents/Pull requests/Checks permissions needed by the selected operation.
  No endpoint upgrades a token or changes a branch rule.
- `Merge` takes a frozen `MergeExpectation` and checks its review number,
  source/target branches, source SHA and exact changed-file set before the
  write and again in the final receipt. It reobserves native gates and always
  submits the approved source SHA.
  An accepted request or unknown response is not proof of a merge. Both APIs
  fence the source head, not an atomic target-head snapshot; the host must read
  the actual merge commit and verify approved bytes before establishing a
  baseline. Already-merged observation is a fact, never business approval.
- Decision synchronization posts an explicitly attributed system note, not a
  native approval. Stable note markers recover serial retries and detect a
  conflicting body. The host must serialize each durable operation; the remote
  comment APIs do not offer an atomic idempotency key for concurrent writers.
- `Error` contains safe codes, status, retryability and uncertainty, with no
  provider response bodies, token-bearing URLs or raw transport diagnostics.
  Provider rates, permission denial and unknown outcomes remain distinct.

All public operations are independently bounded by the supplied context and
HTTP deadlines. A repository must already have the configured base branch;
this adapter does not initialize an empty repository or rewrite repository rules.

## Verification

`provider_test.go` uses isolated `httptest` servers and actual HTTP adapters.
It covers successful publication/read/merge on both providers, original effect
recovery after lost replies, branch ownership ambiguity, foreign edits, extra
files, symlink modes, partial diffs, old heads, native gates, audit note identity,
pagination and credential redirection. It does not contact a real Git account
or prove compatibility with a particular self-hosted deployment.

```sh
go -C server test ./internal/materialgit -count=1
go -C server test ./internal/materialgit -race -count=1
go -C server vet ./internal/materialgit
```

## Primary references

- [GitHub pull requests](https://docs.github.com/en/rest/pulls/pulls)
- [GitHub branch protection](https://docs.github.com/en/rest/branches/branch-protection)
- [GitHub branches and ruleset protection](https://docs.github.com/en/rest/branches/branches)
- [GitHub GraphQL Ref](https://docs.github.com/en/graphql/reference/git#ref)
- [GitHub active branch rules](https://docs.github.com/en/rest/repos/rules#get-rules-for-a-branch)
- [GitHub Git references](https://docs.github.com/en/rest/git/refs)
- [GitLab branches](https://docs.gitlab.com/api/branches/)
- [GitLab commits and file preconditions](https://docs.gitlab.com/api/commits/)
- [GitLab merge requests and detailed merge status](https://docs.gitlab.com/api/merge_requests/)
- [GitLab repository tree](https://docs.gitlab.com/api/repositories/)

API documentation and simulated HTTP verification do not attest to a deployed
repository's rules, credential scope, connectivity or actual remote merge.
