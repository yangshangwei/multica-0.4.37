"""Reconstruct actual completed v14 inventories without invoking any tests."""
import collections
import datetime
import hashlib
import json
from pathlib import Path
import re

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v14'
PREVIOUS = HERE.parents[1] / 'execution-20261103/fullcheck-v13'


def read(path):
    return json.loads(Path(path).read_text())


def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def ref(path):
    return {'path': str(path), 'sha256': digest(path)}


def write(name, data):
    with (HERE / name).open('x') as output:
        json.dump(data, output, ensure_ascii=False, indent=2)
        output.write('\n')


run_path = OUT / 'make-check-v14.json'
run = read(run_path)
assert run['state'] == 'complete'
finish = read(HERE / 'execution-finished.json')
log_path = OUT / 'make-check-v14.log'
assert digest(log_path) == run['log_sha256']
log = re.sub(r'\x1b\[[0-9;]*[a-zA-Z]', '', log_path.read_text(errors='replace'))
expected_go = [p['package'] for p in read(PREVIOUS / 'go-inventory.json')['packages']]
packages, terminals, skips, failures = [], [], [], []
for metadata in sorted((OUT / 'go-packages-v14').glob('*.json')):
    item = read(metadata)
    raw = metadata.with_suffix('.jsonl')
    counts = collections.Counter()
    package_terminals = []
    outputs = collections.defaultdict(list)
    parse_errors = 0
    for line in raw.read_bytes().splitlines():
        try:
            event = json.loads(line)
        except (ValueError, UnicodeError):
            parse_errors += 1
            continue
        name, action = event.get('Test'), event.get('Action')
        if name and action == 'output':
            outputs[name].append(event.get('Output', ''))
        if name and action in ('run', 'pass', 'fail', 'skip'):
            counts[action] += 1
        if name and action in ('pass', 'fail', 'skip'):
            row = {'package': event['Package'], 'test': name, 'action': action,
                   'time': event.get('Time'), 'elapsed': event.get('Elapsed')}
            terminals.append(row)
            if action in ('skip', 'fail'):
                detail = {**row, 'output': ''.join(outputs[name]), 'raw': str(raw)}
                (skips if action == 'skip' else failures).append(detail)
        if not name and action in ('pass', 'fail', 'skip'):
            package_terminals.append(action)
    actual = {key: counts[key] for key in ('run', 'pass', 'fail', 'skip')}
    packages.append({**{key: item.get(key) for key in ('package', 'database', 'template', 'state', 'exit_code', 'command', 'started_at', 'finished_at', 'compiled_without_tests')},
                     'metadata': ref(metadata), 'raw': ref(raw), 'counts': actual,
                     'counts_match_metadata': actual == item.get('counts'),
                     'raw_matches_metadata': digest(raw) == item.get('log_sha256'),
                     'package_terminals': package_terminals, 'non_JSON_lines': parse_errors})
totals = {key: sum(p['counts'][key] for p in packages) for key in ('run', 'pass', 'fail', 'skip')}
unrun_packages = sorted(set(expected_go) - {p['package'] for p in packages})
go = {'scope': 'Actual v14 package/raw terminal inventory; race and compile-only remain distinct.',
      'expected_package_source': ref(PREVIOUS / 'go-inventory.json'),
      'expected_package_qualification': 'Server source tree unchanged between v13 and v14; only seven E2E files changed.',
      'package_count': len(packages), 'expected_package_count': len(expected_go), 'named_counts': totals,
      'packages': packages, 'named_terminal_inventory': terminals, 'skipped': skips, 'failed': failures,
      'unrun_packages': unrun_packages, 'unique_package_databases': len({p['database'] for p in packages}),
      'all_packages_requested_race': all('-race' in p['command'] for p in packages),
      'compile_only_packages': sum(p['compiled_without_tests'] is True for p in packages)}
write('go-inventory.json', go)

ts_files = []
for line_no, line in enumerate(log.splitlines(), 1):
    match = re.match(r'(@[^ :]+):test:\s+[✓✔]\s+(\S+)\s+\((\d+) tests?\)', line)
    if match:
        ts_files.append({'package': match[1], 'file': match[2], 'tests': int(match[3]), 'line': line_no, 'status': 'passed'})
ts_summaries = {}
for line in log.splitlines():
    match = re.search(r'(@[^ :]+):test:\s+Tests\s+(.+)', line)
    if match:
        ts_summaries[match[1]] = {kind: int(count) for count, kind in re.findall(r'(\d+)\s+(passed|failed|skipped|todo)', match[2])}
ts = {'scope': 'Actual configured Vitest reporter: named file inventory and package test totals; individual case names are not emitted for every successful test.',
      'packages': ts_summaries, 'files': ts_files, 'total_passed': sum(p.get('passed', 0) for p in ts_summaries.values()),
      'raw': ref(log_path)}
write('typescript-inventory.json', ts)

report_path = OUT / 'makecheck-playwright-results-v9.json'
previous_pw = read(PREVIOUS / 'playwright-inventory.json')
tests = []
report = read(report_path) if report_path.exists() else {}


