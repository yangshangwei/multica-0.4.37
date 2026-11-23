"""Refresh execution references into fresh proposals; acceptance remains manual."""

import argparse
import copy
import hashlib
import json
import pathlib
import tempfile
from collections import Counter


HERE = pathlib.Path(__file__).resolve().parent
TASK = HERE.parents[1]
REPO = TASK.parents[2]
CATALOG_PATH = TASK / "research/s6-evidence-map.json"
MATRIX_PATH = TASK / "acceptance-matrix.json"


def read_json(path):
    return json.loads(path.read_text())


def artifact(path, recorded=()):
    result = {"path": str(path.relative_to(REPO)) if path.is_relative_to(REPO) else str(path),
              "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
    for previous in recorded:
        previous_path = pathlib.Path(previous["path"])
        if not previous_path.is_absolute():
            previous_path = REPO / previous_path
        if previous_path.resolve() == path.resolve() and previous.get("sha256") not in (None, result["sha256"]):
            raise ValueError(f"Recorded artifact changed: {result['path']}")
    return result


def specs(suite):
    yield from suite.get("specs", [])
    for child in suite.get("suites", []):
        yield from specs(child)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=pathlib.Path, help="New output directory; existing directories are refused.")
    args = parser.parse_args()
    original = read_json(MATRIX_PATH)
    matrix = copy.deepcopy(original)
    catalog = read_json(CATALOG_PATH)
    assert original["task_status"] == "in_progress" and original["full_acceptance_complete"] is False
    assert len(original["criteria"]) == len({row["id"] for row in original["criteria"]}) == 97
    required = {identity for row in matrix["criteria"] for key in ["test_identities", "missing_current_test_evidence"]
                for identity in row.get(key, [])}
    required.update(identity for entry in catalog["evidence"].values() for identity in entry.get("executed_tests", {}))
    current = {}
    summaries = {}
    runs = {
        "continuation-service-1": "go-service-partition-1.jsonl",
        "continuation-service-2": "go-service-partition-2.jsonl",
        "continuation-service-3": "go-service-partition-3.jsonl",
        "continuation-service-redis": "go-service-redis.jsonl",
        "continuation-go-other": "go-other-packages.jsonl",
        "continuation-go-remaining": "go-remaining-packages.jsonl",
        "continuation-go-other-redis": "go-other-redis.jsonl",
        "continuation-linux-router": "linux-router.jsonl",
    }
    for run_id, name in runs.items():
        path = HERE / name
        events = [json.loads(line) for line in path.read_text().splitlines()]
        packages = [row for row in events if row["Action"] in ["pass", "fail", "skip"] and "Test" not in row]
        assert packages and not any(row["Action"] == "fail" for row in events), run_id
        results = {}
        skipped = []
        for row in events:
            if "Test" not in row:
                continue
            identity = row["Package"].split("/server/", 1)[-1] + "::" + row["Test"]
            if row["Action"] == "pass":
                current.setdefault(identity, []).append(run_id)
                if identity in required:
                    results[identity] = {"status": "passed", "elapsed": row.get("Elapsed")}
            if row["Action"] == "skip":
                skipped.append(identity)
        counts = {
            "passed_top_level": sum(row["Action"] == "pass" and "Test" in row and "/" not in row["Test"] for row in events),
            "passed_including_subtests": sum(row["Action"] == "pass" and "Test" in row for row in events),
            "failed": 0, "skipped": len(skipped),
        }
        summaries[run_id] = {**counts, "package_outcomes": [{"package": row["Package"].split("/server/", 1)[-1],
                                                           "status": row["Action"]} for row in packages],
                             "skipped_identities": skipped}
        native = run_id == "continuation-linux-router"
        previous = catalog["evidence"].get(run_id, {})
        refreshed_artifact = artifact(path, previous.get("artifacts", []))
        catalog["evidence"][run_id] = {**previous,
            "kind": "linux_kernel_router_daemon" if native else "go_race_tests", "result": "passed",
            "artifacts": [refreshed_artifact] + [item for item in previous.get("artifacts", [])
                                                if item["path"] != refreshed_artifact["path"]],
            "report": previous.get("report", str((HERE / "pipeline-verification.md").relative_to(REPO))),
            "working_directory": "server", "environment": {"os_arch": "linux/arm64" if native else "darwin/arm64",
                "toolchain": "go1.26.6", "identities": "synthetic", "schema_migration": 737,
                "agent_cli_guard": not native, "actual_linux_namespaces": native, **previous.get("environment", {})},
            "source_manifest": previous.get("source_manifest", str((HERE / "source-environment-manifest.json").relative_to(REPO))),
            "executed_tests": results, "execution_counts": counts,
            "count_note": "One invocation; repeated and nested tests do not add acceptance criteria.",
            "limits": list(dict.fromkeys(previous.get("limits", []) + [
                "No authenticated provider accounts, paid quota, packaged Main or other-OS containment.",
                "Original conditional/platform skips are retained; Redis and Linux supplements have separate exact results."])),
        }
    partitions = read_json(HERE / "go-service-partitions.json")
    selected = [name for part in partitions.values() for name in part["tests"]]
    assert len(selected) == len(set(selected)) == 1153
    assert all("internal/service::" + name in current for name in selected)
    summaries["service_partition_union"] = {"distinct_top_level_passed": 1153, "missing": [], "overlap": []}

    report_root = REPO / ".omx/reports/cwc-gap-recovery-20261021"
    browser_runs = {
        "continuation-mentions-web": (report_root / "continuation-mentions-web/attempt-6.json", "mention-journeys.md", False),
        "continuation-mentions-native": (report_root / "continuation-mentions-desktop/attempt-2.json", "mention-journeys.md", True),
        "continuation-controls-web": (report_root / "continuation-controls-web-v5.json", "control-journeys.md", False),
        "continuation-controls-native": (report_root / "continuation-controls-desktop-v2.json", "control-journeys.md", True),
        "continuation-admin-web": (HERE / "playwright-admin-web.json", "pipeline-verification.md", False),
        "continuation-admin-native": (HERE / "playwright-admin-desktop.json", "pipeline-verification.md", True),
        "continuation-prerequisites": (HERE / "playwright-prerequisites.json", "pipeline-verification.md", False),
    }
    browser_identities = {}
    for run_id, (path, report, native) in browser_runs.items():
        result = read_json(path)
        stats = result["stats"]
        assert stats["expected"] and not any(stats[key] for key in ["unexpected", "skipped", "flaky"]), run_id
        identities = {}
        for suite in result["suites"]:
            for spec in specs(suite):
                identity = "e2e/" + spec["file"] + "::" + spec["title"]
                outcomes = [outcome for test in spec["tests"] for outcome in test["results"]]
                assert outcomes and all(outcome["status"] == "passed" and outcome["retry"] == 0 for outcome in outcomes)
                identities[identity] = {"status": "passed", "line": spec["line"],
                                        "duration_ms": sum(outcome["duration"] for outcome in outcomes)}
        browser_identities[run_id] = list(identities)
        previous = catalog["evidence"].get(run_id, {})
        refreshed_artifact = artifact(path, previous.get("artifacts", []))
        catalog["evidence"][run_id] = {**previous,
            "kind": "actual_electron_ipc_harness" if native else "production_web_browser", "result": "passed",
            "artifacts": [refreshed_artifact] + [item for item in previous.get("artifacts", [])
                                                if item["path"] != refreshed_artifact["path"]],
            "report": previous.get("report", str((HERE / report).relative_to(REPO))),
            "working_directory": ".", "executed_tests": identities, "execution_counts": stats,
            "source_manifest": previous.get("source_manifest", str((HERE / "source-environment-manifest.json").relative_to(REPO))),
            "environment": {"identities": "synthetic", "api": "http://localhost:18985" if "mentions" in run_id or "prerequisites" in run_id else "http://localhost:18996",
                "web_mode": "production", "native_client_os": "darwin" if native else None,
                "packaged_main": False, "real_provider_account": False, **previous.get("environment", {})},
            "limits": list(dict.fromkeys(previous.get("limits", []) + [
                "Native renderer/Main/preload harness is separate from installed full Main startup.",
                "Pending Permit without proof establishes the server/UI barrier, not a running provider process."])),
        }
    mapping = {"AC90": ["continuation-mentions-web", "continuation-mentions-native"],
               "AC91": ["continuation-mentions-web", "continuation-mentions-native"],
               "AC97": ["continuation-mentions-web", "continuation-mentions-native"],
               "AC95": ["continuation-controls-web", "continuation-controls-native", "continuation-linux-router"]}
    for criterion in ["AC65", "AC66", "AC69", "AC70"]:
        mapping[criterion] = ["continuation-admin-web", "continuation-admin-native"]
    for row in matrix["criteria"]:
        identities = row.get("test_identities", []) + row.get("missing_current_test_evidence", [])
        matching_runs = {run_id for identity in identities for run_id in current.get(identity, [])}
        matching_runs.update(mapping.get(row["id"], []))
        existing = {item["run_id"] for item in row.get("evidence", [])}
        row["evidence"].extend({"run_id": run_id, "catalog": "research/s6-evidence-map.json",
                                "scope": catalog["evidence"][run_id]["kind"], "coverage": "partial_support"}
                               for run_id in sorted(matching_runs - existing))
        row["test_identities"] = list(dict.fromkeys(identities + [identity for run_id in mapping.get(row["id"], [])
                                                               for identity in browser_identities.get(run_id, [])]))
        row["missing_current_test_evidence"] = [identity for identity in row.get("missing_current_test_evidence", []) if identity not in current]
        if row["status"] != "passed":
            for evidence in row["evidence"]:
                evidence["coverage"] = "partial_support"
    protected = ["id", "requirements", "criterion", "source", "source_line", "primary_owner", "delivery_slices"]
    before = [{key: row[key] for key in protected} for row in original["criteria"]]
    after = [{key: row[key] for key in protected} for row in matrix["criteria"]]
    assert before == after and len(after) == 97
    counts = Counter(row["status"] for row in matrix["criteria"])
    proposal = {"promoted_ids": [], "acceptance_review_required": True,
                "status_counts": dict(counts), "protected_definition_sha256": hashlib.sha256(json.dumps(before, sort_keys=True, ensure_ascii=False).encode()).hexdigest(),
                "missing_current_test_evidence": {row["id"]: row["missing_current_test_evidence"] for row in matrix["criteria"] if row["missing_current_test_evidence"]}}
    if args.output_dir is None:
        output_directory = pathlib.Path(tempfile.mkdtemp(prefix="multica-evidence-refresh-"))
    else:
        output_directory = args.output_dir.resolve()
        output_directory.mkdir(parents=True, exist_ok=False)
    proposal["output_directory"] = str(output_directory)
    for name, value in [("go-execution-summary.json", summaries), ("ledger-proposal.json", proposal),
                        ("acceptance-matrix.next.json", matrix), ("s6-evidence-map.next.json", catalog)]:
        (output_directory / name).write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps(proposal, ensure_ascii=False))


if __name__ == "__main__":
    main()
