"""Read-only audit of retained pipeline receipts and task-owned listeners.

Writes only pipeline-audit.json beside this script. It runs no tests, builds,
native clients, provider executables, or task/ledger mutations.
"""

import collections
import datetime
import hashlib
import json
import pathlib
import re
import subprocess
import urllib.request


HERE = pathlib.Path(__file__).resolve().parent
TASK = HERE.parents[1]
REPO = TASK.parents[2]
OLD = TASK / "research/continuation-2026-10-21"
MODULE = "github.com/multica-ai/multica/server/"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read(path):
    return json.loads(path.read_text())


def command(args, cwd=REPO):
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True)
    return {"command": args, "cwd": str(cwd), "exit_code": result.returncode,
            "stdout": result.stdout, "stderr": result.stderr}


def events(name):
    path = OLD / name
    rows = [json.loads(line) for line in path.read_text().splitlines() if line]
    passed = []
    skipped = []
    outcomes = []
    output = collections.defaultdict(list)
    failed = []
    for line, row in enumerate(rows, 1):
        package = row.get("Package", "").removeprefix(MODULE)
        identity = package + "::" + row.get("Test", "")
        if row["Action"] == "output":
            output[identity].append(row.get("Output", "").strip())
        if row["Action"] == "pass" and "Test" in row:
            passed.append({"line": line, "identity": identity})
        if row["Action"] == "skip" and "Test" in row:
            skipped.append({"line": line, "identity": identity})
        if row["Action"] == "fail":
            failed.append({"line": line, "identity": identity})
        if row["Action"] in {"pass", "skip", "fail"} and "Test" not in row:
            outcomes.append({"line": line, "package": package,
                             "outcome": row["Action"], "elapsed": row.get("Elapsed")})
    for item in skipped:
        item["reason_output"] = output[item["identity"]]
    return {"path": str(path.relative_to(REPO)), "sha256": sha(path),
            "started_at": rows[0].get("Time"), "finished_at": rows[-1].get("Time"),
            "passed_top_level": sum("/" not in r["identity"].split("::", 1)[1] for r in passed),
            "passed_including_subtests": len(passed), "failed": failed,
            "skipped": skipped, "package_outcomes": outcomes, "pass_events": passed}


def safe_http(port, path, keys=None):
    url = f"http://127.0.0.1:{port}{path}"
    try:
        with urllib.request.urlopen(url, timeout=8) as response:
            body = response.read()
            status = response.status
        result = {"url": url, "status": status, "sha256": hashlib.sha256(body).hexdigest()}
        if keys:
            data = json.loads(body)
            result["data"] = {key: data[key] for key in keys if key in data}
        return result
    except Exception as error:
        return {"url": url, "error": type(error).__name__ + ": " + str(error)}


report = {"at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
          "scope": "Retained receipts and current read-only provenance; no test/build/runtime mutation",
          "head": command(["git", "rev-parse", "HEAD"])["stdout"].strip()}
run_names = ["go-service-partition-1.jsonl", "go-service-partition-2.jsonl",
             "go-service-partition-3.jsonl", "go-service-redis.jsonl",
             "go-other-packages.jsonl", "go-remaining-packages.jsonl",
             "go-other-redis.jsonl", "linux-router.jsonl"]
runs = {name: events(name) for name in run_names}
report["runs"] = runs
identities = (OLD / "go-service-identities.txt").read_text().splitlines()
expected = {line for line in identities if re.fullmatch(r"Test\w+", line)}
ignored = [line for line in identities if line not in expected]
service_pass = [r["identity"].split("::", 1)[1]
                for name in run_names[:4] for r in runs[name]["pass_events"]
                if "/" not in r["identity"].split("::", 1)[1]]
