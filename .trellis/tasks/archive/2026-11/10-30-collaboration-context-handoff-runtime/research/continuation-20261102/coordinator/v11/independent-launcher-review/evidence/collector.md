# V11 independent provenance collector

This standard-library collector performs file reads and read-only Git queries. It never invokes a product binary, Provider, package manager, build, test, database, API or network service. It writes only its JSON result beside this document. Git runs with optional locking and prompting disabled. Run once through an owner-controlled working command surface; transport errors are recorded rather than retried.

```python
from __future__ import annotations
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess

ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
CANDIDATE = Path('/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103')
RUNTIME = ROOT / '.trellis/tasks/10-30-collaboration-context-handoff-runtime'
INPUT = ROOT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/owned-candidate-preparation'
OWN = RUNTIME / 'research/continuation-20261102/coordinator/v11/independent-launcher-review/evidence'
HOOK = Path('/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/hook_outputs/01a19e8c-c576-78f0-8cf2-bb870c384442/7bfdffea-c656-4b3e-bffa-0362e225bf09.txt')
PRIVATE = CANDIDATE / '.omx/private/final-check-v11/owned-bindings-v11.json'
SOURCE_ROOTS = ['server', 'apps', 'packages', 'scripts', 'Makefile', 'package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml']
EXPECTED = {
    'head': '304f74dd3ba4ddea862c94917c5bb0770355a351',
    'checkout_source_id': '3f9788baf2ad3d922df86c4817b7fe0f3183e1a5c6c0e40d43fc76cd4f2c1d88',
    'source_fingerprint': '26e38d9e62f8a7cfa8cba243f2387934ee067677b6fa58fde2e12f0026b66920',
    'private_bindings_sha256': 'ccc11e4d7e79a19c964b75d4b2c3354268a4bf28fa7ad153c23aa0a95b30b6ba',
}
OUT = {'schema': 'v11-independent-provenance-readonly-v1', 'expected': EXPECTED, 'errors': []}

def label(path: Path | str) -> str:
    s = str(path)
    for base, replacement in [(CANDIDATE, '<candidate>'), (ROOT, '<root>'), (Path('/Users/artisan'), '<home>')]:
        if s == str(base) or s.startswith(str(base) + '/'):
            return replacement + s[len(str(base)):]
    return s

def file_hash(path: Path) -> dict:
    result = {'path': label(path), 'exists': path.exists(), 'is_symlink': path.is_symlink()}
    if not path.exists():
        return result
    result['resolved_path'] = label(path.resolve())
    if not path.is_file():
        result['status'] = 'directory'
        return result
    before = path.stat()
    h = hashlib.sha256()
    with path.open('rb') as f:
        for block in iter(lambda: f.read(1024 * 1024), b''):
            h.update(block)
    after = path.stat()
    result.update(sha256=h.hexdigest(), bytes=after.st_size,
        read_stable=(before.st_size, before.st_mtime_ns, before.st_ino) == (after.st_size, after.st_mtime_ns, after.st_ino))
    return result

def git(args: list[str]) -> dict:
    env = dict(os.environ, GIT_OPTIONAL_LOCKS='0', GIT_TERMINAL_PROMPT='0')
    try:
        result = subprocess.run(['git', '-c', 'core.fsmonitor=false', '-C', str(CANDIDATE), *args],
            text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, timeout=30, check=False)
        return {'args': args, 'exit_code': result.returncode,
            'stdout': label(result.stdout.strip()), 'stderr_present': bool(result.stderr)}
    except Exception as e:
        return {'args': args, 'status': type(e).__name__}

secret_key = re.compile(r'password|passwd|secret|credential|authorization|(^|_)token($|_)|prompt|stdin|stdout|stderr|body|content|messages|payload|environment', re.I)
safe_key = re.compile(r'path|file|hash|sha|fingerprint|source|commit|head|name|status|version|kind|mode|scope|owner|thread|count|size|bytes|exit|passed|failed|skip|run|test|tool|command|binding|artifact|build|baseline|current|before|after|candidate|expected|actual|match|reason|gate|budget|consum|remaining|timestamp|date|time|host|arch|platform|package|renderer|preload|cli|desktop|node|pnpm|go|identity|installed|staged|production|preserv|change|delta|drift|bundle|worktree|checkout|evidence|record|resolution|source_id|digest', re.I)

def scrub_string(value: str) -> str:
    value = label(value)
    value = re.sub(r'\b(?:mat|mcn|mul|sk)[_-][A-Za-z0-9_\-]{12,}\b', '<redacted>', value)
    value = re.sub(r'(?:postgres(?:ql)?|redis|https?)://[^\s/]*:[^\s/@]+@[^\s]+', '<redacted-uri>', value)
    value = re.sub(r'(?i)(?:bearer\s+)[A-Za-z0-9_.\-]+', 'Bearer <redacted>', value)
    value = re.sub(r'(?i)(--(?:password|passwd|api-key|access-token|token|secret)(?:=|\s+))(?:\"[^\"]*\"|\x27[^\x27]*\x27|[^\s]+)', r'\1<redacted>', value)
    value = re.sub(r'(?i)(\b[A-Z0-9_]*(?:PASSWORD|SECRET|TOKEN|API_KEY|DATABASE_URL|REDIS_URL)[A-Z0-9_]*=)(?:\"[^\"]*\"|\x27[^\x27]*\x27|[^\s]+)', r'\1<redacted>', value)
    value = re.sub(r'\beyJ[A-Za-z0-9_-]{12,}\.[A-Za-z0-9_-]{12,}\.[A-Za-z0-9_-]{12,}\b', '<redacted-jwt>', value)
    return value

def safe_json(value, key=''):
    if secret_key.search(key):
        if isinstance(value, (dict, list)):
            return {'redacted': True, 'count': len(value)}
        return '<redacted>'
    if isinstance(value, dict):
        return {str(k): safe_json(v, str(k)) for k, v in value.items()}
    if isinstance(value, list):
        return [safe_json(v, key) for v in value]
    if value is None or isinstance(value, (bool, int, float)):
        return value
    if isinstance(value, str):
        if safe_key.search(key) or re.fullmatch(r'[0-9a-f]{40}|[0-9a-f]{64}', value) or value.startswith(('/', './', '.trellis/', '.omx/', 'apps/', 'packages/', 'server/', 'scripts/', 'e2e/')):
            return scrub_string(value)
        return '<unselected-string>'
    return '<unselected-value>'

def json_file(path: Path) -> dict:
    result = file_hash(path)
    if path.is_file():
        try:
            result['metadata'] = safe_json(json.loads(path.read_text()))
        except Exception as e:
            result['parse_status'] = type(e).__name__
    return result

def strings(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for v in value.values():
            yield from strings(v)
    elif isinstance(value, list):
        for v in value:
            yield from strings(v)

OUT['git'] = {name: git(args) for name, args in {
    'head': ['rev-parse', 'HEAD'],
    'status': ['status', '--porcelain=v1', '--untracked-files=normal'],
    'log': ['log', '-25', '--format=%H %s'],
    'changed_working': ['diff', '--name-status', 'HEAD'],
    'changed_index': ['diff', '--cached', '--name-status', 'HEAD'],
    'delta_118fc53': ['show', '--format=%H %s', '--name-status', '118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9'],
}.items()}
OUT['head_matches'] = OUT['git']['head'].get('stdout') == EXPECTED['head']
OUT['private_bindings'] = json_file(PRIVATE)
OUT['private_bindings_hash_matches'] = OUT['private_bindings'].get('sha256') == EXPECTED['private_bindings_sha256']
OUT['input_index'] = []
OUT['input_json'] = []
path_references = set()
if INPUT.exists():
    for p in sorted(INPUT.rglob('*')):
        if p.is_file():
            OUT['input_index'].append(file_hash(p))
            if p.suffix == '.json':
                OUT['input_json'].append(json_file(p))
                try:
                    for s in strings(json.loads(p.read_text())):
                        if s.startswith('/') and len(s) < 1024 and '\n' not in s:
                            path_references.add(s)
                except Exception:
                    pass
if PRIVATE.is_file():
    try:
        for s in strings(json.loads(PRIVATE.read_text())):
            if s.startswith('/') and len(s) < 1024 and '\n' not in s:
                path_references.add(s)
    except Exception:
        pass
OUT['referenced_files'] = []
for s in sorted(path_references):
    p = Path(s)
    if p.is_file() and not re.search(r'(?i)\.env(?:\.|$)|password|credential|private-prompt', p.name):
        OUT['referenced_files'].append(file_hash(p))

def declaration_rows(value, origin: Path):
    if isinstance(value, dict):
        candidates = [v for k, v in value.items() if isinstance(v, str) and re.search(r'(?i)(?:^|_)(?:path|file|filename|relative_path|source_path)$', k)]
        claims = {k: v for k, v in value.items() if isinstance(v, str) and re.fullmatch(r'[0-9a-f]{64}', v) and re.search(r'hash|sha|digest', k, re.I)}
        for raw in candidates:
            paths = [Path(raw)] if raw.startswith('/') else [CANDIDATE / raw, ROOT / raw, origin.parent / raw]
            for p in paths:
                if p.is_file() and claims:
                    actual = file_hash(p)
                    yield {'origin': label(origin), 'file': actual, 'claims': claims,
                        'matches': {k: v == actual.get('sha256') for k, v in claims.items()}}
                    break
        for v in value.values():
            yield from declaration_rows(v, origin)
    elif isinstance(value, list):
        for v in value:
            yield from declaration_rows(v, origin)
OUT['file_hash_declarations'] = []
for p in [PRIVATE, *sorted(INPUT.rglob('*.json'))] if INPUT.exists() else [PRIVATE]:
    if p.is_file():
        try:
            OUT['file_hash_declarations'].extend(declaration_rows(json.loads(p.read_text()), p))
        except Exception as e:
            OUT['errors'].append({'stage': 'file_hash_declarations', 'path': label(p), 'status': type(e).__name__})

OUT['workspace'] = []
for app in ['apps/web', 'apps/desktop', 'packages/core', 'packages/views', 'packages/ui']:
    package = CANDIDATE / app / 'package.json'
    row = file_hash(package)
    if package.is_file():
        data = json.loads(package.read_text())
        row['package_metadata'] = {k: data[k] for k in ['name', 'version', 'main', 'module', 'exports', 'dependencies', 'devDependencies'] if k in data}
    row['workspace_links'] = []
    for name in ['core', 'views', 'ui', 'tsconfig', 'eslint-config']:
        row['workspace_links'].append(file_hash(CANDIDATE / app / 'node_modules/@multica' / name))
    OUT['workspace'].append(row)
OUT['workspace_inputs'] = [file_hash(CANDIDATE / rel) for rel in ['package.json', 'pnpm-workspace.yaml', 'pnpm-lock.yaml', 'turbo.json', 'apps/web/next.config.ts', 'apps/desktop/electron.vite.config.ts']]
OUT['ambient_tool_paths'] = {name: file_hash(Path(p)) if (p := shutil.which(name)) else {'status': 'not_found'} for name in ['go', 'node', 'pnpm']}

tracked = git(['ls-files', '--cached', '--others', '--exclude-standard', '-z', '--', *SOURCE_ROOTS])
OUT['tracked_sources'] = []
if tracked.get('exit_code') == 0:
    digest = hashlib.sha256()
    for rel in sorted(set(tracked.get('stdout', '').split('\x00')) - {''}):
        p = CANDIDATE / rel
        if p.is_file() and not p.is_symlink():
            row = file_hash(p)
            row['relative_path'] = rel
            OUT['tracked_sources'].append(row)
            digest.update(rel.encode('utf-8') + b'\0' + row['sha256'].encode('ascii') + b'\n')
    OUT['source_manifest'] = {'roots': SOURCE_ROOTS, 'file_count': len(OUT['tracked_sources']),
        'algorithm': 'sorted_relative_path_NUL_file_sha256_LF', 'checkout_source_id': digest.hexdigest(),
        'matches_expected': digest.hexdigest() == EXPECTED['checkout_source_id'],
        'file_selection': 'git_ls_files_cached_others_exclude_standard_skip_symlink_nonfile'}
    OUT['e2e_source_binding'] = file_hash(CANDIDATE / 'e2e/collaboration-host-runtime.spec.ts')
else:
    OUT['errors'].append({'stage': 'tracked_sources', 'status': tracked.get('status', 'git_failed')})

snapshot_paths = git(['ls-files', '-z'])
OUT['tracked_snapshot_entries'] = []
if snapshot_paths.get('exit_code') == 0:
    for rel in snapshot_paths.get('stdout', '').split('\x00'):
        if not rel:
            continue
        p = CANDIDATE / rel
        try:
            metadata = p.lstat()
            checksum = hashlib.sha256(os.readlink(p).encode()).hexdigest() if stat.S_ISLNK(metadata.st_mode) else file_hash(p)['sha256']
            OUT['tracked_snapshot_entries'].append({'path': rel, 'sha256': checksum, 'mode': stat.S_IMODE(metadata.st_mode)})
        except Exception as e:
            OUT['errors'].append({'stage': 'tracked_snapshot_entries', 'path': label(p), 'status': type(e).__name__})
    encoded = json.dumps(OUT['tracked_snapshot_entries'], sort_keys=True, separators=(',', ':')).encode()
    snapshot_digest = hashlib.sha256(encoded).hexdigest()
    OUT['tracked_snapshot'] = {'algorithm': 'git_tracked_order_path_sha256_lstat_mode_json_sorted_keys_compact_sha256',
        'file_count': len(OUT['tracked_snapshot_entries']), 'tracked_source_fingerprint': snapshot_digest,
        'matches_expected': snapshot_digest == EXPECTED['source_fingerprint'],
        'symlink_rule': 'sha256_readlink_utf8_bytes', 'file_selection': 'all_git_tracked_in_git_returned_order'}
else:
    OUT['errors'].append({'stage': 'tracked_snapshot', 'status': snapshot_paths.get('status', 'git_failed')})

OUT['context_index'] = []
OUT['selected_context'] = []
context_paths = [RUNTIME / p for p in ['prd.md', 'design.md', 'implement.md', 'check.jsonl']]
for base in [RUNTIME, ROOT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes']:
    if not base.exists():
        continue
    for p in sorted(base.rglob('*')):
        if not p.is_file() or OWN in p.parents:
            continue
        rel = str(p.relative_to(base)).lower()
        if p.name in ['source_manifest.py', 'source-snapshot.py'] or (p.suffix in ['.md', '.json'] and any(k in rel for k in ['registration', 'addendum', 'rki6', 'independent', 'launch', 'source-manifest', 'source_manifest'])):
            OUT['context_index'].append(file_hash(p))
            if p.name in ['source_manifest.py', 'source-snapshot.py'] or (p.suffix == '.md' and p.stat().st_size <= 80000 and any(k in rel for k in ['rki6', 'registration', 'addendum'])):
                context_paths.append(p)
for raw in sorted(path_references):
    p = Path(raw)
    if p.is_file() and p.name in ['source_manifest.py', 'source-snapshot.py']:
        context_paths.append(p)
for p in sorted(INPUT.rglob('source-snapshot.py')) if INPUT.exists() else []:
    context_paths.append(p)
for p in sorted(set(context_paths)):
    if p.is_file():
        # Requirements, registered limits and manifest implementation only;
        # never read private Provider payloads or credential files.
        text = p.read_text(errors='replace')
        OUT['selected_context'].append({'path': label(p), 'sha256': file_hash(p).get('sha256'), 'text': scrub_string(text)})
OUT['hook'] = file_hash(HOOK)
if HOOK.is_file():
    lines = HOOK.read_text(errors='replace').splitlines()
    OUT['hook']['line_count'] = len(lines)
    OUT['hook']['unread_ranges'] = [{'range': [lo, hi], 'text': scrub_string('\n'.join(lines[lo-1:hi]))} for lo, hi in [(451, 900), (1351, len(lines))]]

OUT['git_after'] = {'head': git(['rev-parse', 'HEAD']), 'status': git(['status', '--porcelain=v1', '--untracked-files=normal'])}
OUT['candidate_head_and_status_stable'] = all(OUT['git'][k].get('exit_code') == 0 and OUT['git_after'][k].get('exit_code') == 0 and OUT['git'][k].get('stdout') == OUT['git_after'][k].get('stdout') for k in ['head', 'status'])

OWN.mkdir(parents=True, exist_ok=True)
result_path = OWN / 'collector-result.json'
result_path.write_text(json.dumps(OUT, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'status': 'read_only_collection_complete', 'result_path': label(result_path), 'head_matches': OUT['head_matches'], 'private_bindings_hash_matches': OUT['private_bindings_hash_matches'], 'product_source_matches': OUT.get('source_manifest', {}).get('matches_expected'), 'tracked_snapshot_matches': OUT.get('tracked_snapshot', {}).get('matches_expected'), 'candidate_head_and_status_stable': OUT['candidate_head_and_status_stable'], 'input_files': len(OUT['input_index']), 'source_files': len(OUT['tracked_sources']), 'context_files': len(OUT['selected_context']), 'errors': OUT['errors']}, ensure_ascii=False))
```
