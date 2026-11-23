#!/usr/bin/env python3
"""Read H3a's retained pass and incomplete result without rerunning either."""
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess

MAIN = Path('/Volumes/artisan/code/2026/multica-0.4.37')
ROOT = MAIN.parent / 'mc-runtime-acceptance-20261104'
TASK = MAIN / '.trellis/tasks/10-30-collaboration-context-handoff-runtime'
OUT = TASK / 'research/continuation-20261105/h3a-entry'
HERE = ROOT / '.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1'
PRIVATE = ROOT / '.omx/private/runtime-successor-20261105/RK1-H3a-from-1'
SERIES = ROOT / '.omx/private/rk1/RK1-H3a'
CHECKS, HASHES = [], {}


def sha(path):
    value = hashlib.sha256(Path(path).read_bytes()).hexdigest()
    HASHES[str(path)] = value
    return value


def read(path):
    sha(path)
    return json.loads(Path(path).read_text())


def check(name, value):
    CHECKS.append({'name': name, 'passed': bool(value)})


def stamp(value):
    return datetime.fromisoformat(value.replace('Z', '+00:00'))


def manifest_hash(manifest):
    lines = sorted(f'{p}\0{v["kind"]}\0{v["value"]}\n' for p, v in manifest['entries'].items())
    return hashlib.sha256(''.join(lines).encode('utf-8', 'surrogateescape')).hexdigest()


