"""Retain the full retired desktop tree and install the new staged task build."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v15'
HERE = Path(__file__).resolve().parent
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
EXPECTED = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def inventory(directory):
    assert directory.is_dir() and not directory.is_symlink()
    rows = []
    for path in sorted(directory.rglob('*')):
        assert not path.is_symlink()
        if path.is_file():
            rows.append({'path': str(path.relative_to(directory)), 'sha256': digest(path),
                         'bytes': path.stat().st_size, 'mode': stat.S_IMODE(path.stat().st_mode)})
    assert rows
    return {'path': str(directory), 'files': rows, 'file_count': len(rows),
            'manifest_sha256': hashlib.sha256(json.dumps(rows, sort_keys=True, separators=(',', ':')).encode()).hexdigest()}


def save(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as target:
        json.dump(value, target, indent=2)
        target.write('\n')


assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip() == EXPECTED
assert not subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).strip()
release = json.loads((HERE / 'retired-runtime-release-after.json').read_text())
assert release['state'] == 'released' and not release['remaining']
for pid in (35776, 98904, 99782):
    assert subprocess.run(['ps', '-p', str(pid), '-o', 'command='], capture_output=True, text=True).returncode != 0
for phase in ('cli', 'desktop'):
    path = HERE / 'candidate-setup' / (phase + '-build-v15.json')
    build = json.loads(path.read_text())
    assert build['candidate_sha'] == build['source_head_after'] == EXPECTED
    assert build['exit_code'] == 0 and not build['source_status_after']
    assert digest(HERE / 'candidate-setup' / (phase + '-build-v15.log')) == build['log_sha256']
staged = OUT / 'artifacts-build'
installed = ROOT / 'apps/desktop/out'
retained = OUT / 'retained-desktop-out-before-v15'
assert not retained.exists()
before = inventory(installed)
expected = inventory(staged)
ready = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'candidate_sha': EXPECTED,
         'candidate_status': '', 'staged_root': str(staged), 'root_install_destination': str(installed),
         'installation_performed': False, 'cli_sha256': digest(OUT / 'multica'),
         'desktop': {kind: inventory(staged / kind) for kind in ('renderer', 'preload', 'main')}}
save(HERE / 'staged-desktop-ready.json', ready)
save(HERE / 'desktop-install-before.json', {'recorded_at': datetime.now(timezone.utc).isoformat(),
     'candidate_sha': EXPECTED, 'retired_installed': before, 'staged': expected,
     'retention_destination': str(retained), 'release_sha256': digest(HERE / 'retired-runtime-release-after.json')})
installed.rename(retained)
assert inventory(retained)['files'] == before['files']
shutil.copytree(staged, installed, copy_function=shutil.copy2)
assert inventory(installed)['files'] == expected['files']
assert not subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).strip()
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'coordinator': OWNER,
          'candidate_sha': EXPECTED, 'staged_manifest_sha256': digest(HERE / 'staged-desktop-ready.json'),
          'installed_root': str(installed), 'installed_staged_files': len(expected['files']),
          'bytes_and_modes_match': True, 'previous_installation_retained': str(retained),
          'previous_file_hashes': {row['path']: row['sha256'] for row in before['files']},
          'previous_modes': {row['path']: row['mode'] for row in before['files']},
          'candidate_clean': True, 'other_desktop_not_touched': True, 'complete_check_started': False}
save(HERE.parent / 'desktop-v15-install.json', record)
print(json.dumps({'candidate_sha': EXPECTED, 'staged_files': len(expected['files']),
                  'previous_files_retained': len(before['files']), 'bytes_and_modes_match': True,
                  'candidate_clean': True}))
