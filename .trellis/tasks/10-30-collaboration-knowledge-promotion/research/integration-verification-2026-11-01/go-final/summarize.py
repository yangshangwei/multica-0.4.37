"""Select the last executed result per package without adding reruns together."""
from pathlib import Path
import json
import subprocess

OUT = Path(__file__).resolve().parent
ROOT = OUT.parents[5]


def inventory(path):
    packages = {}
    for line in path.read_text().splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        package = event.get("Package")
        if not package:
            continue
        record = packages.setdefault(package, {"source": path.name, "run": [], "pass": [], "fail": [], "skip": [], "skip_output": {}, "status": None})
        test, action = event.get("Test"), event.get("Action")
        if test:
            if action in ("run", "pass", "fail", "skip"):
                record[action].append(test)
            if action == "output":
                record["skip_output"].setdefault(test, []).append(event.get("Output", ""))
        elif action in ("pass", "fail", "skip"):
            record["status"] = action
    for record in packages.values():
        record["skip_output"] = {name: "".join(record["skip_output"].get(name, [])) for name in record["skip"]}
        record["top_level_passes"] = sum("/" not in n for n in record["pass"])
        record["pass_events"] = len(record["pass"])
        record["leaf_passes"] = sum(not any(other.startswith(name + "/") for other in record["pass"]) for name in record["pass"])
        record["unfinished"] = sorted(set(record["run"]) - set(record["pass"]) - set(record["fail"]) - set(record["skip"]))
    return packages


if __name__ == "__main__":
    latest = {}
    for name in ("rest", "affected-rest", "db-clean"):
        latest.update(inventory(OUT / (name + ".jsonl")))
    expected = set(subprocess.check_output(["go", "-C", "server", "list", "./..."], cwd=ROOT).decode().splitlines())
    missing = sorted(expected - latest.keys())
    failures = [p for p, r in latest.items() if r["status"] == "fail" or r["fail"] or r["unfinished"]]
    summary = {
        "selection": "Latest full result per package: rest, then affected-rest, then db-clean; normal/race and earlier reruns are never added",
        "expected_packages": len(expected), "packages_with_tests": sum(r["status"] == "pass" for r in latest.values()),
        "no_test_packages": [p for p, r in latest.items() if r["status"] == "skip" and not r["run"]],
        "missing_packages": missing, "failed_or_unfinished_packages": failures,
        "top_level_passes": sum(r["top_level_passes"] for r in latest.values()),
        "pass_events": sum(r["pass_events"] for r in latest.values()),
        "leaf_passes": sum(r["leaf_passes"] for r in latest.values()),
        "skipped_tests": [{"package": p, "test": n, "output": r["skip_output"][n]} for p, r in latest.items() for n in r["skip"]],
        "packages": latest,
    }
    (OUT / "combined-inventory.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps({k: v for k, v in summary.items() if k not in ("packages", "skipped_tests")}, indent=2))
    raise SystemExit(bool(missing or failures))