definitions = read(OLD / "go-service-partitions.json")
definition_ids = [name for d in definitions.values() for name in d["tests"]]
report["service_union"] = {
    "expected": len(expected), "passed_distinct": len(set(service_pass)),
    "missing": sorted(expected - set(service_pass)), "extra": sorted(set(service_pass) - expected),
    "pass_overlap": [k for k, v in collections.Counter(service_pass).items() if v > 1],
    "definition_overlap": [k for k, v in collections.Counter(definition_ids).items() if v > 1],
    "definition_missing": sorted(expected - set(definition_ids)), "ignored_inventory_lines": ignored,
}
report["selector_consistency"] = {}
for part, definition in definitions.items():
    expected_part = set(definition["tests"])
    run = runs[f"go-service-partition-{part}.jsonl"]
    passed_part = {r["identity"].split("::", 1)[1] for r in run["pass_events"]
                   if "/" not in r["identity"].split("::", 1)[1]}
    skipped_part = {r["identity"].split("::", 1)[1] for r in run["skipped"]}
    selector = re.compile(definition["pattern"])
    receipt = OLD / f"go-service-partition-{part}-command.json"
    receipt_data = read(receipt) if receipt.exists() else None
    actual_selector = receipt_data["command"][receipt_data["command"].index("-run") + 1] if receipt_data else None
    report["selector_consistency"][part] = {
        "selected": len(expected_part), "unexecuted": sorted(expected_part - passed_part - skipped_part),
        "unexpected": sorted((passed_part | skipped_part) - expected_part),
        "pattern_missing": sorted(name for name in expected_part if not selector.match(name)),
        "command_receipt_present": receipt.exists(),
        "command_pattern_equal": actual_selector == definition["pattern"] if receipt_data else None,
        "receipt_sha256": sha(receipt) if receipt.exists() else None,
        "flags": receipt_data["command"][-6:] if receipt_data else None,
        "exit_code": receipt_data.get("exit_code") if receipt_data else None,
        "database_alias": receipt_data.get("database_alias") if receipt_data else None,
    }
report["redis_supplement_commands"] = {}
for name in ["go-service-redis-command.json", "go-other-redis-command.json"]:
    data = read(OLD / name)
    report["redis_supplement_commands"][name] = {
        "sha256": sha(OLD / name), "exit_code": data["exit_code"],
        "started_at": data["started_at"], "flags": data["command"][-6:],
        "database_alias": data["command"][2], "redis_url": data["redis_url"],
    }
packages = command(["go", "list", "./..."], REPO / "server")
current_packages = {name.removeprefix(MODULE) for name in packages["stdout"].splitlines()}
observed_packages = {r["package"] for run in runs.values() for r in run["package_outcomes"]}
report["package_inventory"] = {
    "command": packages, "current": len(current_packages), "retained": len(observed_packages),
    "missing_current_package_receipts": sorted(current_packages - observed_packages),
    "unexpected_package_receipts": sorted(observed_packages - current_packages),
    "no_test_packages": sorted({r["package"] for run in runs.values()
                                for r in run["package_outcomes"] if r["outcome"] == "skip"}),
}
all_pass = {r["identity"] for run in runs.values() for r in run["pass_events"]}
report["still_skipped"] = [{"artifact": name, **skip} for name, run in runs.items()
                           for skip in run["skipped"] if skip["identity"] not in all_pass]
matrix = read(TASK / "acceptance-matrix.json")
fields = ["id", "requirements", "criterion", "source", "source_line", "primary_owner", "delivery_slices"]
protected = [{key: row[key] for key in fields} for row in matrix["criteria"]]
catalog = read(TASK / "research/s6-evidence-map.json")
artifact_problems = []
refs = 0
for name, evidence in catalog["evidence"].items():
    for artifact in evidence.get("artifacts", []):
        if not isinstance(artifact, dict) or "path" not in artifact:
            continue
        refs += 1
        path = pathlib.Path(artifact["path"])
        path = path if path.is_absolute() else REPO / path
        if not path.is_file():
            artifact_problems.append({"run": name, "path": artifact["path"], "problem": "missing"})
        elif artifact.get("sha256") and sha(path) != artifact["sha256"]:
            artifact_problems.append({"run": name, "path": artifact["path"], "problem": "SHA mismatch"})
report["ledger"] = {
    "definitions_sha256": hashlib.sha256(json.dumps(protected, sort_keys=True, ensure_ascii=False).encode()).hexdigest(),
    "counts": dict(collections.Counter(row["status"] for row in matrix["criteria"])),
    "criteria_count": len(protected), "full_acceptance_complete": matrix["full_acceptance_complete"],
    "artifact_references_checked": refs, "artifact_problems": artifact_problems,
    "remaining_ids": [row["id"] for row in matrix["criteria"] if row["status"] != "passed"],
}
report["task_validate"] = command(["python3", ".trellis/scripts/task.py", "validate", str(TASK.relative_to(REPO))])
tracked = command(["git", "ls-files", "--cached", "--others", "--exclude-standard", "--",
                   "server", "packages", "apps/web", "apps/desktop"])["stdout"].splitlines()
