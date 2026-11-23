# Backend runner independent review

**PASS** for the exact reviewed private runner; zero open execution blockers. This is permission-preparation review, not a test-pass report. Root retains final allocation approval and run supervision.

- Candidate: `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`
- Runner SHA-256: `787f8ad19091bf955a2264bbecabb7560a06042e71a48a466242c5ce051b45df`
- Allocation request SHA-256: `a54fbcbdbf24ec896c484de4f321479d229a174b547f690dceff0490d77350ff`
- Source/tools SHA-256: `252147577459a20f4deba1ddb88551d3d0013db2a6cbe9b9d3f670923e4456bb`
- Scope: 82 unique exact packages and databases, 10413 expected top-level test names. Nine packages have no named tests and remain separately accounted.

## Findings fixed by runner author during review

1. `run.py` previously trusted the unbound allocation request, which selects the actual relative package argument and expected names. It now requires `allocation_request_sha256` in the root permit before consuming that file.
2. JSON accounting previously ignored the Package field and did not gate on package terminal events. It now rejects foreign/missing package identity, reconciles expected names, and requires one valid exact-package terminal.
3. Source auditing previously omitted three real Go-test inputs outside server/scripts. It now binds `packages/core/runtimes/cli-version.ts`, `packages/core/skills/presentation.ts`, and `packages/views/triage/triage-csv-template.json`; refuses nonignored untracked backend/script inputs; and binds physical Go/Node/Bash executables by hash.

## Verified controls

The manifest has no package/database/name duplicates and every package resolves to its declared relative argument. The runner validates private root permit, candidate, request and source hash; checks actual DB name, migration count and zero issues through a bounded read-only probe; uses one dedicated HOME and TMP per package and explicit caches; strips deployment/pool/task-root/model/account environment; wraps tests with the current agent CLI sentinel guard; serializes packages with bounded Go concurrency; retains unique raw logs plus named pass/fail/skip and original failures; and refuses source changes before/after packages and final vet.

## Findings not changed and scope notes

- Two existing test packages themselves create and clean timestamp-named databases. Root inspected and explicitly authorized this test behavior and owns before/after audits for `multica_codex_usage_backfill_`, `multica_v034_upgrade_`, and `multica_v034_fresh_`. The runner adds no DDL. Do not force-drop unowned leftovers.
- The read-only DB probe is a defensive identity check, not a complete empty-table audit. Root's fresh752-migration template and fresh clone allocation establish that prerequisite.
- Skipped cases and packages without named tests are not business passes. Full native/model/conditional acceptance remains separate.
- Whole-product final source/build/endpoint/resource auditing belongs to root. The backend runner does not replace that final gate.
- The 40-minute Go timeout covers test execution; compilation and vet still rely on root process supervision.

## Verification

Read-only source/manifest review and Python AST/manifest consistency checks passed. Author-supplied synthetic summary/source checks v2/v3 were inspected. This reviewer did not execute the runner, connect to any database, run application suites, modify runner code or alter product files. Lint/typecheck are not applicable to this inventory/review-only assignment.
