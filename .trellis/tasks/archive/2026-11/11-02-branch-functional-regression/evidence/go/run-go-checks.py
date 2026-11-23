#!/usr/bin/env python3
"""Execute coordinator-assigned Go permits without allocating databases."""

import datetime
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

REPO = Path('/Volumes/artisan/code/2026/multica-0.4.37')
TASK = REPO / '.trellis/tasks/11-02-branch-functional-regression'
OUT = TASK / 'evidence/go'
PERMITS = json.loads((REPO / '.omx/private/branch-functional-regression-20261102/go-permits.json').read_text())
SOURCE = Path(PERMITS['source_root'])
CANDIDATE = PERMITS['candidate']


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def dump(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def revision():
    return subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=SOURCE, text=True).strip()


def base_env():
    keep = ['HOME', 'USER', 'LOGNAME', 'SHELL', 'LANG', 'LC_ALL', 'PATH', 'GOCACHE', 'GOMODCACHE', 'GOPATH', 'NVM_DIR', 'PNPM_HOME']
    result = {key: os.environ[key] for key in keep if key in os.environ}
    result.update({
        'PATH': '/opt/homebrew/opt/libpq/bin:' + result.get('PATH', ''),
        'GOTOOLCHAIN': 'local',
        'TMPDIR': PERMITS['build_tmp'],
        'TMP': PERMITS['build_tmp'],
        'TEMP': PERMITS['build_tmp'],
        'PGCONNECT_TIMEOUT': '3',
    })
    return result


def read_permit_env(path):
    values = {}
    for line in Path(path).read_text().splitlines():
        if not line or line.startswith('#'):
            continue
        key, raw = line.split('=', 1)
        parts = shlex.split(raw)
        values[key] = parts[0] if parts else ''
    return values


results = []
started = now()
OUT.mkdir(parents=True, exist_ok=True)
assert revision() == CANDIDATE
dump(OUT / 'source-start.json', {
    'at': started,
    'candidate': CANDIDATE,
    'source_root': str(SOURCE),
    'product_status': subprocess.check_output(['git', 'status', '--porcelain', '--', 'server', 'apps', 'packages', 'e2e', 'scripts'], cwd=SOURCE, text=True),
    'permit_sha256': sha(REPO / '.omx/private/branch-functional-regression-20261102/go-permits.json'),
    'go_version': subprocess.check_output(['go', 'version'], env=base_env(), text=True).strip(),
})


def save_progress(state):
    dump(OUT / 'progress.json', {
        'state': state, 'started_at': started, 'updated_at': now(), 'candidate': CANDIDATE,
        'commands_finished': len(results), 'packages_planned': len(PERMITS['packages']),
        'packages_finished': sum(r['kind'] == 'package' for r in results),
        'failed_commands': [r['name'] for r in results if r['exit_code'] != 0],
        'results': results,
    })


def run(name, command, environment, kind, permit=None):
    assert revision() == CANDIDATE, 'Candidate changed; refusing further execution'
    folder = OUT / name
    folder.mkdir(exist_ok=False)
    record = {
        'name': name, 'kind': kind, 'command': command, 'cwd': str(SOURCE),
        'candidate': CANDIDATE, 'started_at': now(), 'state': 'running',
        'database': permit['database'] if permit else None,
        'environment_keys': sorted(environment),
        'real_provider_authorization': False,
    }
    if permit:
        record['permit_env_sha256'] = sha(Path(permit['private_env_file']))
        record['private_env_file'] = permit['private_env_file']
        if permit['database']:
            query = ['/opt/homebrew/opt/libpq/bin/psql', environment['DATABASE_URL'], '-X', '-At', '-v', 'ON_ERROR_STOP=1', '-c', 'SELECT current_database()']
            identity = subprocess.run(query, env=environment, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            (folder / 'database-identity.stderr.txt').write_text(identity.stderr)
            record['database_identity'] = identity.stdout.strip()
            record['database_identity_exit'] = identity.returncode
            if identity.returncode != 0 or identity.stdout.strip() != permit['database']:
                record.update({'state': 'complete', 'exit_code': 2, 'finished_at': now(), 'classification': 'BLOCKED_DATABASE_IDENTITY'})
                dump(folder / 'run.json', record)
                results.append(record)
                save_progress('running')
                print(name, 'BLOCKED database identity', flush=True)
                return
        else:
            assert 'DATABASE_URL' not in environment
    dump(folder / 'run.json', record)
    print(name, 'START', flush=True)
    with (folder / 'stdout.jsonl').open('w') as stdout, (folder / 'stderr.txt').open('w') as stderr:
        process = subprocess.run(command, cwd=SOURCE, env=environment, stdout=stdout, stderr=stderr)
    record.update({'state': 'complete', 'exit_code': process.returncode, 'finished_at': now(), 'candidate_after': revision()})
    if kind == 'package':
        outcomes = {}
        package_outcome = None
        outputs = {}
        for line in (folder / 'stdout.jsonl').read_text(errors='replace').splitlines():
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            test = event.get('Test')
            action = event.get('Action')
            if test and action == 'output':
                outputs.setdefault(test, []).append(event.get('Output', ''))
            if action in ['pass', 'fail', 'skip']:
                if test:
                    outcomes[test] = action
                else:
                    package_outcome = action
        record['top_level'] = {action: sum(value == action and '/' not in test for test, value in outcomes.items()) for action in ['pass', 'fail', 'skip']}
        record['subtests'] = {action: sum(value == action and '/' in test for test, value in outcomes.items()) for action in ['pass', 'fail', 'skip']}
        record['package_outcome'] = package_outcome
        record['no_named_tests'] = not outcomes
        record['failed_tests'] = [test for test, value in outcomes.items() if value == 'fail']
        record['skips'] = [{'test': test, 'output': ''.join(outputs.get(test, []))} for test, value in outcomes.items() if value == 'skip']
        dump(folder / 'test-outcomes.json', outcomes)
    record['stdout_sha256'] = sha(folder / 'stdout.jsonl')
    record['stderr_sha256'] = sha(folder / 'stderr.txt')
    dump(folder / 'run.json', record)
    results.append(record)
    save_progress('running')
    print(name, 'END', process.returncode, json.dumps(record.get('top_level', {})), flush=True)


run('00-build', ['go', '-C', 'server', 'build', './...'], base_env(), 'build')
run('00-vet', ['go', '-C', 'server', 'vet', '-p', '2', './...'], base_env(), 'vet')
for index, permit in enumerate(PERMITS['packages'], 1):
    environment = base_env()
    environment.update(read_permit_env(permit['private_env_file']))
    command = ['bash', 'scripts/go-test-with-agent-cli-guard.sh', '--', 'go', '-C', 'server', 'test', '-race', '-count=1', '-timeout=40m', '-p', '1', '-parallel', '2', '-json', permit['package']]
    run(f'{index:02d}-{permit["relative"].replace("/", "_")}', command, environment, 'package', permit)

save_progress('complete')
dump(OUT / 'source-end.json', {
    'at': now(), 'candidate': revision(),
    'product_status': subprocess.check_output(['git', 'status', '--porcelain', '--', 'server', 'apps', 'packages', 'e2e', 'scripts'], cwd=SOURCE, text=True),
})
print('GO CHECKS COMPLETE', flush=True)
