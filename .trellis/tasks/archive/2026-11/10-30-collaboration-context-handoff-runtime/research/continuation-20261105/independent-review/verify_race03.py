#!/usr/bin/env python3
"""Read retained fake execution evidence, without running tests or services."""
import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess

MAIN = Path('/Volumes/artisan/code/2026/multica-0.4.37')
TASK = MAIN / '.trellis/tasks/10-30-collaboration-context-handoff-runtime'
CANDIDATE = MAIN.parent / 'mc-runtime-acceptance-20261104'
PRIVATE = CANDIDATE / '.omx/private/runtime-fake-20261105-race03'
OUT = TASK / 'research/continuation-20261105/independent-review'
PREVIOUS = TASK / 'research/continuation-20261102/rki6-review/verify_evidence.py'
HEAD = '1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc'
FRAMEWORK = '5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9'
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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('mode', choices=['race'])
    parser.add_argument('--output', required=True)
    parser.add_argument('--allocation', type=Path, required=True)
    args = parser.parse_args()
    spec = importlib.util.spec_from_file_location('retained_evidence_checks', PREVIOUS)
    proof = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(proof)
    check, sha = proof.check, proof.sha
    run = PRIVATE / args.mode
    result = json.loads((run / 'result.json').read_text())
    admission = json.loads((run / 'admission.json').read_text())
    source = proof.source(CANDIDATE)
    check('candidate_head', source['head'] == HEAD)
    check('candidate_clean', source['product_dirty_paths'] == [])
    check('source_before_after_current', source['source_id'] == result['source_before']['source_id'] == result['source_after']['source_id'])
    check('framework_before_after_current', source['framework_sha256'] == FRAMEWORK == result['source_before']['framework_sha256'] == result['source_after']['framework_sha256'])
    check('framework_17', source['framework_file_count'] == 17)
    for key, path in [('go_sha256', '/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go'),
                      ('psql_sha256', '/opt/homebrew/opt/libpq/bin/psql'),
                      ('guard_sha256', CANDIDATE / 'scripts/go-test-with-agent-cli-guard.sh'),
                      ('guard_names_sha256', CANDIDATE / 'scripts/agent-cli-command-names.txt')]:
        check(key, sha(Path(path)) == result['source_before'][key] == result['source_after'][key])
    allocation = json.loads(args.allocation.read_text())
    permit = next(p for p in allocation['databases'] if p['mode'] == args.mode)
    check('allocation', permit['candidate'] == str(CANDIDATE) if 'candidate' in permit else allocation.get('candidate', allocation.get('root')) == str(CANDIDATE))
    check('allocation_head', allocation['head'] == HEAD)
    check('database_identity', result['database'] == permit['database'] and result['database_identity'] == result['database_identity_after'] == permit['database'] + '|764_collaboration_ai_policy')
    check('config_hash', sha(Path(permit['config'])) == result['config_sha256'])
    check('actual_exit', result['exit_code'] == 0)
    check('current_runner_hash', sha(TASK / 'research/continuation-20261105/quiet-race03/run_fake_race03.py') == result['runner_sha256'])
    check('normal_accepted_result' if args.mode == 'normal' else 'race_accepted_result', result['accepted'] is True)
    if args.mode == 'race':
        normal = json.loads((CANDIDATE / '.omx/private/runtime-fake-20261105-quiet02/normal/result.json').read_text())
        check('quiet02_normal_prerequisite', normal['accepted'] is True and normal['database'] != result['database'] and normal['finished_at'] <= result['started_at'])
    normal_review = json.loads((OUT / 'quiet02-normal-review.json').read_text())
    check('retained_normal_original_hash', sha(CANDIDATE / '.omx/private/runtime-fake-20261105-quiet02/normal/result.json') == normal_review['actual_result_sha256'])
    check('same_normal_source_framework_tools', all(normal['source_before'][k] == result['source_before'][k] for k in ['head', 'source_id', 'framework_sha256', 'go_sha256', 'guard_sha256', 'guard_names_sha256']))
    check('admission_matches_result', all(result[k] == v for k, v in admission.items()))
    check('raw_hash', sha(run / 'tests.jsonl') == result['tests_sha256'])
    check('stderr_hash', sha(run / 'stderr.log') == result['stderr_sha256'])
    raw = (run / 'tests.jsonl').read_text()
    events = [json.loads(line) for line in raw.splitlines()]
    names = {action: [e['Test'] for e in events if e.get('Test') and e.get('Action') == action] for action in ('run', 'pass', 'fail', 'skip')}
    check('full_named_inventory', len(names['run']) == len(names['pass']) == 62 and not names['fail'] and not names['skip'])
    check('no_duplicate_or_missing_terminals', Counter(names['run']) == Counter(names['pass']) and all(n == 1 for n in Counter(names['run']).values()))
    check('top_level_inventory', {name for name in names['pass'] if '/' not in name} == TOPS)
    check('no_race_or_guard_failure', 'WARNING: DATA RACE' not in raw and 'unexpected agent CLI invocation:' not in (run / 'stderr.log').read_text())
    check('command_selector', result['command'][-2:] == ['-run', '^(TestRK1FakeProof|TestRK1Evidence.*|TestRK1HandoffAnswersRequireTheirExactLineCounts|TestRK1ProviderWithoutMarkerCannotEnterTestMain|TestRK1TraceSnapshotWaitsForCompletedHandler)$'])
    check('race_mode', ('-race' in result['command']) == (args.mode == 'race'))
    env = result['environment_without_credentials']
    check('no_paid_flags_or_account_environment', not any(k.startswith(('ANTHROPIC_', 'OPENAI_', 'CLAUDE_')) or k in ('MULTICA_RUN_REAL_AGENT_SMOKE', 'MULTICA_RK1_EXECUTABLE', 'MULTICA_KNOWLEDGE_CONTEXT_ENABLED') for k in env))
    check('private_home_tmp', all(Path(env[k]).is_dir() and Path(env[k]).stat().st_mode & 0o077 == 0 for k in ('HOME', 'TMPDIR')))
    check('go_for_nested_build', env['PATH'].startswith('/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin:'))
    files = sorted((run / 'proof').glob('*/RK1-*/public.json'))
    check('actual_public_file_count', len(files) == 22)
    check('exact_fixture_cases', {p.parent.parent.name for p in files} == set(proof.EXPECTED))
    observed = {}
    trials = []
    physical = 0
    for path in files:
        public = json.loads(path.read_text())
        case = path.parent.parent.name
        observed.setdefault(case, []).append((public['trial'], public['status'], public['verdict']))
        indexed = {**public, 'case': case, 'private_path': str(path.parent),
                   'public_sha256': sha(path), 'trace_sha256': sha(path.parent / 'trace.jsonl')}
        trials.append(proof.verify_trial(args.mode, indexed))
        physical += sum(r['physical_launches'] for r in public['runs'])
    check('exact_expected_status_verdicts', {k: sorted(v) for k, v in observed.items()} == proof.EXPECTED)
    check('fakeproof_physical_launches', physical == 32)
    report = {
        'reviewed_at': datetime.now(timezone.utc).isoformat(),
        'reviewer': '/root/acceptance_review', 'mode': args.mode,
        'scope': 'Original completed runtime fake execution; read-only evidence review, no rerun',
        'verdict': 'pass-with-notes' if all(c['passed'] for c in proof.CHECKS) else 'changes-required',
        'checks_passed': sum(c['passed'] for c in proof.CHECKS),
        'checks_failed': [c for c in proof.CHECKS if not c['passed']],
        'named_pass': len(names['pass']), 'top_pass': len(TOPS),
        'public_trials': trials, 'fakeproof_physical_launches': physical,
        'source': source, 'actual_result_sha256': sha(run / 'result.json'),
        'review_script_sha256': sha(Path(__file__)), 'reused_read_only_verifier_sha256': sha(PREVIOUS),
        'hashes': proof.HASHES, 'checks': proof.CHECKS,
        'notes': ['This is a new numbered race-only run, paired with accepted quiet02 normal; prior failures and overlapped results remain unchanged.',
                  'FakeProof physical launch count excludes separate EvidenceFaultProof controls.',
                  'H6 remains manual_review_required; fake success does not prove real-model understanding.',
                  'Root must separately establish and preserve the coordinated quiet window.'],
        'operations': {'tests': 0, 'database_queries': 0, 'services': 0, 'provider_calls': 0},
    }
    output = OUT / args.output
    with output.open('x') as f:
        json.dump(report, f, indent=2)
        f.write('\n')
    print(json.dumps({k: report[k] for k in ('mode', 'verdict', 'checks_passed', 'checks_failed', 'named_pass', 'fakeproof_physical_launches')}))


if __name__ == '__main__':
    main()
