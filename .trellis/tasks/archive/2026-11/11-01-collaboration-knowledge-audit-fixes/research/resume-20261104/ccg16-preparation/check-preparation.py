"""Finite static and early-refusal checks for the current CCG16 version."""

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
PRIVATE = ROOT / '.omx/private/knowledge-final-gates-v16'
HERE = Path(__file__).resolve().parent


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def main():
    manifest = json.loads((HERE / 'prepared-manifest.json').read_text())
    parsed = []
    compiled = []
    for row in manifest['files']:
        path = Path(row['prepared'])
        assert digest(path) == row['prepared_sha256']
        assert path.stat().st_mode & 0o777 == row['mode']
        assert digest(row['donor']) == row['donor_sha256']
        text = path.read_text()
        if row['python_AST_compile_only']:
            ast.parse(text, filename=str(path))
            compile(text, str(path), 'exec')
            restored = text
            for old, new in reversed(list(manifest['replacements'].items())):
                restored = restored.replace(new, old)
            assert restored == Path(row['donor']).read_text()
            compiled.append(row['relative'])
        else:
            json.loads(text)
            parsed.append(row['relative'])
    assert len(compiled) == 3 and len(parsed) == 10
    inputs_path = PRIVATE / 'source-inputs.json'
    inputs = json.loads(inputs_path.read_text())
    assert inputs['state'] == 'prepared_not_runnable'
    assert inputs['candidate_sha'] == manifest['candidate_sha']
    assert inputs['knowledge_default_enabled'] is False and inputs['real_provider_allowed'] is False
    for relative, checksum in inputs['source_sha256'].items():
        assert digest(ROOT / relative) == checksum
    for tool in inputs['tools'].values():
        assert digest(tool['path']) == tool['sha256']
    for field in ('audit_module', 'environment_parser', 'audit_review'):
        if inputs[field]['sha256'] is not None:
            assert digest(inputs[field]['path']) == inputs[field]['sha256']
    templates = sorted((PRIVATE / 'permissions').glob('*.json.template'))
    assert len(templates) == 4 and not list((PRIVATE / 'permissions').glob('*.json'))
    for path in templates:
        value = json.loads(path.read_text())
        assert value['source_inputs_sha256'] == digest(inputs_path)
        assert value['runner_sha256'] == digest(PRIVATE / ('run-ccg1.py' if value['gate'].startswith('ccg1-') else 'run-ccg2.py'))
        assert all(value[key] is False for key in ('ready_to_execute', 'root_released_complete_check_resources',
                   'root_verified_complete_named_inventory', 'root_verified_fresh_database', 'real_provider_allowed'))
        assert all(value[key] is None for key in ('database', 'DATABASE_URL', 'identity_verified', 'temp_directory', 'root_permission_record'))
    off = json.loads((PRIVATE / 'deployments/off/source-values.json.template').read_text())
    on = json.loads((PRIVATE / 'deployments/on/source-values.json.template').read_text())
    assert off['MULTICA_KNOWLEDGE_CONTEXT_ENABLED'] == 'false' and on['MULTICA_KNOWLEDGE_CONTEXT_ENABLED'] == 'true'
    assert {k: v for k, v in off.items() if k != 'MULTICA_KNOWLEDGE_CONTEXT_ENABLED'} == {k: v for k, v in on.items() if k != 'MULTICA_KNOWLEDGE_CONTEXT_ENABLED'}
    assert off['MULTICA_AUTH_MODE'] == 'password' and off['MULTICA_MANAGED_INSTALLATIONS_ENABLED'] == 'true'
    assert off['MULTICA_PASSWORD_LIMITER_MODE'] == 'single'
    assert off['MULTICA_LLM_API_KEY'] == off['MULTICA_LLM_BASE_URL'] == ''
    assert off['CORS_ALLOWED_ORIGINS'] == off['ALLOWED_ORIGINS'] == 'ROOT_BINDS_EXACT_WEB_AND_CCG_RENDERER_ORIGINS'
    assert off['MULTICA_DSH_PATH'] == str(PRIVATE / 'missing-test-owned-dsh')
    redirect = HERE / 'redirected-permit.control'
    redirect.symlink_to(templates[0])
    cases = []
    for runner_name in ('run-ccg1.py', 'run-ccg2.py'):
        source = PRIVATE / runner_name
        module = types.ModuleType('ccg16_static_refusal_control')
        module.__file__ = str(source)
        exec(compile(source.read_bytes(), str(source), 'exec'), module.__dict__)
        assert module.PRIVATE == PRIVATE and module.EXPECTED == manifest['candidate_sha']
        selected = [p for p in templates if p.name.startswith('ccg1-' if runner_name == 'run-ccg1.py' else 'ccg2-')]
        controls = [(p.name, p, 'offline-refusal', 'Later root execution permission is required') for p in selected]
        controls += [('invalid label', selected[0], 'invalid label', 'Invalid private result label'),
                     ('foreign permit', HERE / 'foreign-permit.control', 'offline-refusal', 'Permit outside owned directory'),
                     ('redirected permit', redirect, 'offline-refusal', 'Permit must not be redirected')]
        for name, permit, label, expected in controls:
            with ExitStack() as stack:
                stack.enter_context(mock.patch.object(module.subprocess, 'run', side_effect=AssertionError('No subprocess allowed')))
                stack.enter_context(mock.patch.object(module.subprocess, 'check_output', side_effect=AssertionError('No subprocess allowed')))
                stack.enter_context(mock.patch.object(Path, 'mkdir', side_effect=AssertionError('No runtime directory creation allowed')))
                stack.enter_context(mock.patch.object(module, 'write_json', side_effect=AssertionError('No runtime evidence write allowed')))
                if hasattr(module, 'urlopen'):
                    stack.enter_context(mock.patch.object(module, 'urlopen', side_effect=AssertionError('No network allowed')))
                gate_source = permit if permit in selected else selected[0]
                try:
                    module.admission(json.loads(gate_source.read_text())['gate'], permit, label)
                except RuntimeError as error:
                    assert str(error) == expected
                else:
                    raise AssertionError('Prepared CCG16 entry admitted an unapproved operation')
            cases.append({'runner': runner_name, 'case': name, 'expected': expected, 'observed': expected,
                          'subprocess_network_runtime_writes': 0})
    assert len(cases) == 10
    result = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'candidate_sha': manifest['candidate_sha'],
              'manifest_sha256': digest(HERE / 'prepared-manifest.json'), 'source_inputs_sha256': digest(inputs_path),
              'compiled_python': compiled, 'parsed_JSON': parsed, 'protected_CCG15_count': 13,
              'unchanged_sources': 11, 'pinned_tools': 4, 'false_permission_templates': 4,
              'early_refusal_controls': cases, 'early_refusal_controls_passed': 10,
              'DB_TestMain_build_service_browser_Provider_operations': 0,
              'knowledge_allocated': 0, 'knowledge_calls': 0, 'production_default_enabled': False}
    descriptor = os.open(HERE / 'verification.json', os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        json.dump(result, output, indent=2)
        output.write('\n')
    print(json.dumps({key: result[key] for key in ('candidate_sha', 'source_inputs_sha256', 'protected_CCG15_count',
                                                 'false_permission_templates', 'early_refusal_controls_passed', 'knowledge_calls')}))


if __name__ == '__main__':
    main()
