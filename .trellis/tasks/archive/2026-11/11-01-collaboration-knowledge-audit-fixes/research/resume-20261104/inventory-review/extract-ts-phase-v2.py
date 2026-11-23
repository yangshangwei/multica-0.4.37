"""Read stable real v15 TS spans; never grant whole-check acceptance."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import runpy

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
RAW = ROOT / '.omx/private/final-check-v15/make-check-v15.log'
OBSERVATION = HERE.parent / 'v15-execution/TS-phase-observation.json'
RUN_RESULT = ROOT / '.omx/private/final-check-v15/make-check-v15.json'
parser = argparse.ArgumentParser()
parser.add_argument('--label', required=True)
args = parser.parse_args()
assert re.fullmatch(r'[a-z0-9][a-z0-9._-]*', args.label)
out = HERE / args.label
out.mkdir(mode=0o700)
module = runpy.run_path(str(HERE / 'collect.py'))
source_bytes = RAW.read_bytes()
source_lines = source_bytes.splitlines(keepends=True)
line_offsets = []
offset = 0
for line in source_lines:
    line_offsets.append(offset)
    offset += len(line)

class FrozenSnapshot:
    def read_text(self):
        return source_bytes.decode()

reports = module['vitest_reports'](FrozenSnapshot())
assert set(reports) == set(module['PACKAGES'])
observed_bytes = OBSERVATION.read_bytes()
observed = json.loads(observed_bytes)
assert observed['report_count'] == 5 and observed['wholecheck_accepted'] is False
known = {row['package']: row for row in observed['reports']}
assert set(known) == set(module['PACKAGES'])
spans, lines_to_copy = [], []
for package in module['PACKAGES']:
    assert len(reports[package]) == 1
    row = reports[package][0]
    number = row['raw_line']
    raw_line = source_lines[number - 1]
    clean = module['ANSI'].sub('', raw_line.decode().rstrip('\r\n'))
    match = module['PREFIX'].match(clean)
    assert match and match[1] == package and json.loads(match[2]) == row['report']
    assert row['report_sha256'] == known[package]['raw_object_sha256']
    result = row['report']
    assert len(result['testResults']) == known[package]['files']
    for key in ('numTotalTests', 'numPassedTests', 'numFailedTests', 'numPendingTests', 'numTodoTests', 'success'):
        assert result[key] == known[package][key]
    spans.append({'package': package, 'original_line': number, 'byte_offset': line_offsets[number - 1],
                  'byte_length': len(raw_line), 'raw_span_sha256': hashlib.sha256(raw_line).hexdigest(),
                  'raw_object_sha256': row['report_sha256'], 'phase_observation_hash_matches': True})
    lines_to_copy.append((number, raw_line))
span_path = out / 'actual-ts-reporter-spans.log'
with span_path.open('xb') as target:
    target.write(b''.join(line for _, line in sorted(lines_to_copy)))
problems = []
ts = module['collect_ts'](span_path, out, module['PACKAGES'], problems, ROOT)
assert not problems
assert ts['leaf_count'] == 13244 and ts['file_count'] == 1069 and ts['counts'] == {'passed': 13244}
with RAW.open('rb') as source:
    for span in spans:
        source.seek(span['byte_offset'])
        actual = source.read(span['byte_length'])
        assert hashlib.sha256(actual).hexdigest() == span['raw_span_sha256']
run_bytes = RUN_RESULT.read_bytes()
run = json.loads(run_bytes)
assert run['candidate_sha'] == 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
with (out / 'run-result-at-ts-witness.json').open('xb') as target:
    target.write(run_bytes)
record = {'schema_version': 1, 'recorded_at': datetime.now(timezone.utc).isoformat(),
          'scope': 'Independent actual five TS reporter spans from the same sole v15; Go/browser/final-audit acceptance remains separate.',
          'candidate_sha': run['candidate_sha'], 'collector': module['reference'](HERE / 'collect.py'),
          'package_contract': module['reference'](HERE / 'package-contract-repair-v2.json'),
          'source_raw_path': str(RAW), 'visible_source_prefix_bytes': len(source_bytes),
          'visible_source_prefix_sha256': hashlib.sha256(source_bytes).hexdigest(),
          'selected_raw_spans': spans, 'selected_spans_still_byte_identical_after_collection': True,
          'phase_observation': {'path': str(OBSERVATION), 'sha256': hashlib.sha256(observed_bytes).hexdigest()},
          'copied_actual_raw_spans': module['reference'](span_path), 'TS': ts,
          'TS_named_inventory_complete': True, 'actual_run_state_at_witness': run['state'],
          'actual_make_exit_at_witness': run.get('exit_code'), 'wholecheck_accepted': False,
          'shared_gate_ready': False, 'task7_closed': False,
          'product_tests_or_services_or_databases_started_by_collector': 0,
          'knowledge_budget_allocated': 0, 'knowledge_real_calls': 0}
module['write_json'](out / 'summary.json', record)
print(json.dumps({'summary': str(out / 'summary.json'), 'collector_sha256': record['collector']['sha256'],
                  'packages': [row['package'] for row in ts['packages']], 'TS_files': ts['file_count'],
                  'actual_named_pass': ts['leaf_count'], 'counts': ts['counts'], 'wholecheck_accepted': False}))
