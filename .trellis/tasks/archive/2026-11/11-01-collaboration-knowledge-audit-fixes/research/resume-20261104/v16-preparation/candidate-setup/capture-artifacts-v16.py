# Execution coordinator: 01a1a3c9-2459-7de1-8244-55640c5f6143; template only, no execution authorization.
from pathlib import Path
import hashlib
import json
import stat
import subprocess
from datetime import datetime, timezone

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v16'
HERE = Path(__file__).resolve().parent
EXPECTED = 'eda0d68fb07852b714f3092aa02ca0523d3ad312'

def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()

def manifest(directory):
    if not directory.is_dir() or directory.is_symlink():
        raise RuntimeError('Artifact directory unavailable or redirected')
    rows = []
    for path in sorted(directory.rglob('*')):
        if path.is_symlink():
            raise RuntimeError('Artifact must not contain symlinks')
        if path.is_file():
            rows.append({'path': str(path.relative_to(directory)), 'sha256': digest(path),
                         'bytes': path.stat().st_size, 'mode': stat.S_IMODE(path.stat().st_mode)})
    if not rows:
        raise RuntimeError('Artifact inventory is empty')
    return {'path': str(directory), 'file_count': len(rows),
            'manifest_sha256': hashlib.sha256(json.dumps(rows, sort_keys=True, separators=(',', ':')).encode()).hexdigest(),
            'files': rows}

if subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip() != EXPECTED:
    raise RuntimeError('Candidate HEAD changed')
if subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).strip():
    raise RuntimeError('Candidate must be clean before artifact binding')
for phase in ('cli', 'desktop'):
    build = json.loads((HERE / (phase + '-build-v16.json')).read_text())
    if build.get('exit_code') != 0 or build.get('candidate_sha') != EXPECTED or build.get('source_head_after') != EXPECTED or build.get('source_status_after'):
        raise RuntimeError('Build lacks successful clean same-candidate evidence')
    if digest(HERE / (phase + '-build-v16.log')) != build['log_sha256']:
        raise RuntimeError('Build log changed')
record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'candidate_sha': EXPECTED, 'desktop': {}}
for kind in ('renderer', 'preload', 'main'):
    installed = manifest(ROOT / 'apps/desktop/out' / kind)
    staged = manifest(OUT / 'artifacts-build' / kind)
    if installed['files'] != staged['files']:
        raise RuntimeError('Root-installed desktop differs from new staged build: ' + kind)
    record['desktop'][kind] = {'installed': installed, 'staged': staged}
cli = OUT / 'multica'
cli_build = json.loads((HERE / 'cli-build-v16.json').read_text())
if digest(cli) != cli_build['binary_sha256']:
    raise RuntimeError('CLI changed after build')
record['cli'] = {'path': str(cli), 'sha256': digest(cli), 'bytes': cli.stat().st_size}
with (HERE / 'artifact-bindings-v16.json').open('x') as target:
    json.dump(record, target, indent=2)
    target.write('\n')
print(json.dumps({'candidate_sha': EXPECTED, 'desktop_files': {kind: value['installed']['file_count'] for kind, value in record['desktop'].items()}, 'cli_sha256': record['cli']['sha256']}))
