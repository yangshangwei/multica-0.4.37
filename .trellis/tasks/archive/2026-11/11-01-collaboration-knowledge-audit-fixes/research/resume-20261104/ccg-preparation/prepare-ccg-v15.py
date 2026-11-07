"""Rebind retained CCG preparation; never allocate or execute a product gate."""

import ast
from datetime import datetime, timezone
import difflib
import hashlib
import json
import os
from pathlib import Path
import shlex
import stat
import subprocess
from urllib.parse import urlsplit


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OLD = ROOT / '.omx/private/knowledge-final-gates-v14'
NEW = ROOT / '.omx/private/knowledge-final-gates-v15'
HERE = Path(__file__).resolve().parent
PROJECT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
FINAL_PREPARATION = PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261104/v15-preparation'
OLD_SHA = 'd0b132fff0c477eb2d744933e13db730240231aa'
SHA = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
OLD_OWNER = '01a1a186-9a5c-7b00-96f8-0430b47edb4c'
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
REPLACEMENTS = {
    OLD_SHA: SHA,
    OLD_OWNER: OWNER,
    str(OLD): str(NEW),
    '.omx/private/knowledge-final-gates-v14': '.omx/private/knowledge-final-gates-v15',
    '/private/tmp/multica-ccg-v14-': '/private/tmp/multica-ccg-v15-',
    'knowledge-v14-': 'knowledge-v15-',
}


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def write_bytes(path, data, mode=0o600):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, mode)
    with os.fdopen(descriptor, 'wb') as output:
        output.write(data)
    path.chmod(mode)


def write_json(path, value):
    write_bytes(path, (json.dumps(value, indent=2) + '\n').encode())


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


