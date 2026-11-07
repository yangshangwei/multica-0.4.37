from pathlib import Path
import base64
import hashlib
import json
import os
import stat
import time
from datetime import datetime, timezone
import yaml

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103')
DONOR = ROOT.parent / 'mc-collaboration-final3-20261102'
EVIDENCE = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/owned-workspace-links')
STORE = ROOT / '.omx/private/owned-workspace-links/pnpm-store/v10'

def digest(path, algorithm='sha256'):
    with path.open('rb') as file:
        return hashlib.file_digest(file, algorithm).digest()

def identity(path):
    result = {'path': str(path), 'exists': path.exists(), 'symlink': path.is_symlink()}
    if path.is_symlink():
        result['link'] = os.readlink(path)
    if path.exists():
        info = path.stat()
        result.update(realpath=str(path.resolve()), device=info.st_dev, inode=info.st_ino,
                      size=info.st_size, mode=stat.S_IMODE(info.st_mode))
        if path.is_file():
            result['sha256'] = digest(path).hex()
    return result

started = time.monotonic()
lock_path = ROOT / 'pnpm-lock.yaml'
lock = yaml.safe_load(lock_path.read_text())
seed = json.loads((EVIDENCE / 'store-seed.json').read_text())
index_by_id = {item['package_id']: item for item in seed['packages']}
metadata_by_id = {package_id: json.loads(Path(info['private_index']).read_text())
                  for package_id, info in index_by_id.items()}
issues = []
direct = []
for importer, groups in lock['importers'].items():
    base = (ROOT / importer).resolve()
    for group in ['dependencies', 'devDependencies', 'optionalDependencies']:
        for name, expected in groups.get(group, {}).items():
            version = str(expected['version'])
            link = base / 'node_modules' / name
            if version.startswith('link:'):
                continue
            manifest = link / 'package.json'
            data = json.loads(manifest.read_text()) if manifest.is_file() else None
            wanted = version.split('(', 1)[0]
            observed = data.get('version') if data else None
            row = {'importer': importer, 'group': group, 'dependency': name,
                   'expected_version': wanted, 'lock_snapshot': version, 'observed_version': observed,
                   'link': identity(link), 'package_json': identity(manifest), 'match': wanted == observed}
            if not row['match'] or not link.resolve().is_relative_to(ROOT):
                issues.append({'kind': 'direct_dependency_mismatch', 'row': row})
            direct.append(row)

symlinks = []
for importer in lock['importers']:
    directory = ROOT / importer / 'node_modules'
    if not directory.exists():
        continue
    if directory.is_symlink():
        issues.append({'kind': 'whole_node_modules_symlink', 'path': str(directory)})
        continue
    for base, directories, files in os.walk(directory, followlinks=False):
        for name in directories + files:
            path = Path(base) / name
            if not path.is_symlink():
                continue
            row = {'path': str(path), 'link': os.readlink(path), 'realpath': str(path.resolve()),
                   'exists': path.exists(), 'inside_owned_root': path.resolve().is_relative_to(ROOT)}
            symlinks.append(row)
            if not row['exists'] or not row['inside_owned_root']:
                issues.append({'kind': 'symlink_escape_or_missing_target', 'row': row})

snapshots = []
checked_files = 0
checked_bytes = 0
for slot in sorted((ROOT / 'node_modules/.pnpm').iterdir()):
    if not slot.is_dir() or slot.name == 'node_modules':
        continue
    dependencies = slot / 'node_modules'
    if not dependencies.is_dir():
        continue
    packages = []
    for directory in dependencies.iterdir():
        if directory.is_symlink() or not directory.is_dir():
            continue
        if directory.name.startswith('@'):
            packages.extend(child for child in directory.iterdir()
                            if not child.is_symlink() and child.is_dir() and (child / 'package.json').is_file())
        elif (directory / 'package.json').is_file():
            packages.append(directory)
    for package in sorted(packages):
        manifest = json.loads((package / 'package.json').read_text())
        package_id = f'{manifest["name"]}@{manifest["version"]}'
        metadata = metadata_by_id.get(package_id)
        if not metadata:
            issues.append({'kind': 'installed_package_without_lock_bound_cache', 'package': str(package), 'id': package_id})
            continue
        failures = []
        fingerprint = hashlib.sha256()
        for name, expected in sorted(metadata['files'].items()):
            path = package / name
            if not path.is_file() or path.is_symlink():
                failures.append({'path': name, 'reason': 'not_regular'})
                continue
            algorithm, encoded = expected['integrity'].split('-', 1)
            observed = digest(path, algorithm)
            if observed != base64.b64decode(encoded):
                failures.append({'path': name, 'reason': 'SRI_mismatch', 'expected': expected['integrity'],
                                 'observed': algorithm + '-' + base64.b64encode(observed).decode()})
            fingerprint.update(name.encode() + b'\0' + observed)
            checked_files += 1
            checked_bytes += path.stat().st_size
        snapshot = {'slot': slot.name, 'package': str(package), 'id': package_id,
                    'lock_integrity': lock['packages'][package_id]['resolution']['integrity'],
                    'cache_index_sha256': index_by_id[package_id]['cache_index_sha256'],
                    'base_files_checked': len(metadata['files']), 'base_files_fingerprint': fingerprint.hexdigest(),
                    'failures': failures}
        snapshots.append(snapshot)
        if failures:
            issues.append({'kind': 'installed_base_package_integrity', 'snapshot': snapshot})
    if len(snapshots) % 250 == 0 and snapshots:
        print(json.dumps({'snapshots_verified': len(snapshots), 'files_verified': checked_files,
                          'elapsed_seconds': round(time.monotonic() - started, 1)}), flush=True)

