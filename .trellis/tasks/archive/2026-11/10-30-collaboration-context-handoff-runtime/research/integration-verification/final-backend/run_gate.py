#!/usr/bin/env python3
"""Read-only product-source backend verification for the frozen candidate."""
import collections
import datetime
import hashlib
import json
import os
import pathlib
import platform
import re
import shlex
import subprocess
import sys
import time
import urllib.parse

ROOT = pathlib.Path('/Volumes/artisan/code/2026/mc-runtime-integration-20261031')
OUT = pathlib.Path(__file__).resolve().parent
CANDIDATE = OUT.parent / 'final-candidate/source.json'
ENV_FILE = '/tmp/multica_runtime_full_20261101_2392cdc0.env'
DB_NAME = 'multica_runtime_full_20261101_2392cdc0'
SELECTOR = 'Collaboration|Handoff|Checkpoint|Recording|SelectPreparationAttachments|^TestIssueCreateInTxParentDistinguishesMissingFromDependencyFailure$'
TAGGED_SELECTOR = '^TestHostProviderAccountSmokeAuthorizes(SharedContextBase|OnlyNegotiatedClaimQueries)$'
ENV = os.environ.copy()
REMOVED = sorted(k for k in ENV if k.startswith('MULTICA_CONTEXT_EXPERIMENT') or k.startswith('MULTICA_RUN_REAL'))
for key in REMOVED:
    ENV.pop(key, None)
ENV.update(GOMAXPROCS='2', GOFLAGS='', PGAPPNAME='multica-runtime-final-backend')
SNAPSHOT = json.loads(CANDIDATE.read_text())


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def write_json(name, value):
    (OUT / name).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')


def capture(cmd):
    result = subprocess.run(cmd, cwd=ROOT, env=ENV, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError('preflight command failed: ' + shlex.join(cmd))
    return result.stdout


def source_check(stage):
    expected = {k: v for k, v in SNAPSHOT['files'].items() if k.startswith('server/')}
    current = {}
    for name in expected:
        path = ROOT / name
        current[name] = hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None
    mismatches = [name for name in expected if current[name] != expected[name]]
    listed = set(capture(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '--', 'server']).splitlines())
    extra = sorted(listed - expected.keys())
    result = {'captured_at': now(), 'candidate': str(CANDIDATE.relative_to(ROOT)),
              'candidate_sha256': hashlib.sha256(CANDIDATE.read_bytes()).hexdigest(),
              'candidate_head': SNAPSHOT['head'], 'candidate_source_tree_sha256': SNAPSHOT['source_tree_sha256'],
              'actual_head': capture(['git', 'rev-parse', 'HEAD']).strip(),
              'server_file_count': len(expected), 'mismatches': mismatches,
              'extra_nonignored_server_files': extra, 'matches': not mismatches and not extra,
              'files': current}
    write_json('source-' + stage + '.json', result)
    return result


def discover(package, tagged=False):
    cmd = ['go', '-C', 'server', 'list', '-json']
    if tagged:
        cmd.append('-tags=agentintegration')
    cmd.append(package)
    info = json.loads(capture(cmd))
    names = {}
    for filename in info.get('TestGoFiles', []) + info.get('XTestGoFiles', []):
        path = pathlib.Path(info['Dir']) / filename
        for name in re.findall(r'^func (Test\w+)\(', path.read_text(), re.M):
            if name != 'TestMain':
                names[name] = str(path.relative_to(ROOT))
    return names


