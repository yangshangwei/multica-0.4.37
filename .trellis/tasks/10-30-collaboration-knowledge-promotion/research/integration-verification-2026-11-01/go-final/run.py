"""Record bounded checks without publishing the private task environment."""
from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import os
import subprocess
import sys
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[6]
OUT = Path(__file__).resolve().parent
DATABASE = "multica_knowledge_final_20261101_e7059940"


def now():
    return datetime.now(timezone.utc).isoformat()


def sources():
    names = set()
    for args in (["git", "ls-files", "-z", "server"],
                 ["git", "ls-files", "--others", "--exclude-standard", "-z", "server"]):
        names.update(subprocess.check_output(args, cwd=ROOT).decode().split("\0"))
    return {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest()
            for name in sorted(names - {""}) if (ROOT / name).is_file()}


def write(name, value):
    (OUT / name).write_text(json.dumps(value, indent=2) + "\n")


def run(name, command, timeout, excluded_test_packages=()):
    before = sources()
    record = {"command": command, "started_at": now(), "database": DATABASE,
              "head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT).decode().strip()}
    write(name + "-source-before.json", before)
    with (OUT / (name + ".jsonl")).open("w") as stdout, (OUT / (name + ".stderr")).open("w") as stderr:
        result = subprocess.run(command, cwd=ROOT, stdout=stdout, stderr=stderr, timeout=timeout)
    after = sources()
    write(name + "-source-after.json", after)
    record.update(finished_at=now(), exit_code=result.returncode,
                  source_unchanged=before == after,
                  changed_source=sorted(p for p in before.keys() | after.keys() if before.get(p) != after.get(p)))
    record["excluded_test_packages"] = list(excluded_test_packages)
    record["selected_source_unchanged"] = not any(
        not (p.endswith("_test.go") and any(p.startswith(prefix + "/") for prefix in excluded_test_packages))
        for p in record["changed_source"])
    write(name + "-command.json", record)
    print(json.dumps(record), flush=True)
    return result.returncode


if __name__ == "__main__":
    os.chdir(ROOT)
    mode = sys.argv[1]
    if mode == "db-clean":
        DATABASE = json.loads((OUT / "clean-environment.json").read_text())["database"]
    if mode in ("ordinary-composite", "ordinary-composite-final"):
        DATABASE = "multica_knowledge_composite_20261101_c825abaf"
    elif mode in ("host-composite", "host-composite-final"):
        DATABASE = "multica_knowledge_host_composite_20261101_efb9efe6"
    if urlsplit(os.environ.get("DATABASE_URL", "")).path.lstrip("/") != DATABASE:
        raise SystemExit("Explicit final task database required")
    if os.environ.get("MULTICA_RUN_REAL_AGENT_SMOKE") == "1":
        raise SystemExit("Real agent smoke must be disabled")
    if mode in ("ordinary-composite", "host-composite", "ordinary-composite-final", "host-composite-final"):
        os.environ["GOMAXPROCS"] = "4"
        package, pattern = ("./internal/daemon", "^TestKnowledgeServerDaemonComposite") if mode.startswith("ordinary-composite") else ("./cmd/server", "^TestCollaborationKnowledgeHostComposite")
        command = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "-race", "-tags=knowledgeintegration", "-p", "1", "-parallel", "4", package, "-run", pattern, "-count=1", "-json", "-timeout=5m"]
        raise SystemExit(run(mode, command, 600, ("server/internal/service", "server/internal/handler")))
    if mode == "sqlc":
        before = sources()
        rc = run("sqlc-final", ["go", "-C", "server", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1", "generate"], 180)
        after = sources()
        generated = {p: h for p, h in before.items() if p.startswith("server/pkg/db/generated/")}
        drift = [p for p in generated if generated[p] != after.get(p)]
        write("sqlc-drift.json", {"generated_files": len(generated), "drift": drift, "exit_code": rc})
        raise SystemExit(rc or bool(drift))
    if mode == "full":
        os.environ["GOMAXPROCS"] = "4"
        command = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "-p", "1", "-parallel", "4", "./...", "-count=1", "-json", "-timeout=15m"]
        raise SystemExit(run("full", command, 3600))
    if mode in ("rest", "db", "db-final", "db-clean"):
        os.environ["GOMAXPROCS"] = "4"
        db_packages = ["github.com/multica-ai/multica/server/internal/service", "github.com/multica-ai/multica/server/internal/handler"]
        packages = subprocess.check_output(["go", "-C", "server", "list", "./..."], cwd=ROOT).decode().splitlines()
        selected = [p for p in packages if (p in db_packages) == mode.startswith("db")]
        write(mode + "-packages.json", selected)
        command = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "-p", "1", "-parallel", "4", *selected, "-count=1", "-json", "-timeout=15m"]
        excluded = ("server/internal/service", "server/internal/handler") if mode == "rest" else ()
        raise SystemExit(run(mode, command, 3600, excluded))
    if mode in ("affected", "affected-rest"):
        os.environ["GOMAXPROCS"] = "4"
        packages = json.loads((OUT / "runtime-lookup-affected-packages.json").read_text())
        excluded = ("server/internal/service", "server/internal/handler") if mode == "affected-rest" else ()
        if excluded:
            packages = [p for p in packages if not any(p.endswith("/" + name.removeprefix("server/")) for name in excluded)]
        write(mode + "-packages.json", packages)
        command = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "-p", "1", "-parallel", "4", *packages, "-count=1", "-json", "-timeout=15m"]
        raise SystemExit(run(mode, command, 3600, excluded))
    if mode == "project-lock":
        os.environ["GOMAXPROCS"] = "4"
        command = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "-race", "-p", "1", "-parallel", "4", "./internal/handler", "-run", "^(TestProject|TestUpdateProject|TestDeleteProject|TestKnowledgePreparationWhileRealOrdinaryWriterIsUncommitted)", "-count=1", "-json", "-timeout=5m"]
        raise SystemExit(run("project-lock", command, 420))
    if mode == "builtin":
        command = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test", "./internal/service", "-run", "^TestBuiltin", "-count=1", "-json", "-timeout=2m"]
        raise SystemExit(run(mode, command, 180))
    if mode in ("vet", "build", "vet-final", "build-final", "vet-combined", "build-combined"):
        operation = mode.split("-", 1)[0]
        excluded = ("server",) if operation == "build" else ()
        raise SystemExit(run(mode, ["go", "-C", "server", operation, "-p", "2", "./..."], 600, excluded))
    raise SystemExit("Unknown mode")
