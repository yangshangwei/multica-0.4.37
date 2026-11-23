"""Materialize retained v15 templates for the recorded candidate; never run Make."""
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
from urllib.parse import urlsplit, urlunsplit


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PROJECT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
HERE = Path(__file__).resolve().parent
OLD_HERE = PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/progress-recheck-20261103-01a1a1c6-97cddb08/v15-preparation'
OUT = ROOT / '.omx/private/final-check-v15'
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
OLD_OWNER = '01a1a1c6-9f9b-7521-8341-f57c97cddb08'
EXPECTED = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
DATABASE = 'multica_closure_v15_seed_01a1a3c9'


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def save(path, value, mode=0o600):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    with os.fdopen(descriptor, 'w') as target:
        json.dump(value, target, indent=2)
        target.write('\n')


def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT, text=True).strip()


assert git('rev-parse', 'HEAD') == EXPECTED and not git('status', '--porcelain')
assert HERE == PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261104/v15-preparation'
assert not (HERE / 'materialization.json').exists()
manifest_path = OLD_HERE / 'prepared-template-manifest.json'
manifest = json.loads(manifest_path.read_text())
assert len(manifest['files']) == 22
for path, checksum in manifest['protected_donor_hashes'].items():
    assert digest(path) == checksum, 'Protected donor drift'
for row in manifest['files']:
    assert digest(row['prepared_template']) == row['prepared_sha256'], 'Protected template drift'
    final = Path(row['final_path'].replace(str(OLD_HERE), str(HERE)))
    assert row['materialize'] is False or not final.exists(), 'Preserve prior materialized file'

temporary = Path(tempfile.mkdtemp(prefix='multica-final-check-v15-', dir='/private/tmp'))
temporary.chmod(0o700)
assert not temporary.is_symlink() and temporary.stat().st_uid == os.geteuid()
assert stat.S_IMODE(temporary.stat().st_mode) == 0o700
git_probe = subprocess.run(['git', '-C', str(temporary), 'rev-parse', '--show-toplevel'], capture_output=True, text=True)
assert git_probe.returncode != 0
for phase in ('cli-build', 'desktop-build'):
    (temporary / phase).mkdir(mode=0o700)
free = shutil.disk_usage(temporary).free
assert free > 5 * 1024**3

with socket.socket() as renderer_socket, socket.socket() as redis_socket:
    renderer_socket.bind(('127.0.0.1', 0))
    redis_socket.bind(('127.0.0.1', 0))
    renderer_port = renderer_socket.getsockname()[1]
    redis_port = redis_socket.getsockname()[1]
assert renderer_port != redis_port

# Load retained task configuration as data, never shell-source it.
old_values = {}
for line in (ROOT / '.omx/private/final-check-v14/check-source-v14.env').read_text().splitlines():
    if line.strip() and not line.lstrip().startswith('#'):
        key, raw = line.split('=', 1)
        tokens = shlex.split(raw)
        assert len(tokens) <= 1
        old_values[key] = tokens[0] if tokens else ''
url = urlsplit(old_values['DATABASE_URL'])
assert url.scheme in ('postgres', 'postgresql') and url.hostname in ('localhost', '127.0.0.1')
assert url.port == 5432 and url.username == 'multica' and url.password == old_values['POSTGRES_PASSWORD']
database_url = urlunsplit((url.scheme, url.netloc, '/' + DATABASE, url.query, url.fragment))
replacements = {
    'PENDING_ROOT_FINAL_SHA': EXPECTED,
    'PENDING_ROOT_RENDERER_PORT': str(renderer_port),
    'PENDING_ROOT_REDIS_PORT': str(redis_port),
    'PENDING_ROOT_PRIVATE_JWT_SECRET': secrets.token_hex(32),
    'PENDING_ROOT_DATABASE_URL': database_url,
    'PENDING_ROOT_BUILD_TEMP_DIRECTORY': str(temporary),
    str(OLD_HERE): str(HERE),
    OLD_OWNER: OWNER,
    'multica_closure_v15_seed_01a1a1c6': DATABASE,
    'multica-v15-01a1a1c6-redis': 'multica-v15-01a1a3c9-redis',
}
rows = []
for row in manifest['files']:
    source = Path(row['prepared_template'])
    destination = Path(row['final_path'].replace(str(OLD_HERE), str(HERE)))
    content = source.read_text()
    changes = {}
    if row['materialize']:
        for key, value in replacements.items():
            if key in content:
                changes[key] = content.count(key)
                content = content.replace(key, value)
        assert not re.search(r'PENDING_ROOT_[A-Z_]+', content), 'Pending operative value'
        if destination.suffix == '.py':
            ast.parse(content, filename=str(destination))
        destination.parent.mkdir(parents=True, exist_ok=True)
        descriptor = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL, int(row['mode'], 8))
        with os.fdopen(descriptor, 'w') as target:
            target.write(content)
    else:
        assert source == destination and digest(destination) == row['prepared_sha256']
    assert stat.S_IMODE(destination.stat().st_mode) == int(row['mode'], 8)
    rows.append({'template': str(source), 'template_sha256': row['prepared_sha256'],
                 'original_final_path': row['final_path'], 'final_path': str(destination),
                 'final_sha256': digest(destination), 'mode': row['mode'],
                 'materialized': row['materialize'], 'replacement_occurrences': changes})

