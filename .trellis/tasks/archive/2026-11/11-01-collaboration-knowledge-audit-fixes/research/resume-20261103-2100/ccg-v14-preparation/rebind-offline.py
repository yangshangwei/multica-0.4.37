from pathlib import Path
from datetime import datetime, timezone
import ast
import hashlib
import json

HERE = Path(__file__).resolve().parent
MAIN = HERE.parents[4]
TASK = HERE.parents[2]
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OLD = ROOT / '.omx/private/knowledge-final-gates-v13'
PRIVATE = ROOT / '.omx/private/knowledge-final-gates-v14'
EXPECTED = 'd0b132fff0c477eb2d744933e13db730240231aa'
OWNER = '01a1a186-9a5c-7b00-96f8-0430b47edb4c'
OLD_SHA = '85ef5d20578f63c7c4f879f035acbd414e80f306'
OLD_EVIDENCE = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/execution-20261103/final-candidate')
NEW_EVIDENCE = HERE.parent / 'candidate-v14'
PRIOR_REVIEW = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/execution-20261103/ccg-current-source-review/review.json')

def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

def write(path, value):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    with path.open('x') as target:
        target.write(value)
    path.chmod(0o600)

def encode(value):
    return json.dumps(value, indent=2) + '\n'

def rebind(text):
    return (text.replace(str(OLD), str(PRIVATE)).replace(OLD_SHA, EXPECTED)
            .replace(str(OLD_EVIDENCE), str(NEW_EVIDENCE))
            .replace('knowledge-final-gates-v13', 'knowledge-final-gates-v14')
            .replace('final-check-v13', 'final-check-v14').replace('make-check-v13', 'make-check-v14')
            .replace('final-state-audit-v13', 'final-state-audit-v14')
            .replace('/private/tmp/multica-ccg-v13-', '/private/tmp/multica-ccg-v14-')
            .replace('knowledge-v13-', 'knowledge-v14-'))

prior = json.loads(PRIOR_REVIEW.read_text())
assert not PRIVATE.exists(), 'Preserve any earlier v14 preparation'
originals = {str(path): digest(path) for path in sorted(OLD.rglob('*')) if path.is_file()}
assert len(originals) == 13
for path, sha in originals.items():
    assert prior['reviewed_files'][path]['sha256'] == sha
contents = {str(Path(path).relative_to(OLD)): rebind(Path(path).read_text()) for path in originals}
inputs = json.loads(contents['source-inputs.json'])
inputs['state'] = 'prepared_pending_final_audit_review_and_later_root_permission'
inputs['coordinator'] = OWNER
for item in inputs['tools'].values():
    item['sha256'] = digest(item['path'])
for name in inputs['source_sha256']:
    inputs['source_sha256'][name] = digest(ROOT / name)
inputs['source_sha256']['server/internal/auth/password_config.go'] = digest(ROOT / 'server/internal/auth/password_config.go')
for name in ['audit_module', 'environment_parser']:
    inputs[name]['sha256'] = digest(inputs[name]['path'])
review = Path(inputs['audit_review']['path'])
inputs['audit_review']['sha256'] = digest(review) if review.exists() else None
inputs['audit_review']['state'] = 'current_review_present_requires_root_finalization' if review.exists() else 'pending_final_independent_review'
binding = json.loads((ROOT / '.omx/private/final-check-v14/owned-bindings-v14-provenance.json').read_text())
renderer = binding['renderer_url'].rstrip('/')
inputs['forbidden_renderer_origins'] = [renderer, renderer.replace('127.0.0.1', 'localhost')]
contents['source-inputs.json'] = encode(inputs)
for relative, content in contents.items():
    if relative.endswith('.py') or relative.endswith('.py.txt'):
        ast.parse(content)
    if relative.endswith('source-values.json.template'):
        values = json.loads(content)
        values['MULTICA_PASSWORD_LIMITER_MODE'] = 'single'
        content = encode(values)
    if relative.startswith('permissions/'):
        permit = json.loads(content)
        permit.update(ready_to_execute=False, coordinator=OWNER,
                      source_inputs_sha256=hashlib.sha256(contents['source-inputs.json'].encode()).hexdigest(),
                      runner_sha256=hashlib.sha256(contents['run-ccg1.py' if permit['gate'].startswith('ccg1') else 'run-ccg2.py'].encode()).hexdigest())
        assert not any(permit.get(key) for key in ['root_released_complete_check_resources', 'root_verified_complete_named_inventory', 'root_verified_fresh_database'])
        assert permit['database'] is None and permit['DATABASE_URL'] is None and permit['real_provider_allowed'] is False
        content = encode(permit)
    if relative.endswith('.json') or relative.endswith('.json.template'):
        json.loads(content)
    write(PRIVATE / relative, content)
manifest = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'candidate_root': str(ROOT), 'candidate_sha': EXPECTED,
            'coordinator': OWNER, 'state': 'offline_prepared_not_runnable', 'original_files': originals,
            'prepared_files': {str(p): {'sha256': digest(p), 'bytes': p.stat().st_size} for p in sorted(PRIVATE.rglob('*')) if p.is_file()},
            'source_inputs_sha256': digest(PRIVATE / 'source-inputs.json'),
            'source_binding_count': len(inputs['source_sha256']), 'audit_review_state': inputs['audit_review']['state'],
            'audit_review_hash_bound': inputs['audit_review']['sha256'] is not None,
            'template_correction': {'field': 'MULTICA_PASSWORD_LIMITER_MODE', 'value': 'single', 'scope': 'one task-owned API per serial OFF/ON deployment',
                                    'reason': 'password_config.go rejects an unset mode; dev-env.sh does not fill it; no current deployment changed'},
            'fullcheck_renderer_forbidden_origins': inputs['forbidden_renderer_origins'],
            'reference_template_note': 'ccg-admission-template.py.txt is retained historical preparation material; execute and review actual run-ccg1.py/run-ccg2.py, which retain raw symlink checks',
            'fullrun_or_CCG_resources_or_binaries_or_accounts_allocated': False, 'ready_permits': 0,
            'Go_UI_Playwright_DB_API_Web_renderer_Redis_Provider_invocations': 0,
            'production_knowledge_enabled': False, 'knowledge_budget_allocated': 0, 'knowledge_real_calls': 0}
assert all(digest(path) == sha for path, sha in originals.items())
write(HERE / 'preparation.json', encode(manifest))
print(json.dumps({'prepared_files': len(manifest['prepared_files']), 'source_inputs': len(inputs['source_sha256']),
                  'audit_review_hash_bound': manifest['audit_review_hash_bound'], 'ready_permits': 0, 'source_inputs_sha256': manifest['source_inputs_sha256']}))
