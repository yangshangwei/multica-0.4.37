"""Read retained #7 evidence; never initialize a runtime or run a product test."""
from collections import Counter
from datetime import datetime, timezone
import ast
import hashlib
import json
from pathlib import Path
import re
import subprocess

ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
TASK = ROOT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes'
PRIOR = TASK / 'research/progress-recheck-20261103-01a1a1c6-97cddb08'
CANDIDATE = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = Path(__file__).parent
V14 = CANDIDATE / '.omx/private/final-check-v14'
CCG = CANDIDATE / '.omx/private/knowledge-final-gates-v14'

def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()

def read(path):
    return json.loads(Path(path).read_text())

def git(*args):
    return subprocess.check_output(['git', '-C', str(CANDIDATE), *args], text=True).strip()

def reference(path):
    return {'path': str(path), 'sha256': digest(path)}

ansi = re.compile(r'\x1b\[[0-9;]*[a-zA-Z]')
old = read(PRIOR / 'audit7-review/review.json')
head_before, status_before = git('rev-parse', 'HEAD'), git('status', '--porcelain=v1')
hash_mismatches, missing = [], []
for name, expected in old['proof_hashes'].items():
    path = Path(name)
    if not path.is_absolute():
        path = ROOT / path
    if not path.is_file():
        missing.append(str(path))
        continue
    actual = digest(path)
    if actual != expected:
        hash_mismatches.append({'path': str(path), 'expected_sha256': expected, 'actual_sha256': actual})
assert not missing
assert [row['path'] for row in hash_mismatches] == [str(V14 / 'make-check-v14.json')]
v14 = read(V14 / 'make-check-v14.json')
assert v14['state'] == 'complete' and v14['exit_code'] == 2

binding = read(PRIOR / 'v14-live-proof/completion-binding-review.json')
go_terminals, go_counts, package_counts = {}, Counter(), Counter()
go_raw_checks = []
for row in binding['actual_named_raw_Go_source_files']:
    assert digest(row['path']) == row['sha256']
    go_raw_checks.append(row)
    for line_number, line in enumerate(Path(row['path']).read_text().splitlines(), 1):
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get('Action') not in ('pass', 'fail', 'skip'):
            continue
        if event.get('Test'):
            go_counts[event['Action']] += 1
            go_terminals[(event['Package'], event['Test'])] = {
                'action': event['Action'], 'raw': row['path'], 'raw_sha256': row['sha256'], 'raw_line': line_number
            }
        elif event.get('Package'):
            package_counts[event['Action']] += 1
assert dict(go_counts) == {'pass': 20963, 'skip': 22}

dispositions = []
for row in old['dispositions']:
    canonical = row['canonical_regression']
    source = CANDIDATE / canonical['path']
    assert digest(source) == row['source_sha256']
    assert canonical['name'] in source.read_text() or canonical.get('source_template', '<no-template>') in source.read_text()
    proof_lines = []
    for proof in row.get('evidence', []):
        assert digest(proof['path']) == proof['sha256']
        if '-red.' not in proof['path']:
            continue
        for number, line in enumerate(Path(proof['path']).read_text().splitlines(), 1):
            clean = ansi.sub('', line)
            if canonical['name'] in clean and ('--- FAIL:' in clean or re.search(r'(^|\s)FAIL\s', clean) or '×' in clean):
                proof_lines.append({'path': proof['path'], 'sha256': proof['sha256'], 'line': number, 'text': clean.strip()})
    if row['id'] not in ('L7', 'L13'):
        assert proof_lines, row['id']
    package = 'github.com/multica-ai/multica/' + str(Path(canonical['path']).parent)
    green = go_terminals.get((package, canonical['name']))
    if canonical['path'].startswith('server/'):
        assert green and green['action'] == 'pass', row['id']
    dispositions.append({
        'id': row['id'], 'disposition': row['disposition'], 'behavior': row['behavior'],
        'canonical': canonical, 'source_sha256': row['source_sha256'],
        'red_named_failure_witnesses': proof_lines,
        'current_v14_go_green': green,
        'evidence': [{'path': e['path'], 'sha256': e['sha256']} for e in row.get('evidence', [])],
        'exception': row.get('exception'),
        'TS_green_scope': None if green else 'Prior focused/file-level evidence retained; final candidate named leaves must come from successor full-run JSON.'
    })
