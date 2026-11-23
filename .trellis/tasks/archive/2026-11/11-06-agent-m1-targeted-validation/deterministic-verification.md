# Targeted deterministic API verification

Status: implementation-side pass, ready for independent review. One new test file; no production code changed and no defect requiring a fix was found.

## Added behavior coverage

`server/cmd/server/task_capability_targeted_test.go` drives the complete production `NewRouter` through the existing test server. Fixtures use `testutil` with distinct synthetic runtime owner and originator; their allowed invoke relationship is explicit. Task tokens are seeded with valid frozen bindings to isolate router enforcement; actual claim minting belongs to the previously accepted claim matrix and the current separate live CLI campaign.

- Reporter list: ordinary and `open_only` branches, current+foreign `project_ids` union and `include_no_project`, foreign-only union intersection, case alias beside exact key, URL-encoded key/value, and complete pagination with exact IDs, no duplicates and an unchanged scoped total. Different-project and no-project sentinels never appear. Eight accepted variant cases plus one page-walk case.
- Filter refusals: repeated exact/encoded project key, repeated unrelated argument, case-only or union-only filtering without the required exact key, malformed UUID/escape and semicolon parsing. Eight cases return403.
- Encoded routing: issue separator, UUID and collection spellings refuse as403/404 without serializing outside content. The assertion permits the router's pre-Auth404 or capability403; it never treats a redirect or successful response as refusal. Three cases.
- Current runtime owner: membership removal, disabled account and owner rebinding refuse issue/comments/timeline reads and comment writes while the distinct originator and its invoke grant remain intact. Before each mutation, the same token successfully reads and writes one permitted inert comment; the denied write must leave comment count unchanged. Three cases.
- Mutable text/provenance: a real human agent update changes display name, instructions and autonomy to Operator and includes forged template key/version. Actual persisted template provenance remains unchanged. Protected reviewer still reads its own issue but cannot read/update another; old-version and custom controls retain legacy reads/writes. Three cases.

## Executed checks

- `go test -p 1 -json -count=1 -timeout=180s ./cmd/server -run '^TestTaskCapabilityTargeted'`: **29 named pass,0 fail,0 skip** (26 leaf cases plus3 parents), first run. Task-owned database `rnd_m1_targeted_router01`; private HOME; GOMAXPROCS2. Actual database identity verified by the coordinator-provided runner before TestMain. No handler database was used.
- `go vet -p 1 ./cmd/server`: exit0.
- `go build -p 1 -o <private evidence directory>/targeted-server-build ./cmd/server`: exit0; no service was started from this binary.
- `gofmt -l` on the new source: no output. Scoped `git diff --check`: exit0; file is new/untracked, so gofmt is the applicable source-format check.

Raw JSONL, static logs, commands, exits and SHA256 values are retained in the private directory and compact copies under `deterministic-evidence/`. `manifest.json` binds the exact new source hash and baseline HEAD. No first failure was erased: this matrix passed on its first execution; there is no invented red claim. Earlier setup/allocation results owned by root remain separate.

## Unmeasured / remaining

No real Codex call, provider account lookup, remote request, production operation, dependency change, broad suite or additional race run occurred in this slice. Existing race evidence remains separate; these sequential HTTP/DB cases add coverage of list parsing, owner authority and mutable-role provenance. Actual compiled CLI/API interoperability and model behavior are independently owned lanes. Independent `trellis-check` still needs to inspect these new tests and evidence before the parent accepts this slice. No Git commit was made.

## Independent review
Independent29-case replay passed on fresh rnd_m1_targeted_review01. Reviewer added a persisted display-name assertion to the provenance test; focused4-case replay and vet/gofmt passed. Final source binding is independent-evidence/manifest.json; original implementation evidence is retained.
