# v12 SQLC recovery — 2026-11-03

Actual `make sqlc` completed with exit **0** at 2026-11-03T09:37:12Z on candidate `b6c74033f1913e46418ec1b119da33236dcba805`. All **130** generated files are byte-identical before/after; all SQL/schema/Make/configuration inputs also remain unchanged. No product or test source was edited.

The earlier `sqlc-check.json/log` failure remains unchanged. Its `GOPROXY=off` setting prevented the versioned Go run from resolving module metadata. The new command uses the already cached file proxy at `file:///Users/artisan/go/pkg/mod/cache/download`, pinned Go 1.26.6, a fresh task-owned module/build cache and an empty application environment file. `sandbox-exec` denies all network access and permits writes only to the new private run directory and `server/pkg/db/generated`. The raw Go log's “downloading” messages describe local file-proxy extraction, not network downloads.

Evidence:

- `sqlc-resume-01.json` records the exact command, nonsecret environment, timestamps, tool identity and before/after fingerprints.
- Private root: `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/final-check-v12/sqlc-resume-20261103-01/`.
- Private files: `run.py`, `offline.sb`, `empty.env`, `started.json`, `result.json`, `raw.log`, `source-{before,after}.json`, and `generated-{before,after}.json`; caches remain task-owned.
- Input fingerprint: `d1f8660d2199faf5a49b5d197b402fd43b9f34a38163ab3cb9599b5d5d9b95c5`.
- Generated fingerprint: `b6e9aef0c644cc6e76663a39b1fcaa948350a40dc58dd67999710f372ab3dae8`.
- Raw log SHA256: `a38fa5b6e34605fee85cebdb7a25acd0c8998acb4c50978d9d5c5e1c637802ab`.
- Result SHA256: `06e7b68c5b5157a3bc91bf467e9387cd7f463d51f3d3e0b09d3a421ea1f67ae7`.

Validation additionally confirmed `git diff --exit-code` is clean for generated files, migrations, queries, Go module files, sqlc configuration and Makefile. No lint/typecheck/test suite, service, database, renderer or Provider operation ran in this bounded recovery. This result is SQLC generation evidence only; final candidate freezing, independent clearance and the complete repository gate remain coordinator-owned. Reproduction must allocate a new numbered evidence directory; the runner intentionally refuses to overwrite its existing outputs.