assert len(dispositions) == 21 and sum(bool(r['red_named_failure_witnesses']) for r in dispositions) == 19

go_skips = read(PRIOR / 'audit7-review/v14-go-skip-review.json')['rows']
assert len(go_skips) == 22
for row in go_skips:
    assert go_terminals[(row['package'], row['test'])]['action'] == 'skip'
    assert digest(row['raw']) == row['raw_sha256']
    for source in row['source']:
        assert digest(source['path']) == source['sha256']
        text = Path(source['path']).read_text().splitlines()
        assert 'Skip' in text[source['line'] - 1]
    assert row['is_canonical_audit_regression'] is False and row['counted_as_pass'] is False

browser_witness = read(PRIOR / 'v14-live-proof/browser-terminal-observation.json')
assert digest(browser_witness['source_report_path']) == browser_witness['source_report_sha256']
browser_report = read(browser_witness['source_report_path'])
browser_cases = []
def visit(suites):
    for suite in suites:
        visit(suite.get('suites', []))
        for spec in suite.get('specs', []):
            for test in spec.get('tests', []):
                attempts = test.get('results', [])
                browser_cases.append({
                    'spec_id': spec['id'], 'file': spec['file'], 'title': spec['title'],
                    'project': test.get('projectName'), 'repeatEachIndex': test.get('repeatEachIndex'),
                    'final': attempts[-1]['status'] if attempts else 'unrun',
                    'retry': sum(attempt.get('retry', 0) for attempt in attempts),
                    'annotations': test.get('annotations', [])
                })
visit(browser_report['suites'])
assert len(browser_cases) == 441 and not browser_report.get('errors')
assert dict(Counter(row['final'] for row in browser_cases)) == {'skipped': 91, 'passed': 334, 'failed': 16}
assert all(row['retry'] == 0 for row in browser_cases)
actual_map = {(r['spec_id'], r['project'], r['repeatEachIndex']): r['final'] for r in browser_cases}
assert all(actual_map[(r['spec_id'], r['project'], r['repeatEachIndex'])] == r['actual_final_terminal'] for r in browser_witness['cases'])
browser_skips = []
for row in browser_cases:
    if row['final'] != 'skipped':
        continue
    guards = []
    for annotation in row['annotations']:
        if annotation['type'] != 'skip':
            continue
        description = annotation['description']
        location = annotation['location']
        source = Path(location['file'])
        reason_sources = []
        if description not in source.read_text():
            matches = subprocess.check_output(['rg', '-l', '--fixed-strings', description, str(CANDIDATE / 'e2e')], text=True).splitlines()
            assert matches
            reason_sources = [reference(Path(match)) for match in matches]
        lines = source.read_text().splitlines()
        guards.append({
            'reason': description, 'source': str(source), 'source_sha256': digest(source), 'line': location['line'],
            'guard_excerpt': lines[max(0, location['line'] - 5):location['line'] + 3],
            'shared_reason_sources': reason_sources
        })
    assert guards
    file = row['file']
    category = ('required_current_CCG2_OFF_ON' if file == 'collaboration-knowledge-deployed.spec.ts'
                else 'independent_packaged_native_or_installed_gate' if 'installed' in file
                else 'password_or_admin_acceptance_lane' if any(part in file for part in ['password', 'platform-admin', 'admin-', 'administration'])
                else 'explicit_cli_fixture_lane' if any(part in file for part in ['cli-forbidden', 'upstream-backports-cli'])
                else 'deterministic_AI_fixture_lane' if any(part in file for part in ['assist', 'actor-picker'])
                else 'other_dedicated_integration_lane')
    browser_skips.append({**row, 'guards': guards, 'category': category, 'counted_as_pass': False,
                          'task7_handoff': 'Run CCG2 twice on separate fake-only OFF/ON deployments before #7 final closure.' if category == 'required_current_CCG2_OFF_ON' else 'Retain this independent configured/platform scope; it is not a #7 canonical audit regression or a passing substitute.'})
