#!/usr/bin/env python3
"""Task-private QA environment for 11-09-collaboration-business-scenario-qa.

Run from the main checkout root. Every write goes to this task's private root,
its new database, or its evidence directory. Nothing here reads or changes any
other environment.

  allocate        create the private root and allocation record
  snapshot        git-archive the candidate commit into <root>/source, install deps
  build           build API, CLI, host daemon test binary, providers, Web, renderer
  database        create, verify and migrate the task database
  start | verify  start / prove the API, Web and renderer recorded here
  restart-api     --knowledge on|off: restart only the API with that startup flag
  sync            copy this task's test files into the snapshot (hash-recorded)
  audit           prove snapshot tracked files still equal the candidate tree
  run             exec a command in the snapshot with the private env
  stop            SIGTERM only recorded, identity-checked process groups
  scan            credential-shape scan of the evidence directory
"""
import hashlib
import json
import os
import secrets
import signal
import subprocess
import sys
import time
import urllib.request
import uuid
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit

REPO = next(p for p in Path(__file__).resolve().parents if (p / ".git").exists())
TASK = "11-09-collaboration-business-scenario-qa"
EVIDENCE = REPO / "artifacts/qa/collaboration-business-scenarios-20261109"
ALLOCATION = EVIDENCE / "environment/allocation.json"
PORTS = {"api": 59765, "web": 59766, "renderer": 59767}
PROVIDERS = {
    "host": "../e2e/fixtures/collaboration-host-provider.go",
    "memory": "../e2e/fixtures/collaboration-memory-provider.go",
    "closure": "../e2e/fixtures/collaboration-multi-account-provider.go",
}
# Task-owned test sources synced into the snapshot. Missing optional files are skipped.
TEST_FILES = [
    "e2e/collaboration-business-scenarios.spec.ts",
    "e2e/fixtures/collaboration-business-scenarios.ts",
    "e2e/fixtures/collaboration-business-provider.go",
]
TOKEN_SHAPES = ("mcb_", "mct_", "mdt_", "mat_", "mul_", "Bearer ey")


def die(message: str, code: int = 2):
    print(message, file=sys.stderr)
    raise SystemExit(code)


def load() -> dict:
    if not ALLOCATION.is_file():
        die("No allocation; run allocate first")
    return json.loads(ALLOCATION.read_text())


def save(a: dict):
    ALLOCATION.write_text(json.dumps(a, indent=2, sort_keys=True) + "\n")


def private_env(a: dict) -> dict:
    return json.loads(Path(a["env_file"]).read_text())


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def git(*args: str) -> str:
    return subprocess.check_output(["git", "-C", str(REPO), *args], text=True).strip()


def port_free(port: int) -> bool:
    return subprocess.run(["lsof", "-nP", f"-iTCP:{port}", "-sTCP:LISTEN", "-t"], capture_output=True).stdout.strip() == b""


def step(a: dict, name: str, cmd: list, cwd: Path, env: dict | None = None):
    """Run one recorded build/setup step; its log stays in the private root."""
    root = Path(a["private_root"])
    log = root / "logs" / f"{name}.log"
    started = time.time()
    with log.open("wb") as handle:
        code = subprocess.call(cmd, cwd=cwd, env=env, stdout=handle, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL)
    record = {"command": cmd, "cwd": str(cwd), "exit_code": code, "seconds": round(time.time() - started, 1)}
    (root / "state" / f"{name}.json").write_text(json.dumps(record, indent=2) + "\n")
    (EVIDENCE / "environment" / f"{name}.json").write_text(json.dumps(record, indent=2) + "\n")
    if code != 0:
        die(f"{name} failed with exit {code}; see {log}")
    return record


