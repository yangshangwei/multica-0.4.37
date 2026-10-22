"""Run a disjoint, recorded portion of the existing service race suite."""

import datetime
import json
import pathlib
import subprocess
import sys


task_dir = pathlib.Path(__file__).resolve().parent
repo = task_dir.parents[4]
partition = sys.argv[1]
definition = json.loads((task_dir / "go-service-partitions.json").read_text())[partition]
database = {"2": "service_s6_collab", "3": "service_s6_ordinary"}[partition]
command = [
    "python3", "/tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py", database,
    "bash", "../scripts/go-test-with-agent-cli-guard.sh", "go", "test", "-json", "-race",
    "./internal/service", "-run", definition["pattern"],
    "-p", "1", "-parallel", "1", "-count=1", "-timeout=30m",
]
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
with (task_dir / f"go-service-partition-{partition}.jsonl").open("w") as output:
    with (task_dir / f"go-service-partition-{partition}.stderr").open("w") as errors:
        result = subprocess.run(command, cwd=repo / "server", stdout=output, stderr=errors)
summary = {
    "command": command, "cwd": str(repo / "server"), "database_alias": database,
    "started_at": started,
    "finished_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "selected_top_level_tests": len(definition["tests"]), "exit_code": result.returncode,
}
(task_dir / f"go-service-partition-{partition}-command.json").write_text(json.dumps(summary, indent=2) + "\n")
print(json.dumps(summary))
sys.exit(result.returncode)
