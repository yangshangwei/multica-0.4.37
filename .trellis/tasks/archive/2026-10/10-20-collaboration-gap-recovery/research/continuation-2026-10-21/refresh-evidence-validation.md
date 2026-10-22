# Evidence refresh correction — 2026-10-21

Boundary: only `refresh-evidence.py`, its private-fixture regression tests and this report. The existing helper may refresh actual PASS identities and references, but it must never infer acceptance, clear clause gaps, or overwrite coordinator-owned artifacts. Root performs acceptance integration from the independently reviewed decision and exact immutable definitions/source/artifacts.

Correction sequence: first reproduce automatic promotion, discarded canonical identity and overwrite behavior with private 97-row fixtures; remove `--apply`/`--promote` and implicit status/gap changes; retain resolved canonical identities and partial pending-row references; write every output set to a fresh directory; run the focused tests and a read-only current-artifact refresh with fresh temporary output.

No product source, authoritative ledger/catalog, existing next proposal, credential, runtime setting or task lifecycle changes are part of this correction. Test scratch directories contain no real accounts or database access.

## Corrected behavior

- Removed `--apply` and `--promote`; argparse rejects either with exit 2 before reading/writing task artifacts. The helper never changes acceptance status, clause gaps, assessment, rollout limitations, task state or `full_acceptance_complete`.
- Default outputs go into a unique system temporary directory. `--output-dir PATH` accepts only a new directory; an existing directory raises `FileExistsError` before any output write. Existing coordinator `.next` snapshots and summary/proposal files are preserved.
- Canonical test identities are retained, including identities formerly only in `missing_current_test_evidence`. Completed identities leave that missing-execution list, but the criterion and remaining clause gaps stay pending until root reviews/integrates them. Pending references always retain `partial_support`; newly added references do not claim whole-criterion coverage.
- Previously reviewed catalog source/environment/report/limits and supplementary artifacts survive refresh. A catalogued raw-result hash mismatch raises `ValueError` before output, rather than accepting rewritten evidence.
- Failed raw results, incomplete service inventory and duplicate AC IDs refuse refresh. The immutable definition comparison remains; all 97 criterion definitions and primary owners remain unchanged.

Acceptance promotion is intentionally absent from this tool. Root/manual integration requires the independent decision, exact protected-definition hash, actual PASS identities, source and current artifact verification. No new review/approval abstraction or dependency was added.

## Failure-first evidence

Before editing the helper, four private-fixture regression identities were run with Python's built-in unittest runner. All four reproduced their intended gap: automatic `passed` status after the last missing test resolved, missing canonical `test_identities`, overwritten existing coordinator output, and accepted `--apply`/`--promote`. The scoped run ended `FAILED (failures=6)` with no dependency/environment errors; multiple assertions belong to the forbidden-flag test. The initial all-tests discovery also recorded missing new `--output-dir` support and an assertion-context test defect, which was corrected before the clean failure-first run. No real task ledger or existing proposal was used as writable test input.

Exact failure-first command:

```sh
python3 -B - <<'PY'
import sys
import unittest
from pathlib import Path
path = Path('.trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21')
sys.path.insert(0, str(path))
from test_refresh_evidence import RefreshEvidenceTest
names = [
    'test_all_missing_executions_resolved_does_not_promote_or_clear_clause_gaps',
    'test_resolved_canonical_identity_is_retained_and_pending_links_are_partial',
    'test_default_refresh_preserves_acceptance_and_existing_files',
    'test_apply_and_promote_are_rejected_without_touching_private_authoritative_files',
]
suite = unittest.TestSuite(RefreshEvidenceTest(name) for name in names)
result = unittest.TextTestRunner(verbosity=2).run(suite)
raise SystemExit(not result.wasSuccessful())
PY
```

## Final verification

From the repository root:

```sh
python3 -B -m unittest discover -s .trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21 -p test_refresh_evidence.py -v
python3 -B .trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21/refresh-evidence.py
python3 .trellis/scripts/task.py validate .trellis/tasks/10-20-collaboration-gap-recovery
```

Focused tests: **9 passed**, 0 failures/errors/skips, 0.077s. They cover status/gap preservation after all executions resolve; exact canonical identity retention; partial references; forbidden flags; default-output isolation; existing-output refusal; raw test failure; recorded artifact drift; duplicate definitions; retained catalog provenance/limits. All fixtures and mutations stay inside private temporary directories.

The final read-only current-artifact refresh exits 0 and writes only:

`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-evidence-refresh-et6zya14/`

Its proposal says `promoted_ids: []`, `acceptance_review_required: true`, and **64 passed / 33 pending**, exactly the authoritative baseline at invocation. Independent before/after checks confirm the authoritative matrix, authoritative catalog and all four existing summary/proposal/next files have unchanged SHA-256 bytes; every status/criterion/owner/clause gap/assessment is preserved, all canonical IDs survive, pending links are partial, and existing catalog provenance/limits remain. All four generated JSON outputs parse.

Protected definition SHA-256 remains:

`e29ead58ba9e2c45a4099a52611c55e12dcae3b735136e058aafc87674cf217e`

Final source identities:

| File | SHA-256 |
| --- | --- |
| `refresh-evidence.py` | `8689cf6d773565e06a3b0b9521d8abb3e8670b97c6b1bf75556b87c3983ea265` |
| `test_refresh_evidence.py` | `852b8583a1fb298baaa7e001766216eaec4b17d591dbecd66c5f43759e2a54f1` |

Python AST parse/compile and whitespace checks pass for both scripts. `ruff`, `pyright` and `mypy` are not installed; none was installed for this task-directory Python helper. Product lint/typecheck or service/browser/native suites were not rerun; this change does not touch product modules. Trellis validation and final report whitespace are recorded after writing.

Changed files: `refresh-evidence.py`, `test_refresh_evidence.py`, this report. Simplification: removed automated acceptance and authoritative-write paths; no new dependencies. Remaining boundary: this helper knows a fixed continuation-run inventory and only refreshes execution references. Root remains responsible for immutable criteria, reviewed clause coverage/source/provenance and any acceptance integration. Real provider/OS/installer/fault limitations stay explicit; no commit, archive or rollout change was made.

## Coordinator fresh verification

The coordinator reran the same private-fixture unittest command after reviewing
this source: 9/9 passed, 0 failures/errors/skips, 0.083s. No authoritative or
existing proposal file was used as a writable test fixture. Product state and
gates remain unchanged. This corroborates the implementer result above.
