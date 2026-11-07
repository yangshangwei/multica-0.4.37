from pathlib import Path
from datetime import datetime, timezone
import ast
import hashlib
import json
import os
import secrets
import shlex
import socket
import subprocess
import tempfile
from urllib.parse import urlsplit, urlunsplit

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v14'
OWNER = '01a1a186-9a5c-7b00-96f8-0430b47edb4c'
EXPECTED = 'd0b132fff0c477eb2d744933e13db730240231aa'
PSQL = '/opt/homebrew/opt/libpq/bin/psql'

def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

def save(path, value):
    with path.open('x') as target:
        json.dump(value, target, indent=2)
        target.write('\n')
    path.chmod(0o600)

def command(args, **kwargs):
    return subprocess.run(args, capture_output=True, text=True, check=True, **kwargs)

def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]

assert command(['git', 'rev-parse', 'HEAD'], cwd=ROOT).stdout.strip() == EXPECTED
assert not command(['git', 'status', '--porcelain'], cwd=ROOT).stdout.strip()
freeze = json.loads((HERE.parent / 'candidate-freeze.json').read_text())
assert freeze['sha'] == EXPECTED and freeze['clean'] and freeze['owner'] == OWNER
assert digest(freeze['review_path']) == freeze['review_sha256']
assert freeze['directed_pass'] == 17 and freeze['skip'] == freeze['retry'] == 0
claim = json.loads((ROOT / '.omx/private/knowledge-resume-coordinator-claim.json').read_text())
assert claim['owner'] == OWNER
for name in ('make-check-v14.json', 'full-check-home-v14', 'dev-registry-v14', 'go-packages-v14', 'profiles-v14', 'workspaces-v14'):
    assert not (OUT / name).exists(), 'Unreviewed full-run state exists: ' + name

manifest = json.loads((HERE / 'prepared-template-manifest.json').read_text())
for row in manifest['files']:
    assert digest(row['prepared_template']) == row['prepared_sha256']
    assert digest(row['original_template']) == row['original_sha256']
    if row['materialize']:
        assert not Path(row['final_path']).exists()

allocation_path = OUT / 'build-temp-allocation-v14.json'
old_allocation = allocation_path.read_bytes()
with (HERE / 'retained-before-build-temp-allocation-v14.json').open('xb') as target:
    target.write(old_allocation)
temp = Path(tempfile.mkdtemp(prefix='multica-final-check-v14-', dir='/private/tmp'))
temp.chmod(0o700)
assert subprocess.run(['git', '-C', str(temp), 'rev-parse', '--show-toplevel'], capture_output=True).returncode != 0
allocation = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'directory': str(temp), 'mode': '0700', 'git_root_absent': True,
              'scope': 'v14 private build/fullcheck temp; no complete run yet', 'allocated_by': OWNER,
              'previous_allocation_retained': str(HERE / 'retained-before-build-temp-allocation-v14.json')}
allocation_path.write_text(json.dumps(allocation, indent=2) + '\n')
allocation_path.chmod(0o600)
for phase in ('cli', 'desktop'):
    (temp / (phase + '-build')).mkdir(mode=0o700)

redis_port = free_port()
renderer_port = free_port()
assert redis_port != renderer_port
replacements = {'PENDING_ROOT_APPROVED_COMMIT_SHA': EXPECTED, 'PENDING_ROOT_RENDERER_PORT': str(renderer_port),
                'PENDING_ROOT_REDIS_PORT': str(redis_port), 'PENDING_ROOT_PRIVATE_JWT_SECRET': secrets.token_hex(32)}
outputs = []
for row in manifest['files']:
    if not row['materialize']:
        continue
    text = Path(row['prepared_template']).read_text()
    for old, value in replacements.items():
        text = text.replace(old, value)
    assert 'PENDING_ROOT_' not in text
    path = Path(row['final_path'])
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if path.suffix == '.py':
        ast.parse(text)
    with path.open('x') as target:
        target.write(text)
    path.chmod(0o600)
    outputs.append({'path': str(path), 'sha256': digest(path), 'template_sha256': row['prepared_sha256']})

environment = {}
for line in (OUT / 'check-source-v14.env').read_text().splitlines():
    if line and not line.startswith('#'):
        key, raw = line.split('=', 1)
        value = shlex.split(raw)
        environment[key] = value[0] if value else ''
database = 'multica_closure_v14_seed_01a1a186'
url = urlsplit(environment['DATABASE_URL'])
assert url.path == '/' + database and environment['POSTGRES_DB'] == database
admin = urlunsplit(url._replace(path='/postgres'))
sql_environment = {'PATH': '/usr/bin:/bin', 'HOME': str(OUT / 'metadata-home-not-created'),
                   'PGDATABASE': admin, 'PGCONNECT_TIMEOUT': '5', 'PGOPTIONS': '-c statement_timeout=30000'}
sql_command = [PSQL, '-X', '-v', 'ON_ERROR_STOP=1', '-At', '-c']
server_identity = command(sql_command + ["SELECT json_build_object('database',current_database(),'server_addr',inet_server_addr(),'server_port',inet_server_port(),'server_version',version(),'max_connections',current_setting('max_connections'),'connections',(SELECT count(*) FROM pg_stat_activity))"], env=sql_environment).stdout.strip()
assert not command(sql_command + ["SELECT 1 FROM pg_database WHERE datname='" + database + "'"], env=sql_environment).stdout.strip(), 'Seed DB already exists; preserve it'
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'coordinator': OWNER, 'candidate_sha': EXPECTED, 'candidate_root': str(ROOT),
          'source_env_path': str(OUT / 'check-source-v14.env'), 'source_env_sha256': digest(OUT / 'check-source-v14.env'),
          'database': database, 'postgres_endpoint': {'host': url.hostname, 'port': url.port}, 'database_state': 'allocation_ready_not_created',
          'postgres_identity': json.loads(server_identity), 'redis_port': redis_port, 'renderer_port': renderer_port,
          'renderer_url': f'http://127.0.0.1:{renderer_port}/', 'renderer_started': False,
          'redis_name': 'multica-v14-01a1a186-redis', 'redis_image': 'redis:7-alpine', 'redis_started': False,
          'future_registry': str(OUT / 'dev-registry-v14'), 'registry_created': False,
          'allocation': allocation, 'materialized_files': outputs, 'script_sha256': digest(__file__),
          'real_provider_allowed': False, 'knowledge_default': False}
save(HERE / 'resource-allocation-before.json', record)
created = command(sql_command + ['CREATE DATABASE "' + database + '" WITH TEMPLATE template0 OWNER "multica"'], env=sql_environment)
task_sql_env = {**sql_environment, 'PGDATABASE': environment['DATABASE_URL']}
actual = command(sql_command + ['SELECT current_database()'], env=task_sql_env).stdout.strip()
assert actual == database
record.update(database_state='created_empty', database_create_exit=created.returncode, actual_database=actual)
save(HERE / 'resource-allocation-created.json', record)
print(json.dumps({'materialized_files': len(outputs), 'candidate_sha': EXPECTED, 'database': actual,
                  'redis_port': redis_port, 'renderer_port': renderer_port, 'temp': str(temp), 'no_fullcheck': True}))
