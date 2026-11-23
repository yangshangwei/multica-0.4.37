"""Prepare the thirteen CCG16 inputs; never execute or grant a product gate."""

import ast
from datetime import datetime, timezone
import difflib
import hashlib
import json
import os
from pathlib import Path
import shlex
from urllib.parse import urlsplit


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OLD = ROOT / '.omx/private/knowledge-final-gates-v15'
NEW = ROOT / '.omx/private/knowledge-final-gates-v16'
HERE = Path(__file__).resolve().parent
FULL_PREPARATION = HERE.parent / 'v16-preparation'
OLD_SHA = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
SHA = 'eda0d68fb07852b714f3092aa02ca0523d3ad312'
REPLACEMENTS = {OLD_SHA: SHA, 'v15': 'v16', 'V15': 'V16'}


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def write(path, content, mode=0o600):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, mode)
    with os.fdopen(descriptor, 'w') as target:
        target.write(content)
    path.chmod(mode)


def save(path, value):
    write(path, json.dumps(value, indent=2) + '\n')


def rebind(value):
    if isinstance(value, str):
        for old, new in REPLACEMENTS.items():
            value = value.replace(old, new)
        return value
    if isinstance(value, list):
        return [rebind(item) for item in value]
    if isinstance(value, dict):
        return {key: rebind(item) for key, item in value.items()}
    return value


def current_file(path, state):
    exists = path.is_file() and not path.is_symlink()
    if exists:
        path.read_text()
    return {'path': str(path), 'sha256': digest(path) if exists else None,
            'state': 'actual_materialized_bytes_pending_independent_binding' if exists else state}


