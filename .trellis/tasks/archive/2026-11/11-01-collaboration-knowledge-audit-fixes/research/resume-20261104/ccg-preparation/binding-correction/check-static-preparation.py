"""Check current prepared bytes and actual early CCG refusal paths offline."""

import ast
from contextlib import ExitStack
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import types
from unittest import mock


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = ROOT / '.omx/private/knowledge-final-gates-v15'
HERE = Path(__file__).resolve().parent


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def main():
    manifest = json.loads((HERE / 'prepared-manifest.json').read_text())
    compiled = []
    parsed_json = []
    for row in manifest['files']:
        path = Path(row['prepared'])
        if digest(path) != row['prepared_sha256']:
            raise RuntimeError('Prepared bytes changed: ' + row['relative'])
        if path.stat().st_mode & 0o777 != row['mode']:
            raise RuntimeError('Prepared mode changed: ' + row['relative'])
        if digest(row['original']) != row['original_sha256']:
            raise RuntimeError('Protected v14 bytes changed')
        source = path.read_text()
        if row['runner_body_unchanged_after_constant_normalization']:
            compile(source, str(path), 'exec')
            restored = source
            for old, new in reversed(list(manifest['replacements'].items())):
                restored = restored.replace(new, old)
            if restored != Path(row['original']).read_text():
                raise RuntimeError('Runner has an unregistered body change')
            ast.parse(source)
            compiled.append(row['relative'])
        else:
            json.loads(source)
            parsed_json.append(row['relative'])
    inputs_path = PRIVATE / 'source-inputs.json'
    inputs = json.loads(inputs_path.read_text())
    if inputs['knowledge_default_enabled'] is not False or inputs['real_provider_allowed'] is not False:
        raise RuntimeError('Real knowledge admission was changed')
    if inputs['audit_review']['sha256'] is not None:
        raise RuntimeError('Future root independent review must remain explicitly pending')
    for field in ('audit_module', 'environment_parser'):
        if digest(inputs[field]['path']) != inputs[field]['sha256']:
            raise RuntimeError('Materialized root input changed')
    for name, checksum in inputs['source_sha256'].items():
        if digest(ROOT / name) != checksum:
            raise RuntimeError('Current selected CCG source changed')
    for tool in inputs['tools'].values():
        if digest(tool['path']) != tool['sha256']:
            raise RuntimeError('Pinned tool changed')
    permissions = sorted((PRIVATE / 'permissions').glob('*.json.template'))
    if len(permissions) != 4 or list((PRIVATE / 'permissions').glob('*.json')):
        raise RuntimeError('Exactly four false templates and no actual permission are required')
    for path in permissions:
        value = json.loads(path.read_text())
        if value['source_inputs_sha256'] != digest(inputs_path):
            raise RuntimeError('Permission source-input hash is stale')
        runner = PRIVATE / ('run-ccg1.py' if value['gate'].startswith('ccg1-') else 'run-ccg2.py')
        if value['runner_sha256'] != digest(runner):
            raise RuntimeError('Permission runner hash is stale')
        if any(value[field] is not False for field in ('ready_to_execute', 'root_released_complete_check_resources', 'root_verified_complete_named_inventory', 'root_verified_fresh_database', 'real_provider_allowed')):
            raise RuntimeError('Prepared permission was enabled')
        if any(value[field] is not None for field in ('database', 'DATABASE_URL', 'identity_verified', 'temp_directory', 'root_permission_record')):
            raise RuntimeError('Unallocated permission fields were materialized')
    off = json.loads((PRIVATE / 'deployments/off/source-values.json.template').read_text())
    on = json.loads((PRIVATE / 'deployments/on/source-values.json.template').read_text())
    if off['MULTICA_KNOWLEDGE_CONTEXT_ENABLED'] != 'false' or on['MULTICA_KNOWLEDGE_CONTEXT_ENABLED'] != 'true':
        raise RuntimeError('Explicit fake OFF/ON template flags changed')
    if {k: v for k, v in off.items() if k != 'MULTICA_KNOWLEDGE_CONTEXT_ENABLED'} != {k: v for k, v in on.items() if k != 'MULTICA_KNOWLEDGE_CONTEXT_ENABLED'}:
        raise RuntimeError('Fake OFF/ON templates have an unexpected difference')
    redirected = HERE / 'redirected-permit.control'
    redirected.symlink_to(permissions[0])
    cases = []
    for runner_name in ('run-ccg1.py', 'run-ccg2.py'):
        runner = PRIVATE / runner_name
        module = types.ModuleType('private_static_ccg_refusal_control')
        module.__file__ = str(runner)
        exec(compile(runner.read_bytes(), str(runner), 'exec'), module.__dict__)
        if module.PRIVATE != PRIVATE or module.EXPECTED != manifest['candidate_sha']:
            raise RuntimeError('Runner constants do not select this candidate')
        selected = [p for p in permissions if p.name.startswith('ccg1-' if runner_name == 'run-ccg1.py' else 'ccg2-')]
        checks = [(p.name, p, 'offline-refusal', 'Later root execution permission is required') for p in selected]
        checks += [('invalid label', selected[0], 'invalid label', 'Invalid private result label'),
                   ('foreign permit', HERE / 'foreign-permit.control', 'offline-refusal', 'Permit outside owned directory'),
                   ('redirected permit', redirected, 'offline-refusal', 'Permit must not be redirected')]
        for name, permit, label, expected in checks:
            with ExitStack() as stack:
                stack.enter_context(mock.patch.object(module.subprocess, 'run', side_effect=AssertionError('No subprocess is allowed')))
                stack.enter_context(mock.patch.object(module.subprocess, 'check_output', side_effect=AssertionError('No subprocess is allowed')))
                stack.enter_context(mock.patch.object(module, 'write_json', side_effect=AssertionError('No runtime evidence write is allowed')))
                stack.enter_context(mock.patch.object(Path, 'mkdir', side_effect=AssertionError('No runtime directory creation is allowed')))
                if hasattr(module, 'urlopen'):
                    stack.enter_context(mock.patch.object(module, 'urlopen', side_effect=AssertionError('No network is allowed')))
                try:
                    gate_source = permit if permit in selected else selected[0]
                    module.admission(json.loads(gate_source.read_text())['gate'], permit, label)
                except RuntimeError as error:
                    if str(error) != expected:
                        raise RuntimeError('Unexpected refusal for ' + name) from error
                else:
                    raise RuntimeError('Prepared entry admitted an unapproved operation')
            cases.append({'runner': runner_name, 'case': name, 'expected': expected, 'observed': expected, 'subprocess_network_runtime_writes': 0})
    if len(cases) != 10:
        raise RuntimeError('Incomplete offline refusal control inventory')
    result = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'candidate_sha': manifest['candidate_sha'], 'prepared_manifest_sha256': digest(HERE / 'prepared-manifest.json'), 'source_inputs_sha256': digest(inputs_path), 'compiled_python_files': compiled, 'parsed_json_files': parsed_json, 'protected_v14_files': len(manifest['protected_v14_sha256']), 'protected_source_files': len(inputs['source_sha256']), 'pinned_tools': len(inputs['tools']), 'false_permission_templates': 4, 'refusal_controls': cases, 'refusal_controls_passed': len(cases), 'product_tests_build_database_services': 0, 'knowledge_allocated': 0, 'knowledge_real_calls': 0, 'production_default_enabled': False}
    descriptor = os.open(HERE / 'static-verification.json', os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        json.dump(result, output, indent=2)
        output.write('\n')
    print(json.dumps({key: result[key] for key in ('candidate_sha', 'protected_v14_files', 'protected_source_files', 'pinned_tools', 'false_permission_templates', 'refusal_controls_passed', 'product_tests_build_database_services', 'knowledge_real_calls')}))


if __name__ == '__main__':
    main()
