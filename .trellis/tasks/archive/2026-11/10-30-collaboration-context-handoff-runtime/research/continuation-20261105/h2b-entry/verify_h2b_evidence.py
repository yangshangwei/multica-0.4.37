#!/usr/bin/env python3
"""Review retained H2b artifacts; no tests, database calls or model execution."""
from collections import Counter
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
from urllib.parse import parse_qs

MAIN = Path('/Volumes/artisan/code/2026/multica-0.4.37')
ROOT = MAIN.parent / 'mc-runtime-acceptance-20261104'
TASK = MAIN / '.trellis/tasks/10-30-collaboration-context-handoff-runtime'
OUT = TASK / 'research/continuation-20261105/h2b-entry'
HERE = ROOT / '.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1'
PRIVATE = ROOT / '.omx/private/runtime-successor-20261105/RK1-H2b-from-1'
SERIES = ROOT / '.omx/private/rk1/RK1-H2b'
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


def json_delta(before, after, path=()):
    if type(before) is not type(after):
        yield path, 'type', before, after
    elif isinstance(before, dict):
        for key in sorted(set(before) | set(after)):
            if key not in before:
                yield path + (key,), 'added', None, after[key]
            elif key not in after:
                yield path + (key,), 'removed', before[key], None
            else:
                yield from json_delta(before[key], after[key], path + (key,))
    elif isinstance(before, list):
        if len(before) != len(after):
            yield path, 'length', len(before), len(after)
        for i, (left, right) in enumerate(zip(before, after)):
            yield from json_delta(left, right, path + (str(i),))
    elif before != after:
        yield path, 'value', before, after


