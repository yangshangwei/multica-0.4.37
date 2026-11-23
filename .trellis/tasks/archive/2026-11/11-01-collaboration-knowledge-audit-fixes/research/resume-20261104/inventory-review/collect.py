"""Collect real complete-check outputs without running tests or services.

Usage: python3 collect.py INPUTS.json --label NEW_RESEARCH_LABEL
INPUTS binds candidate_sha, candidate_root, full_result/raw_log/browser_report
as {path, sha256}, go_directory, and optional final_audit/live_proof specs.
All outputs are new files below this script's research directory.
"""
import argparse
import bisect
from collections import Counter, defaultdict
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re

HERE = Path(__file__).resolve().parent
PACKAGES = ('@multica/core', '@multica/docs', '@multica/views', '@multica/web', '@multica/desktop')
ANSI = re.compile(r'\x1b\[[0-9;]*[a-zA-Z]')
PREFIX = re.compile(r'^\s*((?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+):test:\s?(.*)$')
REPORT_START = re.compile(r'\{\s*"(?:numTotalTestSuites|numPassedTestSuites|numFailedTestSuites|numPendingTestSuites|numTotalTests|success|testResults)"\s*:')
MODULE_PREFIX = 'github.com/multica-ai/multica/'

def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()

def load(path):
    return json.loads(Path(path).read_text())

def reference(path):
    return {'path': str(path), 'sha256': digest(path), 'bytes': Path(path).stat().st_size}

def write_json(path, value):
    with Path(path).open('x') as target:
        json.dump(value, target, ensure_ascii=False, indent=2)
        target.write('\n')

def write_rows(path, rows):
    with Path(path).open('x') as target:
        for row in rows:
            target.write(json.dumps(row, ensure_ascii=False, separators=(',', ':')) + '\n')
    return reference(path)

def pinned(spec, problems, kind):
    if not isinstance(spec, dict) or not isinstance(spec.get('path'), str) or not isinstance(spec.get('sha256'), str):
        problems.append(kind + ': missing path/hash input')
        return None
    path = Path(spec['path'])
    if not path.is_file():
        problems.append(kind + ': missing file ' + str(path))
        return None
    if digest(path) != spec['sha256']:
        problems.append(kind + ': input hash mismatch ' + str(path))
        return None
    return path

def vitest_reports(raw_log):
    """Accept whole reporter objects, never an argv/reporting command marker."""
    streams = defaultdict(list)
    for number, raw in enumerate(raw_log.read_text().splitlines(), 1):
        match = PREFIX.match(ANSI.sub('', raw))
        if match:
            streams[match[1]].append((number, match[2]))
    decoder = json.JSONDecoder()
    found = defaultdict(list)
    for package, lines in streams.items():
        offsets, position = [], 0
        for _, line in lines:
            offsets.append(position)
            position += len(line) + 1
        text = '\n'.join(line for _, line in lines)
        consumed = -1
        for start in REPORT_START.finditer(text):
            if start.start() < consumed:
                continue
            try:
                value, end = decoder.raw_decode(text, start.start())
            except ValueError:
                continue
            required = ('numTotalTests', 'numPassedTests', 'numFailedTests', 'numPendingTests', 'numTodoTests')
            if not isinstance(value, dict) or not isinstance(value.get('testResults'), list):
                continue
            if not all(type(value.get(key)) is int and value[key] >= 0 for key in required) or type(value.get('success')) is not bool:
                continue
            if not all(isinstance(file, dict) and isinstance(file.get('name'), str) and isinstance(file.get('assertionResults'), list) for file in value['testResults']):
                continue
            consumed = end
            encoded = text[start.start():end]
            found[package].append({'report': value, 'raw_json': encoded,
                                   'raw_line': lines[bisect.bisect_right(offsets, start.start()) - 1][0],
                                   'report_sha256': hashlib.sha256(encoded.encode()).hexdigest()})
    return found

