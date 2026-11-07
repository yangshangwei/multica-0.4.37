# Execution coordinator: 01a1a186-9a5c-7b00-96f8-0430b47edb4c; template only, no execution authorization.
#!/usr/bin/env python3
"""Read-only final v14 identities; this is not whole-check acceptance."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import types


HERE = Path(__file__).resolve().parent
ROOT = Path("/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103")
REVIEW = HERE.parent / "independent-review/review.json"
NEXT_PATH = "apps/web/next-env.d.ts"
DEV_IMPORT = b'import "./.next/dev/types/routes.d.ts";'
PRODUCTION_IMPORT = b'import "./.next/types/routes.d.ts";'


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    with Path(path).open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def audit(root, review):
    root = Path(root).resolve()
    out = root / ".omx/private/final-check-v14"
    launcher = out / "run-make-check-v14-provenance.py"
    clearance = json.loads((out / "independent-review-clearance-v14.json").read_text())
    run_path = out / "make-check-v14.json"
    run = json.loads(run_path.read_text())
    require(run.get("state") == "complete" and type(run.get("exit_code")) is int,
            "Final-state audit requires a complete run")
    require(review.get("candidate_root") == str(root), "Independent review identifies another root")
    require(review.get("verdict") in ("pass", "pass-with-notes") and review.get("blocking_findings_open") == 0,
            "Independent review has open blockers")
    require(clearance.get("verdict") in ("pass", "pass-with-notes") and clearance.get("blocking_findings_open") == 0,
            "Execution clearance has open blockers")
    for key in ("candidate_sha", "launcher_sha256", "adapter_sha256", "source_env_sha256", "bindings_sha256"):
        require(clearance.get(key) == review[key], "Execution clearance differs from independent review: " + key)
    require(digest(launcher) == review["launcher_sha256"], "Registered launcher changed")
    module = types.ModuleType("registered_v14_final_audit")
    module.__file__ = str(launcher)
    exec(compile(launcher.read_bytes(), str(launcher), "exec"), module.__dict__)
    require(module.EXPECTED_ROOT == root and module.EXPECTED_CANDIDATE_SHA == review["candidate_sha"],
            "Registered launcher identifies another candidate")
    require(subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip() == review["candidate_sha"],
            "Final candidate HEAD changed")
    status = subprocess.check_output(["git", "status", "--porcelain", "--untracked-files=all"], cwd=root, text=True)
    require(all(line == " M " + NEXT_PATH for line in status.splitlines()),
            "Unregistered final source changes are present")
    source = out / "check-source-v14.env"
    require(digest(source) == review["source_env_sha256"], "Registered source environment changed")
    adapters = {name: digest(module.ADAPTERS / name) for name in ("go", "pnpm")}
    require(adapters == review["adapter_sha256"], "Registered adapter changed")
    binding_path = out / "owned-bindings-v14-provenance.json"
    require(digest(binding_path) == review["bindings_sha256"], "Registered bindings changed")
    registered = json.loads(binding_path.read_text())
    rows = [row for row in registered["source"]["tracked_files"] if row["path"] == NEXT_PATH]
    require(len(rows) == 1, "Registered Next declaration is absent or duplicated")
    next_file = root / NEXT_PATH
    require(next_file.is_file() and not next_file.is_symlink() and next_file.resolve() == next_file,
            "Next declaration file identity changed")
    blob = subprocess.check_output(["git", "show", review["candidate_sha"] + ":" + NEXT_PATH], cwd=root)
    registered_next_hash = hashlib.sha256(blob).hexdigest()
    require(registered_next_hash == rows[0]["sha256"] and blob.splitlines().count(DEV_IMPORT) == 1
            and blob.count(DEV_IMPORT) == 1, "Registered Next declaration does not match its Git blob")
    current = next_file.read_bytes()
    require(current in (blob, blob.replace(DEV_IMPORT, PRODUCTION_IMPORT, 1)),
            "Next declaration contains more than the exact generated import rewrite")
    raw_next_hash = hashlib.sha256(current).hexdigest()
    original_hash = module.file_hash

    def final_hash(path):
        actual = original_hash(path)
        if Path(path) == next_file:
            require(actual == raw_next_hash and not next_file.is_symlink(), "Next declaration changed during audit")
            return registered_next_hash
        return actual

    # Normalize only the byte-proven generated declaration, without source writes.
    # Every registered source, tool, graph and artifact guard runs unchanged.
    module.file_hash = final_hash
    binding = module.verify_owned_bindings(root, out, clearance, module.parse_generated_environment(source.read_text()))
    require(digest(out / "check-source-v14.safe.env") == run.get("effective_env_sha256"),
            "Canonical execution environment changed")
    require(digest(out / "make-check-v14.log") == run.get("log_sha256"), "Complete run log changed")
    expected = {
        "candidate_root": str(root), "candidate_sha": review["candidate_sha"],
        "launcher_sha256": review["launcher_sha256"], "adapter_sha256": adapters,
        "source_env_sha256": review["source_env_sha256"], "owned_bindings_sha256": review["bindings_sha256"],
        "resource_ownership_sha256": clearance["resource_ownership_sha256"],
        "dev_environment_source_id": binding["source"]["checkout_source_id"],
        "canonical_product_source_id": binding["source"]["product_source"]["source_id"],
        "canonical_product_file_count": binding["source"]["product_source"]["file_count"],
        "tracked_snapshot_fingerprint": binding["source"]["tracked_fingerprint"],
        "cli_sha256": binding["cli"]["sha256"],
    }
    require(all(run.get(key) == value for key, value in expected.items()), "Complete run identity differs from final bindings")
    manifest = json.loads(Path(binding["source"]["product_source"]["manifest_path"]).read_text())
    raw_product = {**manifest["sha256"], NEXT_PATH: raw_next_hash}
    raw_source_id = hashlib.sha256("".join(name + "\0" + raw_product[name] + "\n" for name in sorted(raw_product)).encode()).hexdigest()
    return {
        "status": "pass", "scope": "final filesystem identities only; root still owns live resource and result inventory checks",
        "candidate_root": str(root), "candidate_sha": review["candidate_sha"],
        "launcher_sha256": digest(launcher), "bindings_sha256": digest(binding_path),
        "source_env_sha256": digest(source), "adapter_sha256": adapters,
        "run_sha256": digest(run_path), "wholecheck_exit_code": run["exit_code"],
        "wholecheck_passed": run["exit_code"] == 0,
        "clearance_sha256": digest(out / "independent-review-clearance-v14.json"),
        "resource_ownership_sha256": clearance["resource_ownership_sha256"],
        "tracked_file_count": len(binding["source"]["tracked_files"]),
        "workspace_edges": len(binding["workspace"]["edges"]),
        "workspace_exports": len(binding["workspace"]["graph"]["exports"]),
        "canonical_raw_source_id": raw_source_id,
        "canonical_normalized_source_id": binding["source"]["product_source"]["source_id"],
        "dev_environment_source_id": binding["source"]["checkout_source_id"],
        "tracked_snapshot_fingerprint": binding["source"]["tracked_fingerprint"],
        "next_env": {"path": NEXT_PATH, "generated_rewrite": current != blob,
                     "raw_sha256": raw_next_hash, "normalized_sha256": registered_next_hash},
        "source_changes": status.splitlines(), "candidate_files_written": 0, "live_resources_revalidated": False,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="New JSON file below this audit directory")
    args = parser.parse_args()
    output = args.output.resolve()
    require(output.parent.is_relative_to(HERE) and output.parent.is_dir(), "Audit output must be a new file below the owned directory")
    descriptor = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    report = {"recorded_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
    code = 0
    with os.fdopen(descriptor, "w") as sink:
        try:
            report.update(auditor_sha256=digest(__file__), independent_review_sha256=digest(REVIEW))
            report.update(audit(ROOT, json.loads(REVIEW.read_text())))
        except (RuntimeError, OSError, ValueError, KeyError, TypeError) as error:
            report.update(status="fail", error=str(error) if isinstance(error, RuntimeError) else "Required audit file or data is unavailable",
                          live_resources_revalidated=False, candidate_files_written=0)
            code = 2
        sink.write(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"status": report["status"], "scope": "final identities only", "wholecheck_exit_code": report.get("wholecheck_exit_code"), "report": str(output)}))
    return code


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, OSError) as error:
        print("REFUSED: " + (str(error) if isinstance(error, RuntimeError) else "Audit output unavailable"), file=sys.stderr)
        raise SystemExit(2)
