"""One detached authorized launcher invocation, then its registered final audit."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v15'
PYTHON = '/opt/homebrew/opt/python@3.14/bin/python3.14'


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def save(path, value):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as output:
        json.dump(value, output, indent=2)
        output.write('\n')


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


clearance_path = OUT / 'independent-review-clearance-v15.json'
assert digest(clearance_path) == '20b65d8741bd7bb7c39b32727a7f44749ca6eefe6910f0f952afea3bdf806be1'
clearance = json.loads(clearance_path.read_text())
assert clearance['candidate_sha'] == 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
assert clearance['coordinator'] == '01a1a3c9-2459-7de1-8244-55640c5f6143'
assert clearance['executor'] == '/root/shared_executor'
assert clearance['verdict'] in ('pass', 'pass-with-notes') and clearance['blocking_findings_open'] == 0
assert clearance['knowledge_budget_allocated'] == clearance['knowledge_real_calls'] == 0
assert clearance['real_provider_allowed'] is False
assert digest(clearance['review_path']) == '35805fe56528f54818c0151299b2e71f7d358f53ba0bd928bc905303ba28e495'
assert digest(clearance['root_live_confirmation_path']) == clearance['root_live_confirmation_sha256']
for name in ('check-source-v15.safe.env', 'full-check-home-v15', 'go-packages-v15', 'dev-registry-v15', 'profiles-v15', 'workspaces-v15'):
    assert not (OUT / name).exists() and not (OUT / name).is_symlink(), 'Complete execution paths must be fresh'
launcher = OUT / 'run-make-check-v15-provenance.py'
assert digest(clearance['review_path']) == clearance['review_sha256']
assert digest(launcher) == clearance['launcher_sha256']
assert digest(OUT / 'check-source-v15.env') == clearance['source_env_sha256']
assert digest(OUT / 'owned-bindings-v15-provenance.json') == clearance['bindings_sha256']
assert digest(OUT / 'resource-ownership-v15.json') == clearance['resource_ownership_sha256']
assert digest(clearance['final_state_audit_path']) == clearance['final_state_audit_sha256']
assert all(digest(OUT / 'check-bin' / name) == value for name, value in clearance['adapter_sha256'].items())
assert not (OUT / 'make-check-v15.json').exists() and not (OUT / 'make-check-v15.log').exists()
assert subprocess.check_output(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip() == clearance['candidate_sha']
assert subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain'], text=True).strip() == ''
claim = {
    'claimed_at': now(), 'coordinator': clearance['coordinator'], 'executor': '/root/shared_executor',
    'wrapper_pid': os.getpid(), 'wrapper_sha256': digest(__file__), 'candidate_sha': clearance['candidate_sha'],
    'clearance_sha256': digest(clearance_path), 'launcher_sha256': digest(launcher),
    'mode': 'single_detached_complete_execution_no_rerun', 'Provider_calls': 0,
}
save(OUT / 'execution-claim.json', claim)
env = {**os.environ, 'PYTHONDONTWRITEBYTECODE': '1'}
stdout = OUT / 'launcher-stdout-v15.log'
fd = os.open(stdout, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as sink:
    process = subprocess.Popen([PYTHON, str(launcher)], cwd=ROOT, env=env, stdin=subprocess.DEVNULL,
                               stdout=sink, stderr=subprocess.STDOUT)
    save(HERE / 'launch.json', {**claim, 'launcher_pid': process.pid, 'launcher_stdout': str(stdout),
                              'wrapper': str(Path(__file__).resolve()), 'wrapper_sha256': digest(__file__),
                              'command': [PYTHON, str(launcher)]})
    print(json.dumps({'state': 'running', 'wrapper_pid': os.getpid(), 'launcher_pid': process.pid, 'candidate_sha': clearance['candidate_sha'], 'complete_invocations': 1}), flush=True)
    code = process.wait()
result = {'finished_at': now(), 'launcher_exit_code': code, 'stdout_sha256': digest(stdout),
          'candidate_sha': clearance['candidate_sha'], 'runtime_record': str(OUT / 'make-check-v15.json'),
          'reruns': 0, 'Provider_calls': 0}
if (OUT / 'make-check-v15.json').exists():
    run = json.loads((OUT / 'make-check-v15.json').read_text())
    result.update(make_state=run.get('state'), make_exit_code=run.get('exit_code'))
    assert run.get('state') == 'complete'
    assert digest(clearance['review_path']) == clearance['review_sha256']
    assert digest(clearance['final_state_audit_path']) == clearance['final_state_audit_sha256']
    audit_path = Path(clearance['final_state_audit_path']).parent / 'post-make-v15.json'
    command = [PYTHON, clearance['final_state_audit_path'], '--output', str(audit_path)]
    fd = os.open(HERE / 'final-audit-output.log', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as sink:
        audit = subprocess.run(command, cwd=ROOT, env=env, stdin=subprocess.DEVNULL, stdout=sink, stderr=subprocess.STDOUT)
    result.update(final_audit_exit_code=audit.returncode, final_audit_path=str(audit_path),
                  final_audit_sha256=digest(audit_path) if audit_path.is_file() else None)
else:
    result.update(make_state='not_started', final_audit='not_run_without_complete_make_record')
save(HERE / 'execution-finished.json', result)
print(json.dumps(result), flush=True)