def main():
    if NEW.exists() or NEW.is_symlink() or (HERE / 'prepared-manifest.json').exists():
        raise RuntimeError('New CCG16 namespace must be absent')
    old_inputs = json.loads((OLD / 'source-inputs.json').read_text())
    if old_inputs['candidate_sha'] != OLD_SHA or digest(OLD / 'source-inputs.json') != '90dc201a2b340c76329b1ccd8697e9bd0f7baf087ebfcde5075f558547da2eb9':
        raise RuntimeError('Unexpected current CCG15 donor')
    for relative, checksum in old_inputs['source_sha256'].items():
        if digest(ROOT / relative) != checksum:
            raise RuntimeError('Selected CCG source changed: ' + relative)
    for name, tool in old_inputs['tools'].items():
        if digest(tool['path']) != tool['sha256']:
            raise RuntimeError('Pinned tool changed: ' + name)
    files = sorted(path for path in OLD.rglob('*') if path.is_file())
    if len(files) != 13 or any(path.is_symlink() for path in files):
        raise RuntimeError('Unexpected CCG15 inventory')
    protected = {str(path): digest(path) for path in files}
    save(HERE / 'protected-CCG15-before.json', protected)
    NEW.mkdir(mode=0o700)
    rows = []
    for donor in files:
        relative = donor.relative_to(OLD)
        output = NEW / relative
        original = donor.read_text()
        is_python = donor.suffix == '.py' or donor.name.endswith('.py.txt')
        if is_python:
            content = rebind(original)
            restored = content
            for old, new in reversed(list(REPLACEMENTS.items())):
                restored = restored.replace(new, old)
            if restored != original:
                raise RuntimeError('Unexpected nonconstant runner change')
            ast.parse(content, filename=str(output))
            compile(content, str(output), 'exec')
        else:
            value = rebind(json.loads(original))
            if relative.name == 'source-inputs.json':
                value['state'] = 'prepared_not_runnable'
                value['audit_module'] = current_file(FULL_PREPARATION / 'final-state-audit-v16/final_state_audit.py', 'pending_actual_V16_auditor')
                value['environment_parser'] = current_file(ROOT / '.omx/private/final-check-v16/run-make-check-v16-provenance.py', 'pending_actual_V16_parser')
                review = FULL_PREPARATION / 'independent-review/review.json'
                value['audit_review'] = {'path': str(review), 'sha256': None, 'state': 'pending_actual_V16_independent_review'}
                if review.is_file() and not review.is_symlink():
                    actual = json.loads(review.read_text())
                    if actual['candidate_sha'] != SHA or actual['candidate_root'] != str(ROOT) or actual['verdict'] not in ('pass', 'pass-with-notes') or actual['blocking_findings_open'] != 0:
                        raise RuntimeError('Available V16 independent review is not the current accepted admission review')
                    value['audit_review'] = {'path': str(review), 'sha256': digest(review), 'state': 'actual_V16_admission_review_only; whole_result_pending'}
                value['pending_bindings'] = ['actual_full_result_and_independent_final_identity',
                                             'four_actual_permission_resource_and_freshness_fields',
                                             'current_CLI_and_fake_binary_builds']
                for name in ('audit_module', 'environment_parser', 'audit_review'):
                    if value[name]['sha256'] is None:
                        value['pending_bindings'].append(name + '.actual_sha256')
                source_env = ROOT / '.omx/private/final-check-v16/check-source-v16.env'
                renderer = None
                if source_env.is_file() and not source_env.is_symlink():
                    for line in source_env.read_text().splitlines():
                        if line.startswith('COLLABORATION_ELECTRON_RENDERER_URL='):
                            renderer = urlsplit(shlex.split(line.split('=', 1)[1])[0])
                if renderer is not None and renderer.hostname in ('127.0.0.1', 'localhost') and renderer.username is None and renderer.password is None and renderer.port is not None:
                    for host in ('127.0.0.1', 'localhost'):
                        origin = renderer.scheme + '://' + host + ':' + str(renderer.port)
                        if origin not in value['forbidden_renderer_origins']:
                            value['forbidden_renderer_origins'].append(origin)
                else:
                    value['pending_bindings'].append('forbidden_renderer_origins.current_V16_origin_pair')
            elif relative.name == 'source-values.json.template':
                value['CORS_ALLOWED_ORIGINS'] = 'ROOT_BINDS_EXACT_WEB_AND_CCG_RENDERER_ORIGINS'
                value['ALLOWED_ORIGINS'] = 'ROOT_BINDS_EXACT_WEB_AND_CCG_RENDERER_ORIGINS'
                value['MULTICA_DSH_PATH'] = str(NEW / 'missing-test-owned-dsh')
            content = json.dumps(value, indent=2) + '\n'
        mode = donor.stat().st_mode & 0o777
        write(output, content, mode)
        rows.append({'relative': str(relative), 'donor': str(donor), 'donor_sha256': digest(donor),
                     'prepared': str(output), 'prepared_sha256': digest(output), 'mode': mode,
                     'python_AST_compile_only': is_python, 'runner_inverse_constant_bytes_match': is_python})
    inputs_path = NEW / 'source-inputs.json'
    for path in sorted((NEW / 'permissions').glob('*.json.template')):
        value = json.loads(path.read_text())
        if any(value[key] is not False for key in ('ready_to_execute', 'root_released_complete_check_resources',
               'root_verified_complete_named_inventory', 'root_verified_fresh_database', 'real_provider_allowed')):
            raise RuntimeError('All four permission templates must remain false')
        value['source_inputs_sha256'] = digest(inputs_path)
        value['runner_sha256'] = digest(NEW / ('run-ccg1.py' if value['gate'].startswith('ccg1-') else 'run-ccg2.py'))
        path.write_text(json.dumps(value, indent=2) + '\n')
        next(row for row in rows if row['prepared'] == str(path))['prepared_sha256'] = digest(path)
    for row in rows:
        old = Path(row['donor']).read_text()
        new = Path(row['prepared']).read_text()
        write(HERE / 'diffs' / (row['relative'].replace('/', '__') + '.diff'),
              ''.join(difflib.unified_diff(old.splitlines(keepends=True), new.splitlines(keepends=True), fromfile=row['donor'], tofile=row['prepared'])))
    if any(digest(path) != checksum for path, checksum in protected.items()):
        raise RuntimeError('Old CCG15 bytes changed')
    result = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'candidate_root': str(ROOT),
              'candidate_sha': SHA, 'donor_candidate_sha': OLD_SHA, 'state': 'prepared_not_runnable',
              'replacements': REPLACEMENTS, 'files': rows, 'protected_CCG15': protected,
              'source_sha256': old_inputs['source_sha256'], 'tools': old_inputs['tools'],
              'source_inputs_sha256': digest(inputs_path), 'permissions_all_false': True,
              'actual_permissions_created': 0, 'templates': 13, 'python_AST_compile_only': 3,
              'JSON_parsed': 10, 'DB_build_service_browser_TestMain_Provider_operations': 0,
              'knowledge_allocated': 0, 'knowledge_calls': 0, 'production_default_enabled': False}
    save(HERE / 'prepared-manifest.json', result)
    print(json.dumps({'candidate_sha': SHA, 'templates': 13, 'source_inputs_sha256': result['source_inputs_sha256'],
                      'permissions_all_false': True, 'actual_operations': 0}))


if __name__ == '__main__':
    main()
