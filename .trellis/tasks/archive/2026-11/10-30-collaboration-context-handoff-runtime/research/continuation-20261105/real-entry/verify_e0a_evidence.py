#!/usr/bin/env python3
"""Read the retained E0a run and reconcile facts without rerunning it."""
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess

MAIN = Path('/Volumes/artisan/code/2026/multica-0.4.37')
ROOT = MAIN.parent / 'mc-runtime-acceptance-20261104'
TASK = MAIN / '.trellis/tasks/10-30-collaboration-context-handoff-runtime'
OUT = TASK / 'research/continuation-20261105/real-entry'
PRIVATE = ROOT / '.omx/private/runtime-real-20261105'
HERE = ROOT / '.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1'
TRIAL = ROOT / '.omx/private/rk1/RK1-E0a/RK1-E0a-t1'
CHECKS, HASHES = [], {}


def sha(path):
    value = hashlib.sha256(Path(path).read_bytes()).hexdigest()
    HASHES[str(path)] = value
    return value


def read(path):
    sha(path)
    return json.loads(Path(path).read_text())


def check(name, passed):
    CHECKS.append({'name': name, 'passed': bool(passed)})


def stamp(value):
    return datetime.fromisoformat(value.replace('Z', '+00:00'))


def main():
    helper = read(PRIVATE / 'run-result.json')
    build = read(PRIVATE / 'build-result.json')
    precheck = read(PRIVATE / 'check-result.json')
    permit = read(PRIVATE / 'permit-used-run.json')
    closed = read(PRIVATE / 'permit.json')
    attempt = read(PRIVATE / 'run-attempt.json')
    capture = read(PRIVATE / 'captures.json')
    post = read(PRIVATE / 'post-run-binding-observation.json')
    public = read(HERE / 'results/RK1-E0a-t1.json')
    private_public = read(TRIAL / 'public.json')
    invocations = list((ROOT / '.omx/private/rk1/RK1-E0a').glob('invocation-*.json'))
    check('exact_one_invocation', len(invocations) == 1)
    invocation = read(invocations[0])
    check('invocation_hash_retained', sha(invocations[0]) == helper['invocation_files'][0]['sha256'])
    check('public_copy_exact', public == private_public and sha(HERE / 'results/RK1-E0a-t1.json') == helper['public_sha256'])
    check('single_real_E0a_slot', public['label'] == 'RK1-E0a-t1' and public['mode'] == 'real' and public['trial'] == 1)
    check('raw_finished_pass', public['status'] == 'finished' and public['verdict'] == 'pass')
    check('answer_exact', public['answer'] == 'HOST_SMOKE_OK')
    check('launcher_exit_and_stop', invocation['exit_code'] == invocation['executor_exit_code'] == helper['exit_code'] == 0 and invocation['stop_state'] == {'triggered': False, 'incomplete_or_blocking_trials': [], 'missing_requested_trials': [], 'unexpected_trials': []})
    check('actual_original_launcher_command', invocation['command'] == ['go', 'test', '-tags=agentintegration', './cmd/server', '-run', '^TestRK1RealProvider$', '-count=1', '-v', '-timeout=12m'])
    check('same_build_check_run_binding', build['binding_before'] == build['binding_after'] == precheck['binding'] == helper['binding_before'] == attempt['binding'] == permit['binding'])
    check('precheck_and_build_pass', build['exit_code'] == precheck['exit_code'] == 0)
    check('permit_bound_build_check', sha(PRIVATE / 'build-result.json') == permit['build_result_sha256'] and sha(PRIVATE / 'check-result.json') == permit['check_result_sha256'])
    check('permit_and_attempt_bound', sha(PRIVATE / 'permit-used-run.json') == attempt['permit_sha256'] and permit['allow_run'] is True)
    check('actual_finished_before_window_end', stamp(invocation['finished_at']) < stamp(permit['window_expires_at']))
    check('permits_closed_after_run', all(closed[k] is False for k in ('allow_build', 'allow_check', 'allow_run')))
    check('helper_refusal_retained', helper['ready_for_independent_review'] is False and helper['evidence_error_type'] == 'ValueError' and 'binding_after' not in helper)

    run = public['runs'][0]
    rid = run['run_id']
    check('single_measured_run', len(public['runs']) == 1 and run['measured'] is True and run['physical_launches'] == run['authorized_attempts'] == 1)
    launch_files = list((TRIAL / 'provider' / rid).glob('launch-*/launch.json'))
    check('one_launch_record', len(launch_files) == 1)
    launch = read(launch_files[0])
    launch_dir = launch_files[0].parent
    plan = read(TRIAL / 'provider' / (rid + '.json'))
    check('real_registered_provider', plan['mode'] == 'real' and Path(plan['real_executable']).resolve() == Path(invocation['provider_executable_target']))
    check('complete_successful_launch', launch['mode'] == 'real' and launch['evidence_state'] == 'complete' and launch['exit_code'] == 0)
    check('argv_exact_public', [launch['args']] == run['argv'])
    check('expected_model_no_resume', launch['args'][launch['args'].index('--model') + 1] == 'claude-opus-5-5' and '--resume' not in launch['args'] and '--no-session-persistence' not in launch['args'])
    check('private_variables_removed', {'DATABASE_URL', 'RK1_PROVIDER_PLAN_DIR'} <= set(launch['removed_env']))
    stdin = read(launch_dir / 'stdin-first.jsonl')
    prompt = stdin['message']['content'][0]['text']
    prompt_hash = hashlib.sha256(prompt.encode()).hexdigest()
    envelope = json.loads(prompt[prompt.index('\n{') + 1:])['handoff']
    check('stdin_prompt_hash', run['stdin_prompt_sha256'] == [prompt_hash])
    check('handoff_input_run_identity', envelope['run_id'] == rid)
    raw_stdout = (launch_dir / 'stdout.jsonl').read_text()
    sha(launch_dir / 'stdout.jsonl')
    frames = [json.loads(line) for line in raw_stdout.splitlines() if line.strip()]
    init = [f for f in frames if f.get('type') == 'system' and f.get('subtype') == 'init']
    terminal = [f for f in frames if f.get('type') == 'result']
    check('observed_provider_model', len(init) == 1 and init[0]['claude_code_version'] == public['provider_version'] == '2.1.318' and init[0]['model'] == 'claude-opus-5-5' and public['observed_models'] == ['claude-opus-5-5'])
    check('raw_provider_result_exact', len(terminal) == 1 and terminal[0]['is_error'] is False and terminal[0]['result'] == 'HOST_SMOKE_OK')

    trace = [json.loads(line) for line in (TRIAL / 'trace.jsonl').read_text().splitlines() if line.strip()]
    sha(TRIAL / 'trace.jsonl')
    check('trace_sequence', [x['seq'] for x in trace] == list(range(1, len(trace) + 1)))
    check('trace_complete_before_public', all(stamp(x['at']) <= stamp(public['finished_at']) for x in trace))
    check('trace_requests_success', all(x['status'] == 200 for x in trace))
    def events(suffix):
        return [x for x in trace if x['path'].endswith(suffix)]
    claims, permits, binds = events('/claim'), events('/permit'), events('/handoff/bind')
    check('negotiated_claim', len(claims) == 1 and claims[0]['response']['handoff_version'] == 1 and claims[0]['response']['recording_version'] == 1 and not claims[0]['response'].get('knowledge_context', False))
    attempts = {(x['response']['attempt_seq'], x['response']['attempt_nonce']) for x in permits}
    check('one_attempt_two_permits', len(permits) == 2 and len(attempts) == 1)
    check('actual_input_binding', len(binds) == 1 and binds[0]['request']['prompt_sha256'] == prompt_hash and binds[0]['request']['snapshot_id'] == envelope['snapshot_id'] and (binds[0]['request']['attempt_seq'], binds[0]['request']['attempt_nonce']) in attempts)
    contexts = events('/context')
    check('actual_cli_context_read', len(contexts) == 1 and contexts[0].get('platform') == 'cli' and contexts[0]['response']['issue_id'] == envelope['issue_id'])
    tool_results = [c for f in frames if f.get('type') == 'user' for c in f.get('message', {}).get('content', []) if c.get('type') == 'tool_result']
    check('actual_context_shown_to_model', len(tool_results) == 1 and envelope['issue_id'] in json.dumps(tool_results[0].get('content')))
    check('no_business_writes', run['business_writes'] == 0 and not any(x['method'] != 'GET' and x['path'].endswith(('/comments', '/memory', '/squad-activity')) for x in trace))
    check('two_checkpoints', len(events('/checkpoints')) == public['criteria']['checkpoints'] == run['run_events']['record_checkpoint'] == 2)
    check('one_termination', len(events('/termination')) == 1)
    outcomes, proofs, seals = events('/outcome'), events('/proof'), events('/recording/seal')
    check('exact_outcome_success', len(outcomes) == 1 and outcomes[0]['request']['outcome'] == 'success' and outcomes[0]['request']['body'] == 'HOST_SMOKE_OK')
    check('accepted_proof', len(proofs) == 1 and proofs[0]['request']['state'] == 'host_exit_observed' and proofs[0]['response']['accepted'] is True)
    check('completed_seal_actual_ACK', len(seals) == 1 and seals[0]['request']['reason'] == 'completed' and seals[0]['request']['observed_through'] == seals[0]['request']['acknowledged_through'] == 3 and seals[0]['response']['sealed'] is True and seals[0]['response']['retained_through'] == 3)
    check('public_lifecycle_complete', run['progress_state'] == 'settled' and run['proof_state'] == 'host_exit_observed' and run['business_outcome'] == 'success' and run['recording_incomplete'] is False and run['outcome_record_completeness'] == 'complete')
    ledgers = list((TRIAL / 'ledger' / rid).glob('*.json'))
    check('one_private_ledger', len(ledgers) == 1 and ledgers[0].stat().st_mode & 0o077 == 0)
    ledger = read(ledgers[0])
    check('durable_outcome_proof_seal', ledger['phase'] == 'proof_acknowledged' and ledger['host']['outcome_recorded'] is True and ledger['host']['recording']['seal_accepted'] is True and len(ledger['host']['launches']) == 1)

    observations = capture['observations']
    check('captures_original_match', observations == helper['captures'] and not capture['errors'] and not helper['observer_errors'])
    nested = [x for x in observations if Path(x['path']).parent.name == '001']
    prepared = [x for x in observations if Path(x['path']).parent.name == 'bin']
    expected = str(Path(launch['cwd']).parent / 'runs' / rid / 'bin/multica')
    check('exact_two_capture_paths', len(observations) == 2 and len(nested) == len(prepared) == 1 and prepared[0]['path'] == expected)
    check('all_three_actual_CLI_bytes_equal', sha(PRIVATE / 'upfront-multica') == build['cli_sha256'] and all(sha(x['copy']) == x['sha256'] == build['cli_sha256'] for x in observations))
    check('captures_precede_launch', all(stamp(x['at']) < stamp(launch['started_at']) for x in observations))
    check('captured_CLI_sizes', all(Path(x['copy']).stat().st_size == x['size'] == build['size'] for x in observations))
    check('private_capture_permissions', all(Path(x['copy']).stat().st_mode & 0o077 == 0 for x in observations))
    check('CLI_build_metadata_go_module', 'go1.26.6' in build['build_info'] and 'github.com/multica-ai/multica/server/cmd/multica' in build['build_info'])
    budget = invocation['launch_budget']
    check('budget_exact', budget['cap'] == 25 and budget['reserved'] == 1 and budget['before']['consumed'] == 0 and budget['after']['consumed'] == 1 and budget['remaining'] == 24)
    check('public_private_count_agree', budget['after']['public_per_trial'] == budget['after']['private_per_trial'] == budget['after']['counted_per_trial'] == {'RK1-E0a-t1': 1})

    previous = TASK / 'research/continuation-20261102/rki6-review/verify_evidence.py'
    spec = importlib.util.spec_from_file_location('source_checker', previous)
    code = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(code)
    source = code.source(ROOT)
    before = helper['binding_before']
    check('current_product_framework_stable', source['source_id'] == before['source_id'] and source['framework_sha256'] == before['framework_sha256'] and source['head'] == before['head'] and source['product_dirty_paths'] == [])
    status = subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain=v1', '--untracked-files=all'], text=True)
    allowed = '?? ' + str((HERE / 'results/RK1-E0a-t1.json').relative_to(ROOT)) + '\n'
    check('only_allowed_public_status', status == allowed)
    for key, path in [('helper_sha256', PRIVATE / 'e0a_entry.py'), ('config_sha256', PRIVATE / 'config.json'),
                      ('registration_sha256', HERE / 'framework-registration.json'), ('addendum_sha256', HERE / 'runtime-addendum.json'),
                      ('launcher_sha256', HERE / 'run_rk1.py'), ('git_sha256', before['git']),
                      ('python_sha256', before['python']), ('go_sha256', '/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go'),
                      ('provider_sha256', invocation['provider_executable_target'])]:
        check('retained_binding:' + key, sha(path) == before[key])
    check('operator_at_start_matches_registration', invocation['operator_claude_files'] == read(HERE / 'framework-registration.json')['operator_configuration'] == post['operator_before'])
    changed = [k for k in post['operator_after'] if post['operator_before'].get(k) != post['operator_after'][k]]
    check('post_operator_delta_identified', changed == post['operator_changed_keys'] == ['~/.claude/plugins/'])
    check('post_metadata_other_fields_same', post['source_id'] == before['source_id'] and post['head'] == before['head'] and not post['source_dirty'] and all(post[k] is True for k in ['framework_equal', 'launcher_equal', 'provider_equal', 'env_equal']))
    report = {
        'reviewed_at': datetime.now(timezone.utc).isoformat(), 'reviewer': '/root/acceptance_review',
        'scope': 'Actual RK1-E0a-t1 raw behavior/lifecycle/artifacts; configuration stability is a separate unmet condition',
        'behavior_verdict': 'verified_finished_pass' if all(x['passed'] for x in CHECKS) else 'changes-required',
        'overall_acceptance': 'HELD_CONFIGURATION_STABILITY_NOT_PROVEN',
        'checks_passed': sum(x['passed'] for x in CHECKS), 'checks_failed': [x for x in CHECKS if not x['passed']],
        'run_id': rid, 'answer': public['answer'], 'provider_version_observed': public['provider_version'],
        'requested_model': public['requested_model'], 'observed_models': public['observed_models'],
        'actual_invocation': {'exit': invocation['exit_code'], 'executor_exit': invocation['executor_exit_code'], 'stop_state': invocation['stop_state'], 'launch_budget': budget},
        'source': source, 'trace_events': len(trace), 'CLI_sha256': build['cli_sha256'],
        'captures_bound_to_actual_run': True,
        'helper_failure': {'original_exit': 1, 'original_review_ready': False, 'original_error': 'ValueError',
                           'identified_condition': 'binding(): require(operator == reg.operator_configuration) during post-run binding',
                           'operator_before_plugin_hash': post['operator_before']['~/.claude/plugins/'],
                           'operator_after_plugin_hash': post['operator_after']['~/.claude/plugins/'],
                           'metadata_observation_at': post['at'],
                           'same_passive_captures_reconciled_without_rerun': True,
                           'operator_stability_reconciled': False},
        'disposition': ['Preserve actual launcher/public finished-pass and helper exit1/review_readyfalse together.',
                        'Count1 real launch of25; remaining24. No E0a replacement or rerun.',
                        'Keep subsequent scenarios held until the configuration evidence gap is dispositioned and a new window is granted.',
                        'Plugin mtime observations cannot reconstruct prelaunch bytes or prove that changes were harmless metadata or made by this Provider.',
                        'Do not overwrite original helper report or silently update its acceptance flag.'],
        'checks': CHECKS, 'hashes': HASHES,
        'operations': {'tests': 0, 'database': 0, 'Provider': 0, 'binary_execution': 0, 'plugin_tree_scan': 0},
    }
    with (OUT / 'e0a-independent-review.json').open('x') as f:
        json.dump(report, f, indent=2); f.write('\n')
    print(json.dumps({k: report[k] for k in ['behavior_verdict', 'overall_acceptance', 'checks_passed', 'checks_failed']}))


if __name__ == '__main__':
    main()
