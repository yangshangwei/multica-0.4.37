#!/usr/bin/env python3
"""Summarize the preserved frozen-candidate commands and hash their evidence."""
import collections
import datetime
import hashlib
import json
import pathlib
import sys

OUT = pathlib.Path(__file__).resolve().parent


def load(name):
    return json.loads((OUT / name).read_text())


def save(name, value):
    (OUT / name).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')


def main():
    gate = load('gate-summary.json')
    environment = load('environment.json')
    before, after = load('source-before.json'), load('source-after.json')
    gofmt = load('gofmt-summary.json')
    inventory = load('test-inventory.json')
    summaries = {}
    for path in OUT.glob('*-summary.json'):
        if path.name not in ('gate-summary.json', 'gofmt-summary.json'):
            summary = json.loads(path.read_text())
            summaries[summary['command']] = (path.name.removesuffix('-summary.json'), summary)
    totals = {'top_level': collections.Counter(), 'subcase': collections.Counter()}
    failures, skips = [], []
    for suite in gate['suites']:
        for kind, counts in suite.get('counts', {}).items():
            totals[kind].update(counts)
        failures.extend(suite.get('failures', []))
        skips.extend(suite.get('skips', []))
    gate.update(totals={k: dict(v) for k,v in totals.items()},
                source_scope_parity=before['scope_parity'] and after['scope_parity'],
                server_file_count=after['server_file_count'],
                evidence_finalized_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                gofmt={k:gofmt[k] for k in ('exit_code','checked_go_file_count','unformatted_files','successful_gate')})
    gate['all_commands_passed'] = all(s['successful_gate'] for s in gate['suites']) and gate['source_unchanged'] and gate['source_scope_parity'] and gofmt['successful_gate']
    save('gate-summary.json', gate)
    save('all-skips.json', skips)
    save('all-failures.json', failures)
    previous_failures = ['TestCollaborationRunRecordsCurrentAuthorityAndLegacyAbsence',
        'TestCollaborationRuntimePreparationDerivedLateDeliveryDoesNotStarveHealthyRoot',
        'TestCollaborationRuntimePreparationJoinBeforeFirstClaimReleasesOnlyUnclaimedWork']
    service = load('service-summary.json')
    repaired = {name: next((state for state,names in service['outcomes'].items() if name in names), 'not_run') for name in previous_failures}
    save('previous-failure-coverage.json', repaired)
    rows = ['# Corrected full backend gate', '',
        'Result: **' + ('pass' if gate['all_commands_passed'] else 'failed') + '**.',
        'Candidate: `' + gate['candidate_source_tree_sha256'] + '`; HEAD `' + after['actual_head'] + '` plus the recorded source diff.',
        'Database: `' + gate['database'] + '`. Service and handler ran sequentially on this fresh dedicated database.',
        'Environment: `' + environment['go_version'] + '`; PostgreSQL 17.11. All test commands used the repository ambient-agent CLI guard, `-race -count=1 -p 1 -parallel 2 -timeout 40m`, and `GOMAXPROCS=2`. Real-agent flags were absent.', '',
        '| Suite | Exit | Top-level pass / fail / skip | Subcase pass / fail / skip | Missing named tests |',
        '| --- | --- | --- | --- | --- |']
    for suite in gate['suites']:
        name, _ = summaries[suite['command']]
        def counts(kind):
            value = suite.get('counts',{}).get(kind)
            return '—' if value is None else ' / '.join(str(value.get(s,0)) for s in ('pass','fail','skip'))
        rows.append('| ' + name + ' | ' + str(suite['exit_code']) + ' | ' + counts('top_level') + ' | ' + counts('subcase') + ' | ' + str(len(suite.get('missing_expected_tests',[]))) + ' |')
    rows += ['', 'Formatting: ' + ('pass' if gofmt['successful_gate'] else 'fail') + ' across ' + str(gofmt['checked_go_file_count']) + ' changed Go files.',
        'All ' + str(after['server_file_count']) + ' server file hashes unchanged: `' + str(gate['source_unchanged']).lower() + '`. Tracked/untracked server file-set parity: `' + str(gate['source_scope_parity']).lower() + '`.', '',
        '## Previously failing tests', '']
    rows.extend('- `' + name + '`: **' + status + '**.' for name,status in repaired.items())
    rows += ['', '## Skips', '']
    rows.extend('- `' + item['test'] + '`: ' + item['output'].replace('\n', ' ').strip() for item in skips)
    if not skips:
        rows.append('None.')
    if failures:
        rows += ['', '## Failures', '']
        rows.extend('- `' + item['test'] + '`: see `all-failures.json` for the complete assertion output.' for item in failures)
    rows += ['', 'Exact commands, exits, timings, environment metadata, expected test names, raw JSONL/logs, and all failure/skip output are preserved here. `source-before.json` and `source-after.json` bind the run to `final-candidate/source-round3.json`; `evidence-sha256.json` hashes the evidence files.',
        'The first run remains untouched under `../final-backend/`. No product source changed during this run.', '',
        'Frontend checks and production Web/native Desktop validation are owned by the parent. No real-provider smoke or model behavior trial ran here. This macOS Go 1.27.0 run does not establish the separate CI Go 1.26 or other-native-platform result.', '']
    (OUT/'README.md').write_text('\n'.join(rows))
    manifest = {str(p.relative_to(OUT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(OUT.rglob('*')) if p.is_file() and p.name != 'evidence-sha256.json'}
    save('evidence-sha256.json', {'captured_at':gate['evidence_finalized_at'],'files':manifest})
    print(json.dumps({'all_commands_passed':gate['all_commands_passed'],'totals':gate['totals'],'source_unchanged':gate['source_unchanged'],'scope_parity':gate['source_scope_parity'],'previous_failures':repaired},indent=2))
    return 0 if gate['all_commands_passed'] else 1


if __name__ == '__main__':
    sys.exit(main())
