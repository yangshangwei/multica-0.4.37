"""Prepare inert V16 templates from actual reviewed V15 bytes; run no gate."""

import ast
from datetime import datetime, timezone
import difflib
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import stat


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PROJECT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
HERE = Path(__file__).resolve().parent
OLD_HERE = HERE.parent / 'v15-preparation'
OLD_OUT = ROOT / '.omx/private/final-check-v15'
OUT = ROOT / '.omx/private/final-check-v16'
OLD_SHA = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
GO_HASH = '70ca24bb88b2a8df93ae4694e9fd9739c523e8f35755a10bb799b7561b934fce'


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def write(path, text, mode=0o600):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    with os.fdopen(descriptor, 'w') as output:
        output.write(text)
    path.chmod(mode)


def save(path, value):
    write(path, json.dumps(value, indent=2) + '\n')


def replace(text, replacements):
    for old, new in replacements.items():
        text = text.replace(old, new)
    return text


def main():
    require_absent = OUT.exists() or OUT.is_symlink() or (HERE / 'prepared-template-manifest.json').exists()
    if require_absent:
        raise RuntimeError('Preserve existing V16 namespace; this preparation is exclusive')
    materialization_path = OLD_HERE / 'materialization.json'
    materialization = json.loads(materialization_path.read_text())
    if materialization['candidate_sha'] != OLD_SHA or len(materialization['files']) != 22:
        raise RuntimeError('Unexpected reviewed V15 materialization')
    donors = []
    for row in materialization['files']:
        path = Path(row['final_path'])
        if digest(path) != row['final_sha256']:
            raise RuntimeError('Reviewed V15 operative file changed: ' + str(path))
        donors.append((path, row['mode']))
    extras = [OLD_HERE / 'prove-current-resources.py', HERE.parent / 'v15-execution/execute-once.py']
    donors += [(path, '0600') for path in extras]
    allocation = json.loads((OLD_OUT / 'build-temp-allocation-v15.json').read_text())
    clearance_path = OLD_OUT / 'independent-review-clearance-v15.json'
    clearance = json.loads(clearance_path.read_text())
    review_path = Path(clearance['review_path'])
    if digest(review_path) != clearance['review_sha256'] or clearance['candidate_sha'] != OLD_SHA:
        raise RuntimeError('Unexpected retained admission review')
    replacements = {
        OLD_SHA: 'PENDING_ROOT_FINAL_SHA',
        allocation['directory']: 'PENDING_ROOT_BUILD_TEMP_DIRECTORY',
        str(materialization['renderer_port']): 'PENDING_ROOT_RENDERER_PORT',
        str(materialization['redis_port']): 'PENDING_ROOT_REDIS_PORT',
        '725f8e4c-8979-4fc2-b105-36a6ce89a2e5': 'PENDING_ROOT_DEPLOYMENT_UUID',
        digest(clearance_path): 'PENDING_ROOT_CLEARANCE_SHA256',
        clearance['review_sha256']: 'PENDING_ROOT_INDEPENDENT_REVIEW_SHA256',
        'v15': 'v16',
        'V15': 'V16',
    }
    protected = {str(path): digest(path) for path, _ in donors}
    for path in [materialization_path, clearance_path, review_path, OLD_OUT / 'build-temp-allocation-v15.json',
                 OLD_OUT / 'make-check-v15.json', OLD_OUT / 'make-check-v15.log',
                 OLD_HERE / 'final-state-audit-v15/post-make-v15.json',
                 OLD_HERE / 'install-staged-desktop.py', OLD_HERE / 'release-owned-runtimes.py']:
        protected[str(path)] = digest(path)
    ccg = ROOT / '.omx/private/knowledge-final-gates-v15'
    protected_ccg = {str(path): digest(path) for path in sorted(ccg.rglob('*')) if path.is_file()}
    save(HERE / 'protected-before.json', {'v15': protected, 'CCG15': protected_ccg})
    OUT.mkdir(mode=0o700)
    files = []
    python_count = 0
    for donor, mode in donors:
        final = Path(replace(str(donor), {'v15': 'v16'}))
        if donor.name == 'execute-once.py':
            template = HERE / 'execution/execute-once.py.in'
        elif donor.is_relative_to(OLD_HERE):
            template = HERE / (str(donor.relative_to(OLD_HERE)).replace('v15', 'v16') + '.in')
        else:
            template = Path(str(final) + '.in')
        original = donor.read_text()
        if donor.name == 'check-source-v15.env':
            lines, redacted = [], []
            secret_markers = {'DATABASE_URL': 'PENDING_ROOT_DATABASE_URL',
                              'POSTGRES_PASSWORD': 'PENDING_ROOT_POSTGRES_PASSWORD',
                              'JWT_SECRET': 'PENDING_ROOT_PRIVATE_JWT_SECRET'}
            for line in original.splitlines():
                if not line or line.lstrip().startswith('#'):
                    lines.append(replace(line, replacements))
                    redacted.append(line)
                    continue
                key, raw = line.split('=', 1)
                tokens = shlex.split(raw)
                if len(tokens) > 1:
                    raise RuntimeError('Reviewed environment has unexpected assignment shape')
                value = tokens[0] if tokens else ''
                new_value = secret_markers.get(key, replace(value, replacements))
                lines.append(key + '=' + shlex.quote(new_value))
                redacted.append(key + '=' + ('<redacted>' if key in secret_markers else raw))
            content = '\n'.join(lines) + '\n'
            diff_before = '\n'.join(redacted) + '\n'
            body_unchanged = None
        elif donor.name == 'go':
            if digest(donor) != GO_HASH:
                raise RuntimeError('Go adapter differs from the accepted 70ca bytes')
            content, diff_before, body_unchanged = original, original, True
        else:
            content = replace(original, replacements)
            restored = content
            for old, new in reversed(list(replacements.items())):
                restored = restored.replace(new, old)
            if restored != original:
                raise RuntimeError('Nonidentity changes in donor rebinding: ' + str(donor))
            diff_before, body_unchanged = original, True
        is_python = donor.suffix == '.py' or donor.name in ('go', 'pnpm')
        if is_python:
            ast.parse(content, filename=str(template))
            compile(content, str(template), 'exec')
            python_count += 1
        write(template, content)
        relative = str(template.relative_to(HERE)) if template.is_relative_to(HERE) else str(template.relative_to(OUT))
        write(HERE / 'diffs' / (relative.replace('/', '__') + '.diff'),
              ''.join(difflib.unified_diff(diff_before.splitlines(keepends=True), content.splitlines(keepends=True),
                                        fromfile=str(donor) + (':redacted' if donor.name.endswith('.env') else ''), tofile=str(template))))
        files.append({'donor': str(donor), 'donor_sha256': digest(donor), 'donor_mode': mode,
                      'prepared_template': str(template), 'prepared_sha256': digest(template),
                      'final_path': str(final), 'mode': mode, 'materialize': True,
                      'template_mode': '0600', 'python_AST_compile_only': is_python,
                      'materialization_phase': 'after_independent_review_and_root_clearance' if donor.name == 'execute-once.py' else 'after_final_source_freeze',
                      'identity_only_body_rebinding_verified': body_unchanged,
                      'pending_fields': sorted(set(re.findall(r'PENDING_ROOT_[A-Z0-9_]+', content))),
                      'research_diff_redacted': donor.name.endswith('.env')})
    future = json.loads(clearance_path.read_text())
    for key in ('candidate_sha', 'launcher_sha256', 'source_env_sha256', 'bindings_sha256',
                'resource_ownership_sha256', 'review_sha256', 'final_state_audit_sha256',
                'final_state_audit_review_sha256', 'issued_at', 'root_live_confirmation_sha256'):
        future[key] = None
    future['verdict'] = 'pending'
    future['blocking_findings_open'] = None
    future['adapter_sha256'] = {'go': GO_HASH, 'pnpm': None}
    future['review_path'] = str(HERE / 'independent-review/review.json')
    future['final_state_audit_path'] = str(HERE / 'final-state-audit-v16/final_state_audit.py')
    future['root_live_confirmation_path'] = None
    for group in future['source_identities'].values():
        for key in list(group):
            if key != 'purpose':
                group[key] = None
    clearance_template = OUT / 'independent-review-clearance-v16.json.template'
    save(clearance_template, future)
    files.append({'donor': str(clearance_path), 'donor_sha256': digest(clearance_path),
                  'prepared_template': str(clearance_template), 'prepared_sha256': digest(clearance_template),
                  'final_path': str(OUT / 'independent-review-clearance-v16.json'), 'mode': '0600',
                  'template_mode': '0600', 'materialize': False, 'kind': 'unissued_flat_clearance_schema',
                  'root_after_review_binding_required': True})
    cache_template = HERE / 'seed-warm-go-cache.py.in'
    cache_source = cache_template.read_text()
    ast.parse(cache_source, filename=str(cache_template))
    compile(cache_source, str(cache_template), 'exec')
    python_count += 1
    files.append({'donor': None, 'prepared_template': str(cache_template), 'prepared_sha256': digest(cache_template),
                  'final_path': str(HERE / 'seed-warm-go-cache.py'), 'mode': '0600', 'template_mode': '0600',
                  'materialize': True, 'kind': 'fixed_task_Darwin_COW_cache_seed_only',
                  'python_AST_compile_only': True, 'pending_fields': ['PENDING_ROOT_FINAL_SHA']})
    counts = {}
    for filename, expected in [('fake-preflight-v16-provenance.py.in', 46),
                               ('adapter-env-preflight-v16.py.in', 7),
                               ('final_state_audit_test.py.in', 16)]:
        row = next(row for row in files if Path(row['prepared_template']).name == filename)
        tree = ast.parse(Path(row['prepared_template']).read_text())
        count = sum(isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name.startswith('test_') for node in ast.walk(tree))
        if count != expected:
            raise RuntimeError('Control definition inventory changed')
        counts[filename] = count
    if any(digest(path) != checksum for path, checksum in {**protected, **protected_ccg}.items()):
        raise RuntimeError('V15/CCG15 protected file changed during static preparation')
    result = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'owner': OWNER,
              'candidate_root': str(ROOT), 'donor_candidate_sha': OLD_SHA, 'candidate_sha': None,
              'state': 'templates_only_final_source_resources_and_permissions_pending',
              'materialization_donor': {'path': str(materialization_path), 'sha256': digest(materialization_path)},
              'replacements': {key: value for key, value in replacements.items()}, 'files': files,
              'protected_v15': protected, 'protected_CCG15': protected_ccg,
              'pending_fields': sorted({field for row in files for field in row.get('pending_fields', [])}),
              'seed_database_proposal': 'multica_closure_v16_seed_01a1a3c9',
              'Redis_name_proposal': 'multica-v16-01a1a3c9-redis',
              'temp_prefix': '/private/tmp/multica-final-check-v16-',
              'Go_adapter_sha256': GO_HASH, 'legacy_reporting_basenames_retained': True,
              'python_AST_compile_only_count': python_count, 'control_source_definitions': counts,
              'control_sources_total': sum(counts.values()), 'controls_executed': 0,
              'cache_seed_executed': False, 'DB_build_service_test_operations': 0,
              'knowledge_budget_allocated': 0, 'knowledge_real_calls': 0, 'production_default_enabled': False}
    save(HERE / 'prepared-template-manifest.json', result)
    print(json.dumps({'templates': len(files), 'python_AST_compile_only': python_count,
                      'control_sources': sum(counts.values()), 'actual_controls_or_runtime': 0,
                      'candidate_sha': None, 'Go_adapter': GO_HASH}))


if __name__ == '__main__':
    main()