def summarize(stem, expected):
    events = []
    non_json = []
    for line in (OUT / (stem + '.jsonl')).read_text().splitlines():
        try:
            events.append(json.loads(line))
        except ValueError:
            non_json.append(line)
    terminal = [e for e in events if e.get('Test') and e.get('Action') in ('pass', 'fail', 'skip')]
    started = {e['Test'] for e in events if e.get('Test') and e.get('Action') == 'run'}
    counts = {'top_level': collections.Counter(), 'subcase': collections.Counter()}
    outcomes = collections.defaultdict(list)
    outputs = collections.defaultdict(list)
    for event in events:
        if event.get('Output'):
            outputs[event.get('Test', '')].append(event['Output'])
    for event in terminal:
        counts['subcase' if '/' in event['Test'] else 'top_level'][event['Action']] += 1
        outcomes[event['Action']].append(event['Test'])
    skips = [{'test': name, 'output': ''.join(outputs[name])} for name in outcomes['skip']]
    failures = [{'test': name, 'output': ''.join(outputs[name])} for name in outcomes['fail']]
    (OUT / (stem + '.log')).write_text(''.join(e.get('Output', '') for e in events) + '\n'.join(non_json))
    write_json(stem + '-skips.json', skips)
    write_json(stem + '-failures.json', failures)
    return {'counts': {k: dict(v) for k, v in counts.items()}, 'started_top_level_count': sum('/' not in n for n in started),
            'expected_top_level_count': len(expected), 'missing_expected_tests': sorted(set(expected) - started),
            'outcomes': dict(outcomes), 'package_outcomes': [e for e in events if not e.get('Test') and e.get('Action') in ('pass', 'fail', 'skip')],
            'non_json_lines': non_json, 'skips': skips, 'failures': failures}


def run_command(stem, command, expected=None):
    started = now()
    argv = ['bash', 'scripts/go-test-with-agent-cli-guard.sh', '--'] + command
    meta = {'started_at': started, 'cwd': str(ROOT), 'argv': argv, 'command': shlex.join(argv),
            'environment_file': ENV_FILE, 'environment_overrides': {'GOMAXPROCS': '2', 'GOFLAGS': '', 'PGAPPNAME': ENV['PGAPPNAME']},
            'environment_unset': REMOVED, 'real_agent_gate_present': 'MULTICA_RUN_REAL_AGENT_SMOKE' in ENV,
            'database': DB_NAME, 'candidate_source_tree_sha256': SNAPSHOT['source_tree_sha256']}
    write_json(stem + '-command.json', meta)
    print('START ' + stem + ': ' + meta['command'], flush=True)
    begin = time.monotonic()
    raw_name = stem + ('.jsonl' if expected is not None else '.log')
    with (OUT / raw_name).open('w') as raw:
        result = subprocess.run(argv, cwd=ROOT, env=ENV, text=True, stdout=raw, stderr=subprocess.STDOUT)
    meta.update(finished_at=now(), elapsed_seconds=round(time.monotonic() - begin, 3), exit_code=result.returncode)
    if expected is not None:
        meta.update(summarize(stem, expected))
        meta['successful_gate'] = result.returncode == 0 and not meta['missing_expected_tests'] and meta['started_top_level_count'] > 0
    else:
        meta['successful_gate'] = result.returncode == 0
    write_json(stem + '-summary.json', meta)
    print('DONE ' + stem + ': ' + json.dumps({k: meta[k] for k in ('exit_code', 'elapsed_seconds', 'counts', 'missing_expected_tests', 'successful_gate') if k in meta}), flush=True)
    return meta


