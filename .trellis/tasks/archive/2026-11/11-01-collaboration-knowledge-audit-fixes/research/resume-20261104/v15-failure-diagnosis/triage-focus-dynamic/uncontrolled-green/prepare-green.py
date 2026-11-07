"""Allocate a fresh no-instrumentation window and copy frozen native artifacts."""
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import secrets
import shlex
import shutil
import socket
import subprocess
import tempfile
import uuid
from urllib.parse import urlsplit, urlunsplit, unquote


HERE = Path(__file__).resolve().parent
ORIGINAL = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
BASE = ORIGINAL / '.omx/private/triage-focus-diagnostic-v15'
RUNTIME = BASE / 'runtime'
PRIVATE = BASE / 'uncontrolled-green'
DB = 'multica_triage_green_v15_01a1a3c9'
NAME = 'triage-green-v15-01a1a3c9'


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def save(path, data):
    path.write_text(json.dumps(data, indent=2) + '\n')
    path.chmod(0o600)


assert not PRIVATE.exists()
PRIVATE.mkdir(mode=0o700)
for name in ['home', 'node-cache', 'go-path', 'registry', 'profiles', 'workspaces', 'catalog']:
    (PRIVATE / name).mkdir(mode=0o700)
for name in ['mcp', 'skills', 'published']:
    (PRIVATE / 'catalog' / name).mkdir(mode=0o700)
subprocess.run(['/bin/cp', '-cRp', str(BASE / 'go-cache'), str(PRIVATE / 'go-cache')], check=True)
desktop = PRIVATE / 'desktop-out'
subprocess.run(['/bin/cp', '-cRp', str(ORIGINAL / 'apps/desktop/out'), str(desktop)], check=True)
native = RUNTIME / 'apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'
assert native.is_file() and native.stat().st_mode & 0o111
rows = []
for path in sorted(desktop.rglob('*')):
    if path.is_file():
        original = ORIGINAL / 'apps/desktop/out' / path.relative_to(desktop)
        assert digest(path) == digest(original)
        rows.append({'path': str(path.relative_to(desktop)), 'sha256': digest(path), 'mode': path.stat().st_mode & 0o777})
save(PRIVATE / 'desktop-artifact-manifest.json', rows)
temporary = Path(tempfile.mkdtemp(prefix='multica-triage-green-v15-', dir='/private/tmp'))
temporary.chmod(0o700)
values = {}
for line in (BASE / 'source.env').read_text().splitlines():
    if line and not line.startswith('#'):
        key, raw = line.split('=', 1)
        tokens = shlex.split(raw)
        values[key] = tokens[0] if tokens else ''
offset = None
for candidate in range(926, 950):
    sockets = []
    try:
        for port in [18080 + candidate, 13000 + candidate, 5174 + candidate]:
            sock = socket.socket()
            sockets.append(sock)
            sock.bind(('127.0.0.1', port))
        offset = candidate
        break
    except OSError:
        pass
    finally:
        for sock in sockets:
            sock.close()
assert offset is not None
old = urlsplit(values['DATABASE_URL'])
api = 'http://localhost:' + str(18080 + offset)
web = 'http://localhost:' + str(13000 + offset)
values.update(DATABASE_URL=urlunsplit((old.scheme, old.netloc, '/' + DB, old.query, '')), POSTGRES_DB=DB,
              JWT_SECRET=secrets.token_hex(32), MULTICA_DEPLOYMENT_ID=str(uuid.uuid4()),
              PORT=str(18080 + offset), BACKEND_PORT=str(18080 + offset), FRONTEND_PORT=str(13000 + offset),
              NEXT_PUBLIC_API_URL=api, NEXT_PUBLIC_WS_URL=api.replace('http', 'ws') + '/ws', REMOTE_API_URL=api,
              FRONTEND_ORIGIN=web, PLAYWRIGHT_BASE_URL=web, MULTICA_PUBLIC_URL=api, MULTICA_APP_URL=web,
              CORS_ALLOWED_ORIGINS=web, ALLOWED_ORIGINS=web,
              MULTICA_MCP_TEMPLATE_DIR=str(PRIVATE / 'catalog/mcp'), E2E_MCP_TEMPLATE_DIR=str(PRIVATE / 'catalog/mcp'),
              MULTICA_SKILL_TEMPLATE_DIR=str(PRIVATE / 'catalog/skills'), MULTICA_RESOURCE_PUBLISH_DIR=str(PRIVATE / 'catalog/published'),
              MULTICA_DSH_PATH=str(PRIVATE / 'missing-test-owned-dsh'))
