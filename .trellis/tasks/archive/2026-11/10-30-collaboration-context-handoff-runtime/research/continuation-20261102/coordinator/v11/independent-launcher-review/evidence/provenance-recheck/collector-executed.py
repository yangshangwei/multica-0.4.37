from __future__ import annotations
import hashlib
import json
import os
from pathlib import Path
import re
import stat

ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
CANDIDATE = Path('/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103')
EVIDENCE = ROOT / '.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261102/coordinator/v11/independent-launcher-review/evidence'
OWN = EVIDENCE / 'provenance-recheck'
PREP = ROOT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/owned-candidate-preparation/provenance-correction-v11'
PRIVATE = CANDIDATE / '.omx/private/final-check-v11'
EXPECTED = {
    'head': '304f74dd3ba4ddea862c94917c5bb0770355a351',
    'product': '4622182a7e129f1712f883eaea36a4756b183f539b5733eb818b7f8df9195fcb',
    'overlay': '3f9788baf2ad3d922df86c4817b7fe0f3183e1a5c6c0e40d43fc76cd4f2c1d88',
    'tracked': '26e38d9e62f8a7cfa8cba243f2387934ee067677b6fa58fde2e12f0026b66920',
    'binding': '6db6435e476b8f448a412d4b097e16e6131ddbf7458450ced1cf69a8f281d80b',
    'launcher': 'dd0f5903b8b604f409fd10505645b9cc3920028ea4af56ad0adab8ee5a96063e',
    'cli': 'c008c02377c4c52665000a0620a5a7b3f90f4d10d54d53e1d74bce97dccc6197',
    'preload': '68a9036e201a462ffd10a1a4890e052a0b6b342c0b74d6d8ac489d9f5d8e78e5',
    'main': '99129d6206e52ac9fdbe7454e1391d7e3b97dd80091f7461fc335a34d61b5678',
    'renderer_index': '9d92714285456d587bb8bcd3ecf2897ad015cf7f63d76f75fea9a541536ae0eb',
    'renderer_manifest': 'bc46754412b91fac5f6649b7b37ad537e067bb9ab6fa6f0c4012cb0fced91d18',
    'canonical_manifest': 'bafbb6dd9ab1f017f2d4ce8f9291bb13d6c7103362fed5b235b3e70c501a8beb',
    'product_count': 7962,
    'tracked_count': 17182,
}
OUT = {'schema': 'v11-narrow-provenance-readonly-v1', 'expected': EXPECTED, 'errors': []}

def label(p):
    s = str(p)
    for base, replacement in [(CANDIDATE, '<candidate>'), (ROOT, '<root>'), (Path('/Users/artisan'), '<home>')]:
        if s == str(base) or s.startswith(str(base) + '/'):
            return replacement + s[len(str(base)):]
    return s

def file_hash(p):
    p = Path(p)
    row = {'path': label(p), 'exists': p.exists()}
    if not p.is_file():
        return row
    before = p.stat()
    h = hashlib.sha256()
    with p.open('rb') as f:
        for block in iter(lambda: f.read(1024 * 1024), b''):
            h.update(block)
    after = p.stat()
    row.update(sha256=h.hexdigest(), bytes=after.st_size, mode=stat.S_IMODE(after.st_mode),
        read_stable=(before.st_size, before.st_mtime_ns, before.st_ino) == (after.st_size, after.st_mtime_ns, after.st_ino))
    return row

secret = re.compile(r'password|secret|credential|authorization|token|prompt|payload|stdin|stdout|stderr|body|content|messages|environment', re.I)
safe = re.compile(r'path|file|hash|sha|fingerprint|source|commit|head|name|status|version|kind|mode|scope|owner|thread|count|size|bytes|exit|pass|fail|skip|run|test|case|tool|binding|artifact|baseline|candidate|expected|actual|match|gate|budget|consum|remaining|host|arch|platform|renderer|preload|cli|desktop|node|pnpm|go|identity|installed|staged|product|canonical|overlay|method|preserv|drift|graph|record', re.I)

