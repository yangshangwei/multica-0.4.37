"""Validate inert V16 template syntax/bindings without executing any template."""

import ast
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shlex


HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v16'


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def main():
    manifest = json.loads((HERE / 'prepared-template-manifest.json').read_text())
    counts = {'templates': 0, 'python_AST_compile_only': 0}
    for row in manifest['files']:
        template = Path(row['prepared_template'])
        assert template.is_file() and not template.is_symlink()
        assert digest(template) == row['prepared_sha256']
        assert template.stat().st_mode & 0o777 == int(row['template_mode'], 8)
        assert not Path(row['final_path']).exists()
        content = template.read_text()
        if row.get('python_AST_compile_only'):
            ast.parse(content, filename=str(template))
            compile(content, str(template), 'exec')
            counts['python_AST_compile_only'] += 1
        if row.get('donor'):
            assert digest(row['donor']) == row['donor_sha256']
        counts['templates'] += 1
    for path, checksum in {**manifest['protected_v15'], **manifest['protected_CCG15']}.items():
        assert digest(path) == checksum
    values = {}
    for line in (OUT / 'check-source-v16.env.in').read_text().splitlines():
        if line and not line.startswith('#'):
            key, raw = line.split('=', 1)
            tokens = shlex.split(raw)
            assert len(tokens) <= 1
            values[key] = tokens[0] if tokens else ''
    assert values['DATABASE_URL'] == 'PENDING_ROOT_DATABASE_URL'
    assert values['JWT_SECRET'] == 'PENDING_ROOT_PRIVATE_JWT_SECRET'
    assert values['POSTGRES_PASSWORD'] == 'PENDING_ROOT_POSTGRES_PASSWORD'
    assert values['MULTICA_DEPLOYMENT_ID'] == 'PENDING_ROOT_DEPLOYMENT_UUID'
    assert values['POSTGRES_DB'] == manifest['seed_database_proposal']
    assert values['MULTICA_AUTH_MODE'] == 'legacy'
    for key in ('MULTICA_KNOWLEDGE_CONTEXT_ENABLED', 'MULTICA_DEVICE_AUTH_ENABLED',
                'MULTICA_MANAGED_INSTALLATIONS_ENABLED', 'MULTICA_PLATFORM_ADMIN_ENABLED'):
        assert values[key] == 'false'
    assert values['MULTICA_LLM_API_KEY'] == values['MULTICA_LLM_BASE_URL'] == ''
    assert digest(OUT / 'check-bin/go.in') == manifest['Go_adapter_sha256']
    pnpm = (OUT / 'check-bin/pnpm.in').read_text()
    assert 'makecheck-vitest-reporting-v16.json' in pnpm
    for basename in ('makecheck-playwright-results-v9.json', 'makecheck-playwright-environment-v9.json'):
        assert basename in pnpm
    launcher = (OUT / 'run-make-check-v16-provenance.py.in').read_text()
    assert '/private/tmp/multica-final-check-v16-' in launcher
    assert 'Task deployment requires a canonical nonzero UUID' in launcher
    assert 'values = parse_generated_environment' in launcher
    clearance = json.loads((OUT / 'independent-review-clearance-v16.json.template').read_text())
    donor_clearance = json.loads((ROOT / '.omx/private/final-check-v15/independent-review-clearance-v15.json').read_text())
    assert set(clearance) == set(donor_clearance)
    assert clearance['verdict'] == 'pending' and clearance['blocking_findings_open'] is None
    assert clearance['candidate_sha'] is None and clearance['review_sha256'] is None
    assert clearance['real_provider_allowed'] is False
    assert clearance['knowledge_budget_allocated'] == clearance['knowledge_real_calls'] == 0
    assert 'final_auditor' not in clearance
    assert clearance['final_state_audit_path'] == str(HERE / 'final-state-audit-v16/final_state_audit.py')
    cache = (HERE / 'seed-warm-go-cache.py.in').read_text()
    assert '.fclonefileat' in cache and 'normal_copy_fallbacks' in cache
    assert not any(value in cache for value in ('shutil.copy', 'os.link(', '.symlink_to(', 'cp -'))
    for name in ('go-build-cache', 'full-check-home-v16', 'go-packages-v16', 'dev-registry-v16',
                 'profiles-v16', 'workspaces-v16', 'metadata-home-not-created',
                 'make-check-v16.json', 'make-check-v16.log', 'check-source-v16.env'):
        assert not (OUT / name).exists() and not (OUT / name).is_symlink()
    assert manifest['control_sources_total'] == 69 and manifest['controls_executed'] == 0
    assert counts['python_AST_compile_only'] == manifest['python_AST_compile_only_count']
    result = {'recorded_at': datetime.now(timezone.utc).isoformat(), **counts,
              'manifest_sha256': digest(HERE / 'prepared-template-manifest.json'),
              'protected_v15_count': len(manifest['protected_v15']),
              'protected_CCG15_count': len(manifest['protected_CCG15']),
              'control_source_definitions': manifest['control_source_definitions'],
              'control_sources_total': 69, 'controls_executed': 0,
              'unissued_flat_clearance_schema_preserved': True,
              'actual_final_candidate_or_resources_bound': False, 'cache_seed_executed': False,
              'DB_build_service_test_Provider_operations': 0,
              'knowledge_budget_allocated': 0, 'knowledge_real_calls': 0,
              'production_default_enabled': False}
    descriptor = os.open(HERE / 'static-validation.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        json.dump(result, output, indent=2)
        output.write('\n')
    print(json.dumps(result))


if __name__ == '__main__':
    main()