def main():
    url = urllib.parse.urlsplit(ENV.get('DATABASE_URL', ''))
    if url.path.removeprefix('/') != DB_NAME:
        raise RuntimeError('refusing unexpected database; no test commands launched')
    # libpq does not expand a URI from PGDATABASE the way -d does. Supply
    # connection fields through environment variables, keeping secrets off argv.
    ENV.update(PGDATABASE=DB_NAME, PGHOST=url.hostname or '', PGPORT=str(url.port or 5432),
               PGUSER=urllib.parse.unquote(url.username or ''),
               PGPASSWORD=urllib.parse.unquote(url.password or ''))
    for key, value in urllib.parse.parse_qsl(url.query):
        if key == 'sslmode':
            ENV['PGSSLMODE'] = value
    before = source_check('before')
    if not before['matches']:
        raise RuntimeError('frozen server candidate mismatch; no test commands launched')
    dbinfo = capture(['psql', '-X', '-A', '-t', '-v', 'ON_ERROR_STOP=1', '-c',
        "SELECT json_build_object('database', current_database(), 'postgres_version', version(), 'migration_count', (SELECT count(*) FROM schema_migrations));"])
    # psql connection fields live only in the environment, never the saved command.
    write_json('environment.json', {'captured_at': now(), 'platform': platform.platform(),
        'go_version': capture(['go', 'version']).strip(),
        'go_env': json.loads(capture(['go', '-C', 'server', 'env', '-json', 'GOOS', 'GOARCH', 'GOVERSION', 'GOMOD', 'GOWORK', 'CGO_ENABLED', 'GOFLAGS'])),
        'database': json.loads(dbinfo), 'database_env_file': ENV_FILE,
        'gated_real_agent_variables_removed': REMOVED, 'real_agent_gate_present': 'MULTICA_RUN_REAL_AGENT_SMOKE' in ENV,
        'resource_bounds': {'GOMAXPROCS': '2', 'go_package_parallelism': 1, 'test_parallelism': 2},
        'source': {k: before[k] for k in before if k != 'files'},
        'launch_command': 'set -a; source ' + shlex.quote(ENV_FILE) + '; set +a; python3 ' + shlex.quote(str(pathlib.Path(__file__).resolve()))})
    (OUT / 'server-diff-before.patch').write_text(capture(['git', 'diff', '--binary', 'HEAD', '--', 'server']))
    (OUT / 'git-status-before.txt').write_text(capture(['git', 'status', '--short']))
    common = ['go', '-C', 'server', 'test', '-race', '-count=1', '-p', '1', '-parallel', '2', '-timeout', '40m', '-json']
    definitions = [('service', './internal/service', SELECTOR, False),
                   ('handler', './internal/handler', SELECTOR, False),
                   ('protocol', './pkg/protocol', None, False),
                   ('cli', './internal/cli', None, False),
                   ('cmd-multica', './cmd/multica', None, False),
                   ('daemon-collaborationrun', './internal/daemon/collaborationrun', None, False),
                   ('tagged-fake-authorizer', './internal/daemon/collaborationrun', TAGGED_SELECTOR, True)]
    expected_by_suite = {}
    for stem, package, selector, tagged in definitions:
        names = discover(package, tagged)
        selected = {n: p for n, p in names.items() if selector is None or re.search(selector, n)}
        expected_by_suite[stem] = selected
        if stem in ('service', 'handler'):
            omitted = [n for n,p in names.items() if pathlib.Path(p).name.startswith('collaboration') and n not in selected]
            if omitted:
                raise RuntimeError('collaboration-file tests missing from selector: ' + ', '.join(omitted))
    write_json('test-inventory.json', {'selector': SELECTOR, 'tagged_selector': TAGGED_SELECTOR, 'suites': expected_by_suite})
    results = []
    for stem, package, selector, tagged in definitions:
        command = common[:]
        if tagged:
            command += ['-tags=agentintegration']
        command += [package]
        if selector:
            command += ['-run', selector]
        results.append(run_command(stem, command, expected_by_suite[stem]))
        write_json('progress.json', {'updated_at': now(), 'completed_suites': [r for r in results]})
    results.append(run_command('go-vet', ['go', '-C', 'server', 'vet', '-p', '1', './...']))
    results.append(run_command('go-build', ['go', '-C', 'server', 'build', '-p', '1', './...']))
    after = source_check('after')
    write_json('gate-summary.json', {'finished_at': now(), 'candidate_source_tree_sha256': SNAPSHOT['source_tree_sha256'],
        'source_unchanged': after['matches'], 'database': DB_NAME, 'suites': results,
        'all_commands_passed': all(r['successful_gate'] for r in results) and after['matches'],
        'real_provider_executed': False, 'frontend_checks': 'Owned by parent; outside this backend gate'})
    print('FINAL source_unchanged=' + str(after['matches']) + ' all_commands_passed=' + str(all(r['successful_gate'] for r in results)), flush=True)
    return 0 if all(r['successful_gate'] for r in results) and after['matches'] else 1


if __name__ == '__main__':
    sys.exit(main())
