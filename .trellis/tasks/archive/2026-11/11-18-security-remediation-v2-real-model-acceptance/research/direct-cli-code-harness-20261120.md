# Official CLI C1/D1 platform and artifact harness

The new tagged `direct_cli_code_harness_test.go` leaves consumed P1/O1 source and
product code unchanged. `TestSecurityRemediationDirectC1*` and `...D1*` require
explicit registered scenario, source/fixture/binary/config hashes and the same
real-agent opt-in before provider execution. The provider seam remains the
reviewed official `codex exec` backend; this is not an app-server adapter claim.

## Actual platform and filesystem inputs

The human fixture creates the project and a real `local_directory/in_place`
resource through production APIs. Each code role owns a standalone full Git
clone with its own `.git` directory, no commondir or alternates. Before each
serial worker claim, the human project owner points that resource at the next
role's exact clone. A separate no-provider platform test proved the resource
PUT creates no execution and that the subsequent real claim delivers the updated
path. Every real run checks that claim again. Squad leaders deliberately ignore
project repositories in production preparation; the manifest retains the actual
resource count and ignored paths while separately attesting prepared repository
count zero. The code wrapper must prove the actual scratch cwd is not one of
those ignored paths.

C1 adds a manually created verifier using the exact fenced instruction sample
from current `squads.zh.mdx`. Its stored platform autonomy is empty, template key
empty, version0, exactly as the current product docs describe. The deployment's
verifier filesystem policy is separate; no hidden Contributor provenance or
new built-in role is fabricated. D1 uses a named human owner distinct from the
squad creator, and grants invocation access through actual human API changes
before the scenario begins. The original project owner manages resource paths;
the distinct security owner alone supplies G1/G2 releases.

## Role sequence and decisions

C1 follows fourteen actual tasks: leader intake, restricted G1 review, leader
after human G1, verifier reproduction, leader, implementer repair, leader,
verifier recheck, leader, QA, leader, two restricted G2 reviews, leader close.
D1 follows twelve actual tasks, with security and migration reviews at G1 and
security/code/migration reviews at G2. Within each required review batch, the
harness reads the actual platform queue order and admits any remaining required
reviewer; it never rewrites priorities or tasks to force an order.

Both real human gates write a private pending record and wait for a root-owned
approval file binding scenario, issue, owner, every review comment/source/content
hash, exact decision text hash and current commit. Root must inspect those actual
results before approving. The fixed human decision cannot override a missing
review or patch/test failure. A distinct timestamp is awaited before posting,
because the public comment representation is second-granularity. Fake mode
explicitly scripts its human input and remains non-model evidence.

## Artifact ownership

Only actual role commits are handed off. C1 verifier may change `tests/`, while
baseline tests remain hash-protected; the implementer may change only
`workspace_reader.py`, and all reproduction test hashes remain protected.
The separate `read_summary` variant must remain outside the current patch. D1
implementation may change the local lock and application compatibility shim;
existing tests and vendor sources are protected. Reviewer, QA and verifier
recheck are read-only and cannot change HEAD or leave a dirty tree.

All Git reads after a model run use `contained_git_role_handoff.py` in a no-network
container mounting only that role repo. Export/import also execute the original
bundle verifier inside that container, including its temporary staging clone;
no host Git reads model-owned config, hooks, fsmonitor, filters or include paths.
Only trusted initial fixture initialization and full clones happen before model
access. Exact generated sidecar names are placed in `.git/info/exclude` during
human setup; source `.gitignore` and fixture code are not silently changed.

## Verification and evidence limits

- Actual human resource-switch preparation passed with queue count1 unchanged
  and the correct next-role path in the claim; it invoked no provider.
- Final C1 fake05 passed fourteen actual tasks, two original-commit bundles,
  independent failing reproduction then passing repair/recheck/regression,
  eleven platform criteria and one inert unassigned backlog variant.
- D1 fake03 passed twelve tasks and eleven criteria on the exact final C1
  binary/source (312.65 seconds), with the distinct human security owner and
  security/migration G1 plus security/code/migration G2 reviews. Fake02 remains
  historical evidence from before the final source freeze.
- Historical failures are retained: absent handoff parent, ambiguous joined
  `status`, wrong `parent_id` instead of `parent_issue_id`, and DELETE member's
  actual204 status. All were harness assumptions, not product changes or relaxed
  expected outcomes. Five final SQL shapes were checked directly with empty
  read-only scopes before final reruns.
- Machine verdict remains `platform-checks-passed-semantic-review-pending`.
  Actual red/green output, commit citations, all human/reviewer semantics,
  forbidden-action audits and full workspace changes still require independent
  semantic review in a real sample. Scripted fake patch content is never mounted
  into a real role container.

Current source/binary/evidence hashes are in `direct-cli-code-harness-20261120.json`.
A complete source copy is retained privately before any paid C1/D1 run, unlike
the explicitly recorded first-P1 source-retention limitation.
