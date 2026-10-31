#!/usr/bin/env python3
"""Finalize named-test coverage without rerunning or altering raw Go results."""
import collections
import datetime
import hashlib
import json
import pathlib
import sys

OUT = pathlib.Path(__file__).resolve().parent
RAW = OUT / 'initial-inventory-reports'
RAW.mkdir(exist_ok=True)


def load(name):
    return json.loads((OUT / name).read_text())


def save(name, value):
    (OUT / name).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')


def preserve(name):
    target = RAW / name
    if not target.exists():
        target.write_bytes((OUT / name).read_bytes())


def main():
    gate = load('gate-summary.json')
    inventory = load('test-inventory.json')
    preserve('gate-summary.json')
    preserve('test-inventory.json')
    preserve('progress.json')
    corrected = []
    for suite in gate['suites']:
        if 'expected_top_level_count' not in suite:
            continue
        # Resolve by the exact saved command, avoiding any package-name aliases.
        paths = [p for p in OUT.glob('*-summary.json')
                 if p.name not in ('gate-summary.json', 'gofmt-summary.json')
                 and json.loads(p.read_text()).get('command') == suite['command']]
        if len(paths) != 1:
            raise RuntimeError('expected exactly one original suite summary')
        name = paths[0].name.removesuffix('-summary.json')
        expected = inventory['suites'][name]
        if 'TestMain' in expected:
            preserve(paths[0].name)
            expected.pop('TestMain')
            suite['expected_top_level_count'] = len(expected)
            suite['missing_expected_tests'] = [n for n in suite['missing_expected_tests'] if n != 'TestMain']
            suite['successful_gate'] = suite['exit_code'] == 0 and not suite['missing_expected_tests'] and suite['started_top_level_count'] > 0
            suite['inventory_note'] = 'TestMain is the package entry point, not an emitted named test. See inventory-correction.json; raw Go results are unchanged.'
            save(paths[0].name, suite)
            corrected.append(name)
    save('test-inventory.json', inventory)
    gofmt = load('gofmt-summary.json')
    gate['gofmt'] = {k: gofmt[k] for k in ('exit_code', 'checked_go_file_count', 'unformatted_files', 'successful_gate')}
    gate['all_commands_passed'] = all(s['successful_gate'] for s in gate['suites']) and gate['source_unchanged'] and gofmt['successful_gate']
    gate['evidence_finalized_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    gate['inventory_correction'] = {'suites': corrected, 'record': 'inventory-correction.json',
        'initial_reports_preserved_in': RAW.name, 'raw_go_commands_or_results_changed': False,
        'initial_aggregator_exit': 1 if corrected else 0}
    totals = {'top_level': collections.Counter(), 'subcase': collections.Counter()}
    for suite in gate['suites']:
        for group, counts in suite.get('counts', {}).items():
            totals[group].update(counts)
    gate['totals'] = {k: dict(v) for k,v in totals.items()}
    save('gate-summary.json', gate)
    save('progress.json', {'updated_at': gate['evidence_finalized_at'], 'finished': True, 'all_commands_passed': gate['all_commands_passed']})
    rows = ['# Final backend gate', '',
            'Result: **' + ('pass' if gate['all_commands_passed'] else 'failed') + '**. See `findings.md` for the observed failures and read-only recommendations.',
            'Frozen candidate: `' + gate['candidate_source_tree_sha256'] + '`.',
            'Database: `' + gate['database'] + '`. Service and handler ran sequentially.',
            'Environment: Go 1.27.0, macOS arm64; PostgreSQL 17.11. All Go tests used the ambient-agent CLI guard, `-race -count=1 -p 1 -parallel 2 -timeout 40m` and `GOMAXPROCS=2`. The real-agent gate was absent.',
            '', '| Suite | Exit | Top-level pass / fail / skip | Subcase pass / fail / skip | Missing named tests |',
            '| --- | --- | --- | --- | --- |']
    for suite in gate['suites']:
        command = suite['command']
        label = next((p.name.removesuffix('-summary.json') for p in OUT.glob('*-summary.json')
                      if p.name not in ('gate-summary.json', 'gofmt-summary.json') and json.loads(p.read_text()).get('command') == command), 'unknown')
        def fmt(kind):
            c = suite.get('counts', {}).get(kind)
            return '—' if c is None else ' / '.join(str(c.get(s, 0)) for s in ('pass', 'fail', 'skip'))
        rows.append('| ' + label + ' | ' + str(suite['exit_code']) + ' | ' + fmt('top_level') + ' | ' + fmt('subcase') + ' | ' + str(len(suite.get('missing_expected_tests', []))) + ' |')
    rows += ['', 'Formatting: ' + ('pass' if gofmt['successful_gate'] else 'fail') + ' across ' + str(gofmt['checked_go_file_count']) + ' changed Go files.',
             'Server source hashes unchanged: `' + str(gate['source_unchanged']).lower() + '`; see `source-before.json` and `source-after.json`.',
             '', 'Exact command/environment metadata is in each `*-command.json` and `*-summary.json`. Raw Go JSONL and derived logs are retained. Every skip and failure includes its output in the per-suite JSON files. `test-inventory.json` maps expected test names to source files.',
             '', 'The initial inventory scanner counted `TestMain` in two unfiltered packages. The coverage summary excludes these package entry points; no Go command or raw result changed. Initial reports are retained under `initial-inventory-reports/`; see `inventory-correction.json`. The service failures independently fail the backend gate. An earlier libpq environment preflight failed before test execution and is preserved in `preflight-diagnostic.json`.',
             '', 'Frontend lint/type checks and production Web/native Desktop verification belong to the parent gate. No real-provider smoke or model behavior trial ran here. The recorded toolchain is Go 1.27.0; this run does not establish the repository CI Go 1.26 toolchain result.', '']
    (OUT / 'README.md').write_text('\n'.join(rows))
    manifest = {str(p.relative_to(OUT)): hashlib.sha256(p.read_bytes()).hexdigest()
                for p in sorted(OUT.rglob('*')) if p.is_file() and p.name != 'evidence-sha256.json'}
    save('evidence-sha256.json', {'captured_at': gate['evidence_finalized_at'], 'files': manifest})
    print(json.dumps({'all_commands_passed':gate['all_commands_passed'], 'source_unchanged':gate['source_unchanged'], 'totals':gate['totals'], 'corrected_inventory_only':corrected}, indent=2))
    return 0 if gate['all_commands_passed'] else 1


if __name__ == '__main__':
    sys.exit(main())