def sanitized(v, key=''):
    if secret.search(key):
        return '<redacted>'
    if isinstance(v, dict):
        return {k: sanitized(x, k) for k, x in v.items()}
    if isinstance(v, list):
        return [sanitized(x, key) for x in v] if len(v) <= 12 else {'count': len(v)}
    if isinstance(v, str):
        if re.search(r'(?i)bearer\s|\b(?:mat|mcn|mul|sk)[_-][A-Za-z0-9_-]{12,}|(?:postgres(?:ql)?|redis|https?)://', v):
            return '<redacted-or-omitted-endpoint>'
        return label(v) if safe.search(key) or re.fullmatch('[0-9a-f]{40}|[0-9a-f]{64}', v) else '<omitted>'
    return v

def read_json(p):
    p = Path(p)
    if not p.is_file():
        OUT['errors'].append({'path': label(p), 'status': 'missing_json'})
        return None
    try:
        return json.loads(p.read_text())
    except Exception as e:
        OUT['errors'].append({'path': label(p), 'status': type(e).__name__})
        return None

def walk(v, prefix=''):
    yield prefix, v
    if isinstance(v, dict):
        for k, x in v.items():
            if not secret.search(str(k)):
                yield from walk(x, prefix + '.' + str(k))
    elif isinstance(v, list):
        for i, x in enumerate(v):
            yield from walk(x, prefix + '[' + str(i) + ']')

def product_records(data):
    records = {}
    for location, v in walk(data):
        if not isinstance(v, dict):
            continue
        path = next((v[k] for k in ['path', 'relative_path', 'file'] if isinstance(v.get(k), str)), None)
        checksum = next((v[k] for k in ['sha256', 'sha', 'file_sha256'] if isinstance(v.get(k), str) and re.fullmatch('[0-9a-f]{64}', v[k])), None)
        pairs = [(path, checksum)] if path and checksum else [(k, x) for k, x in v.items() if isinstance(x, str) and re.fullmatch('[0-9a-f]{64}', x)]
        for rel, checksum in pairs:
            if not isinstance(rel, str):
                continue
            if rel.startswith(str(CANDIDATE) + '/'):
                rel = rel[len(str(CANDIDATE)) + 1:]
            if not (rel.startswith(('server/', 'apps/', 'packages/', 'scripts/')) or rel in ['Makefile', 'package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml']):
                continue
            if rel in records and records[rel] != checksum:
                OUT['errors'].append({'stage': 'conflicting_manifest_copy', 'path': rel})
            records[rel] = checksum
    digest = hashlib.sha256()
    for path, checksum in sorted(records.items()):
        digest.update(path.encode() + b'\0' + checksum.encode() + b'\n')
    return records, digest.hexdigest() if records else None

def current_head():
    marker = CANDIDATE / '.git'
    gitdir = marker if marker.is_dir() else (CANDIDATE / marker.read_text().strip().split('gitdir: ', 1)[1]).resolve()
    value = (gitdir / 'HEAD').read_text().strip()
    if re.fullmatch('[0-9a-f]{40}', value):
        return value
    ref = value.split('ref: ', 1)[1]
    common = (gitdir / (gitdir / 'commondir').read_text().strip()).resolve() if (gitdir / 'commondir').is_file() else gitdir
    for base in [gitdir, common]:
        target = base / ref
        if target.is_file():
            return target.read_text().strip()
        packed = base / 'packed-refs'
        if packed.is_file():
            for line in packed.read_text().splitlines():
                parts = line.split(' ')
                if len(parts) == 2 and parts[1] == ref:
                    return parts[0]
    return None

