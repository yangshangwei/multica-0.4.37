# Test temporary-directory maintenance — 2026-11-20

The two control-test setups now allocate their synthetic repositories through
`tempfile.TemporaryDirectory()` under the caller's temporary root. They no longer
create directories beneath the historical CLI evidence root.
`ContainedGit.setUp` also resolves the temporary root before passing it to the
existing canonical-path checks. Cleanup remains registered immediately after
allocation and removes only the test-created directory.

Only `scripts/test_codex_exec_code_container.py` and
`scripts/test_contained_git_role_handoff.py` changed. No runtime script, policy,
image, test method, assertion, frozen fixture, template or admission changed.
This maintenance does not implement the blocked C1 continuation or admit C1/D1.

## Verification

Checkout: `/Volumes/artisan/code/2026/mc-secrem-v2-maintenance-20261120T124811Z`,
base `1861f2aab5d2d7a4d6db927a43b6cf66a10de653`.
Private evidence: `/Volumes/artisan/code/2026/secrem-v2-maintenance-20261120T124811Z/implement`.

The existing suite ran once with an explicit environment allowlist, private
HOME/CODEX/XDG/temporary directories and empty Docker configuration. The only
Docker setting retained was the existing local Unix socket endpoint. State,
HOME and temporary roots returned nonzero with empty output from
`git rev-parse --show-toplevel` before and after the tests. The cached immutable
image remained `sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f`;
the existing contained operations retain `--pull=never` and `--network=none`.
No account configuration or credential was copied or accessed.

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- python3 -B -m unittest discover \
  -s .trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts \
  -p 'test_*.py' -v
```

- **50 passed, 0 failed, 0 skipped**, 60.969 seconds; guarded command exited 0.
- Exact executed test IDs match HEAD's 50-method inventory. ASTs of all test
  method bodies are identical to HEAD; only the two setup allocations changed.
- All 29 task Python files parse; `git diff --check` passed.
- The 59-file task script/fixture inventory differs only in the two assigned
  test files. Frozen fixture bytes remain unchanged.
- Historical evidence top-level names, sizes, modes and mtimes were identical
  before and after. This bounded inventory does not claim a full history audit.
- Task-label container inventories were empty before and after; the new
  temporary root was empty after test cleanup. The newly created private
  execution `state/` directory was removed after retaining configuration and
  command evidence. No historical resource was removed.

No Go, TypeScript, full-repository or real-model check was repeated for these
Python test-setup edits. Real model calls and database operations were zero.
The prior tool-access prerequisite and AC9/AC10 remain unresolved. Independent
review of this change belongs to the main session's check stage.

## Evidence hashes

Paths below are relative to the private evidence directory above. Command
receipts include argv, cwd, time, exit code and output hashes; `hashes.json`
indexes the complete retained evidence.

| Artifact | SHA-256 |
| --- | --- |
| `logs/python-controls.stderr` | `a51fdeb61385d1da0b820979ad61bab0794c86030df3ab36d17773db56f6effb` |
| `logs/python-controls.receipt.json` | `b6b2ae7eb3e366bc254d3109ff1c070e62614eb3f2e8f85e5c4ae57629bfc299` |
| `summary.json` | `7f0fd201da2cd91557a68e61bf15f92d031aa1865afcb1ca0ad5f24227eac40e` |
| `cleanup.json` | `e334b559d515104111c1ad44933c28d4a428bf6a3cc6d4577966e5072d251269` |

No new project-wide convention is introduced: this applies the existing
database/test-state isolation rule and the neighboring tests' standard temporary
directory pattern. No additional spec change is needed for this bounded edit.