current = {name: sha(REPO / name) for name in set(tracked) if (REPO / name).is_file()}
report["source_snapshots"] = {}
for name, snapshot in read(OLD / "source-environment-manifest.json")["snapshots"].items():
    root = pathlib.Path(snapshot["path"])
    differences = []
    for path, digest in sorted(current.items()):
        snapshot_file = root / path
        old_digest = sha(snapshot_file) if snapshot_file.is_file() else None
        if old_digest != digest:
            differences.append({"path": path, "snapshot_sha256": old_digest, "current_sha256": digest,
                                "test_only": path.endswith("_test.go") or ".test." in path or ".spec." in path})
    report["source_snapshots"][name] = {
        "path": str(root), "snapshot_manifest_sha256": sha(OLD / "source-environment-manifest.json"),
        "current_application_files": len(current), "differences": differences,
        "product_differences": [d for d in differences if not d["test_only"]],
    }
report["environments"] = {}
public_keys = ["auth_mode", "platform_admin_available", "managed_installation_supported",
               "password_auth_available", "password_signup_available", "account_binding_available",
               "deployment_id", "messaging_integrations_enabled", "feature_flags",
               "daemon_server_url", "daemon_app_url", "allow_signup"]
for name, api, web, renderer, snapshot, api_receipt, web_receipt in [
    ("legacy", 18985, 13905, 6079, "verification-source",
     "gap-recovery-verification-api-after-history-20261021.running.json",
     "gap-recovery-verification-20261021-web.running.json"),
    ("admin", 18996, 13916, 6090, "verification-admin-source",
     "gap-recovery-admin-20261021-api-post-extraction.running.json",
     "gap-recovery-admin-20261021-web.running.json"),
]:
    root = pathlib.Path(report["source_snapshots"][snapshot]["path"])
    data = {"at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "health": safe_http(api, "/health", ["status", "pid", "commit", "started_at"]),
            "public_config": safe_http(api, "/api/config", public_keys),
            "web": safe_http(web, "/"), "renderer": safe_http(renderer, "/"),
            "api_provenance": read(OLD / api_receipt), "web_provenance": read(OLD / web_receipt),
            "web_build_id": (root / "apps/web/.next/BUILD_ID").read_text().strip(),
            "renderer_index_sha256": sha(root / "apps/desktop/out/renderer/index.html"),
            "processes": []}
    for port in [api, web, renderer]:
        listener = command(["lsof", "-nP", "-iTCP:" + str(port), "-sTCP:LISTEN", "-t"])
        for pid in sorted(set(listener["stdout"].split())):
            data["processes"].append({"port": port, "pid": int(pid),
                "identity": command(["ps", "-p", pid, "-o", "pid=,ppid=,pgid=,lstart=,comm="])["stdout"].strip(),
                "cwd": command(["lsof", "-a", "-p", pid, "-d", "cwd", "-Fn"])["stdout"].strip()})
    data["renderer_matches_built_index"] = data["renderer"].get("sha256") == data["renderer_index_sha256"]
    report["environments"][name] = data
report["browser_runs"] = {}
for name in ["continuation-mentions-web", "continuation-mentions-native", "continuation-controls-web",
             "continuation-controls-native", "continuation-admin-web", "continuation-admin-native",
             "continuation-ac33-cross-client", "continuation-ac33-post-extraction"]:
    evidence = catalog["evidence"][name]
    path = REPO / evidence["artifacts"][0]["path"]
    result = read(path)
    report["browser_runs"][name] = {"path": str(path.relative_to(REPO)), "sha256": sha(path),
                                      "stats": result["stats"], "environment": evidence.get("environment"),
                                      "source_manifest": evidence.get("source_manifest")}
report["checks_executed_here"] = ["go list ./...", "task.py validate", "read-only JSONL/hash/selector audit",
                                 "HTTP health/public-config/Web/renderer", "lsof/ps listener identity"]
report["not_executed_here"] = ["Go test/race/vet", "TypeScript/lint suites", "Playwright/Electron",
                               "make check", "provider accounts/quota", "build/restart/gates"]
(HERE / "pipeline-audit.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
print(json.dumps({"service_union": report["service_union"], "package_count": len(current_packages),
                  "missing_package_receipts": report["package_inventory"]["missing_current_package_receipts"],
                  "ledger": report["ledger"], "snapshot_product_changes": {
                      name: d["product_differences"] for name, d in report["source_snapshots"].items()},
                  "report": str((HERE / "pipeline-audit.json").relative_to(REPO))}, ensure_ascii=False))