def main():
    head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    if head != SHA or NEW.exists() or NEW.is_symlink():
        raise RuntimeError('New namespace must be absent and candidate must match the root freeze')
    inputs = json.loads((OLD / 'source-inputs.json').read_text())
    if inputs['candidate_sha'] != OLD_SHA:
        raise RuntimeError('Unexpected retained CCG candidate')
    sources = {}
    for relative, expected in inputs['source_sha256'].items():
        actual = digest(ROOT / relative)
        baseline = subprocess.check_output(['git', 'show', OLD_SHA + ':' + relative], cwd=ROOT)
        if actual != expected or hashlib.sha256(baseline).hexdigest() != expected:
            raise RuntimeError('Protected CCG source changed: ' + relative)
        sources[relative] = actual
    tools = {}
    for name, specification in inputs['tools'].items():
        actual = digest(specification['path'])
        if actual != specification['sha256']:
            raise RuntimeError('Pinned tool changed: ' + name)
        tools[name] = specification
    originals = sorted(path for path in OLD.rglob('*') if path.is_file())
    if len(originals) != 13 or any(path.is_symlink() for path in originals):
        raise RuntimeError('Unexpected retained preparation inventory')
    protected = {str(path): digest(path) for path in originals}
    write_json(HERE / 'protected-v14-before.json', protected)
    NEW.mkdir(mode=0o700)
    rows = []
    for original in originals:
        relative = original.relative_to(OLD)
        destination = NEW / relative
        old_text = original.read_text()
        is_python = relative.suffix == '.py' or relative.name.endswith('.py.txt')
        if is_python:
            new_text = rebind(old_text)
            compile(new_text, str(destination), 'exec')
            restored = new_text
            for old, new in reversed(list(REPLACEMENTS.items())):
                restored = restored.replace(new, old)
            if restored != old_text:
                raise RuntimeError('Unregistered runner-body change: ' + str(relative))
            if ast.dump(ast.parse(restored), include_attributes=False) != ast.dump(ast.parse(old_text), include_attributes=False):
                raise RuntimeError('Runner predicates changed')
        else:
            value = rebind(json.loads(old_text))
            if relative.name == 'source-inputs.json':
                auditor = FINAL_PREPARATION / 'final-state-audit-v15/final_state_audit.py'
                parser = ROOT / '.omx/private/final-check-v15/run-make-check-v15-provenance.py'
                renderer_url = None
                for line in (ROOT / '.omx/private/final-check-v15/check-source-v15.env').read_text().splitlines():
                    if line.startswith('COLLABORATION_ELECTRON_RENDERER_URL='):
                        renderer_url = urlsplit(shlex.split(line.split('=', 1)[1])[0])
                if renderer_url is None or renderer_url.hostname not in ('127.0.0.1', 'localhost') or renderer_url.username is not None or renderer_url.password is not None or renderer_url.port is None:
                    raise RuntimeError('Current materialized renderer origin is unavailable')
                value['state'] = 'prepared_pending_v15_shared_result_final_identity_and_later_root_permission'
                value['full_run_path'] = str(ROOT / '.omx/private/final-check-v15/make-check-v15.json')
                value['audit_module'] = {'path': str(auditor), 'sha256': digest(auditor), 'state': 'materialized_bytes_only_pending_independent_review'}
                value['audit_review'] = {'path': str(FINAL_PREPARATION / 'independent-review/review.json'), 'sha256': None, 'state': 'pending_v15_independent_final_identity_review'}
                value['environment_parser'] = {'path': str(parser), 'sha256': digest(parser), 'state': 'materialized_bytes_only_pending_independent_review'}
                value['forbidden_renderer_origins'] += [renderer_url.scheme + '://' + host + ':' + str(renderer_url.port) for host in ('127.0.0.1', 'localhost')]
                value['pending_bindings'] = ['audit_review.path_and_sha256', 'actual_full_result_and_independent_final_identity', 'four_actual_permission_resource_and_freshness_fields']
            new_text = json.dumps(value, indent=2) + '\n'
        mode = stat.S_IMODE(original.stat().st_mode)
        write_bytes(destination, new_text.encode(), mode)
        write_bytes(HERE / 'diffs' / (str(relative).replace('/', '__') + '.diff'), ''.join(difflib.unified_diff(old_text.splitlines(keepends=True), new_text.splitlines(keepends=True), fromfile=str(original), tofile=str(destination))).encode())
        rows.append({'relative': str(relative), 'original': str(original), 'original_sha256': protected[str(original)], 'prepared': str(destination), 'prepared_sha256': digest(destination), 'mode': mode, 'runner_body_unchanged_after_constant_normalization': is_python})
    new_inputs = NEW / 'source-inputs.json'
    for path in sorted((NEW / 'permissions').glob('*.json.template')):
        value = json.loads(path.read_text())
        value['source_inputs_sha256'] = digest(new_inputs)
        runner = NEW / ('run-ccg1.py' if value['gate'].startswith('ccg1-') else 'run-ccg2.py')
        value['runner_sha256'] = digest(runner)
        if any(value[field] is not False for field in ('ready_to_execute', 'root_released_complete_check_resources', 'root_verified_complete_named_inventory', 'root_verified_fresh_database', 'real_provider_allowed')):
            raise RuntimeError('Prepared permission must remain false')
        path.write_text(json.dumps(value, indent=2) + '\n')
        row = next(row for row in rows if row['prepared'] == str(path))
        row['prepared_sha256'] = digest(path)
        original = Path(row['original']).read_text()
        write_path = HERE / 'diffs' / (row['relative'].replace('/', '__') + '.diff')
        write_path.write_text(''.join(difflib.unified_diff(original.splitlines(keepends=True), path.read_text().splitlines(keepends=True), fromfile=row['original'], tofile=str(path))))
    if any(digest(path) != checksum for path, checksum in protected.items()):
        raise RuntimeError('Retained preparation changed during this pass')
    report = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'owner': OWNER, 'candidate_root': str(ROOT), 'candidate_sha': SHA, 'state': 'static_preparation_only_not_runnable', 'replacements': REPLACEMENTS, 'files': rows, 'protected_v14_sha256': protected, 'source_sha256': sources, 'tools': tools, 'private_controls_executed': 0, 'product_tests_executed': 0, 'build_database_service_operations': 0, 'knowledge_allocated': 0, 'knowledge_real_calls': 0, 'production_default_enabled': False}
    write_json(HERE / 'prepared-manifest.json', report)
    print(json.dumps({'candidate_sha': SHA, 'prepared_files': len(rows), 'protected_sources': len(sources), 'pinned_tools': len(tools), 'permissions_all_false': True, 'state': report['state']}))


if __name__ == '__main__':
    main()
