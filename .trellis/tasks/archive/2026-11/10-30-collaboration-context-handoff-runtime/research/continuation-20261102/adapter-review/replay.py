"""Independent, database-free review of the pinned v5 verification adapters."""

import ast
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys


ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
OUT = Path(__file__).resolve().parent
ADAPTER = ROOT / '.omx/private/collaboration-resume-20261101/adapter-v5'
CANDIDATE = Path('/Volumes/artisan/code/2026/mc-collaboration-final4-20261102')
LAUNCHER = CANDIDATE / '.omx/private/final-candidate/run-make-check.py'
TEMP = OUT / 'temporary'
TEMP.mkdir(exist_ok=True)
TEMP.chmod(0o700)
os.environ['TMPDIR'] = str(TEMP)
os.environ['PYTHONDONTWRITEBYTECODE'] = '1'
os.environ['GOTOOLCHAIN'] = 'local'
sys.dont_write_bytecode = True

record = {
    'recorded_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'scope': 'independent v5 adapters and prepared wholecheck launcher',
    'network_used': False,
    'database_connections': 0,
    'real_provider_calls': 0,
    'source_files_modified': False,
    'hashes': {},
    'syntax': [],
}

paths = {
    'go': ADAPTER / 'check-bin/go',
    'pnpm': ADAPTER / 'check-bin/pnpm',
    'contracts': ADAPTER / 'check-go-isolation-test.py',
    'launcher': LAUNCHER,
}
for name, path in paths.items():
    content = path.read_bytes()
    record['hashes'][name] = hashlib.sha256(content).hexdigest()
    ast.parse(content, filename=str(path))
    record['syntax'].append({'file': str(path), 'ast_parse': 'pass'})

command = [sys.executable, str(paths['contracts'])]
result = subprocess.run(command, env=os.environ.copy(), capture_output=True, text=True)
log = OUT / 'contracts-replay.log'
log.write_text(result.stdout + result.stderr)
log.chmod(0o600)
record['contracts'] = {
    'command': command,
    'exit_code': result.returncode,
    'named_ok_lines': len([line for line in result.stderr.splitlines() if line.endswith(' ... ok')]),
    'log_sha256': hashlib.sha256(log.read_bytes()).hexdigest(),
}
assert result.returncode == 0, 'Pinned v5 contracts did not pass'
assert record['contracts']['named_ok_lines'] == 11

namespace = {'__file__': str(paths['contracts']), '__name__': 'independent_adapter_review'}
exec(compile(paths['contracts'].read_text(), str(paths['contracts']), 'exec'), namespace)
case = namespace['IsolationLauncherTests']('test_packages_get_distinct_owned_clones_without_recursive_mode')
case.setUp()
try:
    # Replace only the test-created fake executable, never the reviewed adapter.
    case.go.write_text('''#!/usr/bin/env python3
import json, os, sys
arguments = sys.argv[1:]
if arguments[:1] == ['list']:
    print('example/first\\nexample/second')
else:
    entry = {'go': arguments, 'database': os.environ.get('DATABASE_URL'),
             'mode': os.environ.get('MULTICA_TASK_PACKAGE_DB_MODE'),
             'DATABASE_MAX_CONNS': os.environ.get('DATABASE_MAX_CONNS'),
             'DATABASE_MIN_CONNS': os.environ.get('DATABASE_MIN_CONNS')}
    with open(os.environ['CHECK_ISOLATION_FAKE_RECORD'], 'a') as sink:
        sink.write(json.dumps(entry) + '\\n')
    print(json.dumps({'Action': 'run', 'Test': 'TestFixture'}))
    print(json.dumps({'Action': 'pass', 'Test': 'TestFixture'}))
    print(json.dumps({'Action': 'pass', 'Package': arguments[-1]}))
''')
    result = case.run_wrapper('test', '-p', '1', '-parallel', '2', './...',
                              DATABASE_MAX_CONNS='10', DATABASE_MIN_CONNS='1')
    inherited = [entry for entry in case.calls() if 'go' in entry]
    assert result.returncode == 0 and len(inherited) == 2
    assert all(entry['DATABASE_MAX_CONNS'] == '10' and entry['DATABASE_MIN_CONNS'] == '1'
               for entry in inherited)
    record['api_pool_contamination_proof'] = {
        'launcher_assignment': {'DATABASE_MAX_CONNS': '10', 'DATABASE_MIN_CONNS': '1'},
        'fake_go_exit_code': result.returncode,
        'executed_go_commands': inherited,
        'fake_sql_only': True,
        'actual_sql_or_testmain_executed': False,
    }
finally:
    case.doCleanups()

# Exercise the existing source-loader expression against harmless owned input.
source = TEMP / 'loader-probe.env'
source.write_text('REVIEW_DOTENV_EXPANSION="$(printf task_review_expanded)"\n')
source.chmod(0o600)
env = {'PATH': '/opt/homebrew/bin:/usr/bin:/bin', 'TMPDIR': str(TEMP)}
command = ['bash', '-c', 'set -a; . "$1"; env -0', 'trusted-check-env', str(source)]
result = subprocess.run(command, env=env, capture_output=True)
loaded = dict(part.decode().split('=', 1) for part in result.stdout.split(b'\0') if b'=' in part)
assert result.returncode == 0 and loaded['REVIEW_DOTENV_EXPANSION'] == 'task_review_expanded'
record['dotenv_evaluation_proof'] = {
    'command_shape': ['bash', '-c', 'set -a; . "$1"; env -0', 'trusted-check-env', '<owned harmless fixture>'],
    'exit_code': result.returncode,
    'shell_substitution_executed': True,
    'fixture_contains_no_credentials': True,
    'actual_private_env_executed': False,
}
source.unlink()

command = ['/usr/bin/sandbox-exec', '-p', '(version 1) (allow default) (deny network*)',
           str(paths['go']), 'version']
env = {'PATH': '/opt/homebrew/bin:/usr/bin:/bin', 'TMPDIR': str(TEMP), 'GOTOOLCHAIN': 'local'}
result = subprocess.run(command, env=env, capture_output=True, text=True)
record['stripped_non_test_forwarding'] = {
    'command': command,
    'exit_code': result.returncode,
    'stdout': result.stdout.strip(),
    'stderr_sha256': hashlib.sha256(result.stderr.encode()).hexdigest(),
    'env_key_names': sorted(env),
    'network_denied': True,
    'database_environment_absent': True,
    'isolation_markers_absent': True,
}
assert result.returncode == 0

record['shell_syntax'] = []
for name in ['scripts/check.sh', 'scripts/test-go.sh', 'scripts/go-test-with-agent-cli-guard.sh']:
    command = ['bash', '-n', str(CANDIDATE / name)]
    result = subprocess.run(command, capture_output=True, text=True)
    record['shell_syntax'].append({'file': name, 'command': command, 'exit_code': result.returncode})
    assert result.returncode == 0

for name, path in paths.items():
    assert hashlib.sha256(path.read_bytes()).hexdigest() == record['hashes'][name]
record['reviewed_files_unchanged_after_execution'] = True
(OUT / 'verification.json').write_text(json.dumps(record, indent=2) + '\n')
print(json.dumps({'contracts': record['contracts'], 'contamination': 'confirmed_with_fake_children',
                  'dotenv': 'shell_expansion_confirmed_with_owned_fixture',
                  'stripped_go': record['stripped_non_test_forwarding'], 'files_unchanged': True}))
