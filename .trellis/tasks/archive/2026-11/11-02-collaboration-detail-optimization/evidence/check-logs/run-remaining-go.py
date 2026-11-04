#!/usr/bin/env python3
"""Complete the package inventory without replaying the three proven packages."""

import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.parse

ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
PRIVATE = Path('/tmp/multica-detail-verification-20261103')
REPORT = ROOT / '.trellis/tasks/11-02-collaboration-detail-optimization/research/continuation-20261103'
PREFIX = 'github.com/multica-ai/multica/server/'
PROVEN = {'internal/service', 'internal/handler', 'cmd/server'}
TEMPLATE = 'multica_detail_final_template_20261103_101054'
STAMP = datetime.datetime.now().strftime('%Y%m%d_%H%M%S')
PSQL = '/opt/homebrew/opt/libpq/bin/psql'

config = json.loads((PRIVATE / 'final-template-env.json').read_text())
environment = os.environ.copy()
for key in ('DATABASE_MAX_CONNS', 'DATABASE_MIN_CONNS', 'MULTICA_RESOURCE_PUBLISH_DIR',
            'MULTICA_TASK_CONFIG_ROOT', 'MULTICA_TASK_WORKSPACES_ROOT',
            'MULTICA_RUN_REAL_AGENT_SMOKE', 'REDIS_TEST_URL'):
    environment.pop(key, None)
for key in config:
    environment.pop(key, None)
for key in ('POSTGRES_USER', 'POSTGRES_PASSWORD', 'POSTGRES_HOST', 'POSTGRES_PORT'):
    if key in config:
        environment[key] = config[key]
environment['PATH'] = '/opt/homebrew/opt/libpq/bin:' + environment.get('PATH', '')
environment['GOCACHE'] = '/Volumes/artisan/dev-cache/go-build'
environment['GOMODCACHE'] = '/Users/artisan/go/pkg/mod'
environment['GOPATH'] = '/Users/artisan/go'
private_home_root = PRIVATE / ('remaining-go-home-' + STAMP)
private_home_root.mkdir(mode=0o700)
environment['HOME'] = str(private_home_root)


def url_for_database(name):
    if not re.fullmatch(r'[a-z0-9_]+', name):
        raise RuntimeError('invalid generated database identity')
    parts = urllib.parse.urlsplit(config['DATABASE_URL'])
    return urllib.parse.urlunsplit(parts._replace(path='/' + name))


def sql(database_url, statement):
    result = subprocess.run([PSQL, '-X', '-d', database_url, '-At', '-v', 'ON_ERROR_STOP=1', '-c', statement],
                            env=environment, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError('private database operation failed: ' + result.stderr)
    return result.stdout.strip()


if sql(config['DATABASE_URL'], 'SELECT current_database()') != TEMPLATE:
    raise RuntimeError('recorded schema template identity differs')
if sql(config['DATABASE_URL'], 'SELECT count(*) FROM issue') != '0':
    raise RuntimeError('schema template contains issue rows')

packages = subprocess.check_output(['go', '-C', str(ROOT / 'server'), 'list', './...'],
                                   env=environment, text=True).splitlines()
selected = [p for p in packages if p.removeprefix(PREFIX) not in PROVEN]
db_pattern = re.compile(r'DATABASE_URL|testPool|pgxpool\.New|testutil\.(?:TestPool|NewPool|SetupDB)')
db_packages = set()
for path in (ROOT / 'server').rglob('*_test.go'):
    if db_pattern.search(path.read_text()):
        db_packages.add(path.parent.relative_to(ROOT / 'server').as_posix())

records = []
summary_path = REPORT / ('remaining-go-packages-' + STAMP + '.json')
for package in selected:
    relative = package.removeprefix(PREFIX)
    # The migration regression is separately finished before this final package.
    if relative == 'internal/migrations':
        continue
    token = hashlib.sha256(relative.encode()).hexdigest()[:10]
    database = None
    local_env = environment.copy()
    if relative in db_packages:
        database = 'multica_detail_remaining_' + STAMP + '_' + token
        sql(url_for_database('postgres'), f'CREATE DATABASE "{database}" TEMPLATE "{TEMPLATE}"')
        local_env['DATABASE_URL'] = url_for_database(database)
        local_env['POSTGRES_DB'] = database
        actual = sql(local_env['DATABASE_URL'], 'SELECT current_database()')
        if actual != database or sql(local_env['DATABASE_URL'], 'SELECT count(*) FROM issue') != '0':
            raise RuntimeError('fresh package database identity or initial count differs')
        private_config = PRIVATE / (database + '-env.json')
        private_config.write_text(json.dumps({'DATABASE_URL': local_env['DATABASE_URL'], 'POSTGRES_DB': database}) + '\n')
        private_config.chmod(0o600)
    else:
        # A missed DB read must fail rather than mutate the schema template.
        local_env.pop('DATABASE_URL', None)
        local_env.pop('POSTGRES_DB', None)
    label = relative.replace('/', '-')
    log = PRIVATE / ('remaining-go-' + label + '-' + STAMP + '.jsonl')
    package_home = private_home_root / label
    package_home.mkdir(mode=0o700)
    local_env['HOME'] = str(package_home)
    command = ['bash', str(ROOT / 'scripts/go-test-with-agent-cli-guard.sh'), '--',
               'go', '-C', str(ROOT / 'server'), 'test', '-json', '-race', '-p',
               '2' if relative.startswith('pkg/agent') else '1', '-parallel', '2',
               '-timeout=40m', '-count=1', package]
    print(json.dumps({'running': relative, 'database': database, 'log': str(log)}), flush=True)
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    with log.open('w') as stream:
        result = subprocess.run(command, cwd=ROOT, env=local_env, stdout=stream, stderr=subprocess.STDOUT)
    counts = {'pass': [], 'fail': [], 'skip': []}
    for line in log.read_text().splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get('Test') and event.get('Action') in counts:
            counts[event['Action']].append(event['Test'])
    record = {'package': package, 'database': database, 'command': command, 'started_at': started,
              'exit_code': result.returncode, 'output': str(log), 'inventory': counts}
    records.append(record)
    summary_path.write_text(json.dumps({'excluded_already_proven': sorted(PROVEN),
        'migration_package_separately_pending': True, 'records': records}, indent=2) + '\n')
    print(json.dumps({'completed': relative, 'exit_code': result.returncode,
                      'named': {k: len(v) for k, v in counts.items()}}), flush=True)
    if result.returncode:
        raise SystemExit(result.returncode)

print(json.dumps({'complete': str(summary_path), 'packages': len(records)}), flush=True)