assert len(browser_skips) == 91

sqlc_path = Path(old['SQLC']['record'])
sqlc = read(sqlc_path)
assert sqlc['exit_code'] == 0 and sqlc['accepted'] and sqlc['source_inputs_unchanged'] and sqlc['generated_unchanged']
assert sqlc['generated_files_before'] == sqlc['generated_files_after'] == 130
assert sqlc['environment']['GOPROXY'] == 'file:///Users/artisan/go/pkg/mod/cache/download'
assert digest(sqlc['log_path']) == sqlc['log_sha256']
assert digest(sqlc['previous_failed_record']) == sqlc['previous_failed_record_sha256']
source_comparisons = []
for row in old['source_comparisons']:
    source = CANDIDATE / row['path']
    assert digest(source) == row['actual_current_sha256'], row['path']
    source_comparisons.append({'path': row['path'], 'sha256': digest(source), 'matches_previous_audit': True})

ccg_inputs = read(CCG / 'source-inputs.json')
for name, expected in ccg_inputs['source_sha256'].items():
    assert digest(CANDIDATE / name) == expected
ccg_templates, ccg_runners = [], []
for name in ['run-ccg1.py', 'run-ccg2.py']:
    path = CCG / name
    ast.parse(path.read_text())
    ccg_runners.append(reference(path))
for path in sorted((CCG / 'permissions').glob('*.template')):
    permit = read(path)
    assert permit['ready_to_execute'] is False and permit['root_released_complete_check_resources'] is False
    assert permit['root_verified_complete_named_inventory'] is False and permit['root_verified_fresh_database'] is False
    assert permit['real_provider_allowed'] is False
    assert permit['source_inputs_sha256'] == digest(CCG / 'source-inputs.json')
    ccg_templates.append({**reference(path), 'gate': permit['gate'], 'ready_to_execute': False})
assert len(ccg_templates) == 4

head_after, status_after = git('rev-parse', 'HEAD'), git('status', '--porcelain=v1')
current_ac = [dict(row) for row in old['AC1_AC9']]
for row in current_ac:
    if row['id'] == 'AC1':
        row['remaining'] = 'Current frozen successor complete gate and named TS leaf evidence; embedded historical failures and mutants remain retained.'
    if row['id'] == 'AC6':
        row['remaining'] = 'Final successor full-run JSON named TS leaves and current UI/browser binding; prior 7 Web + 1 native scope remains historical.'
    if row['id'] == 'AC8':
        row['remaining'] = 'Sole-owner frozen successor whole make exit0, actual final audit, full Go/TS/browser named and individual skip inventory, and root live resource identity/release.'