def allocate():
    if ALLOCATION.exists():
        die("Allocation already exists; a new environment needs a new task-owned path")
    for name, port in PORTS.items():
        if not port_free(port):
            die(f"Port {port} for {name} is already listening")
    root = REPO.parent / f"qa-private-collab-biz-20261109-{secrets.token_hex(4)}"
    if root.exists():
        die("Private root collision")
    for sub in ["bin", "state", "logs", "home", "uploads", "registry"]:
        (root / sub).mkdir(parents=True, mode=0o700)
    root.chmod(0o700)
    (EVIDENCE / "environment").mkdir(parents=True, exist_ok=True)
    sha = git("rev-parse", "HEAD")
    a = {"task": TASK, "private_root": str(root), "snapshot": str(root / "source"), "candidate_sha": sha,
         "candidate_tree": git("rev-parse", "HEAD^{tree}"), "database": f"multica_qa_biz_20261109_{secrets.token_hex(6)}",
         "api_port": PORTS["api"], "web_port": PORTS["web"], "renderer_port": PORTS["renderer"],
         "deployment_id": str(uuid.uuid4()), "state": "allocated", "created_at": time.strftime("%Y-%m-%dT%H:%M:%S%z")}
    save(a)
    print(json.dumps({k: a[k] for k in ["private_root", "candidate_sha", "database", "api_port", "web_port", "renderer_port"]}))


def tracked_blobs(sha: str) -> dict:
    rows = git("ls-tree", "-r", "--full-tree", sha).splitlines()
    out = {}
    for row in rows:
        meta, path = row.split("\t", 1)
        mode, kind, blob = meta.split()
        if kind == "blob":
            out[path] = (mode, blob)
    return out


def blob_sha(path: Path) -> str:
    data = path.read_bytes() if not path.is_symlink() else os.readlink(path).encode()
    return hashlib.sha1(b"blob %d\0" % len(data) + data).hexdigest()


def audit(write: bool = True):
    a = load()
    source = Path(a["snapshot"])
    expected = tracked_blobs(a["candidate_sha"])
    drift, normalized = [], []
    for path, (mode, blob) in expected.items():
        target = source / path
        if mode == "160000":
            continue
        if not target.exists() and not target.is_symlink():
            drift.append({"path": path, "issue": "missing"})
        elif blob_sha(target) != blob:
            # Checkout attributes (eol=crlf) legitimately change exported bytes;
            # apply the same path attributes before comparing with the blob.
            cleaned = subprocess.run(["git", "-C", str(REPO), "hash-object", f"--path={path}", str(target)],
                                     capture_output=True, text=True).stdout.strip()
            if cleaned == blob:
                normalized.append(path)
            else:
                drift.append({"path": path, "issue": "content"})
    report = {"candidate_sha": a["candidate_sha"], "tracked_files": len(expected), "drift": drift[:50],
              "drift_count": len(drift), "attribute_normalized": normalized,
              "checked_at": time.strftime("%Y-%m-%dT%H:%M:%S%z")}
    if write:
        name = f"source-audit-{time.strftime('%H%M%S')}.json"
        (EVIDENCE / "environment" / name).write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({k: report[k] for k in ["candidate_sha", "tracked_files", "drift_count", "attribute_normalized"]}))
    if drift:
        die("Snapshot differs from the candidate tree")