env_path = PRIVATE / 'source.env'
env_path.write_text(''.join(key + '=' + shlex.quote(value) + '\n' for key, value in values.items()))
env_path.chmod(0o600)
environment = json.loads((BASE / 'startup-environment-retry1.json').read_text())
environment.update(HOME=str(PRIVATE / 'home'), TMPDIR=str(temporary), GOCACHE=str(PRIVATE / 'go-cache'),
                   GOPATH=str(PRIVATE / 'go-path'), NPM_CONFIG_CACHE=str(PRIVATE / 'node-cache'), ENV_FILE=str(env_path),
                   MULTICA_DEV_HOME=str(PRIVATE / 'registry'), MULTICA_DEV_TMPDIR=str(temporary),
                   MULTICA_DEV_PROFILES_HOME=str(PRIVATE / 'profiles'), MULTICA_DEV_WORKSPACES_PARENT=str(PRIVATE / 'workspaces'))
save(PRIVATE / 'startup-environment.json', environment)
sql_env = {'PATH': environment['PATH'], 'HOME': environment['HOME'], 'PGHOST': old.hostname, 'PGPORT': str(old.port),
           'PGUSER': unquote(old.username), 'PGPASSWORD': unquote(old.password), 'PGDATABASE': 'postgres',
           'PGCONNECT_TIMEOUT': '3', 'PGOPTIONS': '-c statement_timeout=30000'}
prefix = ['/opt/homebrew/opt/libpq/bin/psql', '-X', '-At', '-v', 'ON_ERROR_STOP=1', '-c']
assert not subprocess.check_output(prefix + ["SELECT 1 FROM pg_database WHERE datname='" + DB + "'"], env=sql_env, text=True).strip()
subprocess.run(prefix + ['CREATE DATABASE "' + DB + '" WITH TEMPLATE "multica_closure_v15_seed_01a1a3c9" OWNER "multica"'], env=sql_env, capture_output=True, check=True)
observed = subprocess.check_output(prefix + ["SELECT json_build_object('database',current_database(),'latest',(SELECT max(version) FROM schema_migrations))"], env={**sql_env, 'PGDATABASE': DB}, text=True)
witness = json.loads(observed)
assert witness['database'] == DB and witness['latest'] == '764_collaboration_ai_policy'
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'private': str(PRIVATE), 'runtime': str(RUNTIME),
          'database': DB, 'name': NAME, 'api_port': 18080 + offset, 'web_port': 13000 + offset,
          'deployment_id': values['MULTICA_DEPLOYMENT_ID'], 'env_path': str(env_path), 'env_sha256': digest(env_path),
          'startup_environment_path': str(PRIVATE / 'startup-environment.json'), 'db_witness': witness,
          'native_executable': str(native), 'native_executable_sha256': digest(native),
          'desktop_output': str(desktop), 'desktop_files': len(rows), 'desktop_manifest_sha256': digest(PRIVATE / 'desktop-artifact-manifest.json'),
          'fixture_sha256': digest(RUNTIME / 'e2e/fixtures/triage-duplicate-picker.ts'),
          'model_keys_forwarded': False, 'knowledge_budget': 0, 'knowledge_calls': 0}
save(HERE / 'preparation.json', record)
print(json.dumps({key: record[key] for key in ['database', 'api_port', 'web_port', 'fixture_sha256', 'native_executable', 'desktop_files']}))
