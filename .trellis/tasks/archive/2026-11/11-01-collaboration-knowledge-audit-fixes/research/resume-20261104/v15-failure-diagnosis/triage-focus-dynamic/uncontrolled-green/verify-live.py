"""Read-only identity proof for the isolated diagnostic environment."""
from datetime import datetime, timezone
import hashlib
import html
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
from urllib.parse import urlsplit, unquote
from urllib.request import urlopen


HERE = Path(__file__).resolve().parent
prepared = json.loads((HERE / 'preparation.json').read_text())
PRIVATE = Path(prepared['private'])
RUNTIME = Path(prepared['runtime'])
REGISTRY = PRIVATE / 'registry/envs' / prepared['name']


def parsed(path):
    result = {}
    for line in path.read_text().splitlines():
        if line and not line.startswith('#'):
            key, raw = line.split('=', 1)
            tokens = shlex.split(raw)
            result[key] = tokens[0] if tokens else ''
    return result


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


manifest = parsed(REGISTRY / 'manifest.env')
values = parsed(PRIVATE / 'source.env')
assert manifest['DIR'] == str(RUNTIME) and manifest['DB_NAME'] == prepared['database']
assert manifest['DATABASE_URL'] == values['DATABASE_URL']
api = 'http://localhost:' + manifest['BACKEND_PORT']
web = 'http://localhost:' + manifest['FRONTEND_PORT']
with urlopen(api + '/health', timeout=5) as response:
    health = json.loads(response.read())
with urlopen(api + '/api/config', timeout=5) as response:
    config = json.loads(response.read())
assert health['commit'] == 'd41573e3a' and config['deployment_id'] == prepared['deployment_id']
assert config['auth_mode'] == 'legacy'
pid = health['pid']
process = subprocess.check_output(['ps', '-p', str(pid), '-o', 'pid=,ppid=,pgid=,command='], text=True)
cwd = subprocess.check_output(['/usr/sbin/lsof', '-a', '-p', str(pid), '-d', 'cwd', '-Fn'], text=True)
assert str(RUNTIME / 'server') in cwd
raw_env = subprocess.check_output(['ps', 'eww', '-p', str(pid), '-o', 'command='], text=True)
actual_values = {}
for key in ['MULTICA_AUTH_MODE', 'MULTICA_DEPLOYMENT_ID', 'MULTICA_DEVICE_AUTH_ENABLED',
            'MULTICA_KNOWLEDGE_CONTEXT_ENABLED', 'MULTICA_MANAGED_INSTALLATIONS_ENABLED',
            'MULTICA_PLATFORM_ADMIN_ENABLED', 'DATABASE_MAX_CONNS', 'DATABASE_MIN_CONNS']:
    match = re.search(r'(?:^|\s)' + key + r'=([^\s]*)', raw_env)
    actual_values[key] = match.group(1) if match else None
    assert actual_values[key] == values[key]
match = re.search(r'(?:^|\s)DATABASE_URL=([^\s]*)', raw_env)
assert match and match.group(1) == values['DATABASE_URL']
url = urlsplit(values['DATABASE_URL'])
pg_env = {'PATH': '/usr/bin:/bin', 'HOME': str(PRIVATE / 'home'), 'PGHOST': url.hostname,
          'PGPORT': str(url.port), 'PGUSER': unquote(url.username), 'PGPASSWORD': unquote(url.password),
          'PGDATABASE': prepared['database'], 'PGCONNECT_TIMEOUT': '3',
          'PGOPTIONS': '-c default_transaction_read_only=on -c statement_timeout=3000'}
query = "SELECT json_build_object('database',current_database(),'latest',(SELECT max(version) FROM schema_migrations),'cluster_connections',(SELECT count(*) FROM pg_stat_activity),'max_connections',current_setting('max_connections'))"
db = json.loads(subprocess.check_output(['/opt/homebrew/opt/libpq/bin/psql', '-X', '-At', '-v', 'ON_ERROR_STOP=1', '-c', query], env=pg_env, text=True))
assert db['database'] == prepared['database'] and db['latest'] == '764_collaboration_ai_policy'
with urlopen(web + '/login', timeout=10) as response:
    body = response.read()
text = html.unescape(body.decode()).replace('\\"', '"')
bootstrap = {key: value for key, value in re.findall(r'"(apiBaseUrl|wsUrl)"\s*:\s*"([^\"]+)"', text)}
assert bootstrap['apiBaseUrl'] == values['NEXT_PUBLIC_API_URL'] and bootstrap['wsUrl'] == values['NEXT_PUBLIC_WS_URL']
api_provenance = json.loads((REGISTRY / 'api.running.json').read_text())
web_provenance = json.loads((REGISTRY / 'web.running.json').read_text())
assert web_provenance['mode'] == 'production' and api_provenance['source_id'] == web_provenance['source_id']
assert web_provenance['build_id'] == (RUNTIME / 'apps/web/.next/BUILD_ID').read_text().strip()
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'diagnostic_checkout': str(RUNTIME),
          'manifest_sha256': digest(REGISTRY / 'manifest.env'), 'source_env_sha256': digest(PRIVATE / 'source.env'),
          'api': api, 'web': web, 'health': health, 'config_nonsecret': {'auth_mode': config['auth_mode'], 'deployment_id': config['deployment_id']},
          'actual_API_process': process, 'actual_API_cwd': cwd, 'actual_API_flags': actual_values,
          'actual_database_url_matches': True, 'database': db, 'bootstrap': bootstrap,
          'api_provenance': api_provenance, 'web_provenance': web_provenance,
          'html_sha256': hashlib.sha256(body).hexdigest(), 'knowledge_budget': 0, 'knowledge_calls': 0,
          'real_provider_allowed': False, 'test_fixture_writes_started': False}
with (HERE / 'live-before-fixtures.json').open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
print(json.dumps({key: record[key] for key in ('api', 'web', 'health', 'database', 'bootstrap', 'test_fixture_writes_started')}))