def snapshot():
    a = load()
    source = Path(a["snapshot"])
    marker = Path(a["private_root"]) / "state/extracted.json"
    if source.exists() and not marker.exists():
        die("Snapshot exists without a completed extraction marker")
    if not source.exists():
        source.mkdir(mode=0o700)
        archive = subprocess.Popen(["git", "-C", str(REPO), "archive", "--format=tar", a["candidate_sha"]], stdout=subprocess.PIPE)
        code = subprocess.call(["tar", "-x", "-C", str(source)], stdin=archive.stdout)
        archive.stdout.close()
        if archive.wait() != 0 or code != 0:
            die("git archive extraction failed")
        marker.write_text(json.dumps({"candidate_sha": a["candidate_sha"], "at": time.strftime("%Y-%m-%dT%H:%M:%S%z")}) + "\n")
    audit()
    # Dependency install uses the ordinary user environment so pnpm reaches its
    # store, minus variables that would skip dev dependencies or retarget Node.
    install_env = {k: v for k, v in os.environ.items() if k not in ("NODE_ENV", "ELECTRON_RUN_AS_NODE", "NODE_OPTIONS")}
    step(a, "pnpm-install", ["pnpm", "install", "--frozen-lockfile", "--prefer-offline"], source, install_env)
    links = {}
    for package in ["core", "ui", "views"]:
        link = source / "node_modules/@multica" / package
        links[package] = str(link.resolve())
        if not str(link.resolve()).startswith(str(source)):
            die(f"Workspace link {package} escapes the snapshot")
    a["workspace_links"] = links
    a["state"] = "snapshot_ready"
    save(a)


def write_env(a: dict):
    root = Path(a["private_root"])
    source_env = {}
    for line in (REPO / ".env").read_text().splitlines():
        if line.startswith("DATABASE_URL="):
            source_env["DATABASE_URL"] = line.split("=", 1)[1].strip().strip('"').strip("'")
    if not source_env.get("DATABASE_URL"):
        die("Shared database endpoint missing")
    parts = urlsplit(source_env["DATABASE_URL"])
    if parts.hostname not in ("localhost", "127.0.0.1"):
        die("Refusing a non-loopback database")
    target = urlunsplit((parts.scheme, parts.netloc, "/" + a["database"], parts.query, parts.fragment))
    admin = urlunsplit((parts.scheme, parts.netloc, "/postgres", parts.query, parts.fragment))
    go_env = json.loads(subprocess.check_output(["go", "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH"], text=True))
    api, web, renderer = (f"http://127.0.0.1:{a[k]}" for k in ["api_port", "web_port", "renderer_port"])
    env = {
        "PATH": os.environ["PATH"], "HOME": str(root / "home"), "USER": os.environ.get("USER", "artisan"),
        "TMPDIR": os.environ.get("TMPDIR", "/tmp"), **go_env,
        "PLAYWRIGHT_BROWSERS_PATH": str(Path.home() / "Library/Caches/ms-playwright"),
        "DATABASE_URL": target, "TASK_DATABASE_NAME": a["database"], "DATABASE_MAX_CONNS": "8", "DATABASE_MIN_CONNS": "1",
        "APP_ENV": "development", "NODE_ENV": "production", "PORT": str(a["api_port"]), "FRONTEND_PORT": str(a["web_port"]),
        "FRONTEND_ORIGIN": web, "JWT_SECRET": secrets.token_hex(32), "ALLOW_SIGNUP": "true",
        "MULTICA_AUTH_MODE": "password", "MULTICA_PASSWORD_LIMITER_MODE": "single", "MULTICA_DEVICE_AUTH_ENABLED": "false",
        "MULTICA_CLOUD_URL": "", "MULTICA_MANAGED_INSTALLATIONS_ENABLED": "true", "MULTICA_DEPLOYMENT_ID": a["deployment_id"],
        "MULTICA_PLATFORM_ADMIN_ENABLED": "false", "DISABLE_WORKSPACE_CREATION": "false",
        "MULTICA_APP_URL": web, "MULTICA_PUBLIC_URL": api, "MULTICA_DAEMON_SERVER_URL": api,
        "LOCAL_UPLOAD_DIR": str(root / "uploads"), "LOCAL_UPLOAD_BASE_URL": api + "/uploads",
        "CORS_ALLOWED_ORIGINS": web + "," + renderer, "ALLOWED_ORIGINS": web + "," + renderer, "ANALYTICS_DISABLED": "true",
        "MULTICA_MESSAGING_INTEGRATIONS_ENABLED": "false", "MULTICA_VCS_INTEGRATION_ENABLED": "false",
        "NEXT_PUBLIC_API_URL": api, "NEXT_PUBLIC_WS_URL": api.replace("http:", "ws:") + "/ws", "REMOTE_API_URL": api, "DOCS_URL": "",
        "COLLABORATION_E2E_API_URL": api, "PLAYWRIGHT_BASE_URL": web, "E2E_PASSWORD_AUTH": "1",
        "COLLABORATION_ELECTRON_RENDERER_URL": renderer, "COLLABORATION_ELECTRON_SOURCE_ROOT": a["snapshot"],
        "COLLABORATION_HOST_DAEMON": str(root / "bin/host-daemon"), "COLLABORATION_HOST_PROVIDER": str(root / "bin/host-provider"),
        "COLLABORATION_HOST_CLI": str(root / "bin/multica"), "MULTICA_DEV_HOME": str(root / "registry"), "NEXT_TELEMETRY_DISABLED": "1",
        "QA_EVIDENCE_ROOT": str(EVIDENCE),
    }
    env_file = root / "environment.json"
    env_file.write_text(json.dumps(env, indent=2) + "\n")
    env_file.chmod(0o600)
    admin_file = root / "state/admin-url.json"
    admin_file.write_text(json.dumps({"admin": admin}) + "\n")
    admin_file.chmod(0o600)
    a["env_file"] = str(env_file)
    a["api_url"], a["web_url"], a["renderer_url"] = api, web, renderer
    a["env_sha256"] = sha256(env_file)
    save(a)


