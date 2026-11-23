"""Read-only syntax, snapshot and whitespace checks for the owned adapter."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

private, evidence = (Path(arg).resolve() for arg in sys.argv[1:3])
manifest = json.loads((evidence / "source-manifest.json").read_text())
checked = []
for item in manifest["final_files"]:
    source = private / item["path"]
    data = source.read_bytes()
    assert hashlib.sha256(data).hexdigest() == item["sha256"]
    assert data == (evidence / "final-source" / item["path"]).read_bytes()
    if item["path"] != "deny-network.sb":
        compile(data, str(source), "exec")
        checked.append(item["path"])

incoming = json.loads((evidence / "incoming-manifest.json").read_text())
live_sources = []
for item in incoming["files"]:
    source = private.parents[3] / item["path"]
    current = hashlib.sha256(source.read_bytes()).hexdigest()
    assert hashlib.sha256((evidence / item["snapshot"]).read_bytes()).hexdigest() == item["sha256"]
    if item["version"] == "adapter-v5":
        assert current == item["sha256"]
    live_sources.append({"path": item["path"], "incoming_sha256": item["sha256"],
                         "current_sha256": current, "matches_incoming": current == item["sha256"]})

whitespace = []
for name in ("check-bin/go", "check-go-isolation-test.py"):
    command = ["git", "diff", "--no-index", "--check", str(evidence / "incoming/adapter-v6" / name), str(private / name)]
    result = subprocess.run(command, text=True, capture_output=True)
    # --no-index uses exit 1 for an ordinary diff even when --check is clean.
    assert result.returncode in (0, 1) and not result.stdout and not result.stderr, result.stdout + result.stderr
    whitespace.append({"command": command, "exit_code": result.returncode, "stdout": result.stdout, "stderr": result.stderr})

report = {"compiled_python_files": checked, "syntax_errors": 0, "snapshot_matches": True,
          "live_sources": live_sources, "whitespace_checks": whitespace, "real_go_tests": 0,
          "real_sql_connections": 0, "real_pnpm_browser_agent_cli_calls": 0}
(evidence / "source-check.json").write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps(report, indent=2))
