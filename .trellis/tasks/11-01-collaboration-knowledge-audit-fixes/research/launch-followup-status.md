# Launch acceptance follow-up

Owner: `/root/knowledge_launch_followups`. The live task payload was empty; scope is recovered from the parent's visible dispatch record for this exact child. Worktree `mc-kfix-lane-launch`, branch `codex/kfix-lane-launch-20261101`, base `f7b610692` was clean. Other worktrees and the coordinator's database suites remain untouched.

This slice closes H2 review coverage gaps, not a reproduced product defect. The existing daemon correctly accepts only its own recorded reachability drop, a fresh server-minted generation with no native ID and otherwise unchanged binding, and then uses that prepared binding for the launch intent. Canonical boundary tests will exercise claim validation, preparation validation and final prompt/intent construction without executing a Provider. Overlay mutants will prove deleting the acceptance checks fails the new tests; no base-red product claim will be fabricated.

Expected files: new `server/internal/daemon/knowledge_context_rebind_boundary_test.go`, the existing execution spec and runtime operator guide, plus task evidence. No new production abstraction or dependency. The spec table will clarify identical request replay and the guide will explicitly distinguish old daemon/new API from new daemon/old API refusal. No database commands, migrations, installed Agent CLI or real Provider calls are permitted.

Validation: guarded targeted normal/race tests, independent overlay mutation runs, daemon build/vet and bounded full daemon suite if resources permit. Independent review and final merged-candidate validation remain the coordinator's gates.

## Live results

- New boundary matrix:19 named tests/subtests pass, no fail or skip.
- Race plus adjacent unreachable-resume and L7 cases:25 named passes, no fail or skip.
- Seven targeted overlay mutants all killed: missing drop provenance, uncleared task pointer, uncleared execution pointer, omitted preparation-session predicate, reused generation, omitted preparation validation and reuse of the claimed launch identity. Production source was never edited by mutations.
- Daemon build/vet and `git diff --check` pass.
- The test's generation setup was simplified after the first run; final normal/race repeats and the bounded full daemon suite are now running against the final test bytes. No full-suite result is claimed yet.
- Spec and operator guide now explicitly describe exact original-key replay after first preparation and the older-daemon/newer-API refusal loop. Product code has no changes.
