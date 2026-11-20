"""Create task-owned API/database and a separate production-Web build."""
import hashlib
import json
import os
import pathlib
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[3]
OUT = pathlib.Path(__file__).resolve().parent
STATE = pathlib.Path(tempfile.mkdtemp(prefix="multica-squad-welcome-"))
os.chmod(STATE, 0o700)
(OUT / "private-state-path.txt").write_text(str(STATE) + "\n")


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def run(args, cwd, env, name):
    with (OUT / f"{name}.log").open("w") as log:
        result = subprocess.run(args, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT)
    (OUT / f"{name}.exit").write_text(str(result.returncode) + "\n")
    if result.returncode:
        raise RuntimeError(f"{name} failed: see {OUT / f'{name}.log'}")


def wait_ready(url):
    for _ in range(150):
        try:
            with urllib.request.urlopen(url, timeout=2) as response:
                return json.loads(response.read()) if url.endswith("/health") else response.status
        except Exception:
            time.sleep(0.2)
    raise RuntimeError("Readiness failed: " + url)


api_port, web_port, renderer_port = free_port(), free_port(), free_port()
container_name = "multica-squad-welcome-" + secrets.token_hex(4)
container = subprocess.check_output([
    "docker", "run", "-d", "--platform", "linux/arm64", "--name", container_name,
    "--label", "multica.qa.owner=squad-welcome-20261120",
    "-e", "POSTGRES_USER=multica", "-e", "POSTGRES_PASSWORD=multica",
    "-e", "POSTGRES_DB=squad_welcome_acceptance", "-p", "127.0.0.1::5432",
    "sha256:215ae4d47393a980e9ef7ff7760f7dad53ec3e84bceb9fbd45cef34e50b5ba0f",
], text=True).strip()
(STATE / "container.txt").write_text(container)
database_port = subprocess.check_output(["docker", "port", container, "5432/tcp"], text=True).strip().rsplit(":", 1)[1]
env = {key: value for key, value in os.environ.items() if key in ["PATH", "HOME", "TMPDIR", "LANG", "USER", "LOGNAME"]}
env.update({
    "DATABASE_URL": f"postgres://multica:multica@127.0.0.1:{database_port}/squad_welcome_acceptance",
    "PORT": str(api_port), "FRONTEND_PORT": str(web_port),
    "FRONTEND_ORIGIN": f"http://localhost:{web_port}",
    "PLAYWRIGHT_BASE_URL": f"http://localhost:{web_port}",
    "NEXT_PUBLIC_API_URL": f"http://localhost:{api_port}",
    "NEXT_PUBLIC_WS_URL": f"ws://localhost:{api_port}/ws",
    "REMOTE_API_URL": f"http://localhost:{api_port}",
    "CORS_ALLOWED_ORIGINS": f"http://localhost:{web_port},http://127.0.0.1:{renderer_port}",
    "ALLOWED_ORIGINS": f"http://localhost:{web_port},http://127.0.0.1:{renderer_port}",
    "MULTICA_AUTH_MODE": "legacy", "MULTICA_DEVICE_AUTH_ENABLED": "false",
    "ANALYTICS_DISABLED": "true", "APP_ENV": "development",
    "JWT_SECRET": secrets.token_hex(32), "LOCAL_UPLOAD_DIR": str(STATE / "uploads"),
    "DATABASE_MAX_CONNS": "8", "DATABASE_MIN_CONNS": "1", "GOMAXPROCS": "2",
    "NEXT_TELEMETRY_DISABLED": "1", "VITE_API_URL": f"http://localhost:{api_port}", "VITE_WS_URL": f"ws://localhost:{api_port}/ws", "COLLABORATION_E2E_API_URL": f"http://localhost:{api_port}", "COLLABORATION_ELECTRON_WS_URL": f"ws://localhost:{api_port}/ws", "COLLABORATION_ELECTRON_RENDERER_URL": f"http://127.0.0.1:{renderer_port}", "MULTICA_DEV_VERIFICATION_CODE": "888888",
})
(STATE / "uploads").mkdir(mode=0o700)
(STATE / "env.json").write_text(json.dumps(env))
os.chmod(STATE / "env.json", 0o600)
for _ in range(100):
    if subprocess.run(["docker", "exec", container, "pg_isready", "-U", "multica"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
        break
    time.sleep(0.2)

print("Migrating task database", flush=True)
run(["go", "run", "./cmd/migrate", "up"], ROOT / "server", env, "migrate")
commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
print("Building task API", flush=True)
run(["go", "build", "-ldflags", f"-X main.commit={commit}", "-o", str(STATE / "api"), "./cmd/server"], ROOT / "server", env, "api-build")
processes = {}


def launch(name, args, cwd):
    with (OUT / f"{name}.log").open("a") as log:
        process = subprocess.Popen(args, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, start_new_session=True)
    processes[name] = {"pid": process.pid, "command": args, "cwd": str(cwd)}
    (STATE / "processes.json").write_text(json.dumps(processes, indent=2))


launch("api", [str(STATE / "api")], ROOT / "server")
health = wait_ready(f"http://localhost:{api_port}/health")
stage = STATE / "candidate"
(stage / "apps").mkdir(parents=True)
shutil.copytree(ROOT / "apps/web", stage / "apps/web", ignore=shutil.ignore_patterns("node_modules", ".next", ".turbo", ".git"))
for dest, source in [("apps/web/node_modules", "apps/web/node_modules"), ("apps/docs", "apps/docs"), ("packages", "packages"), ("node_modules", "node_modules")]:
    (stage / dest).symlink_to(ROOT / source, target_is_directory=True)
for file in ["package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml"]:
    shutil.copy2(ROOT / file, stage / file)
desktop = stage / "apps/desktop"
shutil.copytree(ROOT / "apps/desktop", desktop, ignore=shutil.ignore_patterns("node_modules", "out", ".turbo", ".git", ".env*", "release*", "dist"))
(desktop / "node_modules").symlink_to(ROOT / "apps/desktop/node_modules", target_is_directory=True)
print("Building isolated production Desktop renderer", flush=True)
run(["pnpm", "exec", "electron-vite", "build"], desktop, env, "desktop-build")
launch("renderer", ["python3", "-m", "http.server", str(renderer_port), "--bind", "127.0.0.1", "--directory", str(desktop / "out/renderer")], desktop)
wait_ready(f"http://127.0.0.1:{renderer_port}")
print("Building isolated production Web", flush=True)
run(["pnpm", "exec", "next", "build", "--webpack"], stage / "apps/web", env, "web-build")
launch("web", ["pnpm", "exec", "next", "start", "--port", str(web_port)], stage / "apps/web")
wait_ready(f"http://localhost:{web_port}/login")
sources = list((ROOT / "packages/views/squads/components").glob("squad-welcome*")) + [
    ROOT / "packages/views/squads/components/squads-page.tsx",
    ROOT / "packages/views/locales/en/squads.json", ROOT / "packages/views/locales/zh-Hans/squads.json",
    ROOT / "packages/core/docs/slugs.ts", ROOT / "server/internal/docs/content/pages/squads.json",
    ROOT / "e2e/squad-welcome.spec.ts",
]
(OUT / "environment.json").write_text(json.dumps({
    "task": ".trellis/tasks/11-20-squad-welcome", "commit": commit,
    "sourceRoot": str(ROOT), "candidate": str(stage),
    "web": env["PLAYWRIGHT_BASE_URL"], "api": env["NEXT_PUBLIC_API_URL"],
    "renderer": f"http://127.0.0.1:{renderer_port}", "desktopRoot": str(stage),
    "databaseContainer": container, "processes": processes, "health": health,
    "buildId": (stage / "apps/web/.next/BUILD_ID").read_text().strip(),
    "sources": {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest() for path in sources},
}, indent=2) + "\n")
print("READY " + env["PLAYWRIGHT_BASE_URL"], flush=True)
