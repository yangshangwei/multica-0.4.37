"""Materialize only frozen-source V16 core inputs; leave clearance/wrapper inert."""
import ast
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import shutil
import socket
import stat
import subprocess
import tempfile
import types
import uuid
from urllib.parse import urlsplit, urlunsplit


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v16'
HERE = Path(__file__).resolve().parent
EXPECTED = 'eda0d68fb07852b714f3092aa02ca0523d3ad312'
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
DATABASE = 'multica_closure_v16_seed_01a1a3c9'


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def save(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')


assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip() == EXPECTED
assert not subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).strip()
assert digest(HERE / 'handoff.json') == 'feeb4773ac7788f5be63c9d1f546d2b02c9f118ed5d7d6aa8790558945a66a28'
assert digest(HERE / 'static-independent-review/review.json') == '9ec55551df6f51626ba094504083a123bb8694b5539c35e6ff21509e62893a49'
manifest = json.loads((HERE / 'prepared-template-manifest.json').read_text())
assert len(manifest['files']) == 26
protected = {**manifest['protected_v15'], **manifest['protected_CCG15']}
for path, checksum in protected.items():
    assert digest(path) == checksum, 'Historical protected input drift'
for row in manifest['files']:
    assert digest(row['prepared_template']) == row['prepared_sha256'], 'Prepared template drift'
temporary = Path(tempfile.mkdtemp(prefix='multica-final-check-v16-', dir='/private/tmp'))
temporary.chmod(0o700)
assert stat.S_IMODE(temporary.stat().st_mode) == 0o700 and temporary.stat().st_uid == os.geteuid()
assert subprocess.run(['git', '-C', str(temporary), 'rev-parse', '--show-toplevel'], capture_output=True).returncode != 0
for child in ['cli-build', 'desktop-build']:
    (temporary / child).mkdir(mode=0o700)
with socket.socket() as renderer, socket.socket() as redis:
    renderer.bind(('127.0.0.1', 0))
    redis.bind(('127.0.0.1', 0))
    renderer_port, redis_port = renderer.getsockname()[1], redis.getsockname()[1]
old_values = {}
for line in (ROOT / '.omx/private/final-check-v15/check-source-v15.env').read_text().splitlines():
    if line and not line.startswith('#'):
        key, raw = line.split('=', 1)
        parts = shlex.split(raw)
        old_values[key] = parts[0] if parts else ''
url = urlsplit(old_values['DATABASE_URL'])
assert url.hostname in ('127.0.0.1', 'localhost') and url.port == 5432
deployment = str(uuid.uuid4())
replacements = {
    'PENDING_ROOT_FINAL_SHA': EXPECTED,
    'PENDING_ROOT_RENDERER_PORT': str(renderer_port),
    'PENDING_ROOT_REDIS_PORT': str(redis_port),
    'PENDING_ROOT_BUILD_TEMP_DIRECTORY': str(temporary),
    'PENDING_ROOT_DATABASE_URL': urlunsplit((url.scheme, url.netloc, '/' + DATABASE, url.query, '')),
    'PENDING_ROOT_POSTGRES_PASSWORD': old_values['POSTGRES_PASSWORD'],
    'PENDING_ROOT_PRIVATE_JWT_SECRET': secrets.token_hex(32),
    'PENDING_ROOT_DEPLOYMENT_UUID': deployment,
}
rows, deferred = [], []
for row in manifest['files']:
    if not row.get('materialize') or row.get('materialization_phase') == 'after_independent_review_and_root_clearance':
        deferred.append({'template': row['prepared_template'], 'final_path': row['final_path'], 'reason': 'unissued review/clearance execution phase'})
        continue
    path = Path(row['final_path'])
    assert not path.exists() and not path.is_symlink(), 'Preserve prior or partial core materialization'
    text = Path(row['prepared_template']).read_text()
    changes = {}
    for key, value in replacements.items():
        if key in text:
            changes[key] = text.count(key)
            text = text.replace(key, value)
    assert not re.search(r'PENDING_ROOT_[A-Z_]+', text), 'Pending operative input'
    if path.suffix == '.py' or path.parent.name == 'check-bin':
        ast.parse(text)
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, int(row['mode'], 8))
    with os.fdopen(descriptor, 'w') as stream:
        stream.write(text)
    assert stat.S_IMODE(path.stat().st_mode) == int(row['mode'], 8)
    rows.append({'template': row['prepared_template'], 'template_sha256': row['prepared_sha256'],
                 'path': str(path), 'sha256': digest(path), 'mode': row['mode'], 'replacement_counts': changes})