files = {
    'corrected_binding': PRIVATE / 'owned-bindings-v11-provenance.json',
    'corrected_launcher': PRIVATE / 'run-make-check-v11-provenance.py',
    'revised_artifacts': PREP / 'revised-provenance-artifacts.json',
    'correction_summary': PREP / 'provenance-correction-summary.json',
    'corrected_canonical_manifest': PREP / 'canonical-product-source-manifest.json',
    'preserved_independent_manifest': EVIDENCE / 'direct-source-manifest.json',
    'preserved_artifact_observation': EVIDENCE / 'artifact-actual-observation.json',
    'preserved_verdict': EVIDENCE / 'verdict.json',
    'preserved_discrepancy': EVIDENCE / 'source-binding-discrepancy.json',
    'preserved_original_binding': PRIVATE / 'owned-bindings-v11.json',
}
OUT['files'] = {k: file_hash(p) for k, p in files.items()}
OUT['corrected_file_pins'] = {
    'binding_matches': OUT['files']['corrected_binding'].get('sha256') == EXPECTED['binding'],
    'launcher_matches': OUT['files']['corrected_launcher'].get('sha256') == EXPECTED['launcher'],
    'summary_prefix_matches': OUT['files']['correction_summary'].get('sha256', '').startswith('749aeae4'),
    'canonical_manifest_matches': OUT['files']['corrected_canonical_manifest'].get('sha256') == EXPECTED['canonical_manifest'],
    'original_binding_preserved': OUT['files']['preserved_original_binding'].get('sha256') == 'ccc11e4d7e79a19c964b75d4b2c3354268a4bf28fa7ad153c23aa0a95b30b6ba',
}
data = {k: read_json(p) for k, p in files.items() if p.suffix == '.json'}
OUT['corrected_metadata'] = {k: sanitized(data[k]) for k in ['corrected_binding', 'revised_artifacts', 'correction_summary']}
OUT['identity_field_locations'] = {}
for name in ['corrected_binding', 'revised_artifacts', 'correction_summary']:
    OUT['identity_field_locations'][name] = [{'field': loc, 'value': value} for loc, value in walk(data[name]) if isinstance(value, (str, int)) and value in list(EXPECTED.values())]
new_records, new_digest = product_records(data['corrected_canonical_manifest'])
old_records, old_digest = product_records(data['preserved_independent_manifest'])
OUT['manifest_comparison'] = {'new_count': len(new_records), 'old_count': len(old_records),
    'new_computed_source_id': new_digest, 'old_computed_source_id': old_digest,
    'file_sets_equal': set(new_records) == set(old_records),
    'file_hash_differences': sum(old_records.get(k) != v for k, v in new_records.items()),
    'both_match_canonical': new_digest == old_digest == EXPECTED['product'],
    'both_counts_match': len(new_records) == len(old_records) == EXPECTED['product_count']}
try:
    OUT['current_head'] = current_head()
    OUT['current_head_matches'] = OUT['current_head'] == EXPECTED['head']
except Exception as e:
    OUT['errors'].append({'stage': 'current_head', 'status': type(e).__name__})