def main():
    result = read(PRIVATE / 'run-result.json')
    build, precheck = read(PRIVATE / 'build-result.json'), read(PRIVATE / 'check-result.json')
    permit, closed = read(PRIVATE / 'permit-used-run.json'), read(PRIVATE / 'permit.json')
    attempt, capture = read(PRIVATE / 'run-attempt.json'), read(PRIVATE / 'captures.json')
    inv_files = list(SERIES.glob('invocation-*.json'))
    check('one_original_invocation', len(inv_files) == 1)
    invocation = read(inv_files[0])
    check('invocation_hash', sha(inv_files[0]) == result['invocation_files'][0]['sha256'])
    before, after = result['binding_before'], result['binding_after']
    check('actual_original_and_helper_success', result['exit_code'] == invocation['exit_code'] == invocation['executor_exit_code'] == 0 and result['ready_for_independent_review'] is True)
    check('original_stop_false', invocation['stop_state'] == {'triggered': False, 'incomplete_or_blocking_trials': [], 'missing_requested_trials': [], 'unexpected_trials': []})
    check('original_command', invocation['command'] == ['go', 'test', '-tags=agentintegration', './cmd/server', '-run', '^TestRK1RealProvider$', '-count=1', '-v', '-timeout=26m'])
    check('same_admission_binding', before == build['binding_before'] == build['binding_after'] == precheck['binding'] == permit['binding'] == attempt['binding'])
    check('actual_build_check_success', build['exit_code'] == precheck['exit_code'] == 0)
    check('permit_artifact_hashes', sha(PRIVATE / 'build-result.json') == permit['build_result_sha256'] and sha(PRIVATE / 'check-result.json') == permit['check_result_sha256'] and sha(PRIVATE / 'permit-used-run.json') == attempt['permit_sha256'])
    check('within_window', stamp(invocation['finished_at']) < stamp(permit['window_expires_at']))
    check('closed_current_permits', all(closed[k] is False for k in ['allow_build', 'allow_check', 'allow_run']))
    check('no_automatic_next_or_retry', result['next_scenario_authorized'] is False and result['no_automatic_retry'] is True)
    captures = result['captures']
    check('ten_actual_captures', len(captures) == 10 and captures == capture['observations'] and not result['observer_errors'] and not capture['errors'])
    check('actual_CLI_hashes', sha(PRIVATE / 'upfront-multica') == build['cli_sha256'] and all(sha(c['copy']) == c['sha256'] == build['cli_sha256'] for c in captures))
    check('actual_CLI_sizes_and_privacy', all(Path(c['copy']).stat().st_size == c['size'] == build['size'] and not Path(c['copy']).stat().st_mode & 0o077 for c in captures))
    trial_reviews, expected_paths, init_plugins, poll_observations = [], [], [], []
    paid = seeds = 0
    for i in range(1, 4):
        label = f'RK1-H2b-t{i}'
        directory = SERIES / label
        public = read(HERE / 'results' / (label + '.json'))
        check(label + ':public_copy', public == read(directory / 'public.json'))
        check(label + ':exact_finished_real_trial', public['label'] == label and public['trial'] == i and public['scenario'] == 'RK1-H2b' and public['mode'] == 'real' and public['status'] == 'finished' and public['verdict'] == 'pass')
        trace = [json.loads(line) for line in (directory / 'trace.jsonl').read_text().splitlines() if line.strip()]
        sha(directory / 'trace.jsonl')
        check(label + ':trace_sequence', [x['seq'] for x in trace] == list(range(1, len(trace) + 1)))
        check(label + ':trace_published_after_completion', all(stamp(x['at']) <= stamp(public['finished_at']) for x in trace))
        roles = {run['role']: run for run in public['runs']}
        check(label + ':exact_roles', len(public['runs']) == 3 and set(roles) == {'seed_leader', 'seed_member', 'measured'})
        observations = {}
        for role, run in roles.items():
            prefix, rid = label + ':' + role, run['run_id']
            calls = [x for x in trace if f'/runs/{rid}/' in x['path']]
            def events(suffix):
                return [x for x in calls if x['path'].endswith(suffix)]
            paths = list((directory / 'provider' / rid).glob('launch-*/launch.json'))
            check(prefix + ':one_physical_and_attempt', len(paths) == run['physical_launches'] == run['authorized_attempts'] == 1)
            launch, plan = read(paths[0]), read(directory / 'provider' / (rid + '.json'))
            frame = read(paths[0].parent / 'stdin-first.jsonl')
            prompt = frame['message']['content'][0]['text']
            prompt_hash = hashlib.sha256(prompt.encode()).hexdigest()
            envelope = json.loads(prompt[prompt.index('\n{') + 1:])['handoff']
            claims, permits, binds = events('/claim'), events('/permit'), events('/handoff/bind')
            attempts = {(x['response']['attempt_seq'], x['response']['attempt_nonce']) for x in permits if x['status'] == 200}
            check(prefix + ':claim_negotiation', len(claims) == 1 and claims[0]['status'] == 200 and run['granted'] == {'handoff_version': 1, 'recording_version': 1, 'knowledge_context': False})
            check(prefix + ':two_permits_one_attempt', len(permits) == run['permit_calls'] == 2 and len(attempts) == 1)
            check(prefix + ':actual_input_hash_binding', run['stdin_prompt_sha256'] == [prompt_hash] and envelope['run_id'] == rid and len(binds) == 1 and binds[0]['status'] == 200 and binds[0]['request']['prompt_sha256'] == prompt_hash and binds[0]['request']['snapshot_id'] == envelope['snapshot_id'] and (binds[0]['request']['attempt_seq'], binds[0]['request']['attempt_nonce']) in attempts)
            check(prefix + ':mandatory_preparation', len(events('/memory/base')) == 1 and len(events('/handoff/snapshots')) == 1)
            check(prefix + ':argv_original_model_no_resume', run['argv'] == [launch['args']] and launch['args'][launch['args'].index('--model') + 1] == public['requested_model'] and '--resume' not in launch['args'] and '--no-session-persistence' not in launch['args'])
            check(prefix + ':complete_launch', launch['exit_code'] == 0 and launch['evidence_state'] == 'complete')
            frames = [json.loads(line) for line in (paths[0].parent / 'stdout.jsonl').read_text().splitlines() if line.strip()]
            sha(paths[0].parent / 'stdout.jsonl')
            terminals = [x for x in frames if x.get('type') == 'result']
            outcome = events('/outcome')
            check(prefix + ':raw_success_result_matches_outcome', len(terminals) == 1 and terminals[0]['is_error'] is False and len(outcome) == 1 and outcome[0]['status'] == 200 and outcome[0]['request']['outcome'] == 'success' and terminals[0]['result'] == outcome[0]['request']['body'])
            checkpoints, seals, proofs = events('/checkpoints'), events('/recording/seal'), events('/proof')
            check(prefix + ':checkpoints_retained', len(checkpoints) == run['run_events']['record_checkpoint'] and len(checkpoints) >= 1 and all(x['status'] == 200 for x in checkpoints))
            check(prefix + ':completed_recording_seal', len(seals) == 1 and seals[0]['status'] == 200 and seals[0]['request']['reason'] == 'completed' and seals[0]['request']['observed_through'] == seals[0]['request']['acknowledged_through'] == seals[0]['response']['retained_through'] == run['seal_observed_through'] == run['seal_acknowledged_through'] and seals[0]['response']['sealed'] is True)
            check(prefix + ':termination_proof_settlement', len(events('/termination')) == 1 and events('/termination')[0]['status'] == 200 and len(proofs) == 1 and proofs[0]['status'] == 200 and proofs[0]['response']['accepted'] is True and proofs[0]['request']['state'] == run['proof_state'] == 'host_exit_observed' and run['progress_state'] == 'settled')
            check(prefix + ':public_lifecycle_complete', run['business_outcome'] == 'success' and run['recording_incomplete'] is False and run['outcome_record_completeness'] == 'complete')
            ledgers = list((directory / 'ledger' / rid).glob('*.json'))
            check(prefix + ':private_ledger_retained', len(ledgers) == 1 and not ledgers[0].stat().st_mode & 0o077)
            ledger = read(ledgers[0])
            check(prefix + ':durable_ACKs', ledger['phase'] == 'proof_acknowledged' and ledger['host']['outcome_recorded'] is True and ledger['host']['recording']['seal_accepted'] is True)
            expected_cli = str(Path(launch['cwd']).parent / 'runs' / rid / 'bin/multica')
            expected_paths.append(expected_cli)
            copies = [c for c in captures if c['path'] == expected_cli]
            check(prefix + ':actual_CLI_path', len(copies) == 1 and stamp(copies[0]['at']) <= stamp(launch['started_at']))
            writes = [x for x in calls if x['method'] != 'GET' and x['path'].endswith(('/comments', '/memory', '/squad-activity'))]
            if role == 'measured':
                paid += run['physical_launches']
                check(prefix + ':real_provider', run['measured'] is True and plan['mode'] == launch['mode'] == 'real' and Path(plan['real_executable']).resolve() == Path(invocation['provider_executable_target']))
                initial = [x for x in frames if x.get('subtype') == 'init']
                check(prefix + ':observed_provider_model', len(initial) == 1 and initial[0]['model'] == 'claude-opus-5-5' and initial[0]['claude_code_version'] == public['provider_version'] == '2.1.318')
                init_plugins.append(initial[0]['plugins'])
                check(prefix + ':no_business_write', not writes and run['business_writes'] == 0)
                check(prefix + ':public_answer_raw', outcome[0]['request']['body'] == public['answer'])
            else:
                seeds += run['physical_launches']
                check(prefix + ':fake_seed', run['measured'] is False and plan['mode'] == launch['mode'] == 'fake' and plan['behavior'] == ('delegate' if role == 'seed_leader' else 'report'))
                check(prefix + ':seed_business_writes', len(writes) == run['business_writes'] == (1 if role == 'seed_leader' else 0))
            non_success = [x for x in calls if not 200 <= x['status'] < 300]
            poll_observations.extend({'trial': label, 'role': role, 'seq': x['seq'], 'at': x['at'],
                                      'method': x['method'], 'path': x['path'], 'status': x['status']}
                                     for x in non_success)
            check(prefix + ':non_success_is_only_terminal_poll', all(x['method'] == 'GET' and x['path'].endswith('/stop-intent') and x['status'] == 503 for x in non_success))
            check(prefix + ':no_recorded_runner_error', not run.get('runner_error') and not run.get('lifecycle_incomplete'))
            observations[role] = {'run': run, 'envelope': envelope, 'prompt': prompt, 'outcome': outcome[0]['request']['body'], 'calls': calls, 'plan': plan, 'writes': writes, 'frames': frames}
        leader, member, measured = (observations[k] for k in ['seed_leader', 'seed_member', 'measured'])
        mention = leader['writes'][0]
        member_id = member['run']['run_id']
        check(label + ':real_mention_creates_member', mention['path'].endswith('/comments') and mention['status'] == 201 and mention['request']['content'] == leader['plan']['params']['comment'] and 'mention://agent/' in mention['request']['content'] and mention['response']['run_ids'] == [member_id])
        assignment = member['envelope']['assignment']
        check(label + ':member_immutable_parent_assignment', assignment['role'] == 'member' and assignment['parent_run_id'] == leader['run']['run_id'] and bool(assignment['delegation_id']) and bool(assignment['source_id']))
        delegation = [x for x in member['envelope']['sources'] if x['id'] == assignment['source_id']]
        check(label + ':delegation_text_authority', len(delegation) == 1 and delegation[0]['kind'] == 'delegation' and delegation[0]['trust'] == 'admitted_assignment' and delegation[0]['content'] == mention['request']['content'])
        code = re.search(r'Final verification code: (MEMBER-[A-Z0-9]{6})', member['outcome']).group(1)
        decoy = re.search(r'Preliminary verification code (MEMBER-[A-Z0-9]{6})', member['outcome']).group(1)
        check(label + ':member_result_matches_seed', member['outcome'] == member['plan']['params']['answer'])
        source = [x for x in measured['envelope']['sources'] if x['kind'] == 'provider_result' and x['run_id'] == member_id]
        causes = measured['envelope']['wakeup_causes']
        check(label + ':continuation_envelope_shape', measured['envelope']['assignment']['role'] == 'continuation' and len(causes['items']) > 0 and len(source) == 1 and source[0]['truncated'] is True and source[0]['availability'] == 'available' and code not in source[0]['content'] and decoy in source[0]['content'])
        match = re.fullmatch(r'MEMBER_RUN ([0-9a-f-]{36})[ \t]*\r?\nMEMBER_CODE (MEMBER-[A-Z0-9]{6})[ \t]*\r?\nWOKEN_BY (\S+) ([0-9a-f-]{36})[ \t]*(?:\r?\n)?', public['answer'])
        check(label + ':independent_strict_answer', bool(match) and match[1] == member_id and match[2] == code and any(x['source_kind'] == match[3] and x['source_run_id'] == match[4] for x in causes['items']))
        check(label + ':no_other_member_code', set(re.findall(r'MEMBER-[A-Z0-9]{6}', public['answer'])) == {code} and decoy not in public['answer'])
        original_bytes = member['outcome'].encode()
        prefix_bytes = source[0]['content'].encode()
        check(label + ':decisive_code_beyond4096', len(original_bytes) > 4096 and len(prefix_bytes) == 4096 and original_bytes.startswith(prefix_bytes) and original_bytes.index(code.encode()) >= 4096 and code not in measured['prompt'])
        expected_sha = hashlib.sha256(original_bytes).hexdigest()
        source_calls = [x for x in measured['calls'] if x['method'] == 'GET' and x.get('platform') == 'cli' and x['status'] == 200 and x['path'].endswith('/handoff/sources/' + source[0]['id'])]
        check(label + ':actual_exact_source_read', len(source_calls) >= 1 and all(parse_qs(x['query']).get('source_run_id') == [member_id] and parse_qs(x['query']).get('sha256') == [source[0]['sha256']] and parse_qs(x['query']).get('version') == [source[0]['version']] for x in source_calls))
        returned = [x['response'] for x in source_calls if code in x['response']['source']['content']]
        check(label + ':full_original_content_returned', len(returned) >= 1 and any(x['source']['run_id'] == member_id and x['source']['sha256'] == source[0]['sha256'] == expected_sha and x['source']['content'] == member['outcome'] and x['total_bytes'] == len(original_bytes) and x['source']['truncated'] is False for x in returned))
        tool_uses = {c['id']: c for frame in measured['frames'] for c in frame.get('message', {}).get('content', []) if c.get('type') == 'tool_use'}
        tool_results = [c for frame in measured['frames'] for c in frame.get('message', {}).get('content', []) if c.get('type') == 'tool_result']
        shown = [c for c in tool_results if code in json.dumps(c.get('content'), ensure_ascii=False)]
        paired = [c for c in shown if c.get('tool_use_id') in tool_uses and 'multica collaboration handoff source' in json.dumps(tool_uses[c['tool_use_id']].get('input')) and source[0]['id'] in json.dumps(tool_uses[c['tool_use_id']].get('input'))]
        check(label + ':source_code_shown_in_matching_tool_result', len(paired) >= 1)
        check(label + ':source_not_answer_only', public['criteria']['source_read_through_cli'] is True and public['criteria']['full_code_shown_to_model'] is True)
        check(label + ':criteria_projection', all(public['criteria'][k] is True for k in ['format', 'member_run', 'member_code', 'wake_cause_recorded', 'decoy_absent']) and public['criteria']['member_result_truncated'] is True and public['criteria']['wakeup_completeness'] == causes['completeness'])
        reader_items = measured['envelope']['inputs']['items']
        check(label + ':observed_input_readers_include_three_runs', any(set(roles[r]['run_id'] for r in roles) <= set(x['reader_run_ids']) for x in reader_items))
        trial_reviews.append({'label': label, 'verdict': 'pass', 'member_run': member_id, 'continuation_run': measured['run']['run_id'], 'wake_kind': match[3], 'wake_run': match[4], 'source_truncated': True, 'member_result_bytes': len(original_bytes), 'startup_prefix_bytes': len(prefix_bytes), 'exact_source_reads': len(source_calls), 'matching_tool_results': len(paired), 'started_at': public['started_at'], 'finished_at': public['finished_at']})
    check('all_nine_prepared_CLI_copies', set(expected_paths) == set(result['prepared_expected_paths']) == {c['path'] for c in captures if Path(c['path']).parent.name == 'bin'})
    check('measured_seed_counts', paid == result['public_measured_starts'] == 3 and seeds == result['public_seed_starts'] == 6)
    budget = invocation['launch_budget']
    check('cumulative_budget', budget['cap'] == 25 and budget['reserved'] == 3 and budget['before']['consumed'] == 7 and budget['after']['consumed'] == 10 and budget['remaining'] == 15)
    check('public_private_accounting', budget['after'] == result['private_launch_accounting'] and budget['after']['public_per_trial'] == budget['after']['private_per_trial'] == budget['after']['counted_per_trial'])
    premanifest, postmanifest = read(PRIVATE / 'run-plugins-before.json'), read(PRIVATE / 'plugins-after.json')
    check('full_plugin_digests', manifest_hash(premanifest) == premanifest['tree_sha256'] == before['operator_configuration']['~/.claude/plugins/'] and manifest_hash(postmanifest) == postmanifest['tree_sha256'] == after['operator_configuration']['~/.claude/plugins/'])
    changes = {k: {'before': premanifest['entries'].get(k), 'after': postmanifest['entries'].get(k)} for k in sorted(set(premanifest['entries']) | set(postmanifest['entries'])) if premanifest['entries'].get(k) != postmanifest['entries'].get(k)}
    store = 'store/cc-plugin-agents-md_builtin-b049b47b3618.json'
    check('two_exact_plugin_changes', changes == result['plugin_changes'] and set(changes) == {'known_marketplaces.json', store} and all(x['before'] and x['after'] for x in changes.values()))
    metadata = {}
    for name in premanifest['selected_metadata_copies']:
        pairs = []
        for label, manifest in [('before', premanifest), ('after', postmanifest)]:
            record = manifest['selected_metadata_copies'][name]
            check('selected_copy:' + label + ':' + name, sha(record['path']) == record['sha256'] == manifest['entries'][name]['value'] and Path(record['path']).stat().st_size == record['bytes'] and not Path(record['path']).stat().st_mode & 0o077)
            pairs.append(read(record['path']))
        metadata[name] = pairs
    market_delta = list(json_delta(*metadata['known_marketplaces.json']))
    check('marketplace_timestamps_only', len(market_delta) == 2 and all(path[-1] == 'lastUpdated' and kind == 'value' for path, kind, _, _ in market_delta))
    check('directory_catalog_bytes_unchanged', premanifest['selected_metadata_copies']['plugin-directory-cache-v2.json']['sha256'] == postmanifest['selected_metadata_copies']['plugin-directory-cache-v2.json']['sha256'])
    store_before, store_after = metadata[store]
    check('store_queue_identifier_only', set(store_before) == set(store_after) == {'usage-rows-queued'} and store_before['usage-rows-queued'].split(' ', 1)[1] == store_after['usage-rows-queued'].split(' ', 1)[1] == 'claude-md-or-agents-md ok')
    check('remaining_selected_metadata_identical', all(pair[0] == pair[1] for name, pair in metadata.items() if name not in changes))
    check('all_reported_loaded_plugins_identical', init_plugins[0] == init_plugins[1] == init_plugins[2])
    plugin_root = Path('/Users/artisan/.claude/plugins')
    loaded_roots = [str(Path(x['path']).relative_to(plugin_root)) for x in init_plugins[0] if x['path'] != 'builtin']
    loaded_leaves = [k for k in premanifest['entries'] if any(k.startswith(prefix + '/') for prefix in loaded_roots)]
    check('all_loaded_plugin_code_unchanged', len(loaded_leaves) == 1970 and all(premanifest['entries'][k] == postmanifest['entries'][k] for k in loaded_leaves))
    registry_path = plugin_root / 'installed_plugins.json'
    registry_equal = premanifest['entries']['installed_plugins.json'] == postmanifest['entries']['installed_plugins.json'] and sha(registry_path) == premanifest['entries']['installed_plugins.json']['value']
    check('installed_registry_unchanged', registry_equal)
    registry = read(registry_path)
    check('operator_only_plugin_delta', [k for k in before['operator_configuration'] if before['operator_configuration'][k] != after['operator_configuration'].get(k)] == ['~/.claude/plugins/'])
    source_path = TASK / 'research/continuation-20261102/rki6-review/verify_evidence.py'
    spec = importlib.util.spec_from_file_location('source_only', source_path); source_module = importlib.util.module_from_spec(spec); spec.loader.exec_module(source_module)
    source = source_module.source(ROOT)
    check('source_framework_unchanged', source['head'] == before['head'] == after['head'] and source['source_id'] == before['source_id'] == after['source_id'] and source['framework_sha256'] == before['framework_sha256'] == after['framework_sha256'] and not source['product_dirty_paths'])
    check('immutable_binding_fields_equal', all(before[k] == after[k] for k in before if k not in ['whole_status', 'operator_configuration', 'operator_configuration_sha256']))
    allowed = {f'?? {HERE.relative_to(ROOT)}/results/RK1-H2b-t{i}.json' for i in range(1, 4)}
    status = subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain=v1', '--untracked-files=all'], text=True)
    check('only_expected_public_additions', set(status.splitlines()) == allowed)
    for key, path in [('helper_sha256', PRIVATE.parent / 'runtime_entry.py'), ('config_sha256', PRIVATE / 'config.json'), ('registration_sha256', HERE / 'framework-registration.json'), ('addendum_sha256', HERE / 'runtime-addendum.json'), ('launcher_sha256', HERE / 'run_rk1.py'), ('provider_sha256', invocation['provider_executable_target']), ('go_sha256', '/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go')]:
        check('retained_binding:' + key, sha(path) == before[key])
    prior_review = read(TASK / 'research/continuation-20261105/h2-entry/independent-review.json')
    for path, saved_hash in prior_review['hashes'].items():
        if '/real-model-rk1/results/' in path:
            check('prior_public_unchanged:' + Path(path).stem, sha(path) == saved_hash)
    report = {
        'reviewed_at': datetime.now(timezone.utc).isoformat(), 'reviewer': '/root/acceptance_review',
        'scope': 'Actual original H2b three-trial behavior/lifecycle, CLI artifacts and saved operator delta only',
        'verdict': 'pass-with-notes' if all(x['passed'] for x in CHECKS) else 'changes-required',
        'checks_passed': sum(x['passed'] for x in CHECKS), 'checks_failed': [x for x in CHECKS if not x['passed']],
        'trials': trial_reviews, 'source': source, 'CLI_sha256': build['cli_sha256'],
        'physical_starts': {'new_paid': paid, 'fake_seeds': seeds, 'cumulative_paid': 10, 'remaining': 15},
        'original_stop_triggered': False, 'helper_review_ready': result['ready_for_independent_review'],
        'HTTP_poll_observations': poll_observations,
        'poll_scope': 'Two non-success GET stop-intent observations are retained. Original lifecycle checks require the successful admitted writes/ACKs and no Runner error, not every polling response to be2xx; no timing root cause is asserted.',
        'operator_delta': {'before': premanifest['tree_sha256'], 'after': postmanifest['tree_sha256'], 'changed_files': 2, 'added': 0, 'removed': 0,
                           'marketplaces': 'Two lastUpdated timestamps only; source/install mappings unchanged.',
                           'directory_catalog': 'Exact saved bytes unchanged; no catalog commit/version/summary delta in this invocation.',
                           'builtin_store': 'usage-rows-queued UUID changed; identical fixed suffix; raw UUID omitted',
                           'loaded_plugin_reports_identical': True, 'loaded_plugin_leaves_unchanged': len(loaded_leaves), 'installed_registry_unchanged': True,
                           'assessment': 'Saved bytes show only two marketplace refresh timestamps and the builtin queued-usage identifier changed. Full manifests, installed selection, directory catalog and loaded plugin code otherwise unchanged. Preserve full digest difference; rebind before next invocation.',
                           'limits': 'No writer attribution or continuous/intermediate-loaded-byte guarantee; full tree is changed, not stable.'},
        'coverage_limits': ['H2b proves decisive code fetched beyond truncated startup prefix and exposed through matching Provider tool result.', 'No complete reader-matrix or results/causes pagination beyond32 claim.', 'Seeded delegation/member processes are fake, measured continuations are real.'],
        'recommendation': 'Accept original H2b three-of-three. Root may disposition the specific saved timestamp/usage delta; before H3a require full current operator rebind, exact committed source/CLI/DB and new model window. No automatic next scenario.',
        'next_scenario_authorized': False, 'checks': CHECKS, 'hashes': HASHES,
        'operations': {'tests': 0, 'Go': 0, 'database': 0, 'model': 0, 'full_plugin_scan': 0},
    }
    with (OUT / 'independent-review.json').open('x') as output:
        json.dump(report, output, indent=2); output.write('\n')
    print(json.dumps({k: report[k] for k in ['verdict', 'checks_passed', 'checks_failed', 'physical_starts']}))


if __name__ == '__main__':
    main()
