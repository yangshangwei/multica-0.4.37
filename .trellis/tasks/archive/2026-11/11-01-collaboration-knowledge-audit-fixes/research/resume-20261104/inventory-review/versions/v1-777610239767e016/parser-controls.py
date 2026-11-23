"""Finite synthetic format controls; these are never product acceptance."""
from copy import deepcopy
import argparse
import hashlib
import json
from pathlib import Path
import runpy
import re

HERE = Path(__file__).resolve().parent
module = runpy.run_path(str(HERE / 'collect.py'))
parser = argparse.ArgumentParser()
parser.add_argument('--label', default='synthetic-parser-controls')
args = parser.parse_args()
assert re.fullmatch(r'[a-z0-9][a-z0-9._-]*', args.label)
out = HERE / args.label
out.mkdir(mode=0o700)
packages = module['PACKAGES']

def report(package):
    assertions = [{'ancestorTitles': ['synthetic format'], 'title': status, 'fullName': 'synthetic format ' + status,
                   'status': status, 'failureMessages': []} for status in ['passed', 'skipped', 'todo']]
    return {'numTotalTestSuites': 2, 'numPassedTestSuites': 2, 'numFailedTestSuites': 0, 'numPendingTestSuites': 0,
            'numTotalTests': 3, 'numPassedTests': 1, 'numFailedTests': 0, 'numPendingTests': 1, 'numTodoTests': 1,
            'success': True, 'testResults': [{'name': '/synthetic/' + package.split('/')[-1] + '.test.ts',
                                             'status': 'passed', 'assertionResults': assertions}]}

def scan(name, reports, repeats=1, marker=False, truncate=False):
    directory = out / name
    directory.mkdir(mode=0o700)
    raw = directory / 'synthetic.log'
    lines = []
    for package, value in reports.items():
        payload = json.dumps({'command': ['turbo', 'test', '--reporter=json']}) if marker else json.dumps(value)
        if truncate:
            payload = payload[:-6]
        lines.extend(['\x1b[32m' + package + ':test:\x1b[0m ' + payload] * repeats)
    raw.write_text('\n'.join(lines) + '\n')
    problems = []
    result = module['collect_ts'](raw, directory, packages, problems)
    return result, problems

controls = []
valid = {package: report(package) for package in packages}
result, problems = scan('five-complete-json-reports', valid)
assert not problems and result['leaf_count'] == 15 and result['file_count'] == 5
assert result['counts'] == {'passed': 5, 'skipped': 5, 'todo': 5}
controls.append({'name': 'five full JSON reports preserve named skips/todos and distinguish files from nested suites', 'pass': True})
result, problems = scan('command-markers', valid, marker=True)
assert result['leaf_count'] == 0 and len(problems) == 5
controls.append({'name': 'canonical command transformation markers never become leaf reports', 'pass': True})
result, problems = scan('truncated-reports', valid, truncate=True)
assert result['leaf_count'] == 0 and len(problems) == 5
controls.append({'name': 'truncated JSON cannot become a complete report', 'pass': True})
result, problems = scan('identical-log-repeat', valid, repeats=2)
assert not problems and result['leaf_count'] == 15 and all(r['same_report_log_occurrences'] == 2 for r in result['packages'])
controls.append({'name': 'identical log repeats retain occurrences without double-counting one logical report', 'pass': True})
bad = deepcopy(valid)
bad[packages[0]]['numPassedTests'] = 9
result, problems = scan('aggregate-mismatch', bad)
assert any('numPassedTests disagrees' in p for p in problems)
controls.append({'name': 'aggregate counts must equal actual assertionResults', 'pass': True})
pending = deepcopy(valid)
pending[packages[0]]['testResults'][0]['assertionResults'][0]['status'] = 'pending'
pending[packages[0]]['numPassedTests'] = 0
pending[packages[0]]['numPendingTests'] = 2
result, problems = scan('unsettled-leaf', pending)
assert any('running/queued assertions remain' in p for p in problems)
controls.append({'name': 'running/queued leaves remain incomplete even with matching aggregate counts', 'pass': True})
result, problems = scan('missing-one-package', {k: v for k, v in valid.items() if k != packages[-1]})
assert len(problems) == 1 and packages[-1] in problems[0]
controls.append({'name': 'missing one of five package reporters blocks complete inventory', 'pass': True})
directory = out / 'distinct-report-ambiguity'
directory.mkdir(mode=0o700)
raw = directory / 'synthetic.log'
other = deepcopy(valid[packages[0]])
other['startTime'] = 2
raw.write_text('\n'.join(package + ':test: ' + json.dumps(value) for package, value in valid.items())
               + '\n' + packages[0] + ':test: ' + json.dumps(other) + '\n')
problems = []
result = module['collect_ts'](raw, directory, packages, problems)
assert result['leaf_count'] == 12 and any('ambiguous multiple distinct reports' in p for p in problems)
controls.append({'name': 'distinct same-package reports cannot be selected or merged', 'pass': True})
summary = {'scope': 'Synthetic parser-only controls. No product tests, packages, services, databases or Provider invoked. No result here is real acceptance.',
           'collector_sha256': hashlib.sha256((HERE / 'collect.py').read_bytes()).hexdigest(), 'controls': controls,
           'count': len(controls), 'all_passed': True}
with (out / 'result.json').open('x') as target:
    json.dump(summary, target, indent=2)
    target.write('\n')
print(json.dumps(summary))