def visit(suites, chain=()):
    for suite in suites:
        visit(suite.get('suites', []), chain + (suite.get('title', ''),))
        for spec in suite.get('specs', []):
            for test in spec.get('tests', []):
                results = test.get('results', [])
                last = results[-1] if results else {}
                status = last.get('status', 'unrun')
                if status == 'passed':
                    outcome = 'passed'
                elif status == 'skipped':
                    outcome = 'explicit_skipped'
                elif status in ('failed', 'timedOut', 'interrupted'):
                    outcome = 'failed' if status != 'interrupted' else 'interrupted'
                else:
                    outcome = 'unrun'
                tests.append({'id': test.get('testId', spec.get('id')), 'project': test.get('projectName'),
                              'file': spec['file'], 'title': spec['title'], 'suite': list(chain),
                              'expected_status': test.get('expectedStatus'), 'runner_status': test.get('status'),
                              'actual_status': status, 'outcome': outcome,
                              'annotations': test.get('annotations', []), 'attempts': len(results),
                              'retries': [r.get('retry') for r in results],
                              'duration_ms': sum(r.get('duration', 0) for r in results),
                              'errors': [error for r in results for error in r.get('errors', [])],
                              'attachments': [a for r in results for a in r.get('attachments', [])]})


visit(report.get('suites', []))
observed = {(t['project'], t['file'], t['title']) for t in tests}
expected = {(t['project'], t['file'], t['title']) for t in previous_pw['tests']}
missing = sorted(expected - observed)
for project, file, title in missing:
    tests.append({'project': project, 'file': file, 'title': title, 'outcome': 'unrun', 'actual_status': 'unrun',
                  'qualification': 'Retained current test registration; no terminal in this run'})
pw_counts = dict(collections.Counter(t['outcome'] for t in tests))
pw = {'scope': 'Actual v14 reporter; missing registered tests explicitly unrun. Changed seven files retain test declarations.',
      'source_report': ref(report_path) if report_path.exists() else None, 'stats': report.get('stats'),
      'actual_counts': pw_counts, 'total': len(tests), 'tests': tests,
      'unrun_or_blocked': len(missing) + sum(t['outcome'] == 'unrun' for t in tests if not t.get('qualification')),
      'top_level_errors': report.get('errors', []), 'missing_from_actual_report': missing}
write('playwright-inventory.json', pw)
write('browser-failures.json', [t for t in tests if t['outcome'] in ('failed', 'interrupted')])
write('explicit-skips.json', {'Go': skips, 'Playwright': [t for t in tests if t['outcome'] == 'explicit_skipped'],
                              'qualification': 'Every skip remains a skip; deployed knowledge OFF/ON and separate platform/provider gates are not credited as passes.'})
audit_path = Path(finish['final_audit_path']) if finish.get('final_audit_path') else None
audit = read(audit_path) if audit_path and audit_path.exists() else {'status': 'absent'}
complete_inventory = not unrun_packages and all(p['counts_match_metadata'] and p['raw_matches_metadata'] and p['state'] == 'complete' for p in packages) and report_path.exists() and not missing
static_match = re.search(r'Tasks:\s+(\d+) successful,\s+(\d+) total', log.split('[2/6]')[0])
summary = {'recorded_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
           'candidate_root': str(ROOT), 'candidate_sha': run['candidate_sha'],
           'state': 'complete_passed' if run['exit_code'] == 0 and audit['status'] == 'pass' and complete_inventory else 'complete_failed',
           'raw_run_state': run['state'],
           'started_at': run['started_at'], 'finished_at': run.get('finished_at'),
           'make_exit_code': run['exit_code'], 'launcher_exit_code': finish['launcher_exit_code'],
           'static_tasks_pass': int(static_match[1]) if static_match else None,
           'TS_pass': ts['total_passed'], 'TS_package_pass': ts_summaries,
           'Go': {'packages': len(packages), 'named_counts': totals, 'unrun_packages': unrun_packages,
                  'all_race': go['all_packages_requested_race'], 'compile_only': go['compile_only_packages']},
           'Playwright': {'total': len(tests), **pw_counts, 'unrun': pw['unrun_or_blocked'],
                          'knowledge_pass': sum(t['file'] == 'collaboration-knowledge.spec.ts' and t['outcome'] == 'passed' for t in tests)},
           'final_identity_audit': {'status': audit['status'], 'exit_code': finish.get('final_audit_exit_code'),
                                    'record': ref(audit_path) if audit_path and audit_path.exists() else None,
                                    'scope': 'Filesystem identity only; root separately owns actual API/database/process proof.'},
           'inventory_complete': complete_inventory,
           'wholecheck_machine_conditions_met': run['exit_code'] == 0 and audit['status'] == 'pass' and complete_inventory,
           'root_live_identity_approval_required': True, 'task7_closed': False, 'provider_calls': 0, 'reruns': 0,
           'artifacts': {'run': ref(run_path), 'log': ref(log_path), 'Go_inventory': ref(HERE / 'go-inventory.json'),
                         'TS_inventory': ref(HERE / 'typescript-inventory.json'), 'PW_inventory': ref(HERE / 'playwright-inventory.json')},
           'remaining': 'Root verifies actual live identity and skips; current CCG1 normal/race and CCG2 OFF/ON then independent task7 closure remain separate.'}
write('result-summary.json', summary)
print(json.dumps(summary, ensure_ascii=False))