review = {
    'schema_version': 1, 'reviewed_at': datetime.now(timezone.utc).isoformat(),
    'reviewer': '/root/task7_contract_review', 'active_task': str(TASK),
    'scope': 'Independent read-only retained contract/source/evidence readiness; no runtime, package test, DB, service, browser or Provider operation.',
    'verdict': 'conditional_ready; no new confirmed product blocker; shared full pass, current CCG1/CCG2 and final independent binding still required',
    'task7_closable_now': False, 'independent_of_task6_real_model_and_release': True,
    'candidate': {'root': str(CANDIDATE), 'head_before': head_before, 'head_after': head_after, 'status_before': status_before, 'status_after': status_after},
    'prior_review': reference(PRIOR / 'audit7-review/review.json'),
    'retained_input_hash_check': {'checked': len(old['proof_hashes']), 'missing': missing, 'mismatches': hash_mismatches,
                               'mismatch_disposition': 'Expected state transition of mutable v14 result from running snapshot to actual complete exit2; historical review preserved.'},
    'dispositions': dispositions, 'disposition_count': 21, 'reproduced_red_count': 19,
    'AC1_AC9': current_ac, 'retained_previous_AC1_AC9': old['AC1_AC9'],
    'retained_previous_AC1_AC9_note': 'Historical running-v14 review; actual v14 complete exit2 cannot satisfy the current gate.',
    'source_comparisons': source_comparisons,
    'baseline_product_6aed_4e3': {'only_task_metadata_diff': True, 'paths': subprocess.check_output(['git', '-C', str(ROOT), 'diff', '--name-only', '6aed33aad', '4e3b61500'], text=True).splitlines()},
    'SQLC': {**old['SQLC'], 'record_sha256': digest(sqlc_path), 'actual_environment_fix': {'GOPROXY': sqlc['environment']['GOPROXY'], 'GOMODCACHE': sqlc['environment']['GOMODCACHE'], 'GOTOOLCHAIN': 'local', 'GOSUMDB': 'off'},
             'previous_failed_record': reference(sqlc['previous_failed_record']), 'rerun_here': False},
    'migrations': old['migrations'], 'concurrent_index_cleanup_registered': old['concurrent_index_cleanup_registered'],
    'v14_final': {'result': reference(V14 / 'make-check-v14.json'), 'exit_code': 2, 'wholecheck_accepted': False,
                  'final_identity_audit_pass': binding['final_identity_audit_status'] == 'pass',
                  'Go_named_counts': dict(go_counts), 'Go_package_terminal_counts': dict(package_counts), 'Go_raw_checked': 82,
                  'browser_counts': dict(Counter(row['final'] for row in browser_cases)), 'browser_case_count': 441,
                  'browser_report': reference(browser_witness['source_report_path']), 'browser_retry_count': 0,
                  'TS_scope': {'passed': 13244, 'files': 1069, 'named_leaf_inventory_complete': False}},
    'go_skip_review': {'count': 22, 'rows': go_skips, 'all_raw_and_current_source_guards_match': True, 'none_counted_as_pass': True},
    'browser_skip_review': {'count': 91, 'rows': browser_skips, 'all_named_guards_have_current_source_witnesses': True,
                            'groups': dict(Counter(row['category'] for row in browser_skips)), 'none_counted_as_pass': True},
    'current_CCG1_CCG2': {**old['current_CCG1_CCG2'], 'inputs': reference(CCG / 'source-inputs.json'), 'runners': ccg_runners,
                         'templates': ccg_templates, 'successor_requirement': 'Copy into new versioned namespace; rebind EXPECTED/PRIVATE/temp prefix, final successful full-run audit/parser/review and source/tool/build/deployment identities. Preserve old false templates and failures.'},
    'closure_prerequisites': ['Sole-owner successor whole make exit0', 'Hash-bound final source identity audit pass',
                              'Complete Go/TS/browser named terminals and separately reconciled skips; root live identity and resource release',
                              'Current CCG1 normal then race on distinct fresh migrated DBs', 'Current CCG2 OFF then ON on separately owned fake-only password/managed deployments',
                              'Independent final full-scope #7 review and exact final source binding; no unhandled blocker'],
    'knowledge_budget': {'allocated': 0, 'real_calls': 0, 'production_default_enabled': False, 'other_budget_transfers': False},
    'verification_here': {'source_and_evidence_hashes': 'pass', 'named_red_and_retained_Go_browser_terminal_readback': 'pass', 'CCG_AST': 'pass',
                          'lint': 'not run: explicit read-only scope', 'typecheck': 'not run: explicit read-only scope', 'product_tests': 'not run: sole-owner shared execution retained'}
}
(OUT / 'review.json').write_text(json.dumps(review, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'verdict': review['verdict'], 'dispositions': 21, 'red': 19, 'Go': dict(go_counts), 'browser': review['v14_final']['browser_counts'], 'CCG_templates_false': 4, 'review': str(OUT / 'review.json')}))
