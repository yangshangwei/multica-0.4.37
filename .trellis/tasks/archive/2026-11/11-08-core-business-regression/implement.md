# Execution

1. Capture source and current environment, inspect existing failure evidence.
2. Run frontend static/unit checks and isolated backend package tests in independent lanes.
3. Build API, production Web and desktop renderer; run all browser tests with zero retries.
4. Diagnose failures, add regression coverage and minimally repair actual cause. Repeat impacted complete suites.
5. Execute core conditional authentication/admin/controlled runtime lanes, reconcile E01–E17.
6. Produce Chinese report and exact case inventory, review results and clean up owned processes.

Change boundary: testing and evidence first; code changes only for diagnosed failures, in the owning layer. No unrelated cleanup or new dependencies.