def collect_ts(raw_log, output, expected, problems, candidate=None):
    reports = vitest_reports(raw_log)
    leaves, files, skipped, summaries = [], [], [], []
    for package in sorted(set(reports) - set(expected)):
        problems.append('TS: unexpected canonical report package ' + package)
    for package in expected:
        occurrences = reports.get(package, [])
        distinct = {row['report_sha256'] for row in occurrences}
        if not occurrences:
            problems.append('TS: missing actual complete reporter JSON for ' + package)
            continue
        if len(distinct) != 1:
            problems.append('TS: ambiguous multiple distinct reports for ' + package)
            continue
        captured = occurrences[0]
        report = captured['report']
        counts = Counter()
        package_leaves = []
        seen_files = set()
        for file_index, file in enumerate(report['testResults']):
            source = Path(file['name'])
            source_sha256 = None
            if candidate is not None:
                if not source.is_file() or not source.resolve().is_relative_to(candidate.resolve()):
                    problems.append('TS: tested file is outside the frozen candidate ' + package + ':' + file['name'])
                else:
                    source_sha256 = digest(source)
            if file['name'] in seen_files:
                problems.append('TS: repeated file record ' + package + ':' + file['name'])
            seen_files.add(file['name'])
            files.append({'package': package, 'file': file['name'], 'source_sha256': source_sha256, 'file_index': file_index, 'status': file.get('status'),
                          'assertion_count': len(file['assertionResults']), 'message': file.get('message', '')})
            for index, assertion in enumerate(file['assertionResults']):
                if not isinstance(assertion, dict):
                    problems.append('TS: invalid assertion record ' + package + ':' + file['name'])
                    continue
                status = assertion.get('status')
                if status not in ('passed', 'failed', 'skipped', 'pending', 'todo'):
                    problems.append('TS: unknown assertion status ' + str(status))
                if not isinstance(assertion.get('fullName'), str) or not assertion['fullName'].strip():
                    problems.append('TS: missing actual qualified assertion name ' + package + ':' + file['name'])
                counts[str(status)] += 1
                row = {'package': package, 'file': file['name'], 'file_index': file_index, 'assertion_index': index,
                       **assertion, 'raw_log_line': captured['raw_line'], 'raw_report_sha256': captured['report_sha256']}
                package_leaves.append(row)
                if status in ('skipped', 'pending', 'todo'):
                    skipped.append({**row, 'counted_as_pass': False,
                                    'disposition': 'pending is incomplete; skipped/todo retain their actual source condition and require separate review'})
        metric_checks = {
            'numTotalTests': len(package_leaves), 'numPassedTests': counts['passed'], 'numFailedTests': counts['failed'],
            'numPendingTests': counts['skipped'] + counts['pending'], 'numTodoTests': counts['todo']
        }
        for key, actual in metric_checks.items():
            if report[key] != actual:
                problems.append('TS: ' + package + ' ' + key + ' disagrees with actual assertionResults')
        suite_counts = {key: report.get(key) for key in ('numTotalTestSuites', 'numPassedTestSuites', 'numFailedTestSuites', 'numPendingTestSuites')}
        if all(type(value) is int for value in suite_counts.values()):
            if suite_counts['numTotalTestSuites'] != sum(suite_counts[key] for key in ('numPassedTestSuites', 'numFailedTestSuites', 'numPendingTestSuites')):
                problems.append('TS: inconsistent nested suite aggregates for ' + package)
            if suite_counts['numPendingTestSuites']:
                problems.append('TS: unsettled nested suites remain in ' + package)
        else:
            problems.append('TS: missing actual nested suite aggregates for ' + package)
        if counts['pending']:
            problems.append('TS: running/queued assertions remain in ' + package)
        extracted = output / (package.split('/')[-1] + '-actual-vitest-report.json')
        with extracted.open('x') as target:
            target.write(captured['raw_json'] + '\n')
        summaries.append({'package': package, 'actual_report': reference(extracted), 'raw_line': captured['raw_line'],
                          'same_report_log_occurrences': len(occurrences), 'reported_metrics': {k: report[k] for k in metric_checks},
                          'actual_assertion_statuses': dict(counts), 'actual_file_count': len(report['testResults']),
                          'reported_nested_suite_counts': suite_counts, 'success': report['success'],
                          'suite_count_note': 'Vitest 4.1 suites includes nested describe suites; file count is testResults.length.',
                          'retry_scope': 'JSON reporter omits retry counters; no fabricated TS zero-retry claim.'})
        leaves.extend(package_leaves)
    return {'packages': summaries, 'leaf_inventory': write_rows(output / 'ts-leaves.jsonl', leaves),
            'file_inventory': write_rows(output / 'ts-files.jsonl', files), 'skip_inventory': write_rows(output / 'ts-skips.jsonl', skipped),
            'counts': dict(Counter(row.get('status') for row in leaves)), 'file_count': len(files), 'leaf_count': len(leaves)}