def build():
    a = load()
    if "env_file" not in a:
        write_env(a)
        a = load()
    root, source, env = Path(a["private_root"]), Path(a["snapshot"]), private_env(a)
    server = source / "server"
    step(a, "api-build", ["go", "-C", str(server), "build", "-ldflags", f"-X main.commit={a['candidate_sha']}", "-o", str(root / "bin/api"), "./cmd/server"], source, env)
    step(a, "cli-build", ["go", "-C", str(server), "build", "-o", str(root / "bin/multica"), "./cmd/multica"], source, env)
    step(a, "daemon-build", ["go", "-C", str(server), "test", "-c", "-o", str(root / "bin/host-daemon"), "./internal/daemon"], source, env)
    for name, path in PROVIDERS.items():
        step(a, f"{name}-provider-build", ["go", "-C", str(server), "build", "-o", str(root / f"bin/{name}-provider"), path], source, env)
    step(a, "web-build", ["pnpm", "--filter", "@multica/web", "build"], source, env)
    step(a, "renderer-build", ["pnpm", "--filter", "@multica/desktop", "exec", "electron-vite", "build"], source, env)
    a["binary_hashes"] = {b.name: sha256(b) for b in (root / "bin").iterdir() if b.is_file()}
    a["web_build_id"] = (source / "apps/web/.next/BUILD_ID").read_text().strip()
    a["state"] = "built"
    save(a)


def build_business_provider():
    a = load()
    source, env = Path(a["snapshot"]), private_env(a)
    if not (source / "e2e/fixtures/collaboration-business-provider.go").is_file():
        die("Business provider is not synced")
    step(a, "business-provider-build", ["go", "-C", str(source / "server"), "build", "-o",
         str(Path(a["private_root"]) / "bin/business-provider"), "../e2e/fixtures/collaboration-business-provider.go"], source, env)
    a.setdefault("binary_hashes", {})["business-provider"] = sha256(Path(a["private_root"]) / "bin/business-provider")
    save(a)


