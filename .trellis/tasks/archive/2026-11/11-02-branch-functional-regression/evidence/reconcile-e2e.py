import csv
from collections import Counter
import json
from pathlib import Path

task = Path(__file__).resolve().parent.parent
coverage = task / 'coverage.csv'
with coverage.open(newline='') as stream:
    reader = csv.DictReader(stream)
    fields = reader.fieldnames
    rows = list(reader)
for row in rows:
    row.update(status='NOT_RUN', run_id='', evidence='', note='Collected only; no matching completed run')


def specs(suite):
    yield from suite.get('specs', [])
    for child in suite.get('suites', []):
        yield from specs(child)


executions = []
unmatched = []
paths = []
for result_path in (task / 'evidence').glob('*/results.json'):
    metadata = result_path.parent / 'run.json'
    if metadata.exists():
        run = json.loads(metadata.read_text())
        if run.get('state') == 'complete':
            paths.append((run.get('started_at', ''), result_path, run))
for _, result_path, run in sorted(paths):
    result = json.loads(result_path.read_text())
    for suite in result.get('suites', []):
        for spec in specs(suite):
            for test in spec.get('tests', []):
                matches = [r for r in rows if Path(r['spec']).name == Path(spec['file']).name
                           and int(r['line']) == spec['line'] and r['project'] == test['projectName']
                           and (r['title'] == spec['title'] or r['title'].endswith(' › ' + spec['title']))]
                changed_file = any(Path(p).name == Path(spec['file']).name for p in run.get('harness_dirty_paths', []))
                if not matches and changed_file:
                    matches = [r for r in rows if Path(r['spec']).name == Path(spec['file']).name
                               and r['project'] == test['projectName']
                               and (r['title'] == spec['title'] or r['title'].endswith(' › ' + spec['title']))]
                if len(matches) != 1:
                    unmatched.append({'run': result_path.parent.name, 'spec': spec['file'], 'line': spec['line'], 'title': spec['title'], 'matches': len(matches)})
                    continue
                row = matches[0]
                results = test.get('results', [])
                raw = results[-1].get('status') if results else 'did_not_run'
                status = 'PASS' if test.get('status') == 'expected' else 'FAIL' if test.get('status') in ['unexpected', 'flaky'] else 'NOT_RUN'
                execution = {'case_id': row['case_id'], 'run_id': result_path.parent.name, 'status': status,
                             'framework_status': test.get('status'), 'raw_status': raw,
                             'evidence': str(result_path.relative_to(task)), 'annotations': test.get('annotations', [])}
                executions.append(execution)
                if status != 'NOT_RUN' or row['status'] == 'NOT_RUN':
                    note = 'Framework: ' + str(raw)
                    if changed_file:
                        note += '; reviewed harness patch ' + run.get('harness_patch_sha256', '')
                    row.update(status=status, run_id=result_path.parent.name, evidence=execution['evidence'], note=note)

with coverage.open('w', newline='') as stream:
    writer = csv.DictWriter(stream, fieldnames=fields)
    writer.writeheader()
    writer.writerows(rows)
summary = {'unique_inventory': len(rows), 'unique_status': dict(Counter(r['status'] for r in rows)),
           'execution_count': len(executions), 'execution_status': dict(Counter(e['status'] for e in executions)),
           'completed_runs': [p.parent.name for _, p, _ in sorted(paths)], 'unmatched': unmatched}
summary['original_full_run_still_failed'] = True
summary['harness_changes_recorded_per_run'] = True
(task / 'evidence/e2e-executions.json').write_text(json.dumps(executions, ensure_ascii=False, indent=2) + '\n')
(task / 'evidence/e2e-summary.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(summary, ensure_ascii=False))
if unmatched:
    raise SystemExit(1)