def main():
    result = read(PRIVATE / 'run-result.json')
    build, precheck = read(PRIVATE / 'build-result.json'), read(PRIVATE / 'check-result.json')
    permit, closed = read(PRIVATE / 'permit-used-run.json'), read(PRIVATE / 'permit.json')
    attempt = read(PRIVATE / 'run-attempt.json')
    invocations = list(SERIES.glob('invocation-*.json'))
    check('one_original_invocation', len(invocations) == 1)
    invocation = read(invocations[0])
    check('original_invocation_hash', sha(invocations[0]) == result['invocation_files'][0]['sha256'])
    before, after = result['binding_before'], result['binding_after']
    check('actual_failure_preserved', result['exit_code'] == invocation['exit_code'] == invocation['executor_exit_code'] == 1 and result['ready_for_independent_review'] is False)
    check('original_stop_triggered', invocation['stop_state'] == {'triggered': True, 'incomplete_or_blocking_trials': ['RK1-H3a-t2'], 'missing_requested_trials': [], 'unexpected_trials': []})
    check('original_command_26m_unchanged', invocation['command'] == ['go', 'test', '-tags=agentintegration', './cmd/server', '-run', '^TestRK1RealProvider$', '-count=1', '-v', '-timeout=26m'])
    check('build_check_run_admission_equal', before == build['binding_before'] == build['binding_after'] == precheck['binding'] == permit['binding'] == attempt['binding'])
    check('upfront_build_check_success', build['exit_code'] == precheck['exit_code'] == 0)
    check('permit_hash_bindings', sha(PRIVATE / 'build-result.json') == permit['build_result_sha256'] and sha(PRIVATE / 'check-result.json') == permit['check_result_sha256'] and sha(PRIVATE / 'permit-used-run.json') == attempt['permit_sha256'])
    check('closed_permissions_no_next', all(closed[k] is False for k in ['allow_build', 'allow_check', 'allow_run']) and result['next_scenario_authorized'] is False)
    check('actual_finish_inside_window', stamp(invocation['finished_at']) < stamp(permit['window_expires_at']))
    captures = result['captures']
    check('three_actual_CLI_captures', len(captures) == 3 and not result['observer_errors'])
    check('upfront_nested_and_two_prepared_bytes', sha(PRIVATE / 'upfront-multica') == build['cli_sha256'] and all(sha(x['copy']) == x['sha256'] == build['cli_sha256'] for x in captures))
    check('capture_size_privacy', all(Path(x['copy']).stat().st_size == x['size'] == build['size'] and not Path(x['copy']).stat().st_mode & 0o077 for x in captures))
    trials, poll_observations, loaded_plugins, expected_paths = [], [], [], []
    for i in [1, 2]:
        label = f'RK1-H3a-t{i}'
        directory = SERIES / label
        public = read(HERE / 'results' / (label + '.json'))
        check(label + ':public_exact_copy', public == read(directory / 'public.json'))
        check(label + ':single_real_started_run', public['scenario'] == 'RK1-H3a' and public['trial'] == i and public['mode'] == 'real' and len(public['runs']) == 1)
        run = public['runs'][0]; rid = run['run_id']
        check(label + ':one_paid_launch_and_attempt', run['measured'] is True and run['physical_launches'] == run['authorized_attempts'] == 1)
        launch_files = list((directory / 'provider' / rid).glob('launch-*/launch.json'))
        check(label + ':one_retained_launch', len(launch_files) == 1)
        launch = read(launch_files[0]); plan = read(directory / 'provider' / (rid + '.json'))
        check(label + ':registered_real_provider', launch['mode'] == plan['mode'] == 'real' and Path(plan['real_executable']).resolve() == Path(invocation['provider_executable_target']))
        frame = read(launch_files[0].parent / 'stdin-first.jsonl')
        prompt = frame['message']['content'][0]['text']; prompt_hash = hashlib.sha256(prompt.encode()).hexdigest()
        envelope = json.loads(prompt[prompt.index('\n{') + 1:])['handoff']
        trace = [json.loads(l) for l in (directory / 'trace.jsonl').read_text().splitlines() if l.strip()]; sha(directory / 'trace.jsonl')
        check(label + ':trace_sequence', [x['seq'] for x in trace] == list(range(1, len(trace) + 1)))
        check(label + ':trace_before_public', all(stamp(x['at']) <= stamp(public['finished_at']) for x in trace))
        calls = [x for x in trace if f'/runs/{rid}/' in x['path']]
        def events(suffix):
            return [x for x in calls if x['path'].endswith(suffix)]
        claims, permits, binds = events('/claim'), events('/permit'), events('/handoff/bind')
        attempts = {(x['response']['attempt_seq'], x['response']['attempt_nonce']) for x in permits if x['status'] == 200}
        check(label + ':negotiated_claim', len(claims) == 1 and claims[0]['status'] == 200 and run['granted'] == {'handoff_version': 1, 'recording_version': 1, 'knowledge_context': False})
        check(label + ':two_permits_one_attempt', len(permits) == run['permit_calls'] == 2 and len(attempts) == 1)
        check(label + ':actual_bound_input', run['stdin_prompt_sha256'] == [prompt_hash] and envelope['run_id'] == rid and len(binds) == 1 and binds[0]['request']['prompt_sha256'] == prompt_hash and binds[0]['request']['snapshot_id'] == envelope['snapshot_id'] and (binds[0]['request']['attempt_seq'], binds[0]['request']['attempt_nonce']) in attempts)
        check(label + ':original_argv', run['argv'] == [launch['args']] and '--resume' not in launch['args'] and '--no-session-persistence' not in launch['args'] and launch['args'][launch['args'].index('--model') + 1] == 'claude-opus-5-5')
        check(label + ':evidence_publication_complete', launch['evidence_state'] == 'complete' and run['provider_evidence_states'] == ['complete'])
        stdout = (launch_files[0].parent / 'stdout.jsonl').read_text(); sha(launch_files[0].parent / 'stdout.jsonl')
        frames = [json.loads(l) for l in stdout.splitlines() if l.strip()]
        init = [f for f in frames if f.get('subtype') == 'init']; terminals = [f for f in frames if f.get('type') == 'result']
        check(label + ':observed_version_model', len(init) == 1 and init[0]['claude_code_version'] == public['provider_version'] == '2.1.318' and init[0]['model'] == 'claude-opus-5-5' and public['observed_models'] == ['claude-opus-5-5'])
        loaded_plugins.append(init[0]['plugins'])
        tools = [c for f in frames for c in f.get('message', {}).get('content', []) if c.get('type') == 'tool_result']
        cli = [x for x in calls if x.get('platform') == 'cli' and x['method'] == 'GET' and x['status'] == 200 and x['path'].endswith(('/context', '/comments'))]
        material = json.dumps([x['response'] for x in cli], ensure_ascii=False)
        newest = set(re.findall(r'DECISION UPDATE: the release code is now (REL-[A-Z0-9]{6})', material))
        older = set(re.findall(r'Release plan: the release code is (REL-[A-Z0-9]{6})', material))
        check(label + ':new_and_old_codes_received', len(newest) == len(older) == 1 and newest != older)
        new_code, old_code = next(iter(newest)), next(iter(older))
        visible = [c for c in tools if new_code in json.dumps(c.get('content'), ensure_ascii=False)]
        check(label + ':decisive_fact_tool_visible', len(visible) >= 1)
        check(label + ':initial_large_preview_not_full_material', tools[0]['content'].startswith('<persisted-output>') and new_code not in tools[0]['content'])
        contexts = events('/context')
        check(label + ':legacy_context200', len(contexts) == 1 and len(contexts[0]['response']['comments']) == 200)
        last_comment = contexts[0]['response']['comments'][-1]
        check(label + ':latest_decision_in_context', new_code in last_comment['content'] and last_comment['content'].startswith('DECISION UPDATE:'))
        outcomes, proofs, seals = events('/outcome'), events('/proof'), events('/recording/seal')
        check(label + ':outcome_ACK', len(outcomes) == 1 and outcomes[0]['status'] == 200)
        check(label + ':proof_settled', len(proofs) == 1 and proofs[0]['status'] == 200 and proofs[0]['response']['accepted'] is True and proofs[0]['request']['state'] == run['proof_state'] == 'host_exit_observed' and run['progress_state'] == 'settled')
        check(label + ':termination_received', len(events('/termination')) == 1 and events('/termination')[0]['status'] == 200)
        checkpoints = events('/checkpoints')
        check(label + ':recorded_checkpoints_ACK', len(checkpoints) == run['run_events']['record_checkpoint'] and len(checkpoints) > 0 and all(x['status'] == 200 for x in checkpoints))
        check(label + ':complete_recording_seal', len(seals) == 1 and seals[0]['status'] == 200 and seals[0]['request']['reason'] == 'completed' and seals[0]['request']['observed_through'] == seals[0]['request']['acknowledged_through'] == seals[0]['response']['retained_through'] == run['seal_observed_through'] and run['recording_incomplete'] is False and run['outcome_record_completeness'] == 'complete')
        ledgers = list((directory / 'ledger' / rid).glob('*.json'));check(label + ':private_ledger', len(ledgers) == 1 and not ledgers[0].stat().st_mode & 0o077)
        ledger = read(ledgers[0]);check(label + ':durable_terminal_ACKs', ledger['phase'] == 'proof_acknowledged' and ledger['host']['outcome_recorded'] is True and ledger['host']['recording']['seal_accepted'] is True)
        check(label + ':no_business_writes', run['business_writes'] == 0 and not any(x['method'] != 'GET' and x['path'].endswith(('/comments', '/memory', '/squad-activity')) for x in calls))
        errors = [x for x in calls if not 200 <= x['status'] < 300]
        poll_observations.extend({'trial': label, 'seq': x['seq'], 'at': x['at'], 'method': x['method'], 'path': x['path'], 'status': x['status']} for x in errors)
        check(label + ':only_unscored_poll_error', all(x['method'] == 'GET' and x['path'].endswith('/stop-intent') and x['status'] == 503 for x in errors))
        expected_cli = str(Path(launch['cwd']).parent / 'runs' / rid / 'bin/multica');expected_paths.append(expected_cli)
        copies = [c for c in captures if c['path'] == expected_cli]
        check(label + ':actual_CLI_copy', len(copies) == 1 and stamp(copies[0]['at']) <= stamp(launch['started_at']))
        hostlog = (directory / ('host-' + rid + '.log')).read_text();sha(directory / ('host-' + rid + '.log'))
        launch_seconds = (stamp(launch['finished_at']) - stamp(launch['started_at'])).total_seconds()
        if i == 1:
            check(label + ':finished_pass_original', public['status'] == 'finished' and public['verdict'] == 'pass' and launch['exit_code'] == 0 and len(terminals) == 1 and terminals[0]['is_error'] is False)
            check(label + ':correct_supported_newest_code', set(re.findall(r'REL-[A-Z0-9]{6}', public['answer'])) == {new_code} and old_code not in public['answer'] and terminals[0]['result'] == outcomes[0]['request']['body'] == public['answer'])
            check(label + ':public_scoring_projection', all(public['criteria'][k] is True for k in ['decisive_comment_returned_to_cli', 'decisive_comment_shown_to_model', 'newest_code', 'older_code_absent']))
            pages = events('/comments');check(label + ':bounded_comment_pages_observed', len(pages) == 9 and all(x['response']['total'] == 210 for x in pages))
        else:
            check(label + ':original_incomplete_retained', public['status'] == 'incomplete' and public['verdict'] == 'measured:provider_result_missing' and public['answer'] == '' and public['criteria'] == {})
            check(label + ':terminal_result_frame_missing', len(terminals) == 0 and run['provider_execution_exit_codes'] == [143] and launch['exit_code'] == run['wrapper_exit_code'] == 143)
            check(label + ':observed_host_timeout', 'status=timeout' in hostlog and 'duration=5m0.527s' in hostlog and 'saw_result=false' in hostlog and 'scanner_error=false' in hostlog and 300 <= launch_seconds < 301)
            check(label + ':failure_outcome_timeout_text', outcomes[0]['request']['outcome'] == run['business_outcome'] == 'failure' and outcomes[0]['request']['body'] == 'claude timed out after 5m0s')
            check(label + ':notHTTP_failure', not errors and len(cli) == 1 and len(tools) == 3)
        trials.append({'label': label, 'status': public['status'], 'verdict': public['verdict'], 'run_id': rid, 'launch_exit': launch['exit_code'], 'launch_seconds': launch_seconds, 'result_frames': len(terminals), 'CLI_context_or_comment_reads': len(cli), 'tool_results': len(tools), 'decisive_code_tool_results': len(visible), 'max_tool_result_chars': max(len(str(x['content'])) for x in tools), 'checkpoints': len(checkpoints), 'retained_recording_messages': seals[0]['response']['retained_through'], 'started_at': public['started_at'], 'finished_at': public['finished_at']})
    check('third_trial_never_started', not (SERIES / 'RK1-H3a-t3').exists() and not (HERE / 'results/RK1-H3a-t3.json').exists())
    check('no_subsequent_scenario_invocation', all(not list((ROOT / '.omx/private/rk1' / scenario).glob('invocation-*.json')) for scenario in ['RK1-H3b', 'RK1-H5', 'RK1-H6']))
    check('two_prepared_CLI_paths', set(expected_paths) == set(result['prepared_expected_paths']) == {x['path'] for x in captures if Path(x['path']).parent.name == 'bin'})
    budget = invocation['launch_budget']
    check('actual_paid_budget', result['public_measured_starts'] == 2 and result['public_seed_starts'] == 0 and budget['cap'] == 25 and budget['before']['consumed'] == 10 and budget['after']['consumed'] == 12 and budget['remaining'] == 13)
    check('public_private_count_consistency', budget['after'] == result['private_launch_accounting'] and budget['after']['public_per_trial'] == budget['after']['private_per_trial'] == budget['after']['counted_per_trial'])
    premanifest, postmanifest = read(PRIVATE / 'run-plugins-before.json'), read(PRIVATE / 'plugins-after.json')
    check('plugin_digests_exact', manifest_hash(premanifest) == premanifest['tree_sha256'] == before['operator_configuration']['~/.claude/plugins/'] and manifest_hash(postmanifest) == postmanifest['tree_sha256'] == after['operator_configuration']['~/.claude/plugins/'])
    changes = {k: {'before': premanifest['entries'].get(k), 'after': postmanifest['entries'].get(k)} for k in sorted(set(premanifest['entries']) | set(postmanifest['entries'])) if premanifest['entries'].get(k) != postmanifest['entries'].get(k)}
    store = 'store/cc-plugin-agents-md_builtin-b049b47b3618.json'
    check('only_usage_store_changed', changes == result['plugin_changes'] and set(changes) == {store})
    for manifest_name, manifest in [('before', premanifest), ('after', postmanifest)]:
        for name, item in manifest['selected_metadata_copies'].items():
            check('metadata_snapshot:' + manifest_name + ':' + name, sha(item['path']) == item['sha256'] == manifest['entries'][name]['value'] and Path(item['path']).stat().st_size == item['bytes'])
    store_before = read(premanifest['selected_metadata_copies'][store]['path']);store_after = read(postmanifest['selected_metadata_copies'][store]['path'])
    check('queued_usage_identifier_only', set(store_before) == set(store_after) == {'usage-rows-queued'} and store_before['usage-rows-queued'].split(' ', 1)[1] == store_after['usage-rows-queued'].split(' ', 1)[1] == 'claude-md-or-agents-md ok')
    check('all_other_selected_metadata_identical', all(premanifest['selected_metadata_copies'][k]['sha256'] == postmanifest['selected_metadata_copies'][k]['sha256'] for k in premanifest['selected_metadata_copies'] if k != store))
    check('loaded_plugin_lists_identical', loaded_plugins[0] == loaded_plugins[1])
    plugin_root = Path('/Users/artisan/.claude/plugins');active_roots = [str(Path(x['path']).relative_to(plugin_root)) for x in loaded_plugins[0] if x['path'] != 'builtin']
    active_paths = [k for k in premanifest['entries'] if any(k.startswith(x + '/') for x in active_roots)]
    check('loaded_plugin_code_unchanged', len(active_paths) == 1970 and all(premanifest['entries'][k] == postmanifest['entries'][k] for k in active_paths))
    check('installed_registry_unchanged', premanifest['entries']['installed_plugins.json'] == postmanifest['entries']['installed_plugins.json'])
    source_checker = TASK / 'research/continuation-20261102/rki6-review/verify_evidence.py';spec = importlib.util.spec_from_file_location('source_only', source_checker);module = importlib.util.module_from_spec(spec);spec.loader.exec_module(module);source = module.source(ROOT)
    check('source_framework_stable', source['head'] == before['head'] == after['head'] and source['source_id'] == before['source_id'] == after['source_id'] and source['framework_sha256'] == before['framework_sha256'] == after['framework_sha256'] and not source['product_dirty_paths'])
    check('nonoperator_bindings_equal', all(before[k] == after[k] for k in before if k not in ['whole_status', 'operator_configuration', 'operator_configuration_sha256']))
    allowed = {f'?? {HERE.relative_to(ROOT)}/results/RK1-H3a-t{i}.json' for i in [1, 2]};status = subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain=v1', '--untracked-files=all'], text=True)
    check('only_two_expected_public_files', set(status.splitlines()) == allowed)
    for key, path in [('helper_sha256', PRIVATE.parent / 'runtime_entry.py'), ('config_sha256', PRIVATE / 'config.json'), ('registration_sha256', HERE / 'framework-registration.json'), ('addendum_sha256', HERE / 'runtime-addendum.json'), ('launcher_sha256', HERE / 'run_rk1.py'), ('provider_sha256', invocation['provider_executable_target']), ('go_sha256', '/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go')]:
        check('retained_binding:' + key, sha(path) == before[key])
    prior = read(TASK / 'research/continuation-20261105/h2b-entry/independent-review.json')
    for path, saved_hash in prior['hashes'].items():
        if '/real-model-rk1/results/' in path:
            check('prior_result_unchanged:' + Path(path).stem, sha(path) == saved_hash)
    report = {
        'reviewed_at': datetime.now(timezone.utc).isoformat(), 'reviewer': '/root/acceptance_review',
        'scope': 'Verify original H3a pass, incomplete timeout and required stop; not a successful scenario acceptance',
        'evidence_integrity_verdict': 'pass' if all(x['passed'] for x in CHECKS) else 'changes-required',
        'scenario_verdict': 'STOP_ORIGINAL_INCOMPLETE', 'checks_passed': sum(x['passed'] for x in CHECKS), 'checks_failed': [x for x in CHECKS if not x['passed']],
        'trials': trials, 'trial3': 'NOT_STARTED', 'physical_starts': {'new_paid': 2, 'fake_seeds': 0, 'cumulative_paid': 12, 'remaining': 13},
        'actual_original_exit': 1, 'actual_helper_exit': 1, 'original_stop_triggered': True,
        't2_failure_boundary': 'Host enforces its existing five-minute Provider execution timeout; observed launch exit143 after300.440531seconds, stdout has no result frame, failureoutcome says claude timed out after5m0s. Checkpoint/termination/proof/seal6/6 completed.',
        't2_exposure': 'Decisive code reached context HTTP and later a recorded775-character tool result after the model read the persisted large output. Missing terminal result cannot be attributed simply to an unseen decision.',
        'cause_limits': 'These artifacts establish the local timeout boundary, not why the Provider failed to complete: no proved upstream429/503/TLS issue, network stall, model deliberation cause or product defect. Do not infer one from elapsed time.',
        'operator_delta': {'before': premanifest['tree_sha256'], 'after': postmanifest['tree_sha256'], 'changed_files': 1, 'added': 0, 'removed': 0, 'meaning': 'Only builtin usage-rows-queued identifier changes in saved bytes; fixedsuffix unchanged. OtherselectedJSON,installedregistry and1970loadedpluginleaves unchanged; no evidence this caused the timeout.'},
        'HTTP_poll_observations': poll_observations, 'source': source, 'CLI_sha256': build['cli_sha256'],
        'required_disposition': 'Retain t1pass/t2incomplete/t3unrun, both original exits1 and actual12consumed. Original stop forbids automatically running t3,H3b,H5,H6, replacing t2, extendingtimeout or addinghealthcalls. Read-only diagnosis or a separately scoped technical plan may follow; remaining13 is not permission to bypassstop.',
        'next_scenario_authorized': False, 'checks': CHECKS, 'hashes': HASHES,
        'operations': {'tests': 0, 'Go': 0, 'database': 0, 'model': 0, 'full_plugin_scan': 0},
    }
    with (OUT / 'independent-review.json').open('x') as output:
        json.dump(report, output, indent=2); output.write('\n')
    print(json.dumps({k: report[k] for k in ['evidence_integrity_verdict', 'scenario_verdict', 'checks_passed', 'checks_failed', 'physical_starts']}))


if __name__ == '__main__':
    main()
