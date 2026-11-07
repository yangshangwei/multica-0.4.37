"""Read actual frozen inputs; no application, database, or test execution."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import types
from urllib.parse import urlsplit

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = ROOT / '.omx/private/final-check-v14'


def digest(path):
    with Path(path).open('rb') as file:
        return hashlib.file_digest(file, 'sha256').hexdigest()


def read(path):
    return json.loads(Path(path).read_text())


launcher = PRIVATE / 'run-make-check-v14-provenance.py'
module = types.ModuleType('reviewed_v14_launcher_definitions')
module.__file__ = str(launcher)
exec(compile(launcher.read_bytes(), str(launcher), 'exec'), module.__dict__)
envfile = PRIVATE / 'check-source-v14.env'
values = module.parse_generated_environment(envfile.read_text())
clearance = {'bindings_sha256': digest(PRIVATE / 'owned-bindings-v14-provenance.json'),
             'resource_ownership_sha256': digest(PRIVATE / 'resource-ownership-v14.json')}
bindings = module.verify_owned_bindings(ROOT, PRIVATE, clearance, values)
head = subprocess.check_output(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip()
status = subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain', '--untracked-files=all'], text=True)
assert head == 'd0b132fff0c477eb2d744933e13db730240231aa' and status == ''
source = bindings['source']
assert digest(source['checkout_source_id_helper']) == source['checkout_source_id_helper_sha256']
node = bindings['tools']['node']['path']
metadata_env = {'PATH': str(Path(node).parent) + ':/usr/bin:/bin', 'HOME': str(HERE / 'home')}
actual_development_id = subprocess.check_output([node, source['checkout_source_id_helper'], str(ROOT)], env=metadata_env, text=True)
assert actual_development_id == source['checkout_source_id']
desktop = []
for name, item in bindings['desktop'].items():
    installed = [(row['path'], row['sha256']) for row in item['installed']['files']]
    staged = [(row['path'], row['sha256']) for row in item['staged']['files']]
    assert installed == staged
    desktop.append({'name': name, 'files': len(installed), 'installed_and_staged_equal': True})
references = {}


def check(path, expected):
    actual = digest(path)
    assert actual == expected, str(path)
    references[str(path)] = {'sha256': actual, 'matches': True}


handoff = read(HERE.parent / 'handoff-ready-for-review.json')


def verify_references(value):
    if isinstance(value, dict):
        if 'path' in value and 'sha256' in value:
            check(value['path'], value['sha256'])
        for child in value.values():
            verify_references(child)
    elif isinstance(value, list):
        for child in value:
            verify_references(child)


verify_references(handoff)
resource = read(PRIVATE / 'resource-ownership-v14.json')
verify_references(resource)
root_proof = read(HERE.parent / 'root-resource-confirmation.json')
assert root_proof['resource_ownership_sha256'] == clearance['resource_ownership_sha256']
assert root_proof['candidate_sha'] == head and root_proof['exclusive_complete_window'] is True
assert resource['coordinator'] == '01a1a186-9a5c-7b00-96f8-0430b47edb4c'
assert urlsplit(values['DATABASE_URL']).path == '/' + resource['seed_database']
assert values['REDIS_TEST_URL'] == 'redis://127.0.0.1:55131/0'
assert values['COLLABORATION_ELECTRON_RENDERER_URL'] == resource['renderer']['url'] == 'http://127.0.0.1:55132/'
assert all(values.get(key) == value for key, value in {
    'MULTICA_AUTH_MODE': 'legacy', 'MULTICA_DEVICE_AUTH_ENABLED': 'false',
    'MULTICA_KNOWLEDGE_CONTEXT_ENABLED': 'false', 'MULTICA_MANAGED_INSTALLATIONS_ENABLED': 'false',
    'MULTICA_PLATFORM_ADMIN_ENABLED': 'false'}.items())
assert not values.get('MULTICA_LLM_API_KEY') and not values.get('MULTICA_LLM_BASE_URL')
assert all('$' not in value and all(c.isprintable() for c in value) for value in values.values())
for name in ['make-check-v14.json', 'make-check-v14.log', 'check-source-v14.safe.env',
             'full-check-home-v14', 'go-packages-v14', 'dev-registry-v14', 'profiles-v14', 'workspaces-v14']:
    assert not (PRIVATE / name).exists() and not (PRIVATE / name).is_symlink(), name
for kind in ('cli', 'desktop'):
    path = Path(handoff['builds'][kind]['path'])
    record = read(path)
    assert record['exit_code'] == 0
    references[str(path)]['exit_code'] = record['exit_code']
old_adapter = Path('/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v9/check-bin')
assert all(digest(old_adapter / name) == digest(PRIVATE / 'check-bin' / name) for name in ('go', 'pnpm'))
record = {
    'recorded_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'status': 'pass',
    'candidate_sha': head, 'candidate_status': status, 'candidate_root': str(ROOT),
    'launcher_sha256': digest(launcher), 'adapter_sha256': {name: digest(PRIVATE / 'check-bin' / name) for name in ('go', 'pnpm')},
    'source_env_sha256': digest(envfile), **clearance,
    'tracked_file_count': len(source['tracked_files']), 'tracked_fingerprint': source['tracked_fingerprint'],
    'canonical_product_source_id': source['product_source']['source_id'], 'product_files': source['product_source']['file_count'],
    'development_source_id': actual_development_id,
    'workspace_edges': len(bindings['workspace']['edges']), 'workspace_exports': len(bindings['workspace']['graph']['exports']),
    'actual_workspace_resolution_matches': True, 'owned_cli': bindings['cli'], 'desktop': desktop,
    'root_resource_confirmation': {'path': str(HERE.parent / 'root-resource-confirmation.json'), 'sha256': digest(HERE.parent / 'root-resource-confirmation.json'),
                                   'live_observer': resource['root_observer'], 'revalidated_live_by_reviewer': False},
    'seed_database_name': resource['seed_database'], 'renderer_origin': resource['renderer']['url'],
    'redis_port': resource['redis']['port'], 'environment_is_data_and_make_safe': True,
    'real_model_config_absent': True, 'production_knowledge_default_false': True,
    'all_runtime_writable_state_absent': True, 'donor_adapter_used_by_fake_tests_matches_actual': True,
    'reference_checks': references,
    'operations': {'application_DB_API_Provider': False, 'fullmake_or_Go_tests': False, 'candidate_source_edits': False,
                   'only_readonly_git_and_Node_metadata_executed': True},
}
with (HERE / 'actual-binding-verification.json').open('x') as output:
    json.dump(record, output, indent=2)
    output.write('\n')
print(json.dumps({key: record[key] for key in ('status', 'candidate_sha', 'tracked_file_count', 'product_files', 'workspace_edges', 'workspace_exports', 'desktop')}))
