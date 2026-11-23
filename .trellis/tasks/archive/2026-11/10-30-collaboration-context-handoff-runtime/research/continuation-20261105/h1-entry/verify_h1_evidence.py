#!/usr/bin/env python3
"""Read actual H1 artifacts and saved operator manifests; never rerun a sample."""
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess

MAIN = Path('/Volumes/artisan/code/2026/multica-0.4.37')
ROOT = MAIN.parent / 'mc-runtime-acceptance-20261104'
TASK = MAIN / '.trellis/tasks/10-30-collaboration-context-handoff-runtime'
OUT = TASK / 'research/continuation-20261105/h1-entry'
HERE = ROOT / '.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1'
PRIVATE = ROOT / '.omx/private/runtime-successor-20261105/RK1-H1-from-1'
SERIES = ROOT / '.omx/private/rk1/RK1-H1'
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
    inv_files = list(SERIES.glob('invocation-*.json'))
    check('one_original_invocation', len(inv_files) == 1)
    invocation = read(inv_files[0])
    check('retained_invocation_hash', sha(inv_files[0]) == result['invocation_files'][0]['sha256'])
    before, after = result['binding_before'], result['binding_after']
    check('actual_exit_zero', result['exit_code'] == invocation['exit_code'] == invocation['executor_exit_code'] == 0)
    check('original_stop_false', invocation['stop_state'] == {'triggered': False, 'incomplete_or_blocking_trials': [], 'missing_requested_trials': [], 'unexpected_trials': []})
    check('original_command_timeout', invocation['command'] == ['go', 'test', '-tags=agentintegration', './cmd/server', '-run', '^TestRK1RealProvider$', '-count=1', '-v', '-timeout=26m'])
    check('all_admission_bindings_match', before == build['binding_before'] == build['binding_after'] == precheck['binding'] == permit['binding'] == attempt['binding'])
    check('actual_build_check_pass', build['exit_code'] == precheck['exit_code'] == 0)
    check('permit_bound_artifacts', sha(PRIVATE / 'build-result.json') == permit['build_result_sha256'] and sha(PRIVATE / 'check-result.json') == permit['check_result_sha256'])
    check('run_permit_identity', sha(PRIVATE / 'permit-used-run.json') == attempt['permit_sha256'] and permit['allow_run'] is True)
    check('window_actual_finish', stamp(invocation['finished_at']) < stamp(permit['window_expires_at']))
    check('post_permissions_closed', all(closed[k] is False for k in ['allow_build', 'allow_check', 'allow_run']))
    check('no_next_or_retry', result['next_scenario_authorized'] is False and result['no_automatic_retry'] is True)
    captures = result['captures']
    check('exact_capture_count', len(captures) == 7 and not result['observer_errors'])
    check('all_actual_CLI_bytes', sha(PRIVATE / 'upfront-multica') == build['cli_sha256'] and all(sha(c['copy']) == c['sha256'] == build['cli_sha256'] for c in captures))
    check('actual_CLI_sizes_private', all(Path(c['copy']).stat().st_size == c['size'] == build['size'] and not Path(c['copy']).stat().st_mode & 0o077 for c in captures))
    expected_cli, trial_reviews, init_plugins = [], [], []
    paid = seeds = 0
    for i in range(1, 4):
        label = f'RK1-H1-t{i}'
        directory = SERIES / label
        public = read(HERE / 'results' / (label + '.json'))
        check(label + ':public_copy', public == read(directory / 'public.json'))
        check(label + ':exact_trial', public['label'] == label and public['scenario'] == 'RK1-H1' and public['trial'] == i and public['mode'] == 'real')
        check(label + ':finished_pass', public['status'] == 'finished' and public['verdict'] == 'pass')
        trace = [json.loads(line) for line in (directory / 'trace.jsonl').read_text().splitlines() if line.strip()]
        sha(directory / 'trace.jsonl')
        check(label + ':trace_sequence', [e['seq'] for e in trace] == list(range(1, len(trace) + 1)))
        check(label + ':trace_before_public', all(stamp(e['at']) <= stamp(public['finished_at']) for e in trace))
        roles = {r['role']: r for r in public['runs']}
        check(label + ':two_exact_roles', len(public['runs']) == 2 and set(roles) == {'seed_stopped', 'measured'})
        seed, measured = roles['seed_stopped'], roles['measured']
        seed_plan = read(directory / 'provider' / (seed['run_id'] + '.json'))
        decoy = seed_plan['params']['decoy']
        measured_prompt = None
        for role, run in roles.items():
            prefix = label + ':' + role
            rid = run['run_id']
            calls = [e for e in trace if f'/runs/{rid}/' in e['path']]
            def events(suffix):
                return [e for e in calls if e['path'].endswith(suffix)]
            launch_files = list((directory / 'provider' / rid).glob('launch-*/launch.json'))
            check(prefix + ':one_launch', len(launch_files) == run['physical_launches'] == run['authorized_attempts'] == 1)
            launch = read(launch_files[0])
            plan = read(directory / 'provider' / (rid + '.json'))
            stdin = read(launch_files[0].parent / 'stdin-first.jsonl')
            prompt = stdin['message']['content'][0]['text']
            prompt_hash = hashlib.sha256(prompt.encode()).hexdigest()
            envelope = json.loads(prompt[prompt.index('\n{') + 1:])['handoff']
            binds, permits = events('/handoff/bind'), events('/permit')
            attempts = {(x['response']['attempt_seq'], x['response']['attempt_nonce']) for x in permits if x['status'] == 200}
            check(prefix + ':prompt_hash_public', run['stdin_prompt_sha256'] == [prompt_hash])
            check(prefix + ':actual_prompt_binding', len(binds) == 1 and binds[0]['status'] == 200 and binds[0]['request']['prompt_sha256'] == prompt_hash and binds[0]['request']['snapshot_id'] == envelope['snapshot_id'] and envelope['run_id'] == rid and (binds[0]['request']['attempt_seq'], binds[0]['request']['attempt_nonce']) in attempts)
            check(prefix + ':one_attempt_two_permits', len(attempts) == 1 and len(permits) == run['permit_calls'] == 2)
            claims = events('/claim')
            check(prefix + ':negotiated_claim', len(claims) == 1 and claims[0]['status'] == 200 and run['granted'] == {'handoff_version': 1, 'recording_version': 1, 'knowledge_context': False})
            check(prefix + ':bound_argv_no_resume', run['argv'] == [launch['args']] and '--resume' not in launch['args'] and '--no-session-persistence' not in launch['args'] and launch['args'][launch['args'].index('--model') + 1] == public['requested_model'])
            frames = [json.loads(line) for line in (launch_files[0].parent / 'stdout.jsonl').read_text().splitlines() if line.strip()]
            sha(launch_files[0].parent / 'stdout.jsonl')
            checkpoints = [x for x in events('/checkpoints') if x['status'] == 200]
            check(prefix + ':retained_checkpoint', len(checkpoints) == run['run_events']['record_checkpoint'] == 1)
            check(prefix + ':termination', len([x for x in events('/termination') if x['status'] == 200]) == 1)
            proofs, seals = events('/proof'), events('/recording/seal')
            check(prefix + ':proof_ACK', len(proofs) == 1 and proofs[0]['status'] == 200 and proofs[0]['response']['accepted'] is True and proofs[0]['request']['state'] == 'host_exit_observed' and run['proof_state'] == 'host_exit_observed' and run['progress_state'] == 'settled')
            check(prefix + ':seal_ACK', len(seals) == 1 and seals[0]['status'] == 200 and seals[0]['response']['sealed'] is True)
            ledger_files = list((directory / 'ledger' / rid).glob('*.json'))
            check(prefix + ':one_private_ledger', len(ledger_files) == 1 and not ledger_files[0].stat().st_mode & 0o077)
            ledger = read(ledger_files[0])
            check(prefix + ':durable_proof_seal', ledger['phase'] == 'proof_acknowledged' and ledger['host']['recording']['seal_accepted'] is True and len(ledger['host']['launches']) == 1)
            path = str(Path(launch['cwd']).parent / 'runs' / rid / 'bin/multica')
            expected_cli.append(path)
            copies = [c for c in captures if c['path'] == path]
            check(prefix + ':actual_CLI_path', len(copies) == 1 and stamp(copies[0]['at']) <= stamp(launch['started_at']))
            check(prefix + ':no_business_writes', run['business_writes'] == 0 and not any(e['method'] != 'GET' and e['path'].endswith(('/comments', '/memory', '/squad-activity')) for e in calls))
            if role == 'seed_stopped':
                seeds += run['physical_launches']
                check(prefix + ':fake_stopped_seed', run['measured'] is False and plan['mode'] == launch['mode'] == 'fake' and plan['behavior'] == 'block' and launch['evidence_state'] == 'started' and run['wrapper_exit_code'] == -1)
                check(prefix + ':no_successful_outcome', not any(x['status'] == 200 for x in events('/outcome')) and run['business_outcome'] == '' and not ledger['host'].get('outcome_recorded'))
                check(prefix + ':explicit_incomplete_prefix', run['recording_incomplete'] is True and seals[0]['request']['reason'] == 'authority_lost' and seals[0]['request'].get('observed_through') is None and seals[0]['request']['acknowledged_through'] == seals[0]['response']['retained_through'] == 16)
                check(prefix + ':late_content_refused', all(e['status'] == 200 or e['status'] == 403 and e['path'].endswith(('/checkpoints', '/outcome')) for e in calls))
            else:
                paid += run['physical_launches']
                measured_prompt = prompt
                measured_envelope = envelope
                init = [f for f in frames if f.get('subtype') == 'init']
                terminal = [f for f in frames if f.get('type') == 'result']
                check(prefix + ':real_complete_provider', run['measured'] is True and plan['mode'] == launch['mode'] == 'real' and Path(plan['real_executable']).resolve() == Path(invocation['provider_executable_target']) and launch['exit_code'] == 0 and launch['evidence_state'] == 'complete')
                check(prefix + ':observed_model_version', len(init) == 1 and init[0]['model'] == public['requested_model'] == 'claude-opus-5-5' and init[0]['claude_code_version'] == public['provider_version'] == '2.1.318')
                init_plugins.append(init[0]['plugins'])
                check(prefix + ':raw_answer_exact', len(terminal) == 1 and terminal[0]['is_error'] is False and terminal[0]['result'] == public['answer'])
                outcomes = events('/outcome')
                check(prefix + ':success_outcome_ACK', len(outcomes) == 1 and outcomes[0]['status'] == 200 and outcomes[0]['request']['body'] == public['answer'] and outcomes[0]['request']['outcome'] == 'success' and ledger['host']['outcome_recorded'] is True)
                check(prefix + ':recording_completed', run['recording_incomplete'] is False and run['outcome_record_completeness'] == 'complete' and seals[0]['request']['reason'] == 'completed' and seals[0]['request']['observed_through'] == seals[0]['request']['acknowledged_through'] == seals[0]['response']['retained_through'] == 1)
                check(prefix + ':runtime_requests_success', all(x['status'] == 200 for x in calls))
                check(prefix + ':prompt_only_material', run['cli_reads'] == 0 and run['provider_tool_results'] == 0)
        prior = [x for x in measured_envelope['results']['items'] if x['run_id'] == seed['run_id']]
        check(label + ':prior_fact_exposed', len(prior) == 1 and prior[0]['recording_incomplete'] is True and prior[0]['result_state'] == 'absent')
        expected_answer = f"PRIOR_RUN {seed['run_id']} RECORD INCOMPLETE RESULT ABSENT\nRESULT_TEXT NONE RECORDED"
        check(label + ':independent_semantic_answer', public['answer'] == expected_answer)
        check(label + ':decoy_not_exposed_or_invented', decoy not in measured_prompt and decoy not in public['answer'] and public['criteria']['decoy_exposed_to_run'] is False)
        check(label + ':public_criteria_consistent', all(public['criteria'][k] is True for k in ['format', 'run_id', 'record_incomplete', 'result_absent', 'no_invented_text', 'decoy_absent']))
        trial_reviews.append({'label': label, 'status': public['status'], 'verdict': public['verdict'], 'seed_run': seed['run_id'], 'measured_run': measured['run_id'], 'started_at': public['started_at'], 'finished_at': public['finished_at']})
    check('all_six_prepared_CLI_paths', set(expected_cli) == set(result['prepared_expected_paths']) == {c['path'] for c in captures if Path(c['path']).parent.name == 'bin'})
    check('paid_seed_separate', paid == result['public_measured_starts'] == 3 and seeds == result['public_seed_starts'] == 3)
    budget = invocation['launch_budget']
    check('original_cumulative_budget', budget['cap'] == 25 and budget['reserved'] == 3 and budget['before']['consumed'] == 1 and budget['after']['consumed'] == 4 and budget['remaining'] == 21)
    check('public_private_accounting_agree', budget['after'] == result['private_launch_accounting'] and budget['after']['public_per_trial'] == budget['after']['private_per_trial'] == budget['after']['counted_per_trial'])

    premanifest, postmanifest = read(PRIVATE / 'run-plugins-before.json'), read(PRIVATE / 'plugins-after.json')
    check('pre_manifest_original_digest', manifest_hash(premanifest) == premanifest['tree_sha256'] == before['operator_configuration']['~/.claude/plugins/'])
    check('post_manifest_original_digest', manifest_hash(postmanifest) == postmanifest['tree_sha256'] == after['operator_configuration']['~/.claude/plugins/'])
    changes = {k: {'before': premanifest['entries'].get(k), 'after': postmanifest['entries'].get(k)} for k in sorted(set(premanifest['entries']) | set(postmanifest['entries'])) if premanifest['entries'].get(k) != postmanifest['entries'].get(k)}
    check('exact_plugin_delta', changes == result['plugin_changes'] and len(changes) == 120)
    removed = {k for k, v in changes.items() if v['after'] is None}
    modified = {k for k, v in changes.items() if v['after'] is not None}
    check('delta_shape', len(removed) == 118 and modified == {'.last_inuse_sweep', 'store/cc-plugin-agents-md_builtin-b049b47b3618.json'})
    for manifest in [premanifest, postmanifest]:
        for name, copy in manifest['selected_metadata_copies'].items():
            check('metadata_copy:' + ('before:' if manifest is premanifest else 'after:') + name, sha(copy['path']) == copy['sha256'] == manifest['entries'][name]['value'] and Path(copy['path']).stat().st_size == copy['bytes'] and not Path(copy['path']).stat().st_mode & 0o077)
    selected = premanifest['selected_metadata_copies']
    store_name = 'store/cc-plugin-agents-md_builtin-b049b47b3618.json'
    store_before = read(selected[store_name]['path'])
    store_after = read(postmanifest['selected_metadata_copies'][store_name]['path'])
    check('usage_row_exact_change', set(store_before) == set(store_after) == {'usage-rows-queued'} and store_before['usage-rows-queued'].split(' ', 1)[1] == store_after['usage-rows-queued'].split(' ', 1)[1] == 'claude-md-or-agents-md ok')
    check('other_selected_metadata_unchanged', all(selected[k]['sha256'] == postmanifest['selected_metadata_copies'][k]['sha256'] for k in selected if k != store_name))
    check('same_reported_loaded_plugins_all_trials', init_plugins[0] == init_plugins[1] == init_plugins[2])
    plugin_root = Path('/Users/artisan/.claude/plugins')
    registry_path = plugin_root / 'installed_plugins.json'
    check('installed_registry_unchanged_and_readback_exact', premanifest['entries']['installed_plugins.json'] == postmanifest['entries']['installed_plugins.json'] and sha(registry_path) == premanifest['entries']['installed_plugins.json']['value'])
    registry = read(registry_path)
    removed_roots = sorted({'/'.join(k.split('/')[:4]) for k in removed if k.startswith('cache/claude-plugins-official/')})
    def strings(value):
        if isinstance(value, str):
            yield value
        elif isinstance(value, dict):
            for item in value.values():
                yield from strings(item)
        elif isinstance(value, list):
            for item in value:
                yield from strings(item)
    check('removed_versions_not_installed_references', not any(root in value for root in removed_roots for value in strings(registry)))
    active_roots = [str(Path(p['path']).relative_to(plugin_root)) for p in init_plugins[0] if p['path'] != 'builtin']
    active_paths = [k for k in premanifest['entries'] if any(k.startswith(prefix + '/') for prefix in active_roots)]
    check('loaded_plugin_files_unchanged', len(active_paths) == 1970 and all(premanifest['entries'][k] == postmanifest['entries'].get(k) for k in active_paths))
    check('no_deleted_loaded_plugin_path', not any(k.startswith(prefix + '/') for k in removed for prefix in active_roots))
    sweep = plugin_root / '.last_inuse_sweep'
    check('sweep_after_exact_readback', sha(sweep) == postmanifest['entries']['.last_inuse_sweep']['value'])
    sweep_time = sweep.read_text().strip()
    check('sweep_value_is_trial_time', stamp(trial_reviews[0]['started_at']) <= stamp(sweep_time) <= stamp(trial_reviews[-1]['finished_at']))

    source_module = TASK / 'research/continuation-20261102/rki6-review/verify_evidence.py'
    spec = importlib.util.spec_from_file_location('source_only_checker', source_module)
    module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
    source = module.source(ROOT)
    check('current_source_unchanged', source['head'] == before['head'] == after['head'] and source['source_id'] == before['source_id'] == after['source_id'] and source['framework_sha256'] == before['framework_sha256'] == after['framework_sha256'] and not source['product_dirty_paths'])
    check('only_operator_and_public_status_changed', all(before[k] == after[k] for k in before if k not in ['whole_status', 'operator_configuration', 'operator_configuration_sha256']))
    allowed = {f'?? {HERE.relative_to(ROOT)}/results/RK1-H1-t{i}.json' for i in range(1, 4)}
    status = subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain=v1', '--untracked-files=all'], text=True)
    check('only_expected_new_results', set(status.splitlines()) == allowed)
    for key, path in [('helper_sha256', PRIVATE.parent / 'runtime_entry.py'), ('config_sha256', PRIVATE / 'config.json'), ('registration_sha256', HERE / 'framework-registration.json'), ('addendum_sha256', HERE / 'runtime-addendum.json'), ('launcher_sha256', HERE / 'run_rk1.py'), ('provider_sha256', invocation['provider_executable_target']), ('go_sha256', '/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go')]:
        check('current_binding:' + key, sha(path) == before[key])
    check('prior_E0a_unchanged', sha(HERE / 'results/RK1-E0a-t1.json') == '5b0b221f0401db1424494864778e173daf9b229cdc007255467200c1a68db0ba')
    check('drift_requires_review', result['operator_drift_requires_root_review'] is True)
    report = {
        'reviewed_at': datetime.now(timezone.utc).isoformat(), 'reviewer': '/root/acceptance_review',
        'verdict': 'pass-with-notes' if all(c['passed'] for c in CHECKS) else 'changes-required',
        'scope': 'Three actual H1 trials and separate saved operator delta; no H2 authorization',
        'checks_passed': sum(c['passed'] for c in CHECKS), 'checks_failed': [c for c in CHECKS if not c['passed']],
        'trial_reviews': trial_reviews, 'source': source, 'CLI_sha256': build['cli_sha256'],
        'physical_starts': {'new_paid': paid, 'fake_seed': seeds, 'cumulative_paid': 4, 'remaining': 21},
        'observed_provider': '2.1.318', 'requested_and_observed_model': 'claude-opus-5-5',
        'original_stop_triggered': False, 'helper_review_ready': result['ready_for_independent_review'],
        'operator_delta': {
            'before': premanifest['tree_sha256'], 'after': postmanifest['tree_sha256'],
            'before_leaves': len(premanifest['entries']), 'after_leaves': len(postmanifest['entries']),
            'removed': len(removed), 'changed': len(modified), 'added': 0,
            'removed_plugin_version_roots': removed_roots,
            'temporary_git_metadata_removed': sum(k.startswith('cache/temp_git_') for k in removed),
            'usage_store_change': 'Only usage-rows-queued identifier changes; identical claude-md-or-agents-md ok suffix. Raw private UUID values omitted.',
            'sweep_after_value': sweep_time,
            'installed_registry_unchanged': True, 'removed_versions_referenced_by_registry': False,
            'all_three_reported_loaded_plugin_lists_equal': True, 'loaded_plugin_file_leaves_unchanged': len(active_paths),
            'interpretation': 'Evidence is consistent with removal of unreferenced cached versions, temporary Git residue and usage bookkeeping. Some removed bytes are real hooks/commands, so the assessment relies on registry and actual init references, not cache path names.',
            'limits': 'Before/after manifests do not identify the writer or continuously prove intermediate loaded bytes; only selected metadata contents were backed up. No whole-tree-stability claim.',
        },
        'recommendation': 'Accept original H1 behavior/lifecycle3of3. Root may disposition this specific documented drift without treating it as a proven H1 environment failure. Before H2, review/commit current full operator rebind and exact source/CLI/DB/window; no automatic continuation.',
        'next_scenario_authorized': False, 'checks': CHECKS, 'hashes': HASHES,
        'operations': {'tests': 0, 'database_writes': 0, 'database_reads': 0, 'model_calls': 0, 'plugin_tree_scan': 0},
    }
    with (OUT / 'independent-review.json').open('x') as output:
        json.dump(report, output, indent=2); output.write('\n')
    print(json.dumps({k: report[k] for k in ['verdict', 'checks_passed', 'checks_failed', 'physical_starts']}))


if __name__ == '__main__':
    main()
