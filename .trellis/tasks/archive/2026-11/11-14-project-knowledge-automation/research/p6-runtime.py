#!/usr/bin/env python3
"""Task-owned P6 runtime. No real model/account configuration is inherited."""
import argparse
import datetime
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import traceback
import urllib.parse
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[4]
TASK = ".trellis/tasks/11-14-project-knowledge-automation"
RESEARCH = ROOT / TASK / "research"
STATE = Path(tempfile.gettempdir()) / "multica-knowledge-automation-20261114"
PSQL = "/opt/homebrew/opt/libpq/bin/psql"
sys.dont_write_bytecode = True
EXCLUDED = {"node_modules", ".git", ".next", ".source", ".turbo", ".cache", "__pycache__"}
REQUIRED_SOURCES = {
    "server/internal/service/collaboration_delivery.go",
    "packages/core/collaboration/delivery-schema.ts",
    "packages/views/collaboration/collaboration-detail.tsx",
    "packages/views/collaboration/workflow.tsx",
    "packages/views/collaboration/comment-composer.tsx",
    "server/internal/docs/content/manifest.json",
    "server/internal/docs/content/pages/collaboration-knowledge.json",
    "server/internal/docs/content/pages/environment-variables.json",
}
AUTH_CONFIGURATION = {
    "MULTICA_AUTH_MODE": "password", "MULTICA_MANAGED_INSTALLATIONS_ENABLED": "true",
    "MULTICA_PASSWORD_LIMITER_MODE": "single", "MULTICA_DEVICE_AUTH_ENABLED": "false",
    "MULTICA_PLATFORM_ADMIN_ENABLED": "false", "MULTICA_CLOUD_URL": "",
}
def auth_configuration(database):
    return {**AUTH_CONFIGURATION, "MULTICA_DEPLOYMENT_ID": str(uuid.uuid5(uuid.NAMESPACE_URL, "multica-knowledge-p6/" + database))}


CURRENT_RUN = None


def save(path, value):
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False, encoding="utf-8") as output:
        temporary = Path(output.name)
        os.fchmod(output.fileno(), 0o600)
        output.write(json.dumps(value, indent=2) + "\n")
    os.replace(temporary, path)


def announce(value):
    print(json.dumps(value), flush=True)