cli_path = Path(data['preserved_artifact_observation']['cli_actual']['path'])
OUT['current_cli'] = file_hash(cli_path)
OUT['current_cli']['matches_expected_pin'] = OUT['current_cli'].get('sha256') == EXPECTED['cli']
OUT['current_artifact_roots'] = []
OUT['current_index_file_pins'] = []
for role in ['renderer', 'preload', 'main']:
    for kind in ['installed', 'staged']:
        binding = data['corrected_binding']['desktop'][role][kind]
        root = Path(binding['path'])
        expected_rows = sorted([[row['path'], row['sha256'], row['bytes'], row['mode']] for row in binding['files']], key=lambda row: row[0])
        rows = []
        stable = True
        for p in sorted(root.rglob('*')):
            if p.is_file() and not p.is_symlink():
                row = file_hash(p)
                stable = stable and row.get('read_stable', False)
                rows.append([str(p.relative_to(root)), row['sha256'], row['bytes'], row['mode']])
        expected_map = {row[0]: row[1:] for row in expected_rows}
        actual_map = {row[0]: row[1:] for row in rows}
        prior = next((row for row in data['preserved_artifact_observation']['desktop_actual'] if row['artifact'] == role and row['kind'] == kind), None)
        OUT['current_artifact_roots'].append({'role': role, 'kind': kind, 'root': label(root), 'count': len(rows),
            'expected_count': binding['file_count'], 'record_count_matches': len(rows) == len(expected_rows) == binding['file_count'],
            'file_sets_equal': set(actual_map) == set(expected_map),
            'file_record_differences': sum(actual_map.get(path) != value for path, value in expected_map.items()),
            'normalized_path_sha_size_mode_digest': hashlib.sha256(json.dumps(rows, separators=(',', ':')).encode()).hexdigest(),
            'expected_normalized_path_sha_size_mode_digest': hashlib.sha256(json.dumps(expected_rows, separators=(',', ':')).encode()).hexdigest(),
            'declared_manifest_sha256': binding['manifest_sha256'], 'read_stable': stable,
            'root_matches_preserved_observation': prior is not None and Path(prior['path']) == root,
            'count_matches_preserved_observation': prior is not None and prior['file_count'] == len(rows)})
        index = file_hash(root / ('index.html' if role == 'renderer' else 'index.js'))
        index.update(role=role, kind=kind, matches_expected_pin=index.get('sha256') == EXPECTED['renderer_index' if role == 'renderer' else role])
        OUT['current_index_file_pins'].append(index)
OUT['current_target_file_pins_found'] = {
    'cli': OUT['current_cli']['matches_expected_pin'],
    'renderer_index': all(row['matches_expected_pin'] for row in OUT['current_index_file_pins'] if row['role'] == 'renderer'),
    'preload': all(row['matches_expected_pin'] for row in OUT['current_index_file_pins'] if row['role'] == 'preload'),
    'main': all(row['matches_expected_pin'] for row in OUT['current_index_file_pins'] if row['role'] == 'main'),
}
def changed_fields(a, b, prefix=''):
    if isinstance(a, dict) and isinstance(b, dict):
        result = []
        for key in sorted(set(a) | set(b)):
            if key not in a or key not in b:
                result.append(prefix + '.' + key)
            else:
                result.extend(changed_fields(a[key], b[key], prefix + '.' + key))
        return result
    return [] if a == b else [prefix]
OUT['binding_changed_field_names'] = changed_fields(data['preserved_original_binding'], data['corrected_binding'])
OUT['preserved_report_hashes_after'] = {name: file_hash(files[name]) for name in ['preserved_verdict', 'preserved_discrepancy', 'preserved_independent_manifest', 'preserved_artifact_observation', 'preserved_original_binding']}
OUT['preserved_reports_unchanged'] = all(OUT['files'][name].get('sha256') == row.get('sha256') for name, row in OUT['preserved_report_hashes_after'].items())
OUT['limits'] = ['No original60MB collector read', 'No product/tool/Git command execution', 'No new runtime/fake/SSR/full/real/paid gate proof', 'Exact corrected binding desktop role/kind schema is used, with current disk records and preserved root/count observations', 'Standard manifest comparison reads preserved/corrected records, not a fresh all-source filesystem scan']
OWN.mkdir(parents=True, exist_ok=True)
result = OWN / 'collector-result.json'
result.write_text(json.dumps(OUT, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'status': 'narrow_read_only_collection_complete', 'result': label(result), 'pins': OUT['corrected_file_pins'],
    'manifests': OUT['manifest_comparison'], 'head_matches': OUT.get('current_head_matches'),
    'artifact_target_pins_found': OUT['current_target_file_pins_found'], 'artifact_root_count': len(OUT['current_artifact_roots']),
    'preserved_reports_unchanged': OUT['preserved_reports_unchanged'], 'errors': OUT['errors']}))