NODE_DB = r"""
const { createRequire } = require('node:module');
const pg = createRequire(process.env.SNAPSHOT + '/package.json')('pg');
(async () => {
  const mode = process.argv[1];
  const admin = new pg.Client({ connectionString: process.env.ADMIN_URL, connectionTimeoutMillis: 5000 });
  const out = {};
  if (mode === 'create') {
    await admin.connect();
    try {
      const cap = (await admin.query("SELECT current_setting('max_connections')::int AS maximum, (SELECT count(*)::int FROM pg_stat_activity) AS used")).rows[0];
      out.capacity_before = cap;
      if (cap.maximum - cap.used < 15) throw new Error('Insufficient connection headroom');
      const exists = await admin.query('SELECT 1 FROM pg_database WHERE datname=$1', [process.env.TASK_DATABASE_NAME]);
      if (exists.rowCount) throw new Error('Refusing an existing database');
      if (!/^multica_qa_biz_20261109_[0-9a-f]{12}$/.test(process.env.TASK_DATABASE_NAME)) throw new Error('Database name outside task boundary');
      await admin.query('CREATE DATABASE "' + process.env.TASK_DATABASE_NAME + '"');
      out.created = true;
    } finally { await admin.end(); }
  }
  const check = new pg.Client({ connectionString: process.env.DATABASE_URL, connectionTimeoutMillis: 5000 });
  await check.connect();
  out.current_database = (await check.query('SELECT current_database() AS name')).rows[0].name;
  if (mode === 'gate') out.gate = (await check.query('SELECT enabled, creation_enabled, host_execution_enabled, epoch::text FROM collaboration_execution_gate WHERE id=1')).rows[0] ?? null;
  await check.end();
  if (out.current_database !== process.env.TASK_DATABASE_NAME) throw new Error('Task database identity mismatch');
  console.log(JSON.stringify(out));
})().catch(error => { console.error('database step failed:', error.code ?? error.message); process.exit(2); });
"""


def node_db(a: dict, mode: str) -> dict:
    env = dict(private_env(a))
    env["SNAPSHOT"] = a["snapshot"]
    env["ADMIN_URL"] = json.loads((Path(a["private_root"]) / "state/admin-url.json").read_text())["admin"]
    out = subprocess.run(["node", "-e", NODE_DB, mode], env=env, capture_output=True, text=True)
    if out.returncode != 0:
        die(out.stderr.strip())
    return json.loads(out.stdout)


def database():
    a = load()
    if a.get("database_created"):
        die("Task database already created")
    created = node_db(a, "create")
    a["database_created"], a["capacity_before"] = True, created["capacity_before"]
    save(a)
    verified = node_db(a, "verify")
    if verified["current_database"] != a["database"]:
        die("Identity mismatch before migration")
    step(a, "migrate", ["go", "-C", str(Path(a["snapshot"]) / "server"), "run", "./cmd/migrate", "up"], Path(a["snapshot"]), private_env(a))
    a["gate_after_migrate"] = node_db(a, "gate")["gate"]
    a["state"] = "migrated"
    save(a)


def services_state(a: dict) -> Path:
    return Path(a["private_root"]) / "state/services.json"


def start():
    a = load()
    root, source, env = Path(a["private_root"]), Path(a["snapshot"]), private_env(a)
    records = json.loads(services_state(a).read_text()) if services_state(a).exists() else {}
    commands = {
        "api": ([str(root / "bin/api")], source / "server"),
        "web": (["pnpm", "--filter", "@multica/web", "exec", "next", "start", "--hostname", "127.0.0.1", "--port", str(a["web_port"])], source),
        "renderer": (["node", str(source / "scripts/check-desktop-renderer.mjs"), str(source / "apps/desktop/out/renderer"),
                      str(a["renderer_port"]), str(root / "state/renderer.running.json")], source),
    }
    for name, (cmd, cwd) in commands.items():
        if name in records:
            try:
                os.kill(records[name]["pid"], 0)
                continue
            except ProcessLookupError:
                pass
        if not port_free(a[f"{name}_port"]):
            die(f"Port for {name} is held by another process")
        log = (root / "logs" / f"{name}.log").open("ab")
        service_env = dict(env)
        if name == "api" and a.get("api_knowledge_context") == "on":
            service_env["MULTICA_KNOWLEDGE_CONTEXT_ENABLED"] = "true"
        process = subprocess.Popen(cmd, cwd=cwd, env=service_env, stdout=log, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, start_new_session=True)
        log.close()
        records[name] = {"pid": process.pid, "command": cmd, "cwd": str(cwd), "started_at": time.time()}
        services_state(a).write_text(json.dumps(records, indent=2) + "\n")
    print(json.dumps({name: record["pid"] for name, record in records.items()}))


