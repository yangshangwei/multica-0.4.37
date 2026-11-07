"""Read retained evidence; do not invoke tests, services, or database clients."""
import collections
import datetime
import hashlib
import json
from pathlib import Path
import re
import subprocess

ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
TASK = ROOT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes'
OLD = TASK / 'research/execution-20261103'
OUT = Path(__file__).parent
WT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = WT / '.omx/private'


def sha(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def read(path):
    return json.loads(Path(path).read_text())


def save(name, data):
    with (OUT / name).open('w') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write('\n')


def git(*args):
    return subprocess.check_output(['git', '-C', str(WT), *args], text=True).strip()


hash_checks = {}


def check(path, expected=None):
    p = Path(path)
    if not p.is_absolute():
        p = ROOT / p
    key = str(p)
    actual = sha(p) if p.is_file() else None
    row = {'path': key, 'exists': p.is_file(), 'sha256': actual}
    if expected:
        row.update(expected_sha256=expected, hash_match=actual == expected)
    hash_checks[key] = row
    return row


def refs(value):
    if isinstance(value, dict):
        if isinstance(value.get('path'), str) and value.get('sha256'):
            check(value['path'], value['sha256'])
        for child in value.values():
            refs(child)
    elif isinstance(value, list):
        for child in value:
            refs(child)


source = read(OLD / 'knowledge-gate-rebind/source-evidence.json')
prior = read(OLD / 'knowledge-gate-rebind/review.json')
summary = read(OLD / 'fullcheck-v13/result-summary.json')
go_inventory = read(OLD / 'fullcheck-v13/go-inventory.json')
pw_inventory = read(OLD / 'fullcheck-v13/playwright-inventory.json')
ccg_preparation = read(OLD / 'ccg-current-source-preparation/preparation.json')
ccg_review = read(OLD / 'ccg-current-source-review/review.json')
refs(source)
refs(summary)
refs(ccg_review)
for path, expected in read(OLD / 'knowledge-gate-rebind/manifest.json').items():
    check(OLD / 'knowledge-gate-rebind' / path, expected)
for path, spec in ccg_preparation['files'].items():
    check(path, spec['sha256'])

source_checks = []
for spec in source['source_comparisons']:
    item = check(WT / spec['path'], spec['candidate_sha256'])
    source_checks.append({**spec, 'current_sha256': item['sha256'], 'current_matches_review': item['hash_match']})

all_terminals = []
package_checks = []
go_counts = collections.Counter()
for package in go_inventory['packages']:
    raw_check = check(package['raw'], package['raw_sha256'])
    meta_check = check(package['metadata'], package['metadata_sha256'])
    counts = collections.Counter()
    terminals = []
    parse_errors = 0
    for raw_line in Path(package['raw']).read_bytes().splitlines():
        try:
            event = json.loads(raw_line)
        except (ValueError, UnicodeError):
            parse_errors += 1
            continue
        action, test = event.get('Action'), event.get('Test')
        if test and action in ('run', 'pass', 'fail', 'skip'):
            counts[action] += 1
        if test and action in ('pass', 'fail', 'skip'):
            all_terminals.append({'package': event['Package'], 'test': test, 'action': action, 'time': event.get('Time')})
        if not test and action in ('pass', 'fail', 'skip'):
            terminals.append(action)
    normalized = {key: counts[key] for key in ('run', 'pass', 'fail', 'skip')}
    metadata = read(package['metadata'])
    go_counts.update(counts)
    package_checks.append({
        'package': package['package'], 'database': package['database'], 'command': package['command'],
        'exit_code': package['exit_code'], 'metadata': meta_check, 'raw': raw_check,
        'counts': normalized, 'counts_match_inventory': normalized == package['named_counts'],
        'package_terminals': terminals, 'compiled_without_tests': package['compiled_without_tests'],
        'race_requested': '-race' in package['command'], 'non_JSON_lines': parse_errors,
        'metadata_database': metadata.get('database'),
    })
save('go-package-recheck.json', package_checks)

report_path = Path(pw_inventory['source_report'])
check(report_path, pw_inventory['source_sha256'])
pw_report = read(report_path)
pw_cases = []


def walk(suites):
    for suite in suites:
        walk(suite.get('suites', []))
        for spec in suite.get('specs', []):
            for test in spec.get('tests', []):
                attempts = test.get('results', [])
                statuses = [r.get('status') for r in attempts]
                final = statuses[-1] if statuses else 'unrun'
                pw_cases.append({'file': spec['file'], 'title': spec['title'], 'status': final,
                                 'runner_status': test.get('status'), 'attempts': len(attempts),
                                 'retries': [r.get('retry') for r in attempts],
                                 'annotations': test.get('annotations', [])})


walk(pw_report.get('suites', []))
pw_counts = collections.Counter(t['status'] for t in pw_cases)
log = PRIVATE / 'final-check-v13/make-check-v13.log'
run = read(PRIVATE / 'final-check-v13/make-check-v13.json')
check(log, run['log_sha256'])
plain_log = re.sub(r'\x1b\[[0-9;]*m', '', log.read_text())
static = [{'line': i + 1, 'text': line} for i, line in enumerate(plain_log.splitlines())
          if re.search(r'^\s*Tasks:|^@multica/[^:]+:test:\s+(?:Test Files|Tests)\s+\d+ passed|^@multica/views:test:.*(?:collaboration/knowledge-panel\.test\.tsx|locales/parity\.test\.ts)', line)
          or 'go vet ./...' in line]
ts_counts = {}
for package, count in re.findall(r'@multica/([^:]+):test:\s+Tests\s+([\d,]+) passed', plain_log):
    ts_counts[package] = int(count.replace(',', ''))

dispositions = []
for disposition in source['audit_dispositions']:
    canonical = disposition['canonical_regression']
    path = WT / canonical['path']
    expected = disposition['canonical_source']['candidate_sha256']
    canonical_check = check(path, expected)
    name = canonical['name']
    terminals = [t for t in all_terminals if t['test'] == name]
    item = {key: disposition[key] for key in ('id', 'disposition', 'behavior', 'reviews', 'commits_or_followups') if key in disposition}
    item.update(canonical_regression=canonical, canonical_source=canonical_check,
                canonical_name_present=name in path.read_text(), actual_v13_Go_terminals=terminals,
                evidence=[check(e['path'], e['sha256']) for e in disposition.get('evidence', [])])
    if canonical['path'].endswith('.tsx'):
        item['actual_v13_TS_file_result'] = [x for x in static if 'knowledge-panel.test.tsx' in x['text']]
    if disposition['id'] == 'L13':
        item['canonical_name_present'] = 'composite description; verify locale parity suite plus terminology, not a literal test title'
        item['actual_v13_TS_file_result'] = [x for x in static if 'locales/parity.test.ts' in x['text']]
    if disposition['id'] == 'L7':
        item['classification'] = 'Original audit not reproduced; no red baseline required or manufactured'
        names = ['TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated',
                 'TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit',
                 'TestHostLifecycleStartedFailureStillWaitsAndBlocksFallback']
        item['all_three_actual_v13_terminals'] = [t for t in all_terminals if t['test'] in names]
    dispositions.append(item)
save('dispositions-21.json', dispositions)

skip_rows = []
for skip in go_inventory['skipped']:
    skip_rows.append({**skip, 'task7_counted_as_pass': False,
                      'task7_relation': 'Separate platform/integration/performance/known limitation; not one of the 21 audit canonical regressions or CCG1'})
for case in pw_cases:
    if case['status'] == 'skipped':
        skip_rows.append({**case, 'task7_counted_as_pass': False,
                          'task7_relation': 'Direct CCG2 blocker: need separately bound OFF and ON actual pass' if case['file'] == 'collaboration-knowledge-deployed.spec.ts' else 'Separate scenario/platform/configuration gate; retained explicitly, not passed by v13'})
save('skip-dispositions.json', skip_rows)

sqlc = read(OLD / 'sqlc/sqlc-resume-01.json')
sqlc['current_server_tree'] = git('rev-parse', 'HEAD:server')
sqlc['verified_SQLC_server_tree'] = git('rev-parse', sqlc['candidate_head'] + ':server')
sqlc['server_tree_equal'] = sqlc['current_server_tree'] == sqlc['verified_SQLC_server_tree']
sqlc['current_server_tracked_diff'] = git('diff', '--name-only', 'HEAD', '--', 'server').splitlines()

ac_updates = {
    'AC1': ('retained_dispositions_confirmed_current_sources', '21 rows in dispositions-21.json; 19 product repairs, 1 terminology repair, L7 test-covered non-reproduction. Final whole-candidate gate remains open.'),
    'AC2': ('retained_contract_plus_actual_v13_race', 'Canonical H1 v13 pass and exact current source; full successor gate still required.'),
    'AC3': ('retained_contract_plus_actual_v13_race', 'H3/M3 names passed v13; full history and concurrent-index bytes unchanged.'),
    'AC4': ('retained_contract_plus_actual_v13_race', 'M1/M2 named v13 pass; changed Host lock graph still requires current CCG1 normal/race.'),
    'AC5': ('retained_contract_plus_actual_v13_race', 'H2 named v13 pass; unreachable resume rebinding and L7 three names retain exact source.'),
    'AC6': ('v13_core_views_and_8knowledge_pass_successor_gate_open', 'v13 core/views suites and 7 Web+1 native Desktop knowledge cases passed. Broader successor full check required; no closure from phase passes.'),
    'AC7': ('actual_sqlc_zero_drift_and_concurrent_indexes_confirmed', 'Real SQLC exit0 generated130 unchanged; b6 and85ef server trees equal, no current server dirty source. 761–763 contract retained.'),
    'AC8': ('blocked_current_complete_gate_and_CCG', 'v13 actual race82 packages20963pass0fail22skip, static15 andTS13244; whole run exit2. Need successor make0+finalaudit pass+complete inventory, CCG1 normal/race andCCG2OFF/ON.'),
    'AC9': ('retained_contract_sync_current_sources_confirmed', 'Knowledge specs/built-in instructions exact reviewed source; v13 built-in/current suites passed. Final new candidate identity and gate required.'),
}
ac = [{'id': x['id'], 'state': ac_updates[x['id']][0], 'assessment': ac_updates[x['id']][1],
       'prior_source_reason': x['evidence']} for x in prior['AC1_AC9']]
save('ac-matrix.json', ac)

review = {
    'schema_version': 1, 'reviewed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'reviewer': '/root/closure_inventory', 'candidate_root': str(WT), 'candidate_sha_at_read': git('rev-parse', 'HEAD'),
    'verdict': 'evidence_consistent; closure_blocked_pending_successor_full_gate_and_current_CCG',
    'task7_closable': False, 'new_product_findings': [],
    'candidate_dirty_at_finish': git('status', '--short').splitlines(),
    'hash_checked_paths': len(hash_checks), 'hash_mismatches': [v for v in hash_checks.values() if v.get('hash_match') is False or not v['exists']],
    'source_comparisons': source_checks, 'disposition_counts': dict(collections.Counter(x['disposition'] for x in dispositions)),
    'independent_review_count': len(source['independent_reviews']), 'followup_review_count': len(source['followup_reviews']),
    'Go_actual': {'package_count': len(package_checks), 'counts': {key: go_counts[key] for key in ('run', 'pass', 'fail', 'skip')},
                  'counts_match_each_package': all(x['counts_match_inventory'] for x in package_checks),
                  'unique_databases': len({x['database'] for x in package_checks}),
                  'all_race': all(x['race_requested'] for x in package_checks),
                  'compile_only_packages': sum(x['compiled_without_tests'] for x in package_checks),
                  'package_terminal_failures': [x['package'] for x in package_checks if 'fail' in x['package_terminals']]},
    'TS_actual': {'counts': ts_counts, 'total': sum(ts_counts.values()), 'raw_summary_lines': static},
    'Playwright_actual': {'count': len(pw_cases), 'counts': dict(pw_counts), 'stats': pw_report.get('stats'),
                          'extra_attempts': sum(max(0, x['attempts'] - 1) for x in pw_cases),
                          'knowledge': [x for x in pw_cases if 'knowledge' in x['file']]},
    'SQLC': {k: sqlc[k] for k in ['exit_code', 'generated_files_before', 'generated_files_after', 'generated_unchanged', 'changed_generated_files', 'source_inputs_unchanged', 'server_tree_equal', 'current_server_tracked_diff']},
    'actual_final_audit': summary['final_identity_audit'], 'make_exit': run['exit_code'],
    'CCG': {'templates_candidate': ccg_preparation['candidate_sha'], 'templates_ready': False, 'current_actual_runs': 0,
            'previous_independent_verdict': ccg_review['verdict'],
            'mandatory_rebind': 'Successor SHA, private path, source-inputs, successful full run, reviewed final auditor/parser, tool/source hashes, fresh four DB allocations and OFF/ON resource/build identities; preserve v13 originals.'},
    'remaining_blockers': ['Repair remaining directed viewport failure and verify reviewed successor.',
                           'One successor frozen full make check exit0 with actual final audit pass and complete named/skip inventory plus root live identity proof.',
                           'Fresh CCG1 exact correction normal and race: one named pass each with two starts, corrected input and zero third start.',
                           'Current separate CCG2 OFF and ON deployments: one pass each, no skips/retries, full retained evidence attachment.',
                           'Independent final task7 review of resulting source/evidence; #6 model/release not a prerequisite.'],
    'operations': {'runtime_tests': False, 'lint_or_typecheck_executed': False, 'reason': 'Parent assigned read-only evidence reconciliation; actual retained logs reparsed instead.',
                   'database_API_Provider_calls': 0, 'source_edits': False, 'task_metadata_edits': False,
                   'writes_only': str(OUT)},
}
save('hash-checks.json', list(hash_checks.values()))
save('review.json', review)
print(json.dumps({k: review[k] for k in ['verdict', 'hash_checked_paths', 'hash_mismatches', 'disposition_counts', 'Go_actual', 'SQLC', 'make_exit']}, indent=2))
print('TS', ts_counts, 'PW', dict(pw_counts))