def skip_source(candidate, package, output_lines):
    sources = []
    directory = candidate / package.removeprefix(MODULE_PREFIX)
    for text in output_lines:
        match = re.search(r'(\S+\.go):(\d+):\s*(.*)', text)
        if not match:
            continue
        path = directory / match[1]
        if not path.is_file():
            continue
        number = int(match[2])
        lines = path.read_text().splitlines()
        sources.append({**reference(path), 'line': number, 'reason': match[3],
                        'guard_excerpt': lines[max(0, number - 7):number + 2]})
    return sources

def collect_go(directory, candidate, output, expected_count, expected_packages, problems):
    packages, leaves, skips = [], [], []
    all_counts = Counter()
    seen_packages, seen_databases = set(), set()
    for path in sorted(directory.glob('*.json')):
        metadata = load(path)
        if not isinstance(metadata, dict) or not isinstance(metadata.get('package'), str) or 'command' not in metadata:
            problems.append('Go: unexpected package metadata ' + str(path))
            continue
        package = metadata['package']
        if package in seen_packages:
            problems.append('Go: multiple package executions for ' + package)
        seen_packages.add(package)
        database = metadata.get('database')
        if not database or database in seen_databases:
            problems.append('Go: missing/shared per-package database ' + package)
        seen_databases.add(database)
        raw = path.with_suffix('.jsonl')
        if not raw.is_file():
            problems.append('Go: missing raw output ' + package)
            continue
        raw_sha256 = digest(raw)
        if raw_sha256 != metadata.get('log_sha256'):
            problems.append('Go: raw hash disagrees with metadata ' + package)
        counts, terminal, runs, outputs = Counter(), {}, set(), defaultdict(list)
        package_actions, no_test_files = [], False
        for number, line in enumerate(raw.read_text().splitlines(), 1):
            try:
                event = json.loads(line)
            except ValueError:
                continue
            if not isinstance(event, dict):
                continue
            if event.get('Package') and event['Package'] != package:
                problems.append('Go: raw package identity mismatch ' + package)
            action, test = event.get('Action'), event.get('Test')
            if test and action in ('run', 'pass', 'fail', 'skip'):
                counts[action] += 1
                if action == 'run':
                    if test in runs:
                        problems.append('Go: repeated named test run ' + package + ':' + test)
                    runs.add(test)
                else:
                    if test in terminal:
                        problems.append('Go: repeated terminal ' + package + ':' + test)
                    terminal[test] = {'package': package, 'test': test, 'action': action, 'time': event.get('Time'),
                                      'elapsed': event.get('Elapsed'), 'raw': str(raw), 'raw_sha256': raw_sha256, 'raw_line': number}
            if action == 'output':
                text = event.get('Output', '')
                if test:
                    outputs[test].extend(text.splitlines())
                no_test_files = no_test_files or '[no test files]' in text
            if not test and action in ('pass', 'fail', 'skip'):
                package_actions.append(action)
        if dict(counts) != {key: value for key, value in metadata.get('counts', {}).items() if value}:
            problems.append('Go: named counts disagree with metadata ' + package)
        if runs != set(terminal):
            problems.append('Go: named runs and terminals are incomplete ' + package)
        if len(package_actions) != 1:
            problems.append('Go: package terminal is incomplete/ambiguous ' + package)
        if package_actions == ['pass'] and not runs:
            problems.append('Go: successful package has no observed named test execution ' + package)
        compiled = package_actions == ['skip'] and no_test_files and not terminal
        if metadata.get('package_passed') is not (package_actions == ['pass']):
            problems.append('Go: package completion disagrees with metadata ' + package)
        if compiled != metadata.get('compiled_without_tests'):
            problems.append('Go: no-test-files compilation disagrees with metadata ' + package)
        if metadata.get('state') != 'complete' or type(metadata.get('exit_code')) is not int:
            problems.append('Go: package execution did not finish ' + package)
        packages.append({'package': package, 'database': database, 'metadata': reference(path), 'raw': reference(raw),
                         'counts': dict(counts), 'exit_code': metadata.get('exit_code'), 'terminal_actions': package_actions,
                         'compiled_without_tests': compiled, 'actual_command': metadata['command'], 'race': '-race' in metadata['command']})
        all_counts.update(counts)
        for test, row in terminal.items():
            leaves.append(row)
            if row['action'] == 'skip':
                reasons = outputs[test]
                skips.append({**row, 'actual_test_output': reasons, 'sources': skip_source(candidate, package, reasons),
                              'counted_as_pass': False, 'disposition': 'Compare this exact source guard/identity against the independent final skip disposition.'})
    if len(packages) != expected_count:
        problems.append('Go: expected ' + str(expected_count) + ' package executions, observed ' + str(len(packages)))
    if expected_packages is not None and seen_packages != set(expected_packages):
        problems.append('Go: package identity set differs from frozen expected inventory')
    return {'packages': packages, 'package_count': len(packages), 'unique_database_count': len(seen_databases),
            'counts': dict(all_counts), 'leaf_count': len(leaves), 'package_inventory': write_rows(output / 'go-packages.jsonl', packages),
            'leaf_inventory': write_rows(output / 'go-leaves.jsonl', leaves), 'skip_inventory': write_rows(output / 'go-skips.jsonl', skips),
            'compiled_no_test_file_packages': [row['package'] for row in packages if row['compiled_without_tests']]}