def file_hash(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def tool(name):
    executable = shutil.which(name)
    if not executable:
        raise RuntimeError(f"Required executable unavailable: {name}")
    return str(Path(executable).resolve())


def git_output(*arguments):
    environment = {"PATH": os.environ["PATH"], "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull}
    return subprocess.check_output([tool("git"), "-C", str(ROOT), *arguments], env=environment, timeout=30)


def source_paths():
    names = git_output("ls-files", "--cached", "--others", "--exclude-standard", "-z").decode().split("\0")
    result = []
    names = set(names) - {""}
    names.update(REQUIRED_SOURCES)
    if (ROOT / "apps/web/next-env.d.ts").is_file():
        names.add("apps/web/next-env.d.ts")
    for name in sorted(names):
        relative = Path(name)
        if relative.is_absolute() or ".." in relative.parts:
            raise RuntimeError("Invalid source path")
        if EXCLUDED.intersection(relative.parts) or relative.name.startswith(".env") or relative.suffix == ".tsbuildinfo":
            continue
        if name.startswith(("server/bin/", "apps/desktop/out/", "apps/desktop/dist/", "apps/desktop/resources/bin/")):
            continue
        if name.startswith(".omx/") or name.startswith(".trellis/") and not (
            name.startswith(".trellis/spec/") or name.startswith(TASK + "/") and relative.suffix in {".md", ".py", ".jsonl"}
        ):
            continue
        path = ROOT / relative
        if path.is_file():
            if not path.resolve().is_relative_to(ROOT):
                raise RuntimeError("Source symlink leaves the checkout")
            result.append(name)
    return result


def normalized_source(name, content):
    if name == "apps/web/next-env.d.ts":
        lines = content.splitlines(keepends=True)
        for index, line in enumerate(lines):
            if line.rstrip(b"\r\n") == b'import "./.next/dev/types/routes.d.ts";':
                lines[index] = line.replace(b".next/dev/types/routes.d.ts", b".next/types/routes.d.ts")
        return b"".join(lines)
    return content


def source_snapshot(root=ROOT, names=None):
    files = {}
    for name in names if names is not None else source_paths():
        path = root / name
        content = path.read_bytes()
        files[name] = {"sha256": hashlib.sha256(content).hexdigest(),
                       "normalized_sha256": hashlib.sha256(normalized_source(name, content)).hexdigest(),
                       "mode": path.stat().st_mode & 0o777,
                       "link": str(path.resolve().relative_to(root)) if path.is_symlink() else None}
    normalized = {name: {key: value for key, value in row.items() if key != "sha256"} for name, row in files.items()}
    return {"source_id": hashlib.sha256(json.dumps(normalized, sort_keys=True).encode()).hexdigest(), "files": files}


def workspace_links(root=ROOT):
    result = {}
    for name in source_paths():
        if Path(name).name != "package.json":
            continue
        scope = root / Path(name).parent / "node_modules/@multica"
        if not scope.is_dir():
            continue
        for path in sorted(scope.iterdir()):
            target = path.resolve()
            if not target.is_relative_to(root) or "node_modules" in target.relative_to(root).parts:
                raise RuntimeError("A workspace dependency points outside its source snapshot")
            result[str(path.relative_to(root))] = str(target.relative_to(root))
    return result


def stage_sources(directory, snapshot, commit):
    for name, row in snapshot["files"].items():
        source = ROOT / name
        target = directory / name
        target.parent.mkdir(parents=True, exist_ok=True)
        if source.is_symlink():
            raw_link = os.readlink(source)
            target.symlink_to(raw_link if not os.path.isabs(raw_link) else directory / row["link"])
        else:
            shutil.copyfile(source, target)
            target.chmod(row["mode"])
    metadata = directory / ".git"
    (metadata / "objects/info").mkdir(parents=True)
    (metadata / "refs").mkdir()
    (metadata / "HEAD").write_text(commit + "\n")
    (metadata / "config").write_text("[core]\n\trepositoryformatversion = 0\n\tbare = false\n")
    objects = Path(git_output("rev-parse", "--git-path", "objects").decode().strip())
    (metadata / "objects/info/alternates").write_text(str((ROOT / objects).resolve()) + "\n")
    for name in snapshot["files"]:
        if Path(name).name != "package.json":
            continue
        original = ROOT / Path(name).parent / "node_modules"
        if not original.is_dir():
            continue
        destination = directory / Path(name).parent / "node_modules"
        destination.mkdir(exist_ok=True)
        for dependency in original.iterdir():
            if dependency.name == "@multica":
                (destination / dependency.name).mkdir(exist_ok=True)
                for package in dependency.iterdir():
                    resolved = package.resolve()
                    if not resolved.is_relative_to(ROOT) or "node_modules" in resolved.relative_to(ROOT).parts:
                        raise RuntimeError("Foreign workspace dependency refused")
                    (destination / dependency.name / package.name).symlink_to(directory / resolved.relative_to(ROOT))
            elif dependency.name not in {".cache", ".vite"}:
                (destination / dependency.name).symlink_to(dependency.resolve())
    if source_snapshot(directory, snapshot["files"])["source_id"] != snapshot["source_id"]:
        raise RuntimeError("Source changed while staging")


def process_table():
    output = subprocess.check_output([tool("ps"), "-ax", "-o", "uid=", "-o", "pid=", "-o", "ppid=", "-o", "pgid=", "-o", "stat=", "-o", "lstart="], text=True, timeout=10)
    result = {}
    for line in output.splitlines():
        fields = line.split(None, 5)
        if len(fields) == 6 and int(fields[0]) == os.getuid() and not fields[4].startswith("Z"):
            result[int(fields[1])] = {"pid": int(fields[1]), "ppid": int(fields[2]), "pgid": int(fields[3]), "birth": fields[5].strip()}
    return result


def same_process(expected, actual):
    return actual is not None and all(expected.get(key) == actual.get(key) for key in ["pid", "pgid", "birth"])


def observe_process(record):
    table = process_table()
    known = record.setdefault("members", {})
    identities = {int(pid) for pid, row in known.items() if same_process(row, table.get(int(pid)))}
    groups = {table[pid]["pgid"] for pid in identities if pid == table[pid]["pgid"]}
    changed = True
    while changed:
        changed = False
        for pid, row in table.items():
            if pid not in identities and (row["ppid"] in identities or row["pgid"] in groups):
                identities.add(pid)
                if pid == row["pgid"]:
                    groups.add(pid)
                changed = True
    for pid in identities:
        known[str(pid)] = table[pid]
    return table, identities


def launch(command, cwd, environment, log):
    tool("ps")
    child = None
    record = None
    try:
        with log.open("xb") as output:
            os.fchmod(output.fileno(), 0o600)
            child = subprocess.Popen(command, cwd=cwd, env=environment, stdin=subprocess.DEVNULL,
                                     stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        record = {"pid": child.pid, "pgid": child.pid, "birth": None,
                  "command": shlex.join(command), "argv": command, "cwd": str(cwd), "log": str(log), "members": {}}
        identity = process_table().get(child.pid)
        if identity is not None:
            record.update({"birth": identity["birth"], "members": {str(child.pid): identity}})
        elif child.poll() is None:
            raise RuntimeError("The new child process identity could not be recorded")
        return child, record
    except BaseException:
        if record is not None and record["birth"] is not None:
            try:
                stop_process(record, child)
            except Exception:
                record["cleanup_failed"] = True
        # The Popen handle proves this direct child even when ps failed. Do not
        # signal an unproved process group; retain its private launch receipt.
        if child is not None and child.poll() is None:
            child.terminate()
            try:
                child.wait(timeout=8)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait(timeout=3)
        if record is not None:
            record["launch_failed"] = True
            save(log.with_suffix(".process.json"), record)
        raise


def stop_process(record, child=None):
    table, identities = observe_process(record)
    groups = {pid for pid in identities if table[pid]["pgid"] == pid}
    for group in groups:
        if same_process(record["members"][str(group)], process_table().get(group)):
            try:
                os.killpg(group, signal.SIGTERM)
            except ProcessLookupError:
                pass
    for pid in identities:
        if same_process(record["members"][str(pid)], process_table().get(pid)):
            try:
                os.kill(pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
    deadline = time.monotonic() + 8
    while time.monotonic() < deadline:
        if child is not None:
            child.poll()
        _, remaining = observe_process(record)
        if not remaining:
            break
        time.sleep(0.2)
    _, remaining = observe_process(record)
    for pid in remaining:
        if same_process(record["members"][str(pid)], process_table().get(pid)):
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        if child is not None:
            child.poll()
        table, remaining = observe_process(record)
        if not remaining:
            break
        time.sleep(0.2)
    record["remaining_owned_pids"] = sorted(remaining)
    record["unproven_group_pids"] = sorted(pid for pid, row in table.items()
                                           if row["pgid"] == record["pgid"] and str(pid) not in record["members"])
    if remaining or record["unproven_group_pids"]:
        raise RuntimeError("Owned cleanup incomplete; unproved processes were left untouched")


def command_step(command, cwd, environment, directory, label, timeout):
    announce({"stage": label, "log": str(directory / (label + ".log"))})
    child, record = launch(command, cwd, environment, directory / (label + ".log"))
    started = time.monotonic()
    notified = started
    try:
        while child.poll() is None:
            observe_process(record)
            save(directory / (label + ".process.json"), record)
            if time.monotonic() - started > timeout:
                raise RuntimeError(f"Bounded stage timed out: {label}")
            if time.monotonic() - notified >= 20:
                announce({"stage": label, "elapsed_seconds": round(time.monotonic() - started)})
                notified = time.monotonic()
            time.sleep(0.25)
        record["exit_code"] = child.returncode
        stop_process(record, child)
        if child.returncode:
            raise RuntimeError(f"Stage failed: {label}; private diagnostics retained")
        return record
    except BaseException:
        stop_process(record, child)
        raise
    finally:
        save(directory / (label + ".process.json"), record)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


def http_bytes(url):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    with opener.open(url, timeout=3) as response:
        if response.status != 200:
            raise RuntimeError("Runtime endpoint is not healthy")
        return response.read(2 * 1024 * 1024)


def listener_pid(port):
    result = subprocess.run([tool("lsof"), "-nP", "-t", f"-iTCP:{port}", "-sTCP:LISTEN"], capture_output=True, text=True, timeout=10)
    pids = {int(value) for value in result.stdout.split()}
    return next(iter(pids)) if result.returncode == 0 and len(pids) == 1 else None


def test_inventory(report):
    inventory = []

    def visit(suites, parents=()):
        for suite in suites:
            titles = (*parents, suite.get("title", ""))
            for specification in suite.get("specs", []):
                for test in specification.get("tests", []):
                    inventory.append({"title": " / ".join((*titles, specification["title"])),
                                      "expected_status": test["expectedStatus"], "results": [result["status"] for result in test["results"]]})
            visit(suite.get("suites", []), titles)

    visit(report["suites"])
    return inventory


def provider_keys(root):
    source = (root / "server/internal/daemon/agents_probe.go").read_text()
    keys = set(re.findall(r'"(MULTICA_[A-Z0-9_]+_PATH)"', source))
    for path in (root / "server/pkg/agent").glob("*.go"):
        keys.update(value + "_PATH" for value in re.findall(r'EnvPrefix:\s*"(MULTICA_[A-Z0-9_]+)"', path.read_text()))
    return sorted(keys | {"MULTICA_OMP_PATH"})


def private_directory(path):
    path.mkdir(mode=0o700, parents=True, exist_ok=True)
    if path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o077:
        raise RuntimeError("Runtime directory is not private and owned")


class Runtime:
    def __init__(self, allocation_file):
        global CURRENT_RUN
        self.allocation_file = Path(allocation_file).resolve(strict=True)
        self.allocation = json.loads(self.allocation_file.read_text())
        allocation = self.allocation
        if allocation.get("task") != TASK or allocation.get("source_root") != str(ROOT) or allocation.get("real_accounts") is not False or allocation.get("created_by") != "p6-runtime.py":
            raise RuntimeError("Allocation identity mismatch")
        self.run = Path(allocation["run_directory"]).resolve(strict=True)
        if not self.run.is_relative_to(STATE.resolve()) or not re.fullmatch(r"p6-browser-[a-f0-9]{8}", self.run.name) or self.run.is_relative_to(ROOT):
            raise RuntimeError("Allocation is not an external P6 runtime")
        if self.run.stat().st_uid != os.getuid() or self.run.stat().st_mode & 0o077:
            raise RuntimeError("Allocation directory is not private and owned")
        if json.loads((self.run / "allocation.json").read_text()) != allocation:
            raise RuntimeError("Allocation differs from the retained private record")
        self.environment_file = self.run / "environment.json"
        if self.environment_file.stat().st_uid != os.getuid() or self.environment_file.stat().st_mode & 0o077:
            raise RuntimeError("Allocation environment is not private and owned")
        self.original_environment = json.loads(self.environment_file.read_text())
        database_url = self.original_environment.get("DATABASE_URL", "")
        self.database_url = urllib.parse.urlparse(database_url)
        if "$" in database_url or any(ord(character) < 32 for character in database_url) or self.database_url.scheme not in {"postgres", "postgresql"} or self.database_url.hostname not in {"localhost", "127.0.0.1", "::1"}:
            raise RuntimeError("Explicit literal loopback database configuration required")
        if not re.fullmatch(r"mca_knowledge_browser_[a-f0-9]{12}", allocation.get("database", "")) or urllib.parse.unquote(self.database_url.path[1:]) != allocation["database"]:
            raise RuntimeError("Database differs from its browser allocation")
        ports = allocation.get("ports", {})
        if set(ports) != {"api", "web", "renderer", "model"} or len(set(ports.values())) != 4:
            raise RuntimeError("Invalid allocated ports")
        for name, port in ports.items():
            if type(port) is not int or not 1024 < port < 65536 or allocation.get(name + "_origin") != f"http://127.0.0.1:{port}":
                raise RuntimeError("Services must use their allocated loopback origins")
        CURRENT_RUN = self.run

    def load(self, name):
        path = self.run / (name + ".json")
        record = json.loads(path.read_text()) if path.exists() else None
        if record is not None and name == "runtime":
            if record.get("task") != TASK or record.get("allocation_sha256") != file_hash(self.allocation_file) or not Path(record["directory"]).resolve().is_relative_to(self.run):
                raise RuntimeError("Retained process record belongs to a different allocation")
            for process in record["processes"].values():
                if process.get("pgid") != process.get("pid") or not Path(process["cwd"]).resolve().is_relative_to(self.run) or not Path(process["log"]).resolve().is_relative_to(self.run):
                    raise RuntimeError("Retained launch is not owned by this runtime")
        return record

    def lock(self):
        descriptor = os.open(self.run / "lifecycle.lock", os.O_CREAT | os.O_RDWR, 0o600)
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            os.close(descriptor)
            raise RuntimeError("Another lifecycle operation owns this allocation") from None
        return os.fdopen(descriptor, "a")

    def environment(self, stage, directory, browsers_path):
        allocation = self.allocation
        origins = {name: allocation[name + "_origin"] for name in allocation["ports"]}
        signing_key = self.original_environment.get("JWT_SECRET", "")
        if not signing_key.startswith("p6-synthetic-local-signing-key-"):
            raise RuntimeError("A synthetic P6 signing key is required")
        environment = masked_environment(self.run / "home", self.run / "tmp")
        directories = [str(Path(tool(name)).parent) for name in ["python3", "go", "node", "pnpm", "git"]]
        environment["PATH"] = ":".join(dict.fromkeys([*directories, "/usr/bin", "/bin", "/usr/sbin", "/sbin"]))
        for key in ["GOCACHE", "GOMODCACHE", "GOPATH"]:
            value = self.original_environment.get(key)
            if not isinstance(value, str) or not Path(value).is_absolute():
                raise RuntimeError("Explicit Go cache binding is required")
            environment[key] = value
        environment.update({
            "DATABASE_URL": self.original_environment["DATABASE_URL"], "JWT_SECRET": signing_key,
            **auth_configuration(allocation["database"]),
            "MULTICA_CLI_INSTALL_COMMAND": "", "MULTICA_CLI_INSTALL_COMMAND_MACOS": "",
            "MULTICA_CLI_INSTALL_COMMAND_DARWIN": "",
            "MULTICA_CLI_INSTALL_COMMAND_LINUX": "", "MULTICA_CLI_INSTALL_COMMAND_WINDOWS": "",
            "MULTICA_DEV_VERIFICATION_CODE": "846215", "DATABASE_MAX_CONNS": "6", "DATABASE_MIN_CONNS": "0",
            "PORT": str(allocation["ports"]["api"]), "FRONTEND_PORT": str(allocation["ports"]["web"]),
            "DESKTOP_RENDERER_PORT": str(allocation["ports"]["renderer"]),
            "NEXT_PUBLIC_API_URL": origins["api"], "API_URL": origins["api"], "REMOTE_API_URL": origins["api"],
            "NEXT_PUBLIC_WS_URL": origins["api"].replace("http", "ws", 1) + "/ws",
            "FRONTEND_ORIGIN": origins["web"], "PLAYWRIGHT_BASE_URL": origins["web"],
            "CORS_ALLOWED_ORIGINS": ",".join([origins["web"], origins["renderer"]]),
            "ALLOWED_ORIGINS": ",".join([origins["web"], origins["renderer"]]),
            "MULTICA_KNOWLEDGE_CONTEXT_ENABLED": "true", "MULTICA_LLM_BASE_URL": origins["model"] + "/v1",
            "MULTICA_LLM_API_KEY": "p6-synthetic-local-gateway-key", "MULTICA_LLM_DEFAULT_MODEL": "p6-controlled",
            "MULTICA_LLM_MAX_RETRIES": "0", "KNOWLEDGE_MODEL_PORT": str(allocation["ports"]["model"]),
            "KNOWLEDGE_MODEL_LOG": str(self.run / "logs/model.jsonl"), "P6_MODEL_URL": origins["model"],
            "P6_RUN_DIRECTORY": str(self.run), "P6_DATABASE": allocation["database"],
            "P6_BINARY": str(directory / "bin/multica"), "P6_PROVIDER_CAPTURE": str(self.run / "logs/provider.jsonl"),
            "PLAYWRIGHT_BROWSERS_PATH": str(browsers_path or self.run / "home/browsers"),
            "PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD": "1", "COLLABORATION_DETAIL_TASK": TASK,
            "COLLABORATION_DETAIL_ALLOCATION_FILE": str(directory / "execution-allocation.json"),
            "COLLABORATION_DETAIL_PROVENANCE_FILE": str(self.run / "provenance.json"),
            "COLLABORATION_E2E_API_URL": origins["api"], "COLLABORATION_ELECTRON_SOURCE_ROOT": str(stage),
            "COLLABORATION_ELECTRON_RENDERER_URL": origins["renderer"],
            "COLLABORATION_ELECTRON_WS_URL": origins["api"].replace("http", "ws", 1) + "/ws",
            "RATE_LIMIT_AUTH": "1000", "RATE_LIMIT_AUTH_VERIFY": "1000", "PYTHONDONTWRITEBYTECODE": "1",
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
        })
        environment.pop("AUTH_MODE", None)
        for key in provider_keys(stage):
            environment[key] = str(directory / "bin" / ("missing-" + key.lower()))
        fake = directory / "bin/claude"
        shutil.copyfile(stage / "e2e/fixtures/project-knowledge-provider.py", fake)
        fake.chmod(0o700)
        environment["MULTICA_CLAUDE_PATH"] = str(fake)
        return environment

    def assert_build(self, build):
        if build is None or build.get("state") != "built" or build.get("task") != TASK or build.get("allocation_sha256") != file_hash(self.allocation_file) or build.get("source_environment_sha256") != file_hash(self.environment_file):
            raise RuntimeError("A complete build for this unchanged allocation is required")
        stage = Path(build["source_root"]).resolve()
        if not stage.is_relative_to(self.run) or git_output("rev-parse", "HEAD").decode().strip() != build["commit"] or source_snapshot()["source_id"] != build["source_id"]:
            raise RuntimeError("Source changed after the source-ready build; stop owned services and rebuild")
        if source_snapshot(stage, build["source_snapshot"]["files"])["source_id"] != build["source_id"] or (stage / ".git/HEAD").read_text().strip() != build["commit"]:
            raise RuntimeError("The isolated source snapshot changed")
        if workspace_links() != build["workspace_links"] or workspace_links(stage) != build["workspace_links"]:
            raise RuntimeError("Workspace dependency bindings changed")
        for filename, digest in build["artifact_hashes"].items():
            path = Path(filename)
            if not path.resolve().is_relative_to(self.run) or file_hash(path) != digest:
                raise RuntimeError("Production build artifact changed")
        for row in build["tools"].values():
            if file_hash(Path(row["path"])) != row["sha256"]:
                raise RuntimeError("A pinned execution tool changed")
        environment_file = Path(build["environment_file"])
        if not environment_file.resolve().is_relative_to(self.run) or file_hash(environment_file) != build["environment_sha256"]:
            raise RuntimeError("Runtime configuration changed")
        environment = json.loads(environment_file.read_text())
        if any(environment.get(key) != value for key, value in auth_configuration(self.allocation["database"]).items()):
            raise RuntimeError("Synthetic password/managed-installation configuration changed")
        fake = Path(environment["MULTICA_CLAUDE_PATH"])
        if fake != Path(build["directory"]) / "bin/claude" or file_hash(fake) != file_hash(stage / "e2e/fixtures/project-knowledge-provider.py"):
            raise RuntimeError("The provider executable is not the frozen fake fixture")
        for key in provider_keys(stage):
            if key != "MULTICA_CLAUDE_PATH" and (environment.get(key) != str(Path(build["directory"]) / "bin" / ("missing-" + key.lower())) or Path(environment[key]).exists() or Path(environment[key]).is_symlink()):
                raise RuntimeError("A real provider probe is no longer denied")
        return stage, environment

    def sql(self, environment, statement, diagnostics):
        parsed = self.database_url
        sql_environment = {**environment, "PGHOST": parsed.hostname, "PGPORT": str(parsed.port or 5432),
                           "PGUSER": urllib.parse.unquote(parsed.username or ""), "PGPASSWORD": urllib.parse.unquote(parsed.password or ""),
                           "PGDATABASE": self.allocation["database"], "PGCONNECT_TIMEOUT": "5",
                           "PGOPTIONS": "-c default_transaction_read_only=on -c statement_timeout=5000"}
        result = subprocess.run([PSQL, "-XAt", "-v", "ON_ERROR_STOP=1", "-c", statement], env=sql_environment,
                                capture_output=True, text=True, timeout=15)
        if result.returncode:
            with diagnostics.open("a") as output:
                os.fchmod(output.fileno(), 0o600)
                output.write(result.stderr)
            raise RuntimeError("Owned database probe failed; private diagnostics retained")
        return json.loads(result.stdout)

    def database_ready(self, build, environment, diagnostics):
        stage = Path(build["source_root"])
        expected = sorted(path.name.removesuffix(".up.sql") for path in (stage / "server/migrations").glob("*.up.sql"))
        ledger = self.sql(environment, "SELECT json_build_object('database',current_database(),'versions',coalesce(json_agg(version ORDER BY version),'[]'::json)) FROM schema_migrations", diagnostics)
        gates = self.sql(environment, "SELECT json_build_object('database',current_database(),'gates',coalesce(json_agg(json_build_object('creation_enabled',creation_enabled,'enabled',enabled,'host_execution_enabled',host_execution_enabled)),'[]'::json)) FROM collaboration_execution_gate", diagnostics)
        if ledger["database"] != self.allocation["database"] or ledger["versions"] != expected or gates["database"] != self.allocation["database"] or gates["gates"] != [{"creation_enabled": True, "enabled": False, "host_execution_enabled": False}]:
            raise RuntimeError("Migration ledger or manual-only collaboration gates differ from the frozen build")
        return {"database": ledger["database"], "count": len(expected), "latest": expected[-1], "gates": gates["gates"]}

    def save_runtime(self, record):
        save(Path(record["directory"]) / "runtime.json", record)
        save(self.run / "runtime.json", record)

    def build(self, timeout, browsers_path):
        runtime = self.load("runtime")
        if runtime and any(observe_process(record)[1] for record in runtime["processes"].values()):
            raise RuntimeError("Stop this allocation's owned services before rebuilding")
        for name in ["home", "tmp", "bin", "logs", "results"]:
            private_directory(self.run / name)
        directory = self.run / ("build-" + uuid.uuid4().hex[:12])
        private_directory(directory)
        private_directory(directory / "bin")
        stage = directory / "source"
        private_directory(stage)
        commit = git_output("rev-parse", "HEAD").decode().strip()
        snapshot = source_snapshot()
        if not REQUIRED_SOURCES.issubset(snapshot["files"]):
            raise RuntimeError("Required product/generated-help source inputs are missing")
        links = workspace_links()
        tools = {name: {"path": tool(name)} for name in ["go", "node", "pnpm", "python3", "git", "ps", "lsof"]}
        if not Path(PSQL).is_file():
            raise RuntimeError("The recorded PostgreSQL client is unavailable")
        tools["psql"] = {"path": str(Path(PSQL).resolve())}
        for row in tools.values():
            row["sha256"] = file_hash(Path(row["path"]))
        record = {"task": TASK, "state": "building", "directory": str(directory), "source_root": str(stage),
                  "original_source_root": str(ROOT), "commit": commit, "source_id": snapshot["source_id"],
                  "source_snapshot": snapshot, "workspace_links": links, "tools": tools,
                  "allocation_file": str(self.allocation_file), "allocation_sha256": file_hash(self.allocation_file),
                  "source_environment_sha256": file_hash(self.environment_file),
                  "source_ready": True, "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
        save(directory / "build.json", record)
        try:
            announce({"stage": "snapshot", "source_id": record["source_id"], "source_root": str(stage)})
            stage_sources(stage, snapshot, commit)
            environment = self.environment(stage, directory, browsers_path)
            execution_allocation = {**self.allocation, "source_root": str(stage), "run_directory": str(self.run),
                                    "original_source_root": str(ROOT), "parent_allocation_file": str(self.allocation_file),
                                    "parent_allocation_sha256": record["allocation_sha256"], "source_id": record["source_id"], "commit": commit}
            save(directory / "execution-allocation.json", execution_allocation)
            save(directory / "environment.json", environment)
            record.update({"environment_file": str(directory / "environment.json"),
                           "environment_sha256": file_hash(directory / "environment.json"),
                           "execution_allocation_file": str(directory / "execution-allocation.json"),
                           "execution_allocation_sha256": file_hash(directory / "execution-allocation.json")})
            version = "v0.0.0-0-g" + commit[:12]
            for name in ["server", "multica", "migrate"]:
                flags = []
                if name != "migrate":
                    ldflags = f"-X main.version={version} -X main.commit={commit[:12]}"
                    if name == "multica":
                        ldflags += f" -X main.date={record['created_at']}"
                    flags = ["-ldflags", ldflags]
                command_step([tools["go"]["path"], "-C", str(stage / "server"), "build", "-buildvcs=false", "-trimpath", *flags,
                              "-o", str(directory / "bin" / name), "./cmd/" + name], stage, environment, directory, "build-" + name, timeout)
            for package, command, label in [("@multica/web", "mdx", "web-mdx"), ("@multica/desktop", "build", "desktop-build"), ("@multica/web", "build", "web-build")]:
                command_step([tools["pnpm"]["path"], "--dir", str(stage), "--filter", package, command], stage, environment, directory, label, timeout)
            artifacts = {}
            for folder in [stage / "apps/web/.next", stage / "apps/web/.source", stage / "apps/desktop/out", stage / "apps/desktop/resources/bin", directory / "bin"]:
                for path in sorted(folder.rglob("*")):
                    if path.is_file() and not (folder.name == ".next" and path.relative_to(folder).parts[0] == "cache") and path.name not in {"trace", "trace-build"}:
                        artifacts[str(path)] = file_hash(path)
            build_id = (stage / "apps/web/.next/BUILD_ID").read_text().strip()
            for name in ["apps/desktop/out/renderer/index.html", "apps/desktop/out/preload/index.js"]:
                if str(stage / name) not in artifacts:
                    raise RuntimeError("Production Desktop renderer/preload artifacts are missing")
            if not build_id:
                raise RuntimeError("Production Web build ID is missing")
            record.update({"state": "built", "build_id": build_id, "artifact_hashes": artifacts})
            self.assert_build(record)
            save(directory / "build.json", record)
            save(self.run / "build.json", record)
            announce({"state": "built", "build": str(directory / "build.json"), "source_root": str(stage), "source_id": record["source_id"], "build_id": build_id})
        except BaseException:
            record["state"] = "build_failed"
            save(directory / "build.json", record)
            raise

    def prove_services(self, runtime, build, verify_build=True):
        if verify_build:
            stage, environment = self.assert_build(build)
        else:
            stage = Path(build["source_root"])
            environment = json.loads(Path(build["environment_file"]).read_text())
        for name, record in runtime["processes"].items():
            table, _ = observe_process(record)
            if not same_process(record, table.get(record["pid"])) or listener_pid(self.allocation["ports"][name]) != record["pid"]:
                raise RuntimeError("A runtime listener no longer matches its owned launch")
        origins = {name: self.allocation[name + "_origin"] for name in self.allocation["ports"]}
        health = json.loads(http_bytes(origins["api"] + "/health"))
        http_bytes(origins["api"] + "/readyz")
        config = json.loads(http_bytes(origins["api"] + "/api/config"))
        if (health.get("pid") != runtime["processes"]["api"]["pid"] or health.get("commit") != build["commit"][:12]
                or config.get("auth_mode") != AUTH_CONFIGURATION["MULTICA_AUTH_MODE"]
                or config.get("managed_installation_supported") is not True
                or config.get("deployment_id") != environment["MULTICA_DEPLOYMENT_ID"]
                or not re.fullmatch(r"[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}", config.get("deployment_id", ""))):
            raise RuntimeError("API health or password/managed authentication provenance differs")
        if http_bytes(origins["renderer"]) != (stage / "apps/desktop/out/renderer/index.html").read_bytes():
            raise RuntimeError("The renderer is not serving the frozen production output")
        served = json.loads(Path(runtime["renderer_proof_file"]).read_text())
        if served.get("pid") != runtime["processes"]["renderer"]["pid"] or served.get("origin") != origins["renderer"] or Path(served.get("renderer", "")).resolve() != stage / "apps/desktop/out/renderer" or served.get("index_sha256") != file_hash(stage / "apps/desktop/out/renderer/index.html"):
            raise RuntimeError("The renderer's own launch proof differs")
        web = http_bytes(origins["web"] + "/login").decode()
        if 'apiBaseUrl\\":\\"' + origins["api"] not in web or 'wsUrl\\":\\"' + environment["NEXT_PUBLIC_WS_URL"] not in web:
            raise RuntimeError("Production Web uses different API/WebSocket origins")
        model = json.loads(http_bytes(origins["model"] + "/control/state"))
        if not isinstance(model.get("calls"), list) or not isinstance(model.get("held"), list):
            raise RuntimeError("The controlled local model is not ready")
        execution_allocation = Path(build["execution_allocation_file"])
        if file_hash(execution_allocation) != build["execution_allocation_sha256"]:
            raise RuntimeError("Execution allocation changed")
        source_hashes = {name: row["sha256"] for name, row in source_snapshot(stage, build["source_snapshot"]["files"])["files"].items()}
        proof = {"runtime_permitted": True, "task": TASK, "source_root": str(stage), "original_source_root": str(ROOT),
                 "allocation_file": str(execution_allocation), "allocation_sha256": file_hash(execution_allocation),
                 "parent_allocation_file": str(self.allocation_file), "parent_allocation_sha256": file_hash(self.allocation_file),
                 "api_url": origins["api"], "production_web_url": origins["web"], "renderer_url": origins["renderer"],
                 "model_url": origins["model"], "database": self.allocation["database"], "source_hashes": source_hashes,
                 "environment_sha256": build["environment_sha256"], "build_file": str(Path(build["directory"]) / "build.json"),
                 "auth_mode": config["auth_mode"], "managed_installation_supported": config["managed_installation_supported"],
                 "deployment_id": config["deployment_id"]}
        for name in ["api", "web", "renderer", "model"]:
            record = runtime["processes"][name]
            proof[name] = {"listener_pid": record["pid"], "commit": build["commit"][:12], "source_id": build["source_id"],
                           "command": record["command"], "pgid": record["pgid"], "birth": record["birth"]}
        proof["web"].update({"mode": "production", "build_id": build["build_id"]})
        proof["renderer"].update({"mode": "production", "artifact_hashes": {
            str(Path(filename).relative_to(stage)): digest for filename, digest in build["artifact_hashes"].items()
            if Path(filename).is_relative_to(stage / "apps/desktop/out")}})
        return proof

    def start(self, timeout, migration_timeout):
        build = self.load("build")
        stage, environment = self.assert_build(build)
        existing = self.load("runtime")
        if existing and existing.get("state") == "ready":
            if existing.get("runtime_permitted") is not True or self.prove_services(existing, build) != self.load("provenance"):
                raise RuntimeError("Existing listener proof is incomplete; stop only the owned runtime before restarting")
            announce({"state": "ready", "reused": True, "provenance": str(self.run / "provenance.json")})
            return
        if existing and any(observe_process(record)[1] for record in existing["processes"].values()):
            raise RuntimeError("An incomplete owned runtime exists; stop it before starting")
        for port in self.allocation["ports"].values():
            with socket.socket() as listener:
                try:
                    listener.bind(("127.0.0.1", port))
                except OSError:
                    raise RuntimeError("An allocated port is busy; its owner was left untouched") from None
        directory = self.run / ("start-" + uuid.uuid4().hex[:12])
        private_directory(directory)
        runtime = {"task": TASK, "directory": str(directory), "build_directory": build["directory"], "source_id": build["source_id"],
                   "allocation_sha256": build["allocation_sha256"], "environment_sha256": build["environment_sha256"],
                   "state": "starting", "runtime_permitted": False, "processes": {}, "renderer_proof_file": str(directory / "renderer-serving.json")}
        self.save_runtime(runtime)
        children = {}
        try:
            before = self.sql(environment, "SELECT json_build_object('database',current_database(),'connections',(SELECT count(*) FROM pg_stat_activity),'max_connections',current_setting('max_connections')::int)", directory / "database.log")
            runtime["database_before"] = before
            if before["database"] != self.allocation["database"] or before["max_connections"] - before["connections"] < 20:
                raise RuntimeError("Owned database identity or cluster headroom check failed")
            command_step([str(Path(build["directory"]) / "bin/migrate"), "up"], stage / "server", environment, directory, "migrate", migration_timeout)
            runtime["migration_ledger"] = self.database_ready(build, environment, directory / "database.log")
            self.assert_build(build)
            commands = {
                "model": [build["tools"]["node"]["path"], str(stage / "e2e/fixtures/project-knowledge-model.mjs")],
                "api": [str(Path(build["directory"]) / "bin/server")],
                "web": [build["tools"]["node"]["path"], str(stage / "apps/web/node_modules/next/dist/bin/next"), "start", str(stage / "apps/web"), "--hostname", "127.0.0.1", "--port", str(self.allocation["ports"]["web"])],
                "renderer": [build["tools"]["node"]["path"], str(stage / "scripts/check-desktop-renderer.mjs"), str(stage / "apps/desktop/out/renderer"), str(self.allocation["ports"]["renderer"]), runtime["renderer_proof_file"]],
            }
            for name, command in commands.items():
                announce({"stage": "start-" + name, "origin": self.allocation[name + "_origin"], "log": str(directory / (name + ".log"))})
                working_directory = stage / "apps/web" if name == "web" else stage
                child, record = launch(command, working_directory, environment, directory / (name + ".log"))
                children[name] = child
                runtime["processes"][name] = record
                self.save_runtime(runtime)
            deadline = time.monotonic() + timeout
            failure = None
            while time.monotonic() < deadline:
                if any(child.poll() is not None for child in children.values()):
                    raise RuntimeError("An owned service exited before readiness; private logs retained")
                try:
                    proof = self.prove_services(runtime, build, verify_build=False)
                    self.assert_build(build)
                    break
                except Exception as error:
                    failure = type(error).__name__
                    with (directory / "readiness.log").open("a") as output:
                        os.fchmod(output.fileno(), 0o600)
                        output.write(f"{type(error).__name__}: {error}\n")
                    self.save_runtime(runtime)
                    time.sleep(0.5)
            else:
                runtime["readiness_failure"] = failure
                raise RuntimeError("Production runtime readiness deadline exhausted; private logs retained")
            runtime.update({"state": "ready", "runtime_permitted": True, "ready_at": datetime.datetime.now(datetime.timezone.utc).isoformat()})
            self.save_runtime(runtime)
            save(directory / "provenance.json", proof)
            save(self.run / "provenance.json", proof)
            announce({"state": "ready", "provenance": str(self.run / "provenance.json"), "environment": build["environment_file"],
                      "source_root": str(stage), "database": self.allocation["database"], "origins": {name: self.allocation[name + "_origin"] for name in commands}})
        except BaseException:
            runtime["state"] = "start_failed"
            for name, record in reversed(list(runtime["processes"].items())):
                try:
                    stop_process(record, children.get(name))
                except Exception:
                    runtime["state"] = "cleanup_failed"
            self.save_runtime(runtime)
            save(self.run / "provenance.json", {"runtime_permitted": False, "task": TASK, "state": runtime["state"]})
            raise

    def status(self):
        build = self.load("build")
        runtime = self.load("runtime")
        result = {"task": TASK, "allocation": str(self.allocation_file), "run_directory": str(self.run),
                  "database": self.allocation["database"], "state": runtime["state"] if runtime else "built" if build else "allocated",
                  "runtime_permitted": False, "services": {}}
        if build:
            result.update({"source_root": build["source_root"], "source_id": build["source_id"], "build": str(Path(build["directory"]) / "build.json")})
        if runtime:
            table = process_table()
            for name, record in runtime["processes"].items():
                result["services"][name] = {"pid": record["pid"], "owned_process_alive": same_process(record, table.get(record["pid"])),
                                            "origin": self.allocation[name + "_origin"], "listener_pid": listener_pid(self.allocation["ports"][name]), "log": record["log"]}
            if runtime.get("state") == "ready":
                try:
                    if runtime.get("runtime_permitted") is not True or self.prove_services(runtime, build) != self.load("provenance"):
                        raise RuntimeError("Live proof differs from the retained runtime permit")
                    result["runtime_permitted"] = True
                    result["provenance"] = str(self.run / "provenance.json")
                except Exception:
                    result["state"] = "not_ready"
        announce(result)

    def stop(self):
        runtime = self.load("runtime")
        if runtime is None:
            announce({"state": "not_started", "stopped_owned_pids": []})
            return
        runtime["runtime_permitted"] = False
        proof = self.load("provenance") or {"task": TASK}
        proof["runtime_permitted"] = False
        save(self.run / "provenance.json", proof)
        errors = []
        for name, record in reversed(list(runtime["processes"].items())):
            try:
                stop_process(record)
            except Exception:
                errors.append(name)
        runtime["state"] = "cleanup_failed" if errors else "stopped"
        self.save_runtime(runtime)
        announce({"state": runtime["state"], "database_retained": self.allocation["database"], "cleanup_failures": errors})
        if errors:
            raise RuntimeError("Some recorded process ownership could not be safely cleaned up")

    def bind_browsers(self, build, environment, path, directory):
        cache = Path(path)
        if not cache.is_absolute() or not cache.is_dir():
            raise RuntimeError("Supply an explicit installed browser cache; this launcher never downloads browsers")
        cache = cache.resolve()
        environment["PLAYWRIGHT_BROWSERS_PATH"] = str(cache)
        script = """
const {createRequire} = require('node:module');
const path = require('node:path');
const fromRoot = createRequire(process.argv[1]);
const testEntry = fromRoot.resolve('@playwright/test');
const playwrightEntry = createRequire(testEntry).resolve('playwright');
const coreEntry = createRequire(playwrightEntry).resolve('playwright-core');
const coreRoot = path.dirname(coreEntry);
const {registry} = require(path.join(coreRoot, 'lib/server/registry/index.js'));
const {hostPlatform} = require(path.join(coreRoot, 'lib/server/utils/hostPlatform.js'));
console.log(JSON.stringify({platform: process.platform, architecture: process.arch, host_platform: hostPlatform,
  runner_package: path.join(path.dirname(testEntry), 'package.json'),
  core_package: path.join(coreRoot, 'package.json'), registry_file: path.join(coreRoot, 'browsers.json'),
  browsers: ['chromium', 'chromium-headless-shell', 'ffmpeg'].map(name => {
    const entry = registry.findExecutable(name);
    return {name, directory: entry.directory, executable: entry.executablePath()};
  })}));
"""
        result = subprocess.run([build["tools"]["node"]["path"], "-e", script, str(Path(build["source_root"]) / "package.json")],
                                cwd=directory, env=environment, capture_output=True, text=True, timeout=15)
        if result.returncode:
            with (directory / "browser-binding.log").open("w") as output:
                os.fchmod(output.fileno(), 0o600)
                output.write(result.stderr)
            raise RuntimeError("Installed Playwright registry resolution failed; private diagnostics retained")
        binding = json.loads(result.stdout)
        binding.update({"cache": str(cache), "hashes": {}, "measured_at": datetime.datetime.now(datetime.timezone.utc).isoformat()})
        for key in ["runner_package", "core_package", "registry_file"]:
            binding["hashes"][binding[key]] = file_hash(Path(binding[key]))
        started = time.monotonic()
        for browser in binding["browsers"]:
            folder = Path(browser["directory"]).resolve()
            executable = Path(browser["executable"])
            if not folder.is_relative_to(cache) or not (folder / "INSTALLATION_COMPLETE").is_file() or not executable.is_file() or not os.access(executable, os.X_OK):
                raise RuntimeError("Browser revision/resources are not installed in the explicit cache")
            for candidate in [cache, folder, *sorted(folder.rglob("*"))]:
                if candidate.is_symlink() and not candidate.resolve().is_relative_to(cache):
                    raise RuntimeError("Browser cache redirects outside its permitted installation")
                if candidate.stat().st_uid != os.getuid() or candidate.stat().st_mode & 0o022:
                    raise RuntimeError("Browser installation is unowned or writable by other users")
                if candidate.is_file():
                    binding["hashes"][str(candidate)] = file_hash(candidate)
                if time.monotonic() - started > 120:
                    raise RuntimeError("Browser identity measurement exceeded its bounded budget")
        save(directory / "browser-binding.json", binding)
        return binding

    def run_tests(self, timeout, browsers_path):
        build = self.load("build")
        stage, environment = self.assert_build(build)
        runtime = self.load("runtime")
        if runtime is None or runtime.get("state") != "ready" or runtime.get("runtime_permitted") is not True:
            raise RuntimeError("Start and prove the source-ready production runtime before running P6")
        proof = self.prove_services(runtime, build)
        if self.load("provenance") != proof:
            raise RuntimeError("Retained runtime provenance no longer matches the live services")
        directory = self.run / ("test-" + uuid.uuid4().hex[:12])
        private_directory(directory)
        private_directory(directory / "results")
        environment = dict(environment)
        record = {"task": TASK, "state": "preparing", "source_id": build["source_id"], "source_root": str(stage),
                  "allocation_sha256": build["allocation_sha256"], "provenance_sha256": file_hash(self.run / "provenance.json"),
                  "directory": str(directory), "passed": False}
        save(directory / "result.json", record)
        try:
            self.database_ready(build, environment, directory / "database.log")
            binding = self.bind_browsers(build, environment, browsers_path or environment["PLAYWRIGHT_BROWSERS_PATH"], directory)
            environment["PLAYWRIGHT_JSON_OUTPUT_NAME"] = str(directory / "playwright-report.json")
            save(directory / "environment.json", environment)
            record["browser_binding_file"] = str(directory / "browser-binding.json")
            record["browser_binding_sha256"] = file_hash(directory / "browser-binding.json")
            command = [build["tools"]["node"]["path"], str(stage / "node_modules/@playwright/test/cli.js"), "test", "--config", str(stage / "playwright.config.ts"),
                       "project-knowledge-automation.spec.ts", "--workers=1", "--retries=0", "--output", str(directory / "results"), "--reporter=line,json"]
            record["state"] = "running"
            save(directory / "result.json", record)
            process = command_step(command, stage, environment, directory, "playwright", timeout)
            report = json.loads((directory / "playwright-report.json").read_text())
            inventory = test_inventory(report)
            record.update({"process": process, "inventory": inventory})
            if len(inventory) != 2 or not all(test["expected_status"] == "passed" and test["results"] == ["passed"] for test in inventory) or not all(any(platform + ": acceptance," in test["title"] for test in inventory) for platform in ["web", "desktop"]):
                raise RuntimeError("P6 did not run both complete Web/Desktop journeys without skips/retries")
            self.prove_services(runtime, build)
            if any(file_hash(Path(filename)) != digest for filename, digest in binding["hashes"].items()):
                raise RuntimeError("Browser installation changed during P6")
            record.update({"state": "passed", "passed": True})
        except BaseException:
            record["state"] = "failed"
            record["passed"] = False
            report_file = directory / "playwright-report.json"
            if report_file.exists():
                record["report_file"] = str(report_file)
                record["inventory"] = test_inventory(json.loads(report_file.read_text()))
            raise
        finally:
            record["source_stable"] = source_snapshot()["source_id"] == build["source_id"]
            if not record["source_stable"]:
                record.update({"state": "failed", "passed": False})
            save(directory / "result.json", record)
        if not record["passed"]:
            raise RuntimeError("P6 source identity changed during final evidence capture")
        announce({"state": "passed", "result": str(directory / "result.json"), "named_tests": len(inventory), "source_id": build["source_id"]})


def masked_environment(home, temporary, overrides=None):
    """Mask dotenv key names without copying any repository environment value.

    Next config explicitly loads root .env, even from an external cwd. Empty
    entries prevent dotenv's non-overriding loader from importing credentials.
    Callers overlay only known task-owned values after this mask.
    """
    env = {}
    for folder in [ROOT, ROOT / "apps/web", ROOT / "apps/desktop"]:
        for path in folder.glob(".env*"):
            if not path.is_file():
                continue
            for match in re.finditer(r"^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=", path.read_text(), re.M):
                env[match[1]] = ""
    env.update({"PATH": os.environ["PATH"], "HOME": str(home), "TMPDIR": str(temporary),
                "TMP": str(temporary), "TEMP": str(temporary), "LANG": "en_US.UTF-8",
                "SHELL": "/usr/bin/false", "CI": "1", "NEXT_TELEMETRY_DISABLED": "1",
                "ANALYTICS_DISABLED": "true", "APP_ENV": "test", "NODE_ENV": "production",
                "MULTICA_MESSAGING_INTEGRATIONS_ENABLED": "false", "MULTICA_VCS_INTEGRATION_ENABLED": "false",
                "MULTICA_DEVICE_AUTH_ENABLED": "false", "MULTICA_GC_ENABLED": "false",
                "MULTICA_DAEMON_AUTO_UPDATE": "false", "MULTICA_DAEMON_AUTO_RELOAD": "false"})
    cache = json.loads(subprocess.check_output([shutil.which("go"), "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH"], text=True))
    env.update(cache)
    env.update(overrides or {})
    for key in ["GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"]:
        env.pop(key, None)
    return env


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["allocate", "build", "start", "status", "stop", "run"])
    parser.add_argument("--allocation", help="Explicit existing task allocation; no implicit allocation or database fallback")
    parser.add_argument("--source-ready", action="store_true", help="Root has frozen Go, shared/UI source, specs and generated help inputs; required for build/start/run")
    parser.add_argument("--browsers-path", help="Explicit installed Playwright cache; never installed or downloaded by this launcher")
    parser.add_argument("--build-timeout", type=int, default=1200, help="Per-build-stage deadline in seconds (default: 1200)")
    parser.add_argument("--migration-timeout", type=int, default=600, help="Owned migration startup deadline in seconds (default: 600)")
    parser.add_argument("--ready-timeout", type=int, default=180, help="Production listener readiness deadline in seconds (default: 180)")
    parser.add_argument("--test-timeout", type=int, default=1800, help="Complete P6 journey deadline in seconds (default: 1800)")
    args = parser.parse_args()
    if any(not 1 <= value <= 3600 for value in [args.build_timeout, args.migration_timeout, args.ready_timeout, args.test_timeout]):
        parser.error("Stage deadlines must be between 1 and 3600 seconds")
    if args.browsers_path and not Path(args.browsers_path).is_absolute():
        parser.error("Browser cache binding must be an explicit absolute path")
    if args.action == "allocate":
        STATE.mkdir(mode=0o700, parents=True, exist_ok=True)
        run = STATE / ("p6-browser-" + uuid.uuid4().hex[:8])
        run.mkdir(mode=0o700)
        for name in ["home", "tmp", "bin", "logs", "results"]:
            (run / name).mkdir(mode=0o700)
        if subprocess.run(["git", "-C", str(run), "rev-parse", "--show-toplevel"], capture_output=True).returncode == 0:
            raise RuntimeError("Private state must be outside Git")
        spec = importlib.util.spec_from_file_location("owned_go", RESEARCH / "run-owned-go.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        parsed = module.database_url()
        env = masked_environment(run / "home", run / "tmp")
        pg_env = {**env, "PGHOST": parsed.hostname, "PGPORT": str(parsed.port or 5432),
                  "PGUSER": urllib.parse.unquote(parsed.username or ""), "PGPASSWORD": urllib.parse.unquote(parsed.password or ""),
                  "PGDATABASE": "postgres", "PGCONNECT_TIMEOUT": "5"}
        def sql(statement):
            result = subprocess.run([PSQL, "-XAt", "-v", "ON_ERROR_STOP=1", "-c", statement], env=pg_env,
                                    capture_output=True, text=True, timeout=30)
            if result.returncode:
                (run / "logs/database-error.log").write_text(result.stderr)
                raise RuntimeError("Database allocation failed; private diagnostics retained")
            return result.stdout.strip()
        cluster = json.loads(sql("SELECT json_build_object('database',current_database(),'connections',(SELECT count(*) FROM pg_stat_activity),'max_connections',current_setting('max_connections')::int)"))
        if cluster["database"] != "postgres" or cluster["max_connections"] - cluster["connections"] < 20:
            raise RuntimeError("Cluster identity/headroom check failed")
        database = "mca_knowledge_browser_" + uuid.uuid4().hex[:12]
        sql('CREATE DATABASE "' + database + '"')
        pg_env["PGDATABASE"] = database
        if sql("SELECT current_database()") != database:
            raise RuntimeError("Actual database differs from allocation")
        ports = {name: free_port() for name in ["api", "web", "renderer", "model"]}
        if len(set(ports.values())) != len(ports):
            raise RuntimeError("Port allocation collided")
        origins = {name: f"http://127.0.0.1:{port}" for name, port in ports.items()}
        env.update({"DATABASE_URL": urllib.parse.urlunparse(parsed._replace(path="/" + database)),
                    "JWT_SECRET": "p6-synthetic-local-signing-key-" + uuid.uuid4().hex,
                    "MULTICA_DEV_VERIFICATION_CODE": "846215", **auth_configuration(database),
                    "DATABASE_MAX_CONNS": "6", "DATABASE_MIN_CONNS": "0", "PORT": str(ports["api"]),
                    "NEXT_PUBLIC_API_URL": origins["api"], "API_URL": origins["api"], "REMOTE_API_URL": origins["api"],
                    "NEXT_PUBLIC_WS_URL": origins["api"].replace("http", "ws") + "/ws",
                    "FRONTEND_ORIGIN": origins["web"], "PLAYWRIGHT_BASE_URL": origins["web"],
                    "CORS_ALLOWED_ORIGINS": ",".join([origins["web"], origins["renderer"]]),
                    "MULTICA_KNOWLEDGE_CONTEXT_ENABLED": "true", "MULTICA_LLM_BASE_URL": origins["model"] + "/v1",
                    "MULTICA_LLM_API_KEY": "p6-synthetic-local-gateway-key", "MULTICA_LLM_DEFAULT_MODEL": "p6-controlled",
                    "MULTICA_LLM_MAX_RETRIES": "0", "KNOWLEDGE_MODEL_PORT": str(ports["model"]),
                    "KNOWLEDGE_MODEL_LOG": str(run / "logs/model.jsonl"), "P6_MODEL_URL": origins["model"],
                    "P6_RUN_DIRECTORY": str(run), "P6_DATABASE": database, "P6_BINARY": str(run / "bin/multica"),
                    "P6_PROVIDER_CAPTURE": str(run / "logs/provider.jsonl"), "PLAYWRIGHT_BROWSERS_PATH": str(run / "home/browsers"),
                    "COLLABORATION_DETAIL_TASK": TASK, "COLLABORATION_DETAIL_ALLOCATION_FILE": str(run / "allocation.json"),
                    "COLLABORATION_DETAIL_PROVENANCE_FILE": str(run / "provenance.json"),
                    "COLLABORATION_E2E_API_URL": origins["api"], "COLLABORATION_ELECTRON_SOURCE_ROOT": str(ROOT),
                    "COLLABORATION_ELECTRON_RENDERER_URL": origins["renderer"], "RATE_LIMIT_AUTH_VERIFY": "1000"})
        # Deny every known provider probe, including table-derived built-ins.
        keys = provider_keys(ROOT)
        for key in keys:
            env[key] = str(run / "bin" / ("missing-" + key.lower()))
        fake = run / "bin/claude"
        fake.write_text((ROOT / "e2e/fixtures/project-knowledge-provider.py").read_text())
        fake.chmod(0o700)
        env["MULTICA_CLAUDE_PATH"] = str(fake)
        save(run / "environment.json", env)
        allocation = {"task": TASK, "source_root": str(ROOT), "run_directory": str(run), "database": database,
                      **{name + "_origin": origin for name, origin in origins.items()}, "ports": ports, "cluster_before": cluster,
                      "provider_probe_keys": sorted(keys), "real_accounts": False, "created_by": "p6-runtime.py"}
        save(run / "allocation.json", allocation)
        save(RESEARCH / "p6-allocation.json", allocation)
        print(json.dumps({"allocation": str(run / "allocation.json"), "database": database, "ports": ports}))
        return
    if not args.allocation:
        parser.error("An explicit allocation file is required")
    if args.action in {"build", "start", "run"} and not args.source_ready:
        parser.error("Root source-ready signal is required before final build/start/run")
    runtime = Runtime(args.allocation)
    if args.action == "status":
        runtime.status()
        return
    with runtime.lock():
        if args.action == "build":
            runtime.build(args.build_timeout, args.browsers_path)
        elif args.action == "start":
            runtime.start(args.ready_timeout, args.migration_timeout)
        elif args.action == "run":
            runtime.run_tests(args.test_timeout, args.browsers_path)
        elif args.action == "stop":
            runtime.stop()


if __name__ == "__main__":
    def interrupt(signum, frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, interrupt)
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        if CURRENT_RUN is not None:
            diagnostics = CURRENT_RUN / "logs" / ("lifecycle-error-" + uuid.uuid4().hex[:12] + ".log")
            if diagnostics.parent.is_dir():
                with diagnostics.open("w") as output:
                    os.fchmod(output.fileno(), 0o600)
                    traceback.print_exc(file=output)
        message = str(error) if isinstance(error, RuntimeError) else "P6 runtime interrupted" if isinstance(error, KeyboardInterrupt) else "P6 runtime failed; private diagnostics retained"
        print(message, file=sys.stderr)
        raise SystemExit(130 if isinstance(error, KeyboardInterrupt) else 1)
