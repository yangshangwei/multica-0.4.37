"""Verify the exact nine-file outbound commit slice over an external HEAD archive."""

from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
import hashlib
import io
import json
import os
import shutil
import socket
import subprocess
import tarfile
import tempfile
import time


evidence = Path(__file__).resolve().parent
repo = evidence.parents[4]
base_head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip()
expected_head = "cae15dd72ee568f28ce29b19f0c76765b72f26cd"
if base_head != expected_head:
    raise SystemExit("the parent-requested HEAD changed; isolation needs a fresh dispatch")
files = [
    f"server/internal/integrations/{package}/collaboration_outbound_boundary_test.go"
    for package in ("lark", "dingtalk", "telegram", "wecom")
] + [
    "server/internal/integrations/slack/collaboration_typing_boundary_test.go",
    "server/internal/integrations/wecom/relay_outbound.go",
    "server/internal/integrations/wecom/outbound_test.go",
    "server/internal/integrations/wecom/outbound_two_replica_db_test.go",
    "server/internal/integrations/wecom/relay_ordering_db_test.go",
]
copied_hashes = {file: hashlib.sha256((repo / file).read_bytes()).hexdigest() for file in files}
archive = subprocess.check_output(["git", "archive", "--format=tar", base_head], cwd=repo)
isolation = Path(tempfile.mkdtemp(prefix="multica-outbound-isolated-final-20261022-"))
with tarfile.open(fileobj=io.BytesIO(archive)) as content:
    content.extractall(isolation, filter="data")
for file in files:
    target = isolation / file
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(repo / file, target)
    if hashlib.sha256(target.read_bytes()).hexdigest() != copied_hashes[file]:
        raise SystemExit("copy did not preserve exact slice identity: " + file)
with socket.create_connection(("127.0.0.1", 57244), timeout=3) as connection:
    connection.sendall(b"*1\r\n$4\r\nPING\r\n")
    if connection.recv(128) != b"+PONG\r\n":
        raise SystemExit("loopback Redis prerequisite did not answer PONG")
listener_command = ["/usr/sbin/lsof", "-nP", "-iTCP:57244", "-sTCP:LISTEN"]
listener = subprocess.run(listener_command, capture_output=True, text=True)
if listener.returncode != 0 or "LISTEN" not in listener.stdout:
    raise SystemExit("loopback Redis listener was not verified")


def all_source_hashes():
    result = {}
    for path in sorted((isolation / "server").rglob("*.go")):
        result[str(path.relative_to(isolation))] = hashlib.sha256(path.read_bytes()).hexdigest()
    for relative in ("server/go.mod", "server/go.sum", "scripts/go-test-with-agent-cli-guard.sh"):
        result[relative] = hashlib.sha256((isolation / relative).read_bytes()).hexdigest()
    return result


launcher = "/tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py"
command = [
    "python3", launcher, "service_s6_other",
    "env", "REDIS_TEST_URL=redis://127.0.0.1:57244/0",
    "bash", "../scripts/go-test-with-agent-cli-guard.sh",
    "go", "test", "-json", "-race",
    "./internal/integrations/lark", "./internal/integrations/dingtalk",
    "./internal/integrations/telegram", "./internal/integrations/wecom",
    "./internal/integrations/slack", "-run",
    "^Test(CollaborationOutbound|TwoReplicas|Relay_|DeliverRelayed|DrainDelivers|RelayedInbox)",
    "-p", "1", "-parallel", "1", "-count=1", "-timeout=30m",
]
cwd = isolation / "server"
started_at = datetime.now(timezone.utc).isoformat()
source_before = all_source_hashes()
receipt = {
    "command": command,
    "cwd": str(cwd),
    "started_at": started_at,
    "base_head": base_head,
    "archive_command": ["git", "archive", "--format=tar", base_head],
    "archive_sha256": hashlib.sha256(archive).hexdigest(),
    "copied_files": copied_hashes,
    "copied_file_count": len(files),
    "isolation_directory": str(isolation),
    "database_alias": "service_s6_other",
    "schema_prerequisite": "737_collaboration_runtime_preparation",
    "redis_url": "redis://127.0.0.1:57244/0",
    "redis_ping": "+PONG",
    "redis_listener_command": listener_command,
    "redis_listener_output": listener.stdout,
    "source_sha256": source_before,
    "root_source_writes": False,
    "real_provider_account_access": False,
    "external_im": False,
    "production_gate_change": False,
}
prefix = evidence / "outbound-tail-isolated-final"
prefix.with_name(prefix.name + "-command.json").write_text(json.dumps(receipt, indent=2) + "\n")
stdout_path = prefix.with_suffix(".jsonl")
stderr_path = prefix.with_suffix(".stderr")
env = os.environ.copy()
env["GOTOOLCHAIN"] = "go1.26.6"
started = time.monotonic()
with stdout_path.open("wb") as stdout, stderr_path.open("wb") as stderr:
    completed = subprocess.run(command, cwd=cwd, env=env, stdout=stdout, stderr=stderr)
source_after = all_source_hashes()
tests = {"passed": [], "failed": [], "skipped": []}
packages = {}
actions = Counter()
for line in stdout_path.read_text(errors="replace").splitlines():
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    action = event.get("Action")
    if action not in ("pass", "fail", "skip"):
        continue
    actions[action] += 1
    if event.get("Test"):
        status = {"pass": "passed", "fail": "failed", "skip": "skipped"}[action]
        tests[status].append({"package": event.get("Package"), "test": event["Test"]})
    else:
        packages[event.get("Package")] = action
root_after = {file: hashlib.sha256((repo / file).read_bytes()).hexdigest() for file in files}
result = {
    "exit_code": completed.returncode,
    "started_at": started_at,
    "finished_at": datetime.now(timezone.utc).isoformat(),
    "duration_seconds": round(time.monotonic() - started, 3),
    "base_head": base_head,
    "root_head_after": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
    "archive_sha256": receipt["archive_sha256"],
    "isolation_directory": str(isolation),
    "copied_files": copied_hashes,
    "copied_file_count": len(files),
    "tests": tests,
    "counts": {status: len(items) for status, items in tests.items()},
    "top_level_counts": {status: sum("/" not in item["test"] for item in items) for status, items in tests.items()},
    "packages": packages,
    "go_event_actions": dict(actions),
    "stdout_sha256": hashlib.sha256(stdout_path.read_bytes()).hexdigest(),
    "stderr_sha256": hashlib.sha256(stderr_path.read_bytes()).hexdigest(),
    "source_changed_during_run": [file for file in sorted(set(source_before) | set(source_after)) if source_before.get(file) != source_after.get(file)],
    "source_sha256_after": source_after,
    "root_copied_files_changed_during_run": [file for file in files if copied_hashes[file] != root_after[file]],
    "root_copied_files_sha256_after": root_after,
    "root_source_writes": False,
    "real_provider_account_access": False,
    "external_im": False,
    "production_gate_change": False,
}
prefix.with_name(prefix.name + "-result.json").write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps({key: result[key] for key in ("exit_code", "duration_seconds", "counts", "top_level_counts", "source_changed_during_run", "root_copied_files_changed_during_run")}), flush=True)
raise SystemExit(completed.returncode)