representatives = []
requests = [
    ('apps/web', 'next', ['package.json', 'dist/bin/next', 'dist/server/app-render/app-render.js']),
    ('apps/docs', 'next', ['package.json', 'dist/bin/next']),
    ('apps/web', 'react', ['package.json', 'index.js', 'cjs/react.production.js', 'jsx-runtime.js']),
    ('packages/core', 'react', ['package.json', 'index.js']),
    ('packages/views', 'react', ['package.json', 'index.js']),
    ('packages/ui', 'react', ['package.json', 'index.js']),
    ('apps/mobile', 'react', ['package.json', 'index.js']),
    ('apps/web', 'react-dom', ['package.json', 'server.node.js', 'cjs/react-dom-server.node.production.js']),
    ('.', '@playwright/test', ['package.json', 'index.js', 'cli.js']),
    ('.', 'playwright', ['package.json', 'index.js', 'cli.js']),
    ('.', 'playwright-core', ['package.json', 'index.js', 'cli.js']),
    ('apps/desktop', 'electron', ['package.json', 'index.js', 'path.txt', 'dist/Electron.app/Contents/MacOS/Electron']),
]
for consumer, name, files in requests:
    own = ROOT / consumer / 'node_modules' / name
    donor = DONOR / consumer / 'node_modules' / name
    own_manifest = json.loads((own / 'package.json').read_text())
    donor_manifest = json.loads((donor / 'package.json').read_text()) if (donor / 'package.json').is_file() else None
    rows = []
    for file in files:
        own_file, donor_file = identity(own / file), identity(donor / file)
        match = own_file.get('sha256') is not None and own_file.get('sha256') == donor_file.get('sha256')
        rows.append({'relative_file': file, 'owned': own_file, 'donor': donor_file, 'same_sha256': match})
        if not match:
            issues.append({'kind': 'representative_donor_hash_mismatch', 'consumer': consumer, 'package': name, 'file': file})
    representatives.append({'consumer': consumer, 'package': name, 'owned_version': own_manifest['version'],
                            'donor_version': donor_manifest.get('version') if donor_manifest else None, 'files': rows})

modules_path = ROOT / 'node_modules/.modules.yaml'
modules = yaml.safe_load(modules_path.read_text())
donor_modules = yaml.safe_load((DONOR / 'node_modules/.modules.yaml').read_text())
same_skip_set = set(modules.get('skipped', [])) == set(donor_modules.get('skipped', []))
if modules['storeDir'] != str(STORE) or modules['packageManager'] != 'pnpm@10.28.2':
    issues.append({'kind': 'pnpm_store_or_version_identity'})
if not same_skip_set:
    issues.append({'kind': 'platform_optional_skip_set_differs_from_donor',
                   'owned_only': sorted(set(modules.get('skipped', [])) - set(donor_modules.get('skipped', []))),
                   'donor_only': sorted(set(donor_modules.get('skipped', [])) - set(modules.get('skipped', [])))})

registry_methods = []
for item in representatives:
    if item['package'] == 'electron':
        continue
    file = item['files'][0]['owned']
    package_id = f'{item["package"]}@{item["owned_version"]}'
    info = metadata_by_id[package_id]['files']['package.json']
    hex_digest = base64.b64decode(info['integrity'].split('-', 1)[1]).hex()
    cache = STORE / 'files' / hex_digest[:2] / (hex_digest[2:] + ('-exec' if info['mode'] & 0o111 else ''))
    cache_identity = identity(cache)
    registry_methods.append({'id': package_id, 'owned_package_json': file, 'private_cafs_file': cache_identity,
                             'same_inode': (file['device'], file['inode']) == (cache_identity['device'], cache_identity['inode']),
                             'same_sha256': file['sha256'] == cache_identity['sha256']})

record = {
    'schema_version': 1, 'recorded_at': datetime.now(timezone.utc).isoformat(),
    'mode': 'read-only filesystem/lock/SRI/hash audit; no application modules or repository tests executed',
    'lock_sha256': digest(lock_path).hex(), 'modules_metadata': identity(modules_path),
    'pnpm_metadata': {key: modules.get(key) for key in ['packageManager', 'nodeLinker', 'storeDir', 'virtualStoreDir',
                     'hoistPattern', 'publicHoistPattern', 'pendingBuilds', 'skipped']},
    'platform_optional_skip_set_matches_donor': same_skip_set,
    'direct_dependency_rows': len(direct), 'direct_dependencies': direct,
    'installed_snapshot_count': len(snapshots), 'installed_unique_package_count': len(set(row['id'] for row in snapshots)),
    'base_files_checked': checked_files, 'base_bytes_checked': checked_bytes, 'snapshots': snapshots,
    'symlink_count': len(symlinks), 'symlinks': symlinks, 'representatives': representatives,
    'copy_identity_samples': registry_methods,
    'electron_side_effect_files_checked_separately': str(EVIDENCE / 'electron-cache-restoration.json'),
    'issues': issues, 'issue_count': len(issues), 'elapsed_seconds': round(time.monotonic() - started, 3),
}
with (EVIDENCE / 'layout-integrity.json').open('x') as file:
    json.dump(record, file, indent=2)
    file.write('\n')
print(json.dumps({key: record[key] for key in ['direct_dependency_rows', 'installed_snapshot_count',
                 'installed_unique_package_count', 'base_files_checked', 'base_bytes_checked',
                 'symlink_count', 'platform_optional_skip_set_matches_donor', 'issue_count', 'elapsed_seconds']}, indent=2), flush=True)
if issues:
    print(json.dumps(issues[:20], indent=2), flush=True)
    raise SystemExit(1)
