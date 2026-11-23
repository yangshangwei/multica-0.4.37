from pathlib import Path
import base64
import collections
import ctypes
import hashlib
import json
import os
import re
import shutil
import stat
import sys
import time
from datetime import datetime, timezone
import yaml

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103')
PRIVATE = ROOT / '.omx/private/owned-workspace-links'
EVIDENCE = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/owned-workspace-links')
SOURCE = Path('/Users/artisan/Library/pnpm/store/v10')
TOOL = Path('/Users/artisan/.cache/node/corepack/v1/pnpm/10.28.2')
DESTINATION = PRIVATE / 'pnpm-store/v10'
EXPECTED_HEAD = '304f74dd3ba4ddea862c94917c5bb0770355a351'

libc = ctypes.CDLL(None, use_errno=True)
clonefile = libc.clonefile
clonefile.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
clonefile.restype = ctypes.c_int
methods = collections.Counter()

def copy_file(source, destination):
    source, destination = Path(source), Path(destination)
    if source.is_symlink() or not stat.S_ISREG(source.lstat().st_mode):
        raise RuntimeError(f'Expected a regular cache file: {source}')
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists() or destination.is_symlink():
        raise RuntimeError(f'Refusing to overwrite a seeded file: {destination}')
    if clonefile(os.fsencode(source), os.fsencode(destination), 0) == 0:
        methods['clonefile'] += 1
    else:
        error = ctypes.get_errno()
        shutil.copy2(source, destination)
        methods[f'copy2_after_errno_{error}'] += 1
    if source.stat().st_ino == destination.stat().st_ino and source.stat().st_dev == destination.stat().st_dev:
        raise RuntimeError(f'Unexpected hard link: {destination}')
    return str(destination)

def file_path(info):
    digest = base64.b64decode(info['integrity'].split('-')[1]).hex()
    return Path('files') / digest[:2] / (digest[2:] + ('-exec' if info.get('mode', 0) & 0o111 else ''))

import subprocess
head = subprocess.run(['git', 'rev-parse', 'HEAD'], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip()
if head != EXPECTED_HEAD or PRIVATE.exists():
    raise SystemExit('Candidate drift or private seed directory already exists')
if json.loads((TOOL / 'package.json').read_text())['version'] != '10.28.2':
    raise SystemExit('Wrong cached package manager version')
for rel in ['.omx/private/owned-workspace-links/file', 'node_modules/file']:
    if subprocess.run(['git', 'check-ignore', '-q', rel], cwd=ROOT).returncode:
        raise SystemExit('Intended candidate output is not ignored')

started = time.monotonic()
PRIVATE.mkdir(parents=True)
shutil.copytree(TOOL, PRIVATE / 'pnpm-10.28.2', copy_function=copy_file, symlinks=True)
lock = yaml.safe_load((ROOT / 'pnpm-lock.yaml').read_text())
packages = []
missing = []
files = {}
indices = []
for package_id, package in lock['packages'].items():
    integrity = package.get('resolution', {}).get('integrity')
    if not integrity:
        missing.append({'package_id': package_id, 'reason': 'no_integrity_resolution', 'resolution': package.get('resolution')})
        continue
    digest = base64.b64decode(integrity.split('-')[1]).hex()[:64]
    index_rel = Path('index') / digest[:2] / (digest[2:] + '-' + re.sub(r'[\\/:*?"<>|]', '+', package_id) + '.json')
    source = SOURCE / index_rel
    if not source.exists():
        snapshots = [{'key': key, 'optional': value.get('optional', False)}
                     for key, value in lock['snapshots'].items() if key == package_id or key.startswith(package_id + '(')]
        missing.append({'package_id': package_id, 'index_path': str(source), 'os': package.get('os', []),
                        'cpu': package.get('cpu', []), 'snapshots': snapshots, 'reason': 'index_absent'})
        continue
    raw = source.read_bytes()
    metadata = json.loads(raw)
    indices.append(index_rel)
    packages.append({'package_id': package_id, 'lock_integrity': integrity, 'name': metadata.get('name'),
                     'version': metadata.get('version'), 'cache_index': str(source),
                     'cache_index_sha256': hashlib.sha256(raw).hexdigest(), 'private_index': str(DESTINATION / index_rel),
                     'base_file_count': len(metadata['files']), 'requires_build': metadata.get('requiresBuild', False),
                     'side_effects_keys': list(metadata.get('sideEffects', {}))})
    for name, info in metadata['files'].items():
        rel = file_path(info)
        if not (SOURCE / rel).exists():
            raise RuntimeError(f'Missing content for {package_id}: {name}: {SOURCE / rel}')
        files.setdefault(rel, {'integrity': info['integrity'], 'size': info.get('size', 0), 'mode': info.get('mode', 0)})

for rel in indices:
    copy_file(SOURCE / rel, DESTINATION / rel)
examples = []
for index, (rel, info) in enumerate(sorted(files.items(), key=lambda pair: str(pair[0])), 1):
    source, destination = SOURCE / rel, DESTINATION / rel
    copy_file(source, destination)
    if len(examples) < 12:
        examples.append({'source': str(source), 'destination': str(destination), 'device': source.stat().st_dev,
                         'source_inode': source.stat().st_ino, 'destination_inode': destination.stat().st_ino,
                         'integrity': info['integrity']})
    if index % 10000 == 0:
        print(json.dumps({'copied_content_files': index, 'total_content_files': len(files),
                          'elapsed_seconds': round(time.monotonic() - started, 1)}), flush=True)

for name in ['pnpm-cache', 'pnpm-state', 'pnpm-global', 'pnpm-home', 'tmp']:
    (PRIVATE / name).mkdir()
record = {'schema_version': 1, 'recorded_at': datetime.now(timezone.utc).isoformat(),
          'candidate_sha': head, 'lock_sha256': hashlib.sha256((ROOT / 'pnpm-lock.yaml').read_bytes()).hexdigest(),
          'source_store': str(SOURCE), 'private_store': str(DESTINATION), 'source_device': SOURCE.stat().st_dev,
          'destination_device': DESTINATION.stat().st_dev, 'copy_methods': dict(methods),
          'package_indices_copied': len(packages), 'content_files_copied': len(files),
          'base_content_bytes': sum(info['size'] for info in files.values()),
          'file_inventory_sha256': hashlib.sha256(json.dumps({str(key): val for key, val in sorted(files.items())},
                                    sort_keys=True, separators=(',', ':')).encode()).hexdigest(),
          'packages': packages, 'missing_indices': missing, 'physical_identity_examples': examples,
          'store_projects_directory_copied': False, 'base_files_only': True,
          'side_effects_cache_will_be_disabled': True, 'network_or_scripts_executed': False,
          'elapsed_seconds': round(time.monotonic() - started, 3)}
with (EVIDENCE / 'store-seed.json').open('x') as file:
    json.dump(record, file, indent=2)
    file.write('\n')
print(json.dumps({key: record[key] for key in ['candidate_sha', 'copy_methods', 'package_indices_copied',
                  'content_files_copied', 'base_content_bytes', 'elapsed_seconds']}, indent=2), flush=True)
