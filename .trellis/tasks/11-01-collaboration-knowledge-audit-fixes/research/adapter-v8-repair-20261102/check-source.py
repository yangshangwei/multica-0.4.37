"""Read-only v8 source, official-registry, preservation and whitespace checks."""
import ast
import hashlib
import json
from pathlib import Path
import subprocess
import sys

private, evidence = (Path(arg).resolve() for arg in sys.argv[1:3])
repo = private.parents[3]
manifest = json.loads((evidence / "source-manifest.json").read_text())
compiled = []
for item in manifest["final_files"]:
    source = private / item["path"]
    data = source.read_bytes()
    assert hashlib.sha256(data).hexdigest() == item["sha256"]
    assert data == (evidence / "final-source" / item["path"]).read_bytes()
    if item["path"] in ("check-bin/go", "check-bin/pnpm") or item["path"].endswith(".py"):
        compile(data, str(source), "exec")
        compiled.append(item["path"])

registry = json.loads((private / "go-flag-registry.json").read_text())
declarations = {}
for statement in ast.parse((private / "check-bin/go").read_text()).body:
    if isinstance(statement, ast.Assign) and len(statement.targets) == 1 and isinstance(statement.targets[0], ast.Name):
        name = statement.targets[0].id
        if name == "compile_only":
            break
        if name in ("value_flags", "test_value_flags", "boolean_flags", "test_boolean_flags"):
            declarations[name] = ast.literal_eval(statement.value)
values = declarations["value_flags"] | declarations["test_value_flags"] | {
    "-test." + flag[1:] for flag in declarations["test_value_flags"]}
booleans = declarations["boolean_flags"] | declarations["test_boolean_flags"] | {
    "-test." + flag[1:] for flag in declarations["test_boolean_flags"]}
assert values == set(registry["value_flags"])
assert booleans == set(registry["boolean_flags"])
assert not values & booleans

incoming = json.loads((evidence / "incoming-manifest.json").read_text())
for item in incoming["v7_files"]:
    assert hashlib.sha256((repo / item["path"]).read_bytes()).hexdigest() == item["sha256"]
    assert hashlib.sha256((evidence / item["snapshot"]).read_bytes()).hexdigest() == item["sha256"]
previous = repo / ".trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/adapter-v6-repair-20261102"
previous_hashes = json.loads((previous / "artifact-hashes.json").read_text())
for item in previous_hashes["evidence_files"]:
    assert hashlib.sha256((previous / item["path"]).read_bytes()).hexdigest() == item["sha256"]
pnpm = json.loads((previous / "pnpm-v7.json").read_text())
assert pnpm["wrapper_sha256"] == hashlib.sha256((private / "check-bin/pnpm").read_bytes()).hexdigest()
assert (pnpm["tests_run"], pnpm["failure_count"], pnpm["error_count"]) == (5, 0, 0)

whitespace = []
for name in ("check-bin/go", "check-go-package-binding-test.py"):
    command = ["git", "diff", "--no-index", "--check", str(evidence / "incoming/adapter-v7" / name), str(private / name)]
    result = subprocess.run(command, text=True, capture_output=True)
    # --no-index returns 1 for an ordinary diff even with no --check diagnostics.
    assert result.returncode in (0, 1) and not result.stdout and not result.stderr, result.stdout + result.stderr
    whitespace.append({"command": command, "exit_code": result.returncode, "stdout": result.stdout, "stderr": result.stderr})

report = {"compiled_python_files": compiled, "syntax_errors": 0, "all_source_snapshots_match": True,
          "declared_value_flags_match_official_registry": len(values), "declared_boolean_flags_match_official_registry": len(booleans),
          "incoming_v7_sources_unchanged": len(incoming["v7_files"]),
          "previous_v7_evidence_files_unchanged": len(previous_hashes["evidence_files"]),
          "unchanged_pnpm_proof_reused": {"file": str(previous / "pnpm-v7.json"), "tests_run": 5, "sha256": pnpm["wrapper_sha256"]},
          "whitespace_checks": whitespace, "actual_go_tests_or_testmain": 0, "actual_sql_connections": 0}
(evidence / "source-check.json").write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps(report, indent=2))
