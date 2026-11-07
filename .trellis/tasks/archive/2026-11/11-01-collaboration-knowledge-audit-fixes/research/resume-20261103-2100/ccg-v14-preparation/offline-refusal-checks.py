from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import os
import subprocess
import types
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = ROOT / '.omx/private/knowledge-final-gates-v14'
PYTHON = '/opt/homebrew/opt/python@3.14/bin/python3.14'

def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

home = HERE / 'offline-home'
home.mkdir(mode=0o700)
profile = HERE / 'offline-no-network-no-source-writes.sb'
with profile.open('x') as target:
    target.write('(version 1)\n(allow default)\n(deny network*)\n(deny file-write*)\n')
profile.chmod(0o600)
environment = {'PATH': '/usr/bin:/bin', 'HOME': str(home), 'PYTHONDONTWRITEBYTECODE': '1', 'LC_ALL': 'C'}
initial = {str(path): digest(path) for path in PRIVATE.rglob('*') if path.is_file()}
raw = []
for family, mode in [('ccg1', 'normal'), ('ccg1', 'race'), ('ccg2', 'off'), ('ccg2', 'on')]:
    gate = family + '-' + mode
    command = ['/usr/bin/sandbox-exec', '-f', str(profile), PYTHON, str(PRIVATE / ('run-' + family + '.py')),
               mode, '--permit', str(PRIVATE / 'permissions' / (gate + '.json.template')),
               '--label', 'offline-unready-' + gate]
    result = subprocess.run(command, cwd=ROOT, env=environment, capture_output=True, text=True)
    assert result.returncode == 2 and result.stdout.strip() == 'REFUSED: Later root execution permission is required'
    assert not result.stderr and not (PRIVATE / 'runs').exists()
    log = HERE / (gate + '-unready.log')
    with log.open('x') as target:
        target.write(result.stdout)
    raw.append({'gate': gate, 'command': command, 'exit_code': result.returncode, 'stdout': result.stdout.strip(),
                'log_sha256': digest(log), 'candidate_result_directory_created': False})

cases = []
for family in ['ccg1', 'ccg2']:
    source = PRIVATE / ('run-' + family + '.py')
    module = types.ModuleType('credential_free_' + family)
    module.__file__ = str(source)
    exec(compile(source.read_bytes(), str(source), 'exec'), module.__dict__)
    fixture = HERE / ('fixture-' + family)
    permissions = fixture / 'permissions'
    permissions.mkdir(mode=0o700, parents=True)
    module.PRIVATE = fixture
    mode = 'normal' if family == 'ccg1' else 'off'
    gate = family + '-' + mode
    base = {'ready_to_execute': True, 'gate': gate, 'root_released_complete_check_resources': True,
            'root_verified_complete_named_inventory': True, 'root_verified_fresh_database': True,
            'real_provider_allowed': False, 'candidate_sha': module.EXPECTED, 'runner_sha256': digest(source)}
    scenarios = [
        ('invalid-label', {}, '../invalid', 'Invalid private result label'),
        ('unready', {'ready_to_execute': False}, 'ok', 'Later root execution permission is required'),
        ('wrong-gate', {'gate': 'wrong'}, 'ok', 'Later root execution permission is required'),
        ('unreleased', {'root_released_complete_check_resources': False}, 'ok', 'Complete-check resources and inventory are not cleared by root'),
        ('inventory-not-cleared', {'root_verified_complete_named_inventory': False}, 'ok', 'Complete-check resources and inventory are not cleared by root'),
        ('fresh-db-unproved', {'root_verified_fresh_database': False}, 'ok', 'A fresh fake-only database permission is required'),
        ('provider-not-forbidden', {'real_provider_allowed': True}, 'ok', 'A fresh fake-only database permission is required'),
        ('outside-permit', {}, 'ok', 'Permit outside owned directory'),
        ('redirected-permit', {}, 'ok', 'Permit must not be redirected'),
    ]
    for name, changes, label, expected_error in scenarios:
        path = permissions / (name + '.json')
        with path.open('x') as target:
            json.dump({**base, **changes}, target)
        if name == 'outside-permit':
            path = fixture / 'outside.json'
            with path.open('x') as target:
                json.dump(base, target)
        if name == 'redirected-permit':
            linked = permissions / 'redirected-link.json'
            linked.symlink_to(path)
            path = linked
        with patch.object(module.subprocess, 'run', side_effect=AssertionError('External subprocess forbidden')) as run_spy, \
                patch.object(module.subprocess, 'Popen', side_effect=AssertionError('External process forbidden')) as popen_spy:
            try:
                module.admission(gate, path, label)
                raise AssertionError('Expected refusal: ' + name)
            except RuntimeError as error:
                assert str(error) == expected_error, (name, str(error))
            assert run_spy.call_count == popen_spy.call_count == 0
        assert not (fixture / 'runs').exists()
        cases.append({'runner': source.name, 'case': name, 'refused': True, 'reason': expected_error,
                      'external_processes': 0, 'fixture_result_directory_created': False})

assert all(digest(path) == sha for path, sha in initial.items())
assert not (PRIVATE / 'runs').exists()
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'raw_unready_template_refusals': raw,
          'credential_free_counterexamples': cases, 'raw_refusals': len(raw), 'offline_cases': len(cases),
          'Go_Playwright_DB_API_Web_renderer_Redis_Provider_invocations': 0,
          'prepared_CCG_files_unchanged': True, 'candidate_CCG_result_directories_created': False,
          'private_profile_sha256': digest(profile), 'script_sha256': digest(__file__),
          'notes': 'True flags occur only in these isolated review fixtures; all four actual templates remain unready.'}
with (HERE / 'offline-verification.json').open('x') as target:
    json.dump(record, target, indent=2)
    target.write('\n')
print(json.dumps({'raw_refusals': len(raw), 'offline_cases': len(cases), 'external_processes_from_admission': 0,
                  'prepared_CCG_unchanged': True, 'actual_ready_permits': 0}))
