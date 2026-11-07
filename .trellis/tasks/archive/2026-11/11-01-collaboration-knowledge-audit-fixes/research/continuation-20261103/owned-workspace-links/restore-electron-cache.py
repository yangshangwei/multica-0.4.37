from pathlib import Path
import base64
import ctypes
import hashlib
import json
import os
import plistlib
import shutil
import stat
from datetime import datetime, timezone

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103')
STORE = Path('/Users/artisan/Library/pnpm/store/v10')
EVIDENCE = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/owned-workspace-links')
SOURCE = (ROOT.parent / 'mc-collaboration-final3-20261102/node_modules/electron').resolve()
DESTINATION = (ROOT / 'apps/desktop/node_modules/electron').resolve()

def digest(path, algorithm='sha256'):
    with path.open('rb') as file:
        return hashlib.file_digest(file, algorithm).digest()

def identity(path):
    info = path.stat()
    return {'path': str(path), 'realpath': str(path.resolve()), 'device': info.st_dev, 'inode': info.st_ino,
            'size': info.st_size, 'sha256': digest(path).hex(), 'mode': stat.S_IMODE(info.st_mode)}

if not DESTINATION.is_relative_to(ROOT) or (DESTINATION / 'dist').exists() or (DESTINATION / 'path.txt').exists():
    raise SystemExit('Unexpected owned Electron destination or existing native output')
for package in [SOURCE, DESTINATION]:
    metadata = json.loads((package / 'package.json').read_text())
    if metadata['name'] != 'electron' or metadata['version'] != '39.8.7':
        raise SystemExit('Electron version mismatch')

index_path = next(STORE.glob('index/*/*-electron@39.8.7.json'))
index = json.loads(index_path.read_text())
matched = []
metadata_checks = []
for key, delta in index['sideEffects'].items():
    mismatches = []
    for name, info in delta['added'].items():
        source = SOURCE / name
        if not source.is_file() or source.is_symlink():
            mismatches.append({'file': name, 'reason': 'not_regular'})
            continue
        algorithm, encoded = info['integrity'].split('-', 1)
        if digest(source, algorithm) != base64.b64decode(encoded):
            mismatches.append({'file': name, 'reason': 'content_integrity'})
    metadata_checks.append({'key': key, 'file_count': len(delta['added']), 'mismatches': mismatches})
    if not mismatches:
        matched.append((key, delta['added']))
if not matched:
    raise SystemExit('Retained Electron distribution does not match any cached side-effects index')

key, files = matched[-1]
libc = ctypes.CDLL(None, use_errno=True)
clonefile = libc.clonefile
clonefile.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
clonefile.restype = ctypes.c_int
copied = []
methods = {}
for name, info in files.items():
    if not name.startswith('dist/') and name != 'path.txt':
        continue
    expected = Path(name)
    if expected.is_absolute() or '..' in expected.parts:
        raise SystemExit('Unsafe cached distribution path')
    algorithm, encoded = info['integrity'].split('-', 1)
    hex_digest = base64.b64decode(encoded).hex()
    source = STORE / 'files' / hex_digest[:2] / (hex_digest[2:] + ('-exec' if info['mode'] & 0o111 else ''))
    destination = DESTINATION / expected
    if source.is_symlink() or not source.is_file() or digest(source, algorithm) != base64.b64decode(encoded):
        raise SystemExit(f'Invalid retained distribution content: {name}')
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists() or destination.is_symlink():
        raise SystemExit('Refusing to overwrite native output')
    if clonefile(os.fsencode(source), os.fsencode(destination), 0) == 0:
        method = 'clonefile'
    else:
        method = f'copy2_after_errno_{ctypes.get_errno()}'
        shutil.copy2(source, destination)
    destination.chmod(info['mode'])
    if source.stat().st_dev == destination.stat().st_dev and source.stat().st_ino == destination.stat().st_ino:
        raise SystemExit('Native artifact unexpectedly hard linked')
    if digest(destination, algorithm) != base64.b64decode(encoded) or digest(destination) != digest(SOURCE / name):
        raise SystemExit('Restored distribution content mismatch')
    methods[method] = methods.get(method, 0) + 1
    copied.append({'path': name, 'expected_integrity': info['integrity'], 'source_cache': str(source),
                   'source_donor': str(SOURCE / name), 'destination': str(destination),
                   'sha256': digest(destination).hex(), 'size': destination.stat().st_size,
                   'source_device': source.stat().st_dev, 'source_inode': source.stat().st_ino,
                   'destination_device': destination.stat().st_dev, 'destination_inode': destination.stat().st_ino})

binary = Path((DESTINATION / 'path.txt').read_text().strip())
if binary.is_absolute() or '..' in binary.parts:
    raise SystemExit('Electron binary path not relative')
binary = DESTINATION / 'dist' / binary
plist = plistlib.loads((DESTINATION / 'dist/Electron.app/Contents/Info.plist').read_bytes())
record = {'schema_version': 1, 'recorded_at': datetime.now(timezone.utc).isoformat(),
          'package': 'electron@39.8.7', 'source_package': str(SOURCE), 'owned_package': str(DESTINATION),
          'cache_index': str(index_path), 'cache_index_sha256': digest(index_path).hex(),
          'retained_side_effects_checks': metadata_checks, 'selected_side_effects_key': key,
          'file_count_copied': len(copied), 'copy_methods': methods, 'files': copied,
          'binary': identity(binary), 'donor_binary': identity(SOURCE / 'dist' / binary.relative_to(DESTINATION / 'dist')),
          'bundle_version': plist.get('CFBundleShortVersionString'), 'path_txt': identity(DESTINATION / 'path.txt'),
          'base_package_files_overwritten': False, 'whole_donor_node_modules_symlink_used': False,
          'scripts_binaries_services_or_network_executed': False}
with (EVIDENCE / 'electron-cache-restoration.json').open('x') as file:
    json.dump(record, file, indent=2)
    file.write('\n')
print(json.dumps({key: record[key] for key in ['package', 'file_count_copied', 'copy_methods', 'bundle_version', 'binary']}, indent=2))
