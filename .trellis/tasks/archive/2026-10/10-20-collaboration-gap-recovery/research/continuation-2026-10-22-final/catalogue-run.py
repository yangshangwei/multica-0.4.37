#!/usr/bin/env python3
"""Record the exact task-owned AC96 race run without exposing environment values."""

import datetime
import hashlib
import json
import pathlib
import subprocess
import sys
import time


root = pathlib.Path(__file__).resolve().parents[5]
evidence = pathlib.Path(__file__).resolve().parent
prefix = sys.argv[1]
if not prefix.startswith("catalogue-") or "/" in prefix:
    raise SystemExit("Expected a catalogue-prefixed receipt name")
command = [
    "python3", "/tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py",
    "service_s5", "bash", "../scripts/go-test-with-agent-cli-guard.sh", "go",
    "test", "-json", "-race", "./internal/service", "-run",
    "^TestCollaborationCatalogueAuthorityMatrix$", "-p", "1", "-parallel",
    "1", "-count=1", "-timeout=30m",
]
paths = [
    "server/internal/service/collaboration_catalogue_matrix_test.go",
    "server/internal/service/collaboration_runtime_test.go",
    "server/internal/service/collaboration_owner_departure_targets_test.go",
    "server/internal/service/collaboration_control_managed_test.go",
    "server/internal/service/collaboration_authority_test.go",
    "server/internal/service/collaboration_test.go",
    "server/internal/service/collaboration_authority.go",
    "server/internal/service/collaboration_targets.go",
    "server/internal/service/collaboration_squad.go",
    "server/internal/service/collaboration_runtime.go",
    "server/internal/service/collaboration_runtime_capabilities.go",
    "server/internal/service/collaboration_group_dependencies.go",
    "server/internal/service/collaboration_workflow.go",
    "server/internal/service/source_context.go",
    "server/internal/auth/password_session.go",
    "server/internal/auth/managed_binding.go",
    "server/pkg/db/queries/collaboration_authority.sql",
    "server/pkg/db/queries/collaboration_runtime.sql",
    "server/pkg/db/queries/collaboration_squad.sql",
    "server/pkg/db/generated/collaboration_authority.sql.go",
    "server/pkg/db/generated/collaboration_runtime.sql.go",
    "server/pkg/db/generated/collaboration_squad.sql.go",
]
source = {p: hashlib.sha256((root / p).read_bytes()).hexdigest() for p in paths}
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
metadata = {
    "argv": command,
    "cwd": str(root / "server"),
    "started_at": started,
    "branch": subprocess.check_output(
        ["git", "branch", "--show-current"], cwd=root, text=True
    ).strip(),
    "head": subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=root, text=True
    ).strip(),
    "database_alias": "service_s5",
    "schema_boundary": "through migration 737; task-owned serial DB",
    "source_hashes": source,
    "provider_calls": False,
    "proof": "synthetic service protocol, no kernel execution",
    "production_gates": "unchanged; fixture gate restored by test cleanup",
}
(evidence / (prefix + "-command.json")).write_text(
    json.dumps(metadata, indent=2) + "\n"
)
start = time.monotonic()
with (evidence / (prefix + ".jsonl")).open("w") as stdout, (
    evidence / (prefix + ".stderr")
).open("w") as stderr:
    result = subprocess.run(command, cwd=root / "server", stdout=stdout, stderr=stderr)
rows = []
for line in (evidence / (prefix + ".jsonl")).read_text().splitlines():
    try:
        rows.append(json.loads(line))
    except json.JSONDecodeError:
        pass
summary = {
    "started_at": started,
    "database_alias": "service_s5",
    "head": metadata["head"],
    "finished_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "exit_code": result.returncode,
    "wall_seconds": round(time.monotonic() - start, 3),
    "top_level_passes": [
        r["Test"] for r in rows
        if r.get("Action") == "pass" and r.get("Test") and "/" not in r["Test"]
    ],
    "passes": sum(r.get("Action") == "pass" and "Test" in r for r in rows),
    "failures": [r.get("Test", "package") for r in rows if r.get("Action") == "fail"],
    "skips": [r.get("Test", "package") for r in rows if r.get("Action") == "skip"],
    "source_unchanged": all(
        hashlib.sha256((root / p).read_bytes()).hexdigest() == digest
        for p, digest in source.items()
    ),
}
(evidence / (prefix + "-result.json")).write_text(
    json.dumps(summary, indent=2) + "\n"
)
print(json.dumps(summary, indent=2))
for row in rows:
    output = row.get("Output", "")
    if row.get("Action") == "output" and (
        "collaboration_catalogue_matrix_test.go:" in output or "FAIL" in output
    ):
        print(output.rstrip())
raise SystemExit(result.returncode)