launcher = types.ModuleType('v15_data_parser')
launcher.__file__ = str(OUT / 'run-make-check-v15-provenance.py')
exec(compile(Path(launcher.__file__).read_bytes(), launcher.__file__, 'exec'), launcher.__dict__)
values = launcher.parse_generated_environment((OUT / 'check-source-v15.env').read_text())
assert launcher.deployment_uuid(values) == '725f8e4c-8979-4fc2-b105-36a6ce89a2e5'
assert values['DATABASE_URL'] == database_url and values['POSTGRES_DB'] == DATABASE
for key in ('MULTICA_KNOWLEDGE_CONTEXT_ENABLED', 'MULTICA_DEVICE_AUTH_ENABLED',
            'MULTICA_MANAGED_INSTALLATIONS_ENABLED', 'MULTICA_PLATFORM_ADMIN_ENABLED'):
    assert values[key] == 'false'
assert values['MULTICA_AUTH_MODE'] == 'legacy'
assert values['MULTICA_LLM_API_KEY'] == values['MULTICA_LLM_BASE_URL'] == ''

fresh_paths = [OUT / name for name in ('full-check-home-v15', 'go-packages-v15', 'dev-registry-v15',
                                      'profiles-v15', 'workspaces-v15', 'metadata-home-not-created')]
assert all(not path.exists() and not path.is_symlink() for path in fresh_paths)
for name in ('go-build-cache', 'go-path', 'node-cache', 'build-home-cli', 'build-home-desktop'):
    (OUT / name).mkdir(mode=0o700)
shutil.copytree(ROOT / '.omx/private/final-check-v14/catalog', OUT / 'catalog', copy_function=shutil.copy2)
for path in [OUT / 'catalog', *(path for path in (OUT / 'catalog').rglob('*') if path.is_dir())]:
    assert not path.is_symlink()
    path.chmod(0o700)
link = OUT / 'node_modules'
link.symlink_to(ROOT / 'node_modules', target_is_directory=True)
assert link.resolve() == (ROOT / 'node_modules').resolve()
assert (OUT / 'electron.vite.config.cts').read_bytes() == (ROOT / 'apps/desktop/electron.vite.config.ts').read_bytes()
assert not git('status', '--porcelain')
allocation = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'owner': OWNER,
              'candidate_sha': EXPECTED, 'directory': str(temporary), 'mode': '0700',
              'uid': temporary.stat().st_uid, 'git_root_absent': True,
              'git_probe_exit_code': git_probe.returncode, 'free_bytes': free,
              'children': ['cli-build', 'desktop-build'], 'fresh_execution_paths_absent': [str(path) for path in fresh_paths],
              'node_modules_link': {'path': str(link), 'target': str(ROOT / 'node_modules')},
              'real_provider_allowed': False, 'knowledge_budget_allocated': 0, 'knowledge_calls': 0}
save(OUT / 'build-temp-allocation-v15.json', allocation)
save(HERE / 'materialization.json', {'recorded_at': datetime.now(timezone.utc).isoformat(),
     'owner': OWNER, 'executor': '/root/shared_executor', 'candidate_root': str(ROOT), 'candidate_sha': EXPECTED,
     'manifest_path': str(manifest_path), 'manifest_sha256': digest(manifest_path), 'files': rows,
     'protected_donors': manifest['protected_donor_hashes'], 'replacement_keys': list(replacements),
     'renderer_port': renderer_port, 'redis_port': redis_port, 'database': DATABASE,
     'source_env_sha256': digest(OUT / 'check-source-v15.env'),
     'allocation_sha256': digest(OUT / 'build-temp-allocation-v15.json'),
     'candidate_clean': True, 'complete_check_started': False, 'knowledge_budget_allocated': 0, 'knowledge_calls': 0})
print(json.dumps({'candidate_sha': EXPECTED, 'operative_files': len(rows), 'database': DATABASE,
                  'renderer_port': renderer_port, 'redis_port': redis_port, 'temporary': str(temporary),
                  'mode': '0700', 'git_root_absent': True, 'free_bytes': free, 'complete_check_started': False}))