def collect_browser(report_path, candidate, output, expected_count, problems):
    report = load(report_path)
    rows, skips = [], []
    def visit(suites):
        for suite in suites:
            visit(suite.get('suites', []))
            for spec in suite.get('specs', []):
                for test in spec.get('tests', []):
                    attempts = test.get('results', [])
                    row = {'spec_id': spec.get('id'), 'file': spec.get('file'), 'line': spec.get('line'), 'title': spec.get('title'),
                           'project': test.get('projectName'), 'repeatEachIndex': test.get('repeatEachIndex'),
                           'expected_status': test.get('expectedStatus'), 'report_status': test.get('status'),
                           'terminal': attempts[-1].get('status') if attempts else 'unrun',
                           'attempt_statuses': [a.get('status') for a in attempts],
                           'retry_count': sum(a.get('retry', 0) for a in attempts), 'annotations': test.get('annotations', [])}
                    rows.append(row)
                    if row['terminal'] == 'skipped':
                        if row['expected_status'] != 'skipped' or not any(a.get('type') == 'skip' for a in row['annotations']):
                            problems.append('Browser: undeclared/unexpected skipped case ' + str(row['spec_id']))
                        guards = []
                        for annotation in row['annotations']:
                            if annotation.get('type') != 'skip':
                                continue
                            location = annotation.get('location', {})
                            source = Path(location.get('file', ''))
                            number = location.get('line')
                            guard = {'reason': annotation.get('description'), 'source': str(source), 'line': number}
                            if source.is_file() and type(number) is int:
                                lines = source.read_text().splitlines()
                                guard.update(source_sha256=digest(source), guard_excerpt=lines[max(0, number - 5):number + 3])
                            guards.append(guard)
                        skips.append({**row, 'guards': guards, 'counted_as_pass': False,
                                      'disposition': 'Required CCG2 OFF/ON follow-up.' if row['file'] == 'collaboration-knowledge-deployed.spec.ts' else 'Keep this exact dedicated fixture/platform lane independent; require final source-based disposition.'})
    visit(report.get('suites', []))
    ids = [(r['spec_id'], r['project'], r['repeatEachIndex']) for r in rows]
    if len(ids) != len(set(ids)):
        problems.append('Browser: duplicate canonical case identity')
    if len(rows) != expected_count:
        problems.append('Browser: expected ' + str(expected_count) + ' cases, observed ' + str(len(rows)))
    if any(row['terminal'] == 'unrun' for row in rows):
        problems.append('Browser: unrun cases remain')
    retry = sum(row['retry_count'] for row in rows)
    if retry:
        problems.append('Browser: unexpected retries')
    stats = report.get('stats', {})
    report_counts = Counter(row['report_status'] for row in rows)
    metrics = {'expected': report_counts['expected'], 'unexpected': report_counts['unexpected'], 'flaky': report_counts['flaky'], 'skipped': report_counts['skipped']}
    for key, actual in metrics.items():
        if stats.get(key) != actual:
            problems.append('Browser: ' + key + ' summary disagrees with named report statuses')
    if report.get('errors'):
        problems.append('Browser: out-of-band reporter errors present')
    return {'report': reference(report_path), 'case_count': len(rows), 'terminal_counts': dict(Counter(r['terminal'] for r in rows)),
            'report_status_counts': dict(report_counts), 'stats': stats, 'retry_count': retry, 'report_errors': report.get('errors', []),
            'case_inventory': write_rows(output / 'browser-cases.jsonl', rows), 'skip_inventory': write_rows(output / 'browser-skips.jsonl', skips)}

