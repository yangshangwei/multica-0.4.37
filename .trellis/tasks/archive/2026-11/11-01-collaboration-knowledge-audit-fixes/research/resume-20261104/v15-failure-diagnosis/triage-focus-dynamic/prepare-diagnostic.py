"""Create an isolated frozen checkout, COW inputs and one fresh triage DB."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import shutil
import socket
import stat
import subprocess
import tempfile
import uuid
from urllib.parse import urlsplit, urlunsplit, unquote


ORIGINAL = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PROJECT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
PRIVATE = ORIGINAL / '.omx/private/triage-focus-diagnostic-v15'
RUNTIME = PRIVATE / 'runtime'
HERE = Path(__file__).resolve().parent
EXPECTED = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
DATABASE = 'multica_triage_focus_v15_01a1a3c9'
SEED = 'multica_closure_v15_seed_01a1a3c9'
NAME = 'triage-focus-v15-01a1a3c9'
PSQL = '/opt/homebrew/opt/libpq/bin/psql'


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        json.dump(value, output, indent=2)
        output.write('\n')


def checked(command, **kwargs):
    return subprocess.run(command, capture_output=True, text=True, check=True, **kwargs)


def manifest(directory):
    rows = []
    for path in sorted(directory.rglob('*')):
        if path.is_symlink():
            rows.append({'path': str(path.relative_to(directory)), 'link': os.readlink(path), 'mode': stat.S_IMODE(path.lstat().st_mode)})
        elif path.is_file():
            rows.append({'path': str(path.relative_to(directory)), 'sha256': digest(path), 'mode': stat.S_IMODE(path.stat().st_mode), 'bytes': path.stat().st_size})
    return rows


assert checked(['git', 'rev-parse', 'HEAD'], cwd=ORIGINAL).stdout.strip() == EXPECTED
diagnosis = HERE.parent / 'triage-duplicate-picker/report.json'
assert digest(diagnosis) == '4a515ee9fe899d4c487f2264ad4fd554bfadbcf455c9c387d5229291daea70de'
handoff = PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261104/v15-execution/completion-handoff.json'
baseline = json.loads(handoff.read_text())
assert baseline['make_exit'] == 2 and baseline['registered_final_audit_exit'] == 0
for reference in baseline['references'].values():
    assert digest(reference['path']) == reference['sha256']
assert checked(['git', 'check-ignore', '.omx/private/triage-focus-diagnostic-v15/runtime'], cwd=ORIGINAL).returncode == 0
for port in (18974, 13894):
    assert subprocess.run(['/usr/sbin/lsof', '-nP', '-iTCP:' + str(port), '-sTCP:LISTEN'], capture_output=True).returncode != 0
assert not PRIVATE.exists()
PRIVATE.mkdir(mode=0o700)
save(HERE / 'v15-history-before.json', {'recorded_at': datetime.now(timezone.utc).isoformat(), 'references': baseline['references'],
     'original_status': checked(['git', 'status', '--porcelain'], cwd=ORIGINAL).stdout,
     'web_build_id': (ORIGINAL / 'apps/web/.next/BUILD_ID').read_text().strip(),
     'original_web_manifest': manifest(ORIGINAL / 'apps/web/.next'),
     'original_desktop_manifest': manifest(ORIGINAL / 'apps/desktop/out')})
result = checked(['git', 'worktree', 'add', '--detach', str(RUNTIME), EXPECTED], cwd=ORIGINAL)
assert checked(['git', 'rev-parse', 'HEAD'], cwd=RUNTIME).stdout.strip() == EXPECTED
copy_records = []
sources = [(ORIGINAL / 'node_modules', RUNTIME / 'node_modules')]
for group in ('apps', 'packages'):
    for source in sorted((ORIGINAL / group).glob('*/node_modules')):
        sources.append((source, RUNTIME / source.relative_to(ORIGINAL)))
sources.append((ORIGINAL / '.omx/private/final-check-v15/go-build-cache', PRIVATE / 'go-cache'))
for source, destination in sources:
    assert source.is_dir() and not source.is_symlink()
    before = manifest(source)
    destination.parent.mkdir(parents=True, exist_ok=True)
    checked(['/bin/cp', '-cRp', str(source), str(destination)])
    after = manifest(destination)
    assert before == after, 'COW input bytes/modes/links differ'
    link_rebindings = []
    for path in destination.rglob('*'):
        if path.is_symlink():
            old = os.readlink(path)
            if old.startswith(str(ORIGINAL) + '/'):
                new = old.replace(str(ORIGINAL), str(RUNTIME), 1)
                path.unlink()
                path.symlink_to(new, target_is_directory=Path(new).is_dir())
                link_rebindings.append({'path': str(path.relative_to(destination)), 'before': old, 'after': new})
    copy_records.append({'source': str(source), 'destination': str(destination), 'method': 'APFS cp -cRp',
                         'before': before, 'copied_before_rebinding': after, 'link_rebindings': link_rebindings})
    print(json.dumps({'copied': str(source.relative_to(ORIGINAL)), 'entries': len(before), 'rebinding_count': len(link_rebindings)}), flush=True)
save(PRIVATE / 'copy-manifests.json', copy_records)
cli = ORIGINAL / '.omx/private/final-check-v15/multica'
(RUNTIME / 'server/bin').mkdir(exist_ok=True)
checked(['/bin/cp', '-cp', str(cli), str(RUNTIME / 'server/bin/multica')])
assert digest(RUNTIME / 'server/bin/multica') == digest(cli)
for relative in ('apps/web', 'packages/core', 'packages/ui', 'packages/views'):
    assert (RUNTIME / relative / 'node_modules/@multica/core').resolve() == RUNTIME / 'packages/core' or relative in ('packages/core', 'packages/ui')
temporary = Path(tempfile.mkdtemp(prefix='multica-triage-focus-v15-', dir='/private/tmp'))
temporary.chmod(0o700)
assert subprocess.run(['git', '-C', str(temporary), 'rev-parse', '--show-toplevel'], cwd=RUNTIME, capture_output=True).returncode != 0
for name in ('home', 'go-path', 'node-cache', 'registry', 'profiles', 'workspaces', 'catalog'):
    (PRIVATE / name).mkdir(mode=0o700)
for name in ('mcp', 'skills', 'published'):
    (PRIVATE / 'catalog' / name).mkdir(mode=0o700)
offset = None
for candidate in range(951, 1000):
    ports = (18080 + candidate, 13000 + candidate, 5174 + candidate)
    sockets = []
    try:
        for port in ports:
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
source_values = {}
for line in (ORIGINAL / '.omx/private/final-check-v15/check-source-v15.env').read_text().splitlines():
    if line and not line.startswith('#'):
        key, raw = line.split('=', 1)
        parts = shlex.split(raw)
        source_values[key] = parts[0] if parts else ''
url = urlsplit(source_values['DATABASE_URL'])
values = {key: value for key, value in source_values.items() if not key.startswith(('COLLABORATION_', 'CHANGELOG_', 'REDIS'))}
values.update(DATABASE_URL=urlunsplit((url.scheme, url.netloc, '/' + DATABASE, url.query, '')), POSTGRES_DB=DATABASE,
              JWT_SECRET=secrets.token_hex(32), MULTICA_DEPLOYMENT_ID=str(uuid.uuid4()), PORT=str(18080 + offset),
              BACKEND_PORT=str(18080 + offset), FRONTEND_PORT=str(13000 + offset),
              MULTICA_MCP_TEMPLATE_DIR=str(PRIVATE / 'catalog/mcp'), E2E_MCP_TEMPLATE_DIR=str(PRIVATE / 'catalog/mcp'),
              MULTICA_SKILL_TEMPLATE_DIR=str(PRIVATE / 'catalog/skills'), MULTICA_RESOURCE_PUBLISH_DIR=str(PRIVATE / 'catalog/published'),
              MULTICA_DSH_PATH=str(PRIVATE / 'missing-test-owned-dsh'), NEXT_PUBLIC_API_URL='http://localhost:' + str(18080 + offset),
              NEXT_PUBLIC_WS_URL='ws://localhost:' + str(18080 + offset) + '/ws', REMOTE_API_URL='http://localhost:' + str(18080 + offset),
              FRONTEND_ORIGIN='http://localhost:' + str(13000 + offset), PLAYWRIGHT_BASE_URL='http://localhost:' + str(13000 + offset),
              MULTICA_PUBLIC_URL='http://localhost:' + str(18080 + offset), MULTICA_APP_URL='http://localhost:' + str(13000 + offset),
              CORS_ALLOWED_ORIGINS='http://localhost:' + str(13000 + offset), ALLOWED_ORIGINS='http://localhost:' + str(13000 + offset),
              RATE_LIMIT_AUTH='1000', RATE_LIMIT_AUTH_VERIFY='1000', DATABASE_MAX_CONNS='6', DATABASE_MIN_CONNS='0')
assert all('$' not in value and all(char.isprintable() for char in value) for value in values.values())
env_path = PRIVATE / 'source.env'
fd = os.open(env_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as output:
    output.write(''.join(key + '=' + shlex.quote(value) + '\n' for key, value in values.items()))
environment = json.loads((PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261104/v15-preparation/candidate-setup/cli-build-v15.json').read_text())['environment']
environment.update(HOME=str(PRIVATE / 'home'), GOCACHE=str(PRIVATE / 'go-cache'), GOPATH=str(PRIVATE / 'go-path'),
                   TMPDIR=str(temporary), NPM_CONFIG_CACHE=str(PRIVATE / 'node-cache'), ENV_FILE=str(env_path),
                   MULTICA_DEV_HOME=str(PRIVATE / 'registry'), MULTICA_DEV_TMPDIR=str(temporary),
                   MULTICA_DEV_PROFILES_HOME=str(PRIVATE / 'profiles'), MULTICA_DEV_WORKSPACES_PARENT=str(PRIVATE / 'workspaces'))
bin_path = PRIVATE / 'bin'
bin_path.mkdir(mode=0o700)
pnpm = ORIGINAL / '.omx/private/owned-workspace-links/pnpm-10.28.2/bin/pnpm.cjs'
(bin_path / 'pnpm').symlink_to(pnpm)
environment['PATH'] = str(bin_path) + ':' + environment['PATH']
pg_env = {'PATH': '/usr/bin:/bin', 'HOME': str(PRIVATE / 'home'), 'PGHOST': url.hostname, 'PGPORT': str(url.port),
          'PGUSER': unquote(url.username), 'PGPASSWORD': unquote(url.password), 'PGDATABASE': 'postgres',
          'PGSSLMODE': 'disable', 'PGCONNECT_TIMEOUT': '5', 'PGOPTIONS': '-c statement_timeout=30000'}
sql = [PSQL, '-X', '-v', 'ON_ERROR_STOP=1', '-At', '-c']
assert not checked(sql + ["SELECT 1 FROM pg_database WHERE datname='" + DATABASE + "'"], env=pg_env).stdout.strip()
seed = json.loads(checked(sql + ["SELECT json_build_object('database',current_database(),'latest',(SELECT max(version) FROM schema_migrations),'workspaces',(SELECT count(*) FROM workspace),'issues',(SELECT count(*) FROM issue),'connections',(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()))"], env={**pg_env, 'PGDATABASE': SEED}).stdout)
assert seed['database'] == SEED and seed['latest'] == '764_collaboration_ai_policy' and seed['workspaces'] == seed['issues'] == 0
checked(sql + ['CREATE DATABASE "' + DATABASE + '" WITH TEMPLATE "' + SEED + '" OWNER "multica"'], env=pg_env)
assert checked(sql + ['SELECT current_database()'], env={**pg_env, 'PGDATABASE': DATABASE}).stdout.strip() == DATABASE
save(PRIVATE / 'startup-environment.json', environment)
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'owner': OWNER, 'executor': '/root/shared_executor',
          'candidate': EXPECTED, 'diagnostic_checkout': str(RUNTIME), 'private': str(PRIVATE), 'database': DATABASE,
          'clean_seed_witness': seed, 'db_created': True, 'env_path': str(env_path), 'env_sha256': digest(env_path),
          'startup_environment_path': str(PRIVATE / 'startup-environment.json'), 'temporary': str(temporary),
          'offset': offset, 'api_port': 18080 + offset, 'web_port': 13000 + offset, 'reserved_unused_renderer_port': 5174 + offset,
          'name': NAME, 'deployment_id': values['MULTICA_DEPLOYMENT_ID'], 'CLI_sha256': digest(cli),
          'copy_manifest_path': str(PRIVATE / 'copy-manifests.json'), 'copy_manifest_sha256': digest(PRIVATE / 'copy-manifests.json'),
          'original_source_untouched': True, 'real_provider_allowed': False, 'knowledge_budget': 0, 'knowledge_calls': 0}
save(HERE / 'preparation.json', record)
print(json.dumps({key: record[key] for key in ('candidate', 'diagnostic_checkout', 'database', 'api_port', 'web_port', 'deployment_id', 'real_provider_allowed')}), flush=True)
