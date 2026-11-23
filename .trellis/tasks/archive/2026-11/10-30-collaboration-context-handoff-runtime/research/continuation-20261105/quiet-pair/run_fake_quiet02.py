#!/usr/bin/env python3
"""Record one authorized fake RK1 run; never invokes a model CLI."""
import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from urllib.parse import urlsplit

MAIN = Path('/Volumes/artisan/code/2026/multica-0.4.37')
ROOT = Path('/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104')
TASK = MAIN / '.trellis/tasks/10-30-collaboration-context-handoff-runtime'
PRIVATE = ROOT / '.omx/private/runtime-fake-20261105-quiet02'
PUBLIC = TASK / 'research/continuation-20261105/quiet-pair'
GO = Path('/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go')
PSQL = Path('/opt/homebrew/opt/libpq/bin/psql')
HEAD = '1a32ca36c'
FRAMEWORK = '5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9'
SELECTOR = '^(TestRK1FakeProof|TestRK1Evidence.*|TestRK1HandoffAnswersRequireTheirExactLineCounts|TestRK1ProviderWithoutMarkerCannotEnterTestMain|TestRK1TraceSnapshotWaitsForCompletedHandler)$'
TOPS = {
    'TestRK1FakeProof', 'TestRK1EvidenceFaultProof',
    'TestRK1EvidencePublicationFailureRetainsExecutionAndStops',
    'TestRK1EvidencePublicationKeepsProviderFailureSeparate',
    'TestRK1EvidenceUnfinalizedResultCannotPass',
    'TestRK1EvidenceCachedFinalRecordCannotHideFailedWrapperExit',
    'TestRK1HandoffAnswersRequireTheirExactLineCounts',
    'TestRK1ProviderWithoutMarkerCannotEnterTestMain',
    'TestRK1TraceSnapshotWaitsForCompletedHandler',
}


def now():
    return datetime.now(timezone.utc).isoformat()


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def write(path, data):
    with Path(path).open('x') as out:
        json.dump(data, out, indent=2, ensure_ascii=False)
        out.write('\n')


def git(*args):
    return subprocess.check_output(['git', '-C', str(ROOT), *args], text=True).strip()


def snapshot(path):
    subprocess.run([sys.executable, str(TASK / 'research/real-model-rk1/source_manifest.py'),
                    str(ROOT), str(path)], check=True, stdout=subprocess.DEVNULL)
    manifest = json.loads(path.read_text())
    assert manifest['head'].startswith(HEAD), 'unexpected source HEAD'
    assert not manifest['dirty_product_paths'], 'dirty product source'
    digest = hashlib.sha256()
    files = sorted((ROOT / 'server/cmd/server').glob('rk1_*_test.go'))
    for file in files:
        raw = file.read_bytes()
        digest.update(file.name.encode() + b'\0' + str(len(raw)).encode() + b'\0' + raw)
    assert len(files) == 17 and digest.hexdigest() == FRAMEWORK, 'framework drift'
    assert sha(GO) == 'a1c83801d1756c3eca78366c6b585f2c21c20694fb1c7eb92c446a0580420412', 'Go drift'
    return {'head': manifest['head'], 'source_id': manifest['source_id'],
            'file_count': manifest['file_count'], 'framework_sha256': digest.hexdigest(),
            'framework_files': {str(p.relative_to(ROOT)): sha(p) for p in files},
            'go_sha256': sha(GO), 'psql_sha256': sha(PSQL),
            'guard_sha256': sha(ROOT / 'scripts/go-test-with-agent-cli-guard.sh'),
            'guard_names_sha256': sha(ROOT / 'scripts/agent-cli-command-names.txt'),
            'manifest_sha256': sha(path)}


