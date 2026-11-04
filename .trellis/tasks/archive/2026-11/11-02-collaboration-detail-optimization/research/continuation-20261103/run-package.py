#!/usr/bin/env python3
"""Execute only an explicitly allocated collaboration verification package."""

import argparse
import datetime
import json
import os
import pathlib
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--lane", choices=("service", "handler", "router"), required=True)
    parser.add_argument("--run", required=True)
    parser.add_argument("--label", required=True)
    parser.add_argument("--race", action="store_true")
    parser.add_argument("--allocation-file", type=pathlib.Path)
    args = parser.parse_args()
    task = pathlib.Path(__file__).resolve().parents[2]
    root = task.parents[2]
    private = pathlib.Path("/tmp/multica-detail-verification-20261103")
    allocation = json.loads((args.allocation_file or private / "allocation.json").read_text())
    entry = next(item for item in allocation["allocations"] if item["lane"] == args.lane)
    configuration = json.loads(pathlib.Path(entry["configuration"]).read_text())
    process_environment = os.environ.copy()
    for name in (
        "DATABASE_MAX_CONNS", "DATABASE_MIN_CONNS", "MULTICA_RESOURCE_PUBLISH_DIR",
        "MULTICA_TASK_CONFIG_ROOT", "MULTICA_TASK_WORKSPACES_ROOT",
        "MULTICA_RUN_REAL_AGENT_SMOKE", "REDIS_TEST_URL",
    ):
        process_environment.pop(name, None)
    process_environment.update(configuration)
    process_environment["PATH"] = "/opt/homebrew/opt/libpq/bin:" + process_environment.get("PATH", "")
    package_home = private / (args.lane + "-home")
    package_home.mkdir(mode=0o700, exist_ok=True)
    process_environment["HOME"] = str(package_home)
    actual = subprocess.check_output(
        ["/opt/homebrew/opt/libpq/bin/psql", "-X", "-d", configuration["DATABASE_URL"],
         "-At", "-v", "ON_ERROR_STOP=1", "-c", "SELECT current_database()"],
        env=process_environment, text=True,
    ).strip()
    if actual != entry["database"]:
        raise SystemExit("verification database identity differs from allocation")
    command = ["bash", str(root / "scripts/go-test-with-agent-cli-guard.sh"), "--",
               "go", "-C", str(root / "server"), "test", "-json", "-p", "2", "-parallel", "2", "-timeout=40m", "-count=1"]
    if args.race:
        command.append("-race")
    command.extend([entry["package"], "-run", args.run])
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    output = private / (args.label + "-" + stamp + ".jsonl")
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    with output.open("w") as stream:
        completed = subprocess.run(command, cwd=root, env=process_environment,
                                   stdout=stream, stderr=subprocess.STDOUT)
    inventory = {"pass": [], "fail": [], "skip": []}
    for line in output.read_text().splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get("Test") and event.get("Action") in inventory:
            inventory[event["Action"]].append(event["Test"])
    record = {"started_at": started, "database": actual, "package": entry["package"],
              "command": command, "output": str(output), "exit_code": completed.returncode,
              "inventory": inventory}
    report = pathlib.Path(__file__).parent / (args.label + "-" + stamp + ".json")
    report.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps({"report": str(report), "exit_code": completed.returncode,
                      "named_pass": len(inventory["pass"]), "named_fail": len(inventory["fail"]),
                      "named_skip": len(inventory["skip"]), "output": str(output)}, indent=2))
    sys.exit(completed.returncode)


if __name__ == "__main__":
    main()