assert len(rows) == 24 and len(deferred) == 2
module = types.ModuleType('v16_data_parser')
module.__file__ = str(OUT / 'run-make-check-v16-provenance.py')
exec(compile(Path(module.__file__).read_bytes(), module.__file__, 'exec'), module.__dict__)
values = module.parse_generated_environment((OUT / 'check-source-v16.env').read_text())
assert module.deployment_uuid(values) == deployment and values['POSTGRES_DB'] == DATABASE
assert all(values[key] == 'false' for key in ['MULTICA_KNOWLEDGE_CONTEXT_ENABLED', 'MULTICA_DEVICE_AUTH_ENABLED',
                                             'MULTICA_MANAGED_INSTALLATIONS_ENABLED', 'MULTICA_PLATFORM_ADMIN_ENABLED'])
assert values['MULTICA_AUTH_MODE'] == 'legacy' and values['MULTICA_LLM_API_KEY'] == values['MULTICA_LLM_BASE_URL'] == ''
fresh = [OUT / n for n in ['full-check-home-v16', 'go-packages-v16', 'dev-registry-v16', 'profiles-v16',
                           'workspaces-v16', 'metadata-home-not-created', 'go-build-cache']]
assert all(not p.exists() and not p.is_symlink() for p in fresh)
for name in ['go-path', 'node-cache', 'build-home-cli', 'build-home-desktop']:
    (OUT / name).mkdir(mode=0o700)
shutil.copytree(ROOT / '.omx/private/final-check-v15/catalog', OUT / 'catalog', copy_function=shutil.copy2)
for path in [OUT / 'catalog', *(p for p in (OUT / 'catalog').rglob('*') if p.is_dir())]:
    assert not path.is_symlink()
    path.chmod(0o700)
(OUT / 'node_modules').symlink_to(ROOT / 'node_modules', target_is_directory=True)
assert (OUT / 'node_modules').resolve() == (ROOT / 'node_modules').resolve()
allocation = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'owner': OWNER, 'candidate_sha': EXPECTED,
              'directory': str(temporary), 'uid': temporary.stat().st_uid, 'mode': '0700', 'git_root_absent': True,
              'free_bytes': shutil.disk_usage(temporary).free, 'children': ['cli-build', 'desktop-build'],
              'fresh_paths_absent': [str(p) for p in fresh], 'COW_GOCACHE_pending_absent': True,
              'knowledge_budget_allocated': 0, 'knowledge_calls': 0}
save(OUT / 'build-temp-allocation-v16.json', allocation)
save(HERE / 'materialization.json', {'recorded_at': datetime.now(timezone.utc).isoformat(), 'owner': OWNER,
     'executor': '/root/shared_executor', 'candidate_sha': EXPECTED, 'candidate_root': str(ROOT),
     'manifest_sha256': digest(HERE / 'prepared-template-manifest.json'), 'rows': rows, 'deferred': deferred,
     'protected_inputs': protected, 'renderer_port': renderer_port, 'redis_port': redis_port,
     'deployment_uuid': deployment, 'database': DATABASE, 'allocation_sha256': digest(OUT / 'build-temp-allocation-v16.json'),
     'fullcheck_started': False, 'clearance_issued': False, 'knowledge_budget_allocated': 0, 'knowledge_calls': 0})
print(json.dumps({'core_files': len(rows), 'deferred': len(deferred), 'candidate': EXPECTED, 'database': DATABASE,
                  'renderer_port': renderer_port, 'redis_port': redis_port, 'temporary': str(temporary),
                  'deployment_uuid': deployment, 'go_cache_absent': not (OUT / 'go-build-cache').exists(), 'fullcheck_started': False}))