def db_query(url, sql):
    result = subprocess.run([str(PSQL), '-XAt', '-d', url, '-c', sql],
                            env={'PATH': '/usr/bin:/bin', 'PGOPTIONS': '-c default_transaction_read_only=on'},
                            text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError('read-only database identity probe failed; credentials withheld')
    return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--mode', choices=['normal', 'race'], required=True)
    parser.add_argument('--config', type=Path, required=True)
    args = parser.parse_args()
    os.umask(0o077)
    config = json.loads(args.config.read_text())
    url = config['DATABASE_URL']
    database = config['database']
    assert config['mode'] == args.mode
    assert Path(config['candidate']) == ROOT, 'allocation candidate differs'
    assert database.startswith('multica_rk1_') and urlsplit(url).path == '/' + database
    assert args.config.stat().st_mode & 0o077 == 0, 'private config permissions'
    if args.mode == 'race':
        normal = json.loads((PRIVATE / 'normal/result.json').read_text())
        assert normal['accepted'] is True, 'normal was not accepted'
        assert normal['database'] != database, 'databases must differ'
    PRIVATE.mkdir(mode=0o700, parents=True, exist_ok=True)
    run = PRIVATE / args.mode
    run.mkdir(mode=0o700)
    for name in ['home', 'gocache', 'gopath', 'proof']:
        (run / name).mkdir(mode=0o700)
    tmp = Path(tempfile.mkdtemp(prefix='multica-rk1-' + args.mode + '-', dir='/private/tmp'))
    before = snapshot(run / 'source-before.json')
    identity = db_query(url, 'SELECT current_database(),max(version) FROM schema_migrations')
    assert identity == database + '|764_collaboration_ai_policy', 'DB or migration differs'
    connections = db_query(url, "SELECT count(*),current_setting('max_connections') FROM pg_stat_activity")
    count, limit = map(int, connections.split('|'))
    assert limit - count >= 35, 'insufficient shared database capacity'
    env = {
        'PATH': str(GO.parent) + ':/opt/homebrew/opt/libpq/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin',
        'HOME': str(run / 'home'), 'TMPDIR': str(tmp), 'DATABASE_URL': url,
        'RK1_FAKE_PROOF_OUT': str(run / 'proof'), 'GOROOT': str(GO.parent.parent),
        'GOCACHE': '/Volumes/artisan/dev-cache/go-build', 'GOMODCACHE': '/Users/artisan/go/pkg/mod',
        'GOPATH': str(run / 'gopath'), 'GOTOOLCHAIN': 'local', 'GOENV': 'off',
        'GOPROXY': 'off', 'GOSUMDB': 'off', 'GOTELEMETRY': 'off',
        'GOMAXPROCS': '2', 'GOFLAGS': '-p=2', 'LANG': 'en_US.UTF-8',
    }
    command = ['bash', 'scripts/go-test-with-agent-cli-guard.sh', '--', str(GO), '-C', 'server',
               'test', './cmd/server', '-parallel=2', '-count=1', '-json', '-timeout=40m']
    if args.mode == 'race':
        command.append('-race')
    command += ['-tags=agentintegration', '-run', SELECTOR]
    started = now()
    record = {'mode': args.mode, 'started_at': started, 'database': database,
              'database_identity': identity, 'connection_preflight': connections,
              'source_before': before, 'command': command, 'cwd': str(ROOT),
              'env_sha256': hashlib.sha256(json.dumps(env, sort_keys=True).encode()).hexdigest(),
              'config_sha256': sha(args.config), 'runner_sha256': sha(__file__),
              'environment_without_credentials': {k:v for k,v in env.items() if k != 'DATABASE_URL'},
              'real_provider_calls': 0, 'real_provider_allowed': False}
    write(run / 'admission.json', record)
    with (run / 'tests.jsonl').open('xb') as stdout, (run / 'stderr.log').open('xb') as stderr:
        process = subprocess.Popen(command, cwd=ROOT, env=env, stdout=stdout, stderr=stderr)
        write(run / 'process.json', {'pid': process.pid, 'started_at': started})
        print(json.dumps({'mode': args.mode, 'pid': process.pid, 'state': 'running'}), flush=True)
        exit_code = process.wait()
    counts = Counter()
    tops = Counter()
    failures = []
    invalid = 0
    for line in (run / 'tests.jsonl').read_text().splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            invalid += 1
            continue
        name, action = event.get('Test'), event.get('Action')
        if name and action in ('run', 'pass', 'fail', 'skip'):
            counts[action] += 1
            if '/' not in name and action == 'pass':
                tops[name] += 1
            if action in ('fail', 'skip'):
                failures.append({'test': name, 'action': action})
    public_trials = sorted((run / 'proof').glob('*/RK1-*/public.json'))
    trials = []
    for p in public_trials:
        d = json.loads(p.read_text())
        trials.append({'case': p.parent.parent.name, 'file': str(p), 'sha256': sha(p),
                       'scenario': d.get('scenario'), 'trial': d.get('trial'),
                       'status': d.get('status'), 'verdict': d.get('verdict'), 'mode': d.get('mode')})
    after = snapshot(run / 'source-after.json')
    stable = all(before[k] == after[k] for k in before if k != 'manifest_sha256')
    stderr_text = (run / 'stderr.log').read_text()
    record.update({'finished_at': now(), 'exit_code': exit_code, 'counts': dict(counts),
                   'top_pass': dict(tops), 'failures': failures, 'invalid_json_lines': invalid,
                   'fixture_cases': len([p for p in (run / 'proof').iterdir() if p.is_dir()]),
                   'public_trial_count': len(trials), 'public_trials': trials,
                   'source_after': after, 'source_stable': stable,
                   'tests_sha256': sha(run / 'tests.jsonl'), 'stderr_sha256': sha(run / 'stderr.log'),
                   'guard_violation': 'unexpected agent CLI invocation:' in stderr_text,
                   'database_identity_after': db_query(url, 'SELECT current_database(),max(version) FROM schema_migrations')})
    record['accepted'] = (exit_code == 0 and counts == Counter({'run': 62, 'pass': 62}) and
                          set(tops) == TOPS and all(n == 1 for n in tops.values()) and
                          invalid == 0 and len(trials) == 22 and record['fixture_cases'] == 19 and
                          stable and not record['guard_violation'] and record['database_identity_after'] == identity)
    write(run / 'result.json', record)
    write(PUBLIC / (args.mode + '-result.json'), record)
    print(json.dumps({k: record[k] for k in ['mode', 'exit_code', 'counts', 'accepted', 'source_stable', 'fixture_cases', 'public_trial_count']}), flush=True)
    return 0 if record['accepted'] else 1


if __name__ == '__main__':
    sys.exit(main())