def restart_api(args: list):
    """Restart only the recorded API with the knowledge startup flag on or off."""
    if args not in (["--knowledge", "on"], ["--knowledge", "off"]):
        die("Expected --knowledge on|off")
    a = load()
    records = json.loads(services_state(a).read_text()) if services_state(a).exists() else {}
    api = records.get("api")
    if api:
        try:
            if os.getpgid(api["pid"]) != api["pid"]:
                die("API process group ownership changed")
            os.killpg(api["pid"], signal.SIGTERM)
        except ProcessLookupError:
            pass
        for _ in range(60):
            if port_free(a["api_port"]):
                break
            time.sleep(0.5)
        else:
            die("API port did not close")
        del records["api"]
        services_state(a).write_text(json.dumps(records, indent=2) + "\n")
    a["api_knowledge_context"] = args[1]
    a.setdefault("api_config_history", []).append({"at": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "knowledge_context": args[1]})
    save(a)
    start()
    for _ in range(60):
        try:
            if json.loads(fetch(a["api_url"] + "/health")).get("status") == "ok":
                break
        except Exception:
            time.sleep(0.5)
    verify()


def fetch(url: str) -> bytes:
    with urllib.request.urlopen(url, timeout=5) as response:
        return response.read()


def verify():
    a = load()
    source, env = Path(a["snapshot"]), private_env(a)
    records = json.loads(services_state(a).read_text())
    health = json.loads(fetch(a["api_url"] + "/health"))
    config = json.loads(fetch(a["api_url"] + "/api/config"))
    if health.get("pid") != records["api"]["pid"] or health.get("commit") != a["candidate_sha"]:
        die("API identity mismatch")
    if config.get("auth_mode") != "password" or config.get("deployment_id") != a["deployment_id"] or config.get("managed_installation_supported") is not True:
        die("API configuration mismatch")
    html = fetch(a["web_url"] + "/login").decode()
    if a["api_url"] not in html or env["NEXT_PUBLIC_WS_URL"] not in html:
        die("Web runtime API/WS URL mismatch")
    if fetch(a["renderer_url"] + "/") != (source / "apps/desktop/out/renderer/index.html").read_bytes():
        die("Renderer content mismatch")
    listeners = {}
    for name in ["api", "web", "renderer"]:
        pids = subprocess.check_output(["lsof", "-nP", f"-iTCP:{a[name + '_port']}", "-sTCP:LISTEN", "-t"], text=True).split()
        if len(set(pids)) != 1 or os.getpgid(int(pids[0])) != records[name]["pid"]:
            die(f"Unowned or ambiguous listener for {name}")
        listeners[name] = int(pids[0])
    a["services"], a["listeners"], a["health"], a["state"] = records, listeners, health, "ready"
    a["api_config"] = {k: config.get(k) for k in ["auth_mode", "deployment_id", "managed_installation_supported"]}
    save(a)
    print(json.dumps({"state": "ready", "health": health, "listeners": listeners, "web_build_id": a.get("web_build_id")}))


QA_CONFIG = """import { defineConfig } from "@playwright/test";
import path from "node:path";
const outputRoot = process.env.QA_EVIDENCE_ROOT!;
const run = process.env.QA_RUN_ID ?? "";
if (!/^[a-z0-9-]+$/.test(run)) throw new Error("QA_RUN_ID is required");
export default defineConfig({
  testDir: "./e2e", timeout: 60000, workers: 1, retries: 0,
  outputDir: path.join(outputRoot, run, "test-results"),
  reporter: [["list"], ["json", { outputFile: path.join(outputRoot, run, "playwright.json") }]],
  use: { baseURL: process.env.PLAYWRIGHT_BASE_URL, headless: true, trace: "off", screenshot: "only-on-failure" },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
"""


