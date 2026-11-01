# RV-1 implementation status

Owner: `/root/runtime_rv1`.
Product commit: `58d486656e146237d5de08ccac62dc6b1ec5876b`.
Base commit: `7627916c8bff4fdd9c2b1287a8cfc3f033a9dc85`.
State: **implemented and verified; coordinator independent recheck pending**.

Only `packages/core/collaboration/run-handoff-schema.ts` and its canonical
`run-handoff-schema.test.ts` changed. The existing envelope refinement now
rejects live-only `memory.base_version` in snapshots. Snapshots without that
field preserve their shape; optional live bases remain valid. No helper,
dependency, API field or compatibility fallback was introduced.

| Check | Exit / result |
| --- | --- |
| New canonical snapshot regression before the fix | exit 1; 1 expected failure, 15 filtered skips |
| Complete canonical handoff schema suite after the fix | exit 0; 16 pass, 0 fail/skip |
| `pnpm --filter @multica/core lint` | exit 0 |
| `pnpm --filter @multica/core typecheck` | exit 0 |
| Scoped `git diff --check` | exit 0 |

Commands, exact file hashes, timing and log SHA-256 values are in
`rv1-verification.json` and `rv1-{red,green,lint,typecheck}.json`; raw logs have
the corresponding `.txt` names. No database, migration, Agent CLI or Provider
operation ran. Other authors' pending spec/task/evidence files remain untouched.

This does not close the final merged-candidate, full-suite or independent
review gates. The coordinator can recheck this two-file delta and append its
result to the existing runtime review.
