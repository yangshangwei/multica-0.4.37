"""One detached authorized launcher invocation, then its registered final audit."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v14'
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


clearance_path = OUT / 'independent-review-clearance-v14.json'
clearance = json.loads(clearance_path.read_text())
launcher = OUT / 'run-make-check-v14-provenance.py'
assert digest(clearance['review_path']) == clearance['review_sha256']
assert digest(launcher) == clearance['launcher_sha256']
assert digest(OUT / 'check-source-v14.env') == clearance['source_env_sha256']
assert digest(OUT / 'owned-bindings-v14-provenance.json') == clearance['bindings_sha256']
assert digest(OUT / 'resource-ownership-v14.json') == clearance['resource_ownership_sha256']
assert digest(clearance['final_state_audit_path']) == clearance['final_state_audit_sha256']
assert all(digest(OUT / 'check-bin' / name) == value for name, value in clearance['adapter_sha256'].items())
assert not (OUT / 'make-check-v14.json').exists() and not (OUT / 'make-check-v14.log').exists()
assert subprocess.check_output(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip() == clearance['candidate_sha']
assert subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain'], text=True).strip() == ''
claim = {
    'claimed_at': now(), 'coordinator': clearance['coordinator'], 'executor': '/root/closure_inventory',
    'wrapper_pid': os.getpid(), 'candidate_sha': clearance['candidate_sha'],
    'clearance_sha256': digest(clearance_path), 'launcher_sha256': digest(launcher),
    'mode': 'single_detached_complete_execution_no_rerun', 'Provider_calls': 0,
}
save(OUT / 'execution-claim.json', claim)
env = {**os.environ, 'PYTHONDONTWRITEBYTECODE': '1'}
stdout = OUT / 'launcher-stdout-v14.log'
fd = os.open(stdout, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as sink:
    process = subprocess.Popen([PYTHON, str(launcher)], cwd=ROOT, env=env, stdin=subprocess.DEVNULL,
                               stdout=sink, stderr=subprocess.STDOUT)
    save(HERE / 'launch.json', {**claim, 'launcher_pid': process.pid, 'launcher_stdout': str(stdout),
                              'wrapper': str(Path(__file__).resolve()), 'wrapper_sha256': digest(__file__),
                              'command': [PYTHON, str(launcher)]})
    code = process.wait()
result = {'finished_at': now(), 'launcher_exit_code': code, 'stdout_sha256': digest(stdout),
          'candidate_sha': clearance['candidate_sha'], 'runtime_record': str(OUT / 'make-check-v14.json'),
          'reruns': 0, 'Provider_calls': 0}
if (OUT / 'make-check-v14.json').exists():
    run = json.loads((OUT / 'make-check-v14.json').read_text())
    result.update(make_state=run.get('state'), make_exit_code=run.get('exit_code'))
    assert run.get('state') == 'complete'
    assert digest(clearance['review_path']) == clearance['review_sha256']
    assert digest(clearance['final_state_audit_path']) == clearance['final_state_audit_sha256']
    audit_path = Path(clearance['final_state_audit_path']).parent / 'post-make-v14.json'
    command = [PYTHON, clearance['final_state_audit_path'], '--output', str(audit_path)]
    fd = os.open(HERE / 'final-audit-output.log', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as sink:
        audit = subprocess.run(command, cwd=ROOT, env=env, stdin=subprocess.DEVNULL, stdout=sink, stderr=subprocess.STDOUT)
    result.update(final_audit_exit_code=audit.returncode, final_audit_path=str(audit_path),
                  final_audit_sha256=digest(audit_path) if audit_path.is_file() else None)
else:
    result.update(make_state='not_started', final_audit='not_run_without_complete_make_record')
save(HERE / 'execution-finished.json', result)