def sync():
    a = load()
    source = Path(a["snapshot"])
    hashes = {}
    for rel in TEST_FILES:
        origin = REPO / rel
        if not origin.is_file():
            continue
        if rel in tracked_blobs(a["candidate_sha"]):
            die(f"{rel} is a tracked candidate file; sync only adds task files")
        (source / rel).write_bytes(origin.read_bytes())
        hashes[rel] = sha256(origin)
    (source / "playwright.qa.config.ts").write_text(QA_CONFIG)
    hashes["playwright.qa.config.ts"] = sha256(source / "playwright.qa.config.ts")
    record = {"synced_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "files": hashes}
    (EVIDENCE / "environment" / f"test-sync-{time.strftime('%H%M%S')}.json").write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps(record))


def run(args: list):
    a = load()
    env = private_env(a)
    if args and args[0] == "--provider":
        name = args[1]
        binary = Path(a["private_root"]) / "bin" / f"{name}-provider"
        if not binary.is_file():
            die("Unknown provider")
        env["COLLABORATION_HOST_PROVIDER"] = str(binary)
        args = args[2:]
    while args and "=" in args[0] and args[0].split("=", 1)[0].isupper():
        key, value = args.pop(0).split("=", 1)
        env[key] = value
    if not args:
        die("Expected a command")
    os.chdir(a["snapshot"])
    os.execvpe(args[0], args, env)


def stop():
    a = load()
    records = json.loads(services_state(a).read_text()) if services_state(a).exists() else {}
    for name, record in records.items():
        try:
            if os.getpgid(record["pid"]) != record["pid"]:
                die(f"{name} process group ownership changed")
            os.killpg(record["pid"], signal.SIGTERM)
        except ProcessLookupError:
            pass
    time.sleep(3)
    still = {name: a[f"{name}_port"] for name in ["api", "web", "renderer"] if not port_free(a[f"{name}_port"])}
    a["state"] = "stopped_database_retained" if not still else "stop_incomplete"
    a["stopped_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
    save(a)
    print(json.dumps({"state": a["state"], "still_listening": still}))


def scan():
    a = load()
    secrets_to_find = [v for k, v in private_env(a).items() if k in ("JWT_SECRET",) and v]
    db_password = urlsplit(private_env(a)["DATABASE_URL"]).password
    if db_password:
        secrets_to_find.append(db_password)
    hits = []
    for path in EVIDENCE.rglob("*"):
        if not path.is_file() or path.suffix in (".png", ".jpg", ".webm", ".zip"):
            continue
        text = path.read_text(errors="ignore")
        for shape in TOKEN_SHAPES:
            if shape in text:
                hits.append({"path": str(path.relative_to(EVIDENCE)), "shape": shape})
        for value in secrets_to_find:
            if value and value in text:
                hits.append({"path": str(path.relative_to(EVIDENCE)), "shape": "configured-secret"})
    report = {"scanned_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "hits": hits}
    (EVIDENCE / "credential-scan.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"hits": len(hits)}))
    if hits:
        die("Credential-shaped values found in evidence")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        die(__doc__)
    command, rest = sys.argv[1], sys.argv[2:]
    {"allocate": allocate, "snapshot": snapshot, "build": build, "database": database, "start": start, "verify": verify,
     "sync": sync, "audit": audit, "stop": stop, "scan": scan, "business-provider": build_business_provider,
     "restart-api": lambda: restart_api(rest),
     "run": lambda: run(rest)}.get(command, lambda: die(__doc__))()