def read_rows(spec):
    return [json.loads(line) for line in Path(spec['path']).read_text().splitlines()]

def review_skips(spec, ts, go, browser, output, problems):
    """Reuse only exact names/reasons on still-identical reviewed guard bytes."""
    path = pinned(spec, problems, 'Independent explicit-skip dispositions')
    previous = load(path) if path else {}
    known_go = {(r['package'], r['test']): r for r in previous.get('go_skip_review', {}).get('rows', [])}
    known_browser = {(r['spec_id'], r['project'], r.get('repeatEachIndex')): r for r in previous.get('browser_skip_review', {}).get('rows', [])}
    known_ts = {(r['package'], r['file'], r['fullName']): r for r in previous.get('ts_skip_review', {}).get('rows', [])}
    rows, unresolved = [], []
    def guards_match(sources):
        return bool(sources) and all(Path(r['path']).is_file() and digest(r['path']) == r['sha256'] for r in sources)
    for row in read_rows(go['skip_inventory']):
        known = known_go.get((row['package'], row['test']))
        valid = bool(known and guards_match(known.get('source', []))
                     and all(reason in '\n'.join(row['actual_test_output']) for reason in known.get('reason', [])))
        result = {'kind': 'Go', 'package': row['package'], 'test': row['test'], 'known_disposition_matches': valid,
                  'disposition': known.get('effect_on_task7_and_substitute') if valid else 'Needs independent source/reason review.', 'counted_as_pass': False}
        rows.append(result)
        if not valid:
            unresolved.append(result)
    for row in read_rows(browser['skip_inventory']) if browser else []:
        key = (row['spec_id'], row['project'], row.get('repeatEachIndex'))
        known = known_browser.get(key)
        sources = [{'path': r['source'], 'sha256': r['source_sha256']} for r in known.get('guards', [])] if known else []
        if known:
            sources.extend(source for guard in known.get('guards', []) for source in guard.get('shared_reason_sources', []))
        actual_reasons = [r['reason'] for r in row['guards']]
        known_reasons = [r['reason'] for r in known.get('guards', [])] if known else []
        valid = bool(known and guards_match(sources) and actual_reasons == known_reasons)
        result = {'kind': 'Browser', 'spec_id': row['spec_id'], 'file': row['file'], 'title': row['title'], 'project': row['project'],
                  'known_disposition_matches': valid, 'disposition': known.get('task7_handoff') if valid else 'Needs independent source/reason review.', 'counted_as_pass': False}
        rows.append(result)
        if not valid:
            unresolved.append(result)
    for row in read_rows(ts['skip_inventory']) if ts else []:
        known = known_ts.get((row['package'], row['file'], row['fullName']))
        valid = bool(row['status'] in ('skipped', 'todo') and known and guards_match(known.get('sources', [])))
        result = {'kind': 'TS', 'package': row['package'], 'file': row['file'], 'fullName': row['fullName'], 'status': row['status'],
                  'known_disposition_matches': valid, 'disposition': known.get('reason') if valid else 'Needs independent exact skip/todo declaration review.', 'counted_as_pass': False}
        rows.append(result)
        if not valid:
            unresolved.append(result)
    return {'previous_independent_review': reference(path) if path else None, 'count': len(rows),
            'all_explicit_dispositions_verified': not unresolved, 'unresolved': unresolved,
            'inventory': write_rows(output / 'explicit-skip-dispositions.jsonl', rows),
            'note': 'Source-reviewed explicit skip/todo is a complete independent terminal, never a pass or an automatic full-run failure.'}

