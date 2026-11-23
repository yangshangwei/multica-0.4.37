# Two-case detail acceptance runner

Prepared only: `.omx/private/core-e2e-20261104/runtime/run-detail-final.py`. No service, database or browser actions ran. Python AST parsing passed; independent review and execution are root follow-ups. The script imports the unchanged reviewed lane controller and invokes the existing read-only provenance builder.

It selects only the `detail` row from `final-allocated-private.json`, verifies its exact local database name/URL and task allocation endpoints, requires752 migrations/empty users/issues/admin state/manual-only gate, reads the current environment path from the actual registry, verifies owned current API plus production Web/renderer, then switches only the API using `load_manifest/load_env_file/save_manifest/start_api`. It never calls `up`, migration or DB DDL, never rewrites original allocation files, and uses a private new config with legacy auth, new JWT/deployment identity and disabled device/managed/admin/knowledge features. The previous task-owned resource catalog paths are retained.

After actual config and PID checks it runs the provenance builder, executes exactly `e2e/collaboration-detail-journey.spec.ts` with workers1/retries0, and requires exactly2 expected/zero unexpected/skipped/flaky plus unchanged source/client/API proof and exact database/gate(epoch included). Framework/raw credentials/traces remain under private runtime/runs/<run_id>; public summary contains counters, state and private evidence path. On exception after switch it stops only a reverified exact API and retains failure evidence; uncertain ownership is left for root. On normal completion the owned API remains for inspection and root cleanup.

## Exact permit fields

Create a0600 JSON file under the private control root after the last password browser run is complete.

- approved_by=root; candidate=1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc; runner_sha256=current script hash.
- root_allocation_sha256, database_ledger_sha256, detail_allocation_sha256, test_env_sha256, renderer_sha256, builder_sha256: current corresponding files listed in detail-runner-preparation.json.
- source_root: frozen checkout; source_id: current production source fingerprint after all approved harness patches.
- spec_sha256: frozen e2e/collaboration-detail-journey.spec.ts.
- initial_registry_manifest_sha256: actual private registry/envs/<env_name>/manifest.env immediately before this run.
- current_env_sha256: hash of the environment file named by that actual registry (not original legacy.env or stale active-lane.json).
- deployment_id: fresh valid UUID.
- run_id: new lowercase/digit/hyphen name.
- previous_run:{path,sha256}: an actual completed last browser run record under task evidence or runtime/runs.

Run only after independent review:
`python3 .omx/private/core-e2e-20261104/runtime/run-detail-final.py <absolute-private-permit.json>`

Expected artifact paths: private runtime/runs/<run_id>/{api.env,detail-provenance.json,results.json,run.json,database-before.json,database-after.json}; public evidence/<run_id>/summary.json. The script expects the previous owned API live at entry; restore its exact recorded configuration separately if a prior failure stopped it.
