"""Capture actual task seed/Redis/renderer identity before the sole full entry."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import types
import urllib.request
from urllib.parse import urlsplit, unquote


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v15'
HERE = Path(__file__).resolve().parent
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
EXPECTED = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def checked(args, **kwargs):
    return subprocess.run(args, capture_output=True, text=True, check=True, **kwargs)


assert checked(['git', 'rev-parse', 'HEAD'], cwd=ROOT).stdout.strip() == EXPECTED
assert not checked(['git', 'status', '--porcelain'], cwd=ROOT).stdout.strip()
launcher = types.ModuleType('v15_resource_data_parser')
launcher.__file__ = str(OUT / 'run-make-check-v15-provenance.py')
exec(compile(Path(launcher.__file__).read_bytes(), launcher.__file__, 'exec'), launcher.__dict__)
values = launcher.parse_generated_environment((OUT / 'check-source-v15.env').read_text())
assert launcher.deployment_uuid(values) == '725f8e4c-8979-4fc2-b105-36a6ce89a2e5'
allocation = json.loads((HERE / 'resource-allocation-redis.json').read_text())
migration = json.loads((HERE / 'seed-migration-result.json').read_text())
ready = json.loads((HERE / 'renderer-ready.json').read_text())
assert allocation['candidate_sha'] == migration['candidate_sha'] == ready['candidate_sha'] == EXPECTED
assert migration['build_exit'] == migration['migration_exit'] == 0 and not migration['candidate_status_after']
assert ready['state'] == 'serving' and ready['ownership_verified'] is True
process = checked(['ps', '-p', str(ready['pid']), '-o', 'pid=,ppid=,pgid=,lstart=,command=']).stdout.strip()
assert str(ready['pid']) in process and str(ROOT / 'apps/desktop/out/renderer') in process
assert os.getpgid(ready['pid']) == ready['process_group']
cwd = checked(['/usr/sbin/lsof', '-a', '-p', str(ready['pid']), '-d', 'cwd', '-Fn']).stdout
assert '\nn' + str(ROOT) + '\n' in '\n' + cwd
with urllib.request.urlopen(ready['url'], timeout=3) as response:
    renderer_hash = hashlib.sha256(response.read()).hexdigest()
assert renderer_hash == ready['index_sha256'] == digest(ROOT / 'apps/desktop/out/renderer/index.html')
image = json.loads(checked(['/usr/local/bin/docker', 'inspect', allocation['redis_container_id']]).stdout)[0]
assert image['State']['Running'] and image['Image'] == allocation['redis_image_id']
assert image['Config']['Labels']['multica.task.owner'] == OWNER
assert image['Config']['Labels']['multica.candidate'] == EXPECTED
assert image['NetworkSettings']['Ports']['6379/tcp'] == [{'HostIp': '127.0.0.1', 'HostPort': str(allocation['redis_port'])}]
with socket.create_connection(('127.0.0.1', allocation['redis_port']), timeout=3) as sock:
    sock.sendall(b'*1\r\n$4\r\nPING\r\n')
    assert sock.recv(128) == b'+PONG\r\n'
url = urlsplit(values['DATABASE_URL'])
assert url.path == '/' + allocation['database'] == '/multica_closure_v15_seed_01a1a3c9'
pg_env = {'PATH': '/usr/bin:/bin', 'HOME': str(OUT / 'metadata-home-not-created'),
          'PGHOST': url.hostname, 'PGPORT': str(url.port), 'PGUSER': unquote(url.username),
          'PGPASSWORD': unquote(url.password), 'PGDATABASE': allocation['database'],
          'PGSSLMODE': 'disable', 'PGCONNECT_TIMEOUT': '5',
          'PGOPTIONS': '-c default_transaction_read_only=on -c statement_timeout=30000'}
query = "SELECT json_build_object('database',current_database(),'server_addr',inet_server_addr(),'server_port',inet_server_port(),'latest_migration',(SELECT max(version) FROM schema_migrations),'migration_count',(SELECT count(*) FROM schema_migrations),'max_connections',current_setting('max_connections'),'connections',(SELECT count(*) FROM pg_stat_activity))"
witness = json.loads(checked(['/opt/homebrew/opt/libpq/bin/psql', '-X', '-v', 'ON_ERROR_STOP=1', '-At', '-c', query], env=pg_env).stdout)
assert witness['database'] == allocation['database']
assert witness['latest_migration'] == '764_collaboration_ai_policy'
assert witness['connections'] + 10 < int(witness['max_connections'])
raw = checked(['ps', '-axo', 'pid=,ppid=,pgid=,command=']).stdout
checks = []
for line in raw.splitlines():
    fields = line.strip().split(None, 3)
    if len(fields) != 4:
        continue
    command = fields[3]
    if re.search(r'(^|/)(make|gmake)\s+.*\bcheck\b|(^|/)go\s+test\b|rk1k[^\s]*\.test|run-make-check-v\d+-provenance\.py$', command):
        checks.append(line.strip())
assert not checks, 'Another full/Go/RK1K execution is live'
fresh = [OUT / name for name in ('full-check-home-v15', 'go-packages-v15', 'dev-registry-v15',
                                 'profiles-v15', 'workspaces-v15', 'metadata-home-not-created',
                                 'make-check-v15.json', 'make-check-v15.log', 'check-source-v15.safe.env')]
assert all(not path.exists() and not path.is_symlink() for path in fresh)
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'coordinator': OWNER,
          'executor': '/root/shared_executor', 'candidate_root': str(ROOT), 'candidate_sha': EXPECTED,
          'exclusive_window_verified': True, 'active_complete_or_rk1k_checks': checks,
          'future_execution_paths_absent': [str(path) for path in fresh],
          'source_env_sha256': digest(OUT / 'check-source-v15.env'),
          'owned_bindings_sha256': digest(OUT / 'owned-bindings-v15-provenance.json'),
          'renderer': {'ownership_verified': True, 'url': ready['url'], 'pid': ready['pid'],
                       'process_group': ready['process_group'], 'index_sha256': renderer_hash,
                       'process': process, 'cwd': cwd, 'ready_record_sha256': digest(HERE / 'renderer-ready.json')},
          'redis': {'ownership_verified': True, 'port': allocation['redis_port'],
                    'container_id': allocation['redis_container_id'], 'image_id': image['Image'],
                    'labels': image['Config']['Labels'], 'port_binding': image['NetworkSettings']['Ports']['6379/tcp'],
                    'ping': 'PONG'},
          'seed_database': {'ownership_verified': True, 'name': allocation['database'], 'witness': witness,
                            'allocation_record_sha256': digest(HERE / 'resource-allocation-redis.json'),
                            'migration_record_sha256': digest(HERE / 'seed-migration-result.json')},
          'retired_runtime_release_sha256': digest(HERE / 'retired-runtime-release-after.json'),
          'knowledge_budget_allocated': 0, 'knowledge_calls': 0, 'real_provider_allowed': False,
          'knowledge_default': False, 'complete_check_started': False}
descriptor = os.open(OUT / 'resource-ownership-v15.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(descriptor, 'w') as target:
    json.dump(record, target, indent=2)
    target.write('\n')
print(json.dumps({'candidate_sha': EXPECTED, 'exclusive_window_verified': True,
                  'seed_database': witness['database'], 'latest_migration': witness['latest_migration'],
                  'connections': witness['connections'], 'renderer_pid': ready['pid'],
                  'renderer_port': allocation['renderer_port'], 'redis_port': allocation['redis_port'],
                  'real_provider_allowed': False, 'complete_check_started': False}))