def selected(value, keys):
    for key in keys:
        if not isinstance(value, dict) or key not in value:
            return None
        value = value[key]
    return value

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('inputs', type=Path)
    parser.add_argument('--label', required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'[a-z0-9][a-z0-9._-]*', args.label):
        raise ValueError('Output label must be a fresh research basename')
    inputs = load(args.inputs)
    output = HERE / args.label
    output.mkdir(mode=0o700)
    problems = []
    result_path = pinned(inputs.get('full_result'), problems, 'Complete result')
    raw_log = pinned(inputs.get('raw_log'), problems, 'Actual raw log')
    browser_path = pinned(inputs.get('browser_report'), problems, 'Actual browser report')
    result = load(result_path) if result_path else {}
    if result.get('candidate_sha') != inputs.get('candidate_sha') or result.get('candidate_root') != inputs.get('candidate_root'):
        problems.append('Complete result candidate identity differs from frozen input')
    if result.get('state') != 'complete' or type(result.get('exit_code')) is not int:
        problems.append('Complete result has not actually finished')
    if raw_log and result.get('log_sha256') != digest(raw_log):
        problems.append('Raw whole-check log differs from final result binding')
    declared_packages = inputs.get('expected_ts_packages')
    if declared_packages is not None and (not isinstance(declared_packages, list) or len(declared_packages) != len(PACKAGES) or set(declared_packages) != set(PACKAGES)):
        problems.append('TS: declared package set differs from the exact frozen five test scripts')
    ts = collect_ts(raw_log, output, PACKAGES, problems, Path(inputs['candidate_root'])) if raw_log else None
    directory = Path(inputs['go_directory'])
    candidate = Path(inputs['candidate_root'])
    go = collect_go(directory, candidate, output, inputs.get('expected_go_package_count', 82), inputs.get('expected_go_packages'), problems)
    browser = collect_browser(browser_path, candidate, output, inputs.get('expected_browser_case_count', 441), problems) if browser_path else None
    structural_complete = not problems
    skip_review = review_skips(inputs.get('skip_disposition_review'), ts, go, browser, output, problems)
    audit_path = pinned(inputs.get('final_audit'), problems, 'Final identity audit')
    audit = selected(load(audit_path), inputs.get('audit_selector', [])) if audit_path else None
    audit_pass = isinstance(audit, dict) and audit.get('status') == 'pass' and audit.get('wholecheck_passed') is True
    live_path = pinned(inputs.get('live_proof'), problems, 'Root live proof')
    live = selected(load(live_path), inputs.get('live_selector', [])) if live_path else None
    live_pass = isinstance(live, dict) and live.get('status') == 'pass' and live.get('candidate_sha') == inputs.get('candidate_sha')
    clean_results = bool(ts and go and browser and not ts['counts'].get('failed') and not go['counts'].get('fail')
                         and all(p['success'] is True for p in ts['packages'])
                         and all(p['exit_code'] == 0 and (p['terminal_actions'] == ['pass'] or p['compiled_without_tests']) for p in go['packages'])
                         and not browser['report_status_counts'].get('unexpected') and not browser['terminal_counts'].get('timedOut')
                         and not browser['terminal_counts'].get('interrupted') and not browser['report_status_counts'].get('flaky'))
    summary = {'schema_version': 1, 'collected_at': datetime.now(timezone.utc).isoformat(), 'collector': reference(Path(__file__)),
               'inputs': reference(args.inputs), 'candidate_sha': inputs.get('candidate_sha'), 'candidate_root': inputs.get('candidate_root'),
               'full_result': reference(result_path) if result_path else None, 'actual_make_exit': result.get('exit_code'),
               'raw_log': reference(raw_log) if raw_log else None, 'TS': ts, 'Go': go, 'Browser': browser, 'explicit_skip_dispositions': skip_review,
               'complete_named_inventory': structural_complete, 'all_collected_test_results_clean': clean_results,
               'final_identity_audit': reference(audit_path) if audit_path else None, 'final_identity_audit_pass': audit_pass,
               'root_live_proof': reference(live_path) if live_path else None, 'root_live_proof_pass': live_pass,
               'shared_gate_ready': structural_complete and clean_results and skip_review['all_explicit_dispositions_verified'] and result.get('exit_code') == 0 and audit_pass and live_pass,
               'task7_closed': False, 'CCG1_CCG2_and_independent_final_review_required': True,
               'problems': problems, 'scope': 'Only actual pinned raw inputs parsed; no test, DB, service, browser or Provider was started.',
               'knowledge_budget': {'allocated': 0, 'real_calls': 0, 'production_default_enabled': False}}
    write_json(output / 'summary.json', summary)
    print(json.dumps({'summary': str(output / 'summary.json'), 'inventory_complete': structural_complete,
                      'shared_gate_ready': summary['shared_gate_ready'], 'problems': problems}, ensure_ascii=False))
    return 0 if structural_complete else 1

if __name__ == '__main__':
    raise SystemExit(main())
