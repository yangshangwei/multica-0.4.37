# Independent test-directory maintenance check — 2026-11-20

**PASS for this maintenance change. No implementation defect was found.**
The two test setups use the caller's temporary root, canonicalize the actual
repository path and retain cleanup registration. All test methods and assertions
remain unchanged. This verdict neither admits C1/D1 nor closes AC9/AC10.

Reviewed checkout: `/Volumes/artisan/code/2026/mc-secrem-v2-maintenance-20261120T124811Z`.
Base HEAD: `1861f2aab5d2d7a4d6db927a43b6cf66a10de653`.
Private check evidence:
`/Volumes/artisan/code/2026/secrem-v2-maintenance-20261120T124811Z/check/`.

## Review scope

Read the task PRD, design, implementation plan, `check.jsonl` references,
parent AC9/AC10 and current maintenance checkpoint, root guidance and the relevant
server test-state, task-capability, experiment-evidence and template contracts.
Reviewed every changed or new file present at the review handoff, including the
parent PRD, task metadata and both new maintenance/access reports.

The code diff is limited to `setUp` in two test files: removal of two hardcoded
historical evidence roots and addition of `Path.resolve()` for `ContainedGit`.
The existing `CodePolicy` setup already resolves its path. Immediate
`addCleanup(self.tmp.cleanup)` remains in both. No production or runtime script,
container policy, immutable image, template, frozen fixture, expected result or
admission setting changes.

The existing implementation check ran the complete original suite once.
The reviewer verified its raw records independently and did not repeat that
suite without a new defect. A separate, small probe executed only the first
three setup statements extracted from the actual ASTs: temporary allocation,
cleanup registration and canonical path assignment. It used both a canonical
caller temporary root and a symlink spelling of that same new private root.
All four allocations resolved to the expected private parent, cleanup removed
only each allocated directory and a sibling sentinel remained intact. This
probe did not import runtime modules or execute any test body, Docker operation,
model process or database command.

## Verification

| Check | Result |
| --- | --- |
| Original control-test run | Independently verified **50 PASS, 0 FAIL, 0 SKIP**, 60.969 seconds; guarded command exit 0. Exact result IDs match the 50 methods in HEAD. |
| Test behavior preservation | All 50 method ASTs match HEAD exactly, including assertions. Only the two setup allocations differ. |
| Retained evidence | All 46 entries in the implementer's `hashes.json` match actual bytes. Test stdout/stderr and environment hashes also match the command receipt. |
| Source identity | All 59 script/fixture entries match the retained post-run inventory; comparison with the pre-run inventory changes only the two assigned test files. |
| Frozen inputs | All 16 fixture files match `fixtures/freeze.sha256.json`. |
| Lint/static checks | PASS: fresh AST parsing of all 29 task Python files, task JSON parsing and `git diff --check`. No configured Python linter exists in the root or task directory. |
| Type check | Not applicable to these untyped Python setup edits; no Python type-check configuration is present. No Go/TypeScript files changed, and no Go/TypeScript check was rerun. AST parsing is not represented as type checking. |
| Private test environment | The recorded allowlist has private HOME/CODEX/XDG/temp paths, empty Docker config, a local Unix Docker socket and no database, provider or real-run variables. Pre/post Git-root probes for state, HOME and temp all failed with empty root output as required. |
| Cleanup | Implementation `state/` is absent. Its retained pre/post task-label container inventories are empty and temp residue is empty. The reviewer's own probe state is also removed. |
| Historical evidence boundary | The implementation's before/after top-level names, sizes, modes and mtimes match. This is a bounded metadata comparison, not a complete file-byte audit or proof of all intermediate filesystem writes. Source and command review shows the tests no longer select the historical root. |
| Access and acceptance records | Account access remains unknown; the unauthenticated HTTP403 challenge is explicitly not an entitlement denial. Original screening scope is retained. CLI and SDK counters remain separate and unchanged, real-call increment is zero, `execution_admission=false`, task status remains `in_progress`. |

The three retained public access/documentation responses match their recorded
hashes and authentication flags. The published text supports the report's
identity/organization/project/model/surface qualification and distinction between
application and approval. The reviewer performed no new web/account query;
the Safari outcome is the main session's observation, not a separately repeated
account-status proof. No applicable grant is established by these materials.

A final file-stability check initially detected one concurrent main-session
wording edit: the access report stopped calling the original application link
currently valid and now describes it only as the original receipt's pointer.
The reviewer re-read the report, accepted the narrower claim and updated its
reviewed hash. The initial failed stability check and disposition are retained
in private `document-drift.json`; no source or test evidence changed.

No full-repository checks, historical dump decoding, native CLI probes, C1/D1
flows or actual model runs were repeated. The existing rule for private test
state already covers this change; no additional project-wide spec is needed.

## Reviewed file identities

Task-relative paths below use the real-model acceptance task root, except the
first row. These are the exact bytes reviewed before the main session records
this check's result and subsequent commit/integration/cleanup receipts.

| File | SHA-256 |
| --- | --- |
| Parent `11-18-security-remediation-squad-v2/prd.md` | `849830a2b30c3eff9e356cc6eecb9d627c3a24fcdffe3707c7cd6d9edf2e063a` |
| `implement.md` | `f9f267299f135aa1f46a34a307675a70725b7a7fca49cb6040740b735fe82625` |
| `scripts/test_codex_exec_code_container.py` | `1639596a9706bb8837416a4d865b82921643970a6f90a8b4bf13a51d1c1ae0f0` |
| `scripts/test_contained_git_role_handoff.py` | `20f718a4755f187af4fe0eea96c8410f8e139a986ca05907ad58a1b51c098138` |
| `task.json` | `6129e9f8805076bf6cdb7c5a5c639fe4e7aab9334d72077a8fe3235ca2004096` |
| `research/access-and-maintenance-20261120.md` | `964c8db47866cd8ed6c7ff2db08ac3f3cc04426f5fb7ae333bae567e69b1c74c` |
| `research/test-directory-maintenance-20261120.md` | `e2019af05ae3b68b1a40235337fb037a6e696a13aea5e7b270f8e453ae459e76` |

Private verification summary SHA-256:
`ed5a09c42ea1b8029e3f4d32a313f73626165cd62709dfd5ffd103f2222c22e9`.
Private setup probe receipt SHA-256:
`90992771d1488ab3e0d5e35ede71958245cd663141d7938fcaf728917869955e`.

## Findings (fixed)

None. The reviewer made no source change; this report is the only repository
file written by the reviewer.

## Findings (not fixed)

No new maintenance defect remains. The previously blocked C1 continuation,
applicable formal tool access/review, new environment and independent admission,
D1 real execution and complete verifier handoff remain outside this check's
write and execution scope. The main session owns commit, integration and final
worktree cleanup. AC9/AC10 remain open.
