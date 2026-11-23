"""Inspect completed v14 phases without starting tests or altering old reviews."""

from pathlib import Path
import collections
import datetime
import hashlib
import json
import os
import re
import stat
import subprocess

ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
CANDIDATE = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = CANDIDATE / '.omx/private/final-check-v14'
OUT = Path(__file__).resolve().parent
TASK = ROOT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes'
hashes = {}
old_names = ['review.json', 'review.md', 'recheck.py']


def digest(path):
    path = Path(path)
    value = hashlib.sha256(path.read_bytes()).hexdigest()
    hashes[str(path)] = value
    return value


def read(path):
    digest(path)
    return json.loads(Path(path).read_text())


def write_new(name, value, raw=False):
    path = OUT / name
    with path.open('xb') as output:
        output.write(value if raw else (json.dumps(value, ensure_ascii=False, indent=2) + '\n').encode())
    digest(path)
    return str(path)


old_hashes = {name: digest(OUT / name) for name in old_names}
original_review = read(OUT / 'review.json')
run = read(PRIVATE / 'make-check-v14.json')
run_snapshot = write_new('v14-run-at-phase-review.json', run)
binding = read(PRIVATE / 'owned-bindings-v14-provenance.json')
clearance = read(PRIVATE / 'independent-review-clearance-v14.json')
claim = read(PRIVATE / 'execution-claim.json')
log_path = PRIVATE / 'make-check-v14.log'
log_bytes = log_path.read_bytes()
log_snapshot_sha = hashlib.sha256(log_bytes).hexdigest()
headers = list(re.finditer(rb'^==> \[([1-6])/6\].*$', log_bytes, re.M))
sections = {}
for index, match in enumerate(headers):
    number = int(match[1])
    end = headers[index+1].start() if index+1 < len(headers) else len(log_bytes)
    sections[number] = log_bytes[match.start():end]
assert all(number in sections for number in (1, 2, 3, 4))
raw_files = {number: write_new('v14-phase-' + str(number) + '.raw.log', sections[number], raw=True) for number in (1, 2, 3)}
clean = {number: re.sub(r'\x1b\[[0-9;]*m', '', data.decode(errors='replace')) for number, data in sections.items()}

tracked_mismatches = []
mode_mismatches = []
for expected in binding['source']['tracked_files']:
    path = CANDIDATE / expected['path']
    actual = hashlib.sha256(path.read_bytes()).hexdigest()
    if actual != expected['sha256']:
        tracked_mismatches.append({'path': expected['path'], 'expected': expected['sha256'], 'actual': actual})
    if stat.S_IMODE(path.stat().st_mode) != expected['mode']:
        mode_mismatches.append(expected['path'])
head = subprocess.check_output(['git', '-C', str(CANDIDATE), 'rev-parse', 'HEAD'], text=True).strip()
next_path = CANDIDATE / 'apps/web/next-env.d.ts'
before = subprocess.check_output(['git', '-C', str(CANDIDATE), 'show', head + ':apps/web/next-env.d.ts'])
after = next_path.read_bytes()
allowed_next = before.count(b'"./.next/dev/types/routes.d.ts"') == 1 and after.count(b'"./.next/types/routes.d.ts"') == 1 and after.replace(b'"./.next/types/routes.d.ts"', b'"./.next/dev/types/routes.d.ts"') == before
if not allowed_next:
    allowed_next = before.count(b'".next/dev/types/routes.d.ts"') == 1 and after.count(b'".next/types/routes.d.ts"') == 1 and after.replace(b'".next/types/routes.d.ts"', b'".next/dev/types/routes.d.ts"') == before
unexpected_source_changes = [row for row in tracked_mismatches if row['path'] != 'apps/web/next-env.d.ts' or not allowed_next]
tool_checks = []
for name, tool in binding['tools'].items():
    row = {'name': name, 'path': tool['path'], 'sha256': digest(tool['path']), 'matches': digest(tool['path']) == tool['sha256']}
    if name == 'go':
        row['sdk_members'] = {member: {'sha256': digest(Path(tool['goroot']) / member), 'matches': digest(Path(tool['goroot']) / member) == expected} for member, expected in tool['members'].items()}
    if name == 'pnpm' and 'bundle_path' in tool:
        row['bundle_sha256'] = digest(tool['bundle_path'])
        row['bundle_matches'] = row['bundle_sha256'] == tool['bundle_sha256']
    tool_checks.append(row)
binding_checks = {
    'candidate': head == run['candidate_sha'] == binding['candidate_sha'] == clearance['candidate_sha'] == claim['candidate_sha'],
    'binding_digest': digest(PRIVATE / 'owned-bindings-v14-provenance.json') == run['owned_bindings_sha256'] == clearance['bindings_sha256'],
    'clearance_digest': digest(PRIVATE / 'independent-review-clearance-v14.json') == claim['clearance_sha256'],
    'launcher': digest(PRIVATE / 'run-make-check-v14-provenance.py') == run['launcher_sha256'] == clearance['launcher_sha256'] == claim['launcher_sha256'],
    'go_adapter': digest(PRIVATE / 'check-bin/go') == run['adapter_sha256']['go'] == clearance['adapter_sha256']['go'],
    'pnpm_adapter': digest(PRIVATE / 'check-bin/pnpm') == run['adapter_sha256']['pnpm'] == clearance['adapter_sha256']['pnpm'],
    'source_env': digest(PRIVATE / 'check-source-v14.env') == run['source_env_sha256'] == clearance['source_env_sha256'],
    'development_id': run['dev_environment_source_id'] == binding['source']['checkout_source_id'],
    'canonical_product_id': run['canonical_product_source_id'] == binding['source']['product_source']['source_id'],
    'resource_record_digest': digest(PRIVATE / 'resource-ownership-v14.json') == run['resource_ownership_sha256'] == clearance['resource_ownership_sha256'],
}

static_rows = []
for line in clean[1].splitlines():
    match = re.match(r'^@multica/([^:]+):(lint|typecheck|mdx): cache bypass, force executing (\S+)$', line)
    if match:
        static_rows.append({'package': '@multica/' + match[1], 'task': match[2], 'turbo_hash': match[3], 'raw': line, 'terminal_evidence': 'All 15 actual tasks completed successful in this same forced run; not per-test leaves.'})
static_summary = 'Tasks:    15 successful, 15 total' in clean[1]
digest(CANDIDATE / 'scripts/check.sh')
static_extra = {'command': 'pnpm check:ui-exports', 'evidence': 'scripts/check.sh runs this after Turbo under set -e; actual run reached phase2.', 'literal_stdout': [line for line in clean[1].splitlines() if 'exports' in line or 'PASS' in line]}

package_dirs = {'core': 'packages/core', 'views': 'packages/views', 'docs': 'apps/docs', 'web': 'apps/web', 'desktop': 'apps/desktop'}
ts_packages = []
ts_file_rows = []
slow_rows = []
for package, directory in package_dirs.items():
    prefix = '@multica/' + package + ':test:'
    lines = [line for line in clean[2].splitlines() if line.startswith(prefix)]
    observed = []
    for line in lines:
        match = re.match(re.escape(prefix) + r'\s+✓\s+(.+?)\s+\((\d+) tests?\)\s+(.+)$', line)
        if match:
            row = {'package': '@multica/' + package, 'file': match[1], 'tests_reported': int(match[2]), 'duration_output': match[3], 'terminal': 'file_pass', 'raw': line}
            observed.append(row)
            ts_file_rows.append(row)
            digest(CANDIDATE / directory / match[1])
        elif '✓' in line:
            slow_rows.append({'package': '@multica/' + package, 'raw': line, 'qualification': 'Actual limited slow-test output; not a complete qualified leaf inventory.'})
    file_summary = next((re.search(r'Test Files\s+(\d+) passed \((\d+)\)', line) for line in lines if 'Test Files' in line), None)
    test_summary = next((re.search(r'Tests\s+(\d+) passed \((\d+)\)', line) for line in lines if 'Tests ' in line), None)
    assert file_summary and test_summary
    caches = []
    cache_root = CANDIDATE / directory / 'node_modules/.vite/vitest'
    for cache_path in sorted(cache_root.rglob('results.json')):
        cache = read(cache_path)
        caches.append({'path': str(cache_path), 'sha256': digest(cache_path), 'version': cache.get('version'), 'file_rows': len(cache.get('results', [])), 'record_keys': sorted({key for _, data in cache.get('results', []) for key in data}), 'contains_leaf_name_state_records': False, 'qualification': 'Actual results cache stores file failed/duration only; not leaf-case reporter.'})
    stdout_log = CANDIDATE / directory / '.turbo/turbo-test.log'
    digest(stdout_log)
    config = CANDIDATE / directory / 'vitest.config.ts'
    digest(config)
    manifest = read(CANDIDATE / directory / 'package.json')
    ts_packages.append({'package': '@multica/' + package, 'directory': directory, 'file_summary': [int(file_summary[1]), int(file_summary[2])], 'test_summary': [int(test_summary[1]), int(test_summary[2])], 'observed_file_terminals': len(observed), 'sum_file_tests': sum(row['tests_reported'] for row in observed), 'unique_file_rows': len({row['file'] for row in observed}) == len(observed), 'file_inventory_complete': len(observed) == int(file_summary[1]) == int(file_summary[2]) and sum(row['tests_reported'] for row in observed) == int(test_summary[1]) == int(test_summary[2]), 'leaf_inventory_complete': False, 'script': manifest['scripts']['test'], 'reporter_configuration': 'Default; no reporters/outputFile configured in actual pinned config or forwarded --maxWorkers=2 command', 'cache_evidence': caches, 'stdout_log': str(stdout_log)})

go_packages = []
counts = collections.Counter()
runs = collections.Counter()
terminals = collections.defaultdict(list)
all_named_terminals = []
all_skips = []
raw_errors = []
go_root = PRIVATE / 'go-packages-v14'
for metadata_path in sorted(go_root.glob('*.json')):
    record = read(metadata_path)
    raw_path = metadata_path.with_suffix('.jsonl')
    raw_sha = digest(raw_path)
    observed = collections.Counter()
    outputs = collections.defaultdict(list)
    local_skips = []
    package_terminals = []
    for line_number, line in enumerate(raw_path.read_text(errors='replace').splitlines(), 1):
        try:
            event = json.loads(line)
        except ValueError:
            raw_errors.append({'raw': str(raw_path), 'line': line_number, 'issue': 'Non-JSON Go event'})
            continue
        if event.get('Action') == 'output' and event.get('Test'):
            outputs[event['Test']].append(event.get('Output', ''))
        if event.get('Action') in ('pass', 'fail', 'skip') and not event.get('Test'):
            package_terminals.append(event['Action'])
        if event.get('Test') and event.get('Action') in ('run', 'pass', 'fail', 'skip'):
            observed[event['Action']] += 1
            key = (record['package'], event['Test'])
            if event['Action'] == 'run':
                runs[key] += 1
            else:
                row = {'package': record['package'], 'test': event['Test'], 'action': event['Action'], 'time': event.get('Time'), 'elapsed': event.get('Elapsed'), 'raw': str(raw_path), 'raw_sha256': raw_sha, 'raw_line': line_number}
                terminals[key].append(row)
                all_named_terminals.append(row)
                if event['Action'] == 'skip':
                    local_skips.append(row)
    counts.update(observed)
    for row in local_skips:
        text = ''.join(outputs[row['test']])
        reasons = [line.strip() for line in text.splitlines() if re.search(r'\.go:\d+:', line)]
        source = []
        package_dir = record['package'].split('/server/')[1]
        for reason in reasons:
            match = re.search(r'([^ /]+\.go):(\d+):', reason)
            if match:
                path = CANDIDATE / 'server' / package_dir / match[1]
                line_number = int(match[2])
                lines = path.read_text().splitlines()
                source.append({'path': str(path), 'sha256': digest(path), 'line': line_number, 'condition_excerpt': lines[max(0, line_number-8):line_number+2]})
        all_skips.append({**row, 'reason': reasons, 'source': source, 'counted_as_pass': False})
    expected_counts = record['counts']
    allocation_ok = record['database'] == metadata_path.stem and bool(re.fullmatch(r'ccheck_\d{20}_[0-9a-f]{12}', record['database'])) and record['template'].startswith('multica_check_') and record['template'].endswith('_go') and record['command'][0] == binding['tools']['go']['path'] and record['command'][-1] == record['package'] and record['home'] == str(go_root / 'homes' / record['database']) and record['Go_cache_paths'] == run['Go_cache_paths']
    go_packages.append({'package': record['package'], 'metadata': str(metadata_path), 'metadata_sha256': digest(metadata_path), 'raw': str(raw_path), 'raw_sha256': raw_sha, 'recorded_raw_sha256_matches': raw_sha == record['log_sha256'], 'named_counts': {key: observed[key] for key in ('run', 'pass', 'fail', 'skip')}, 'recorded_counts_matches': all(observed[key] == expected_counts.get(key, 0) for key in ('run', 'pass', 'fail', 'skip')), 'state': record['state'], 'exit_code': record['exit_code'], 'compiled_without_tests': record['compiled_without_tests'], 'package_terminals': package_terminals, 'command': record['command'], 'database': record['database'], 'template': record['template'], 'recorded_allocation_policy_matches': allocation_ok, 'allocation_scope': 'Actual unique package allocation records and hash-pinned adapter verify clone/current_database before child startup; this reviewer executes no SQL and does not duplicate root live witnesses.', 'started_at': record['started_at'], 'finished_at': record['finished_at']})

named_errors = [{'package': key[0], 'test': key[1], 'runs': runs[key], 'terminal_count': len(terminals[key])} for key in set(runs) | set(terminals) if runs[key] != 1 or len(terminals[key]) != 1]
assert not raw_errors
affected = ['internal/service', 'internal/handler', 'internal/daemon', 'internal/realtime', 'cmd/server', 'cmd/migrate', 'internal/migrations', 'pkg/agent', 'pkg/protocol', 'internal/daemon/collaborationrun']
tag_rows = []
race_condition_rows = []
for directory in affected:
    for path in sorted((CANDIDATE / 'server' / directory).rglob('*.go')):
        text = path.read_text(errors='replace')
        for line_number, line in enumerate(text.splitlines(), 1):
            if re.match(r'^//(?:go:build| \+build)', line):
                tag_rows.append({'path': str(path.relative_to(CANDIDATE)), 'line': line_number, 'tag': line})
            if not line.lstrip().startswith('//') and re.search(r'\b(?:GORACE|raceEnabled|RaceEnabled|raceDetector|isRace)\b|runtime\.Race', line):
                race_condition_rows.append({'path': str(path.relative_to(CANDIDATE)), 'line': line_number, 'source': line})
race_tags = [row for row in tag_rows if re.search(r'\brace\b', row['tag'])]


def skip_scope(row):
    name = row['test']
    if 'Linux' in name or name in ('TestPiExecuteAttachesStdinPipe', 'TestCodexExecuteRetriesAfterSignaledProcessIsReaped'):
        return 'Linux platform evidence remains open', 'Current macOS run cannot prove Linux namespace, signal/reap or /proc behavior. #7 default canonical regressions pass independently; no fake substitute establishes Linux. Preserve parent Linux gate.'
    if 'Windows' in name or name == 'TestIsDriveRoot':
        return 'Windows platform evidence remains open', 'Actual Windows path/code-page behavior is unobserved. No #7 canonical regression is omitted; keep parent Windows gate independent.'
    if 'root_bypass' in name:
        return 'Root-only preflight is unobserved', 'Actual root-only sudo preflight excluded on non-root host. Three #7 L7 actual managed termination regressions pass; this skip is not substituted by them.'
    if 'PasswordLimiter' in name:
        return 'Shared Redis limiter integration remains open', 'MULTICA_TEST_REDIS_URL is absent. Root API/CCG deployments use separately declared single mode; this is not a knowledge repair test. Dedicated shared-mode auth proof requires an isolated Redis URL.'
    if 'PGBigm' in name:
        return 'Real pg_bigm opclass integration remains open', 'Missing extension. Knowledge index migrations 761–763 use btree/GIN jsonb_path_ops and pass current migration tests; no pg_bigm result is claimed.'
    if 'UnknownFieldsArePreserved' in name:
        return 'Existing CLI serializer limitation remains', 'Unconditional documented unknown-field round-trip debt. It is separate from strict knowledge descriptor/ledger decoding, whose current L6 regressions pass.'
    if 'WebhookHandler_DBError' in name:
        return 'Existing webhook 500-branch test gap remains', 'Unconditional code-review-protected invariant lacks stub Queries injection. M1 knowledge claim transient-error regression actually passes and does not substitute for webhook 500 coverage.'
    if 'I1P95' in name or 'I1DisableP95' in name:
        return 'Opt-in iteration performance remains unobserved', 'Requires MULTICA_RUN_I1_P95=1 and exclusive DB. Not knowledge-history planner/load benchmarking and not a canonical audit regression.'
    if 'TriageQueueScale' in name:
        return 'Opt-in triage scale baseline remains unobserved', 'Requires MULTICA_TRIAGE_PERF=1 and 10k/2k data. Separate performance scope; no #7 substitute required.'
    if 'FetchFromSkillsSh' in name:
        return 'Live GitHub skill integration remains unobserved', 'Explicit live integration flag absent. Current built-in template tests are observed; no network/provider/account access performed to replace this skip.'
    if 'TriageWireFixtures' in name:
        return 'Conditional fixture export is not requested', 'Cross-package wire-export hook is explicitly gated and not a default product acceptance. Retain the skip; no fake exported evidence is created.'
    if 'McpMarketPublicHTTPRecipeLifecycle' in name:
        return 'Empty HTTP recipe catalog branch is unobserved', 'Catalog has no HTTP recipes. Other MCP/skill/default catalog tests pass; no knowledge canonical regression is omitted and no invented recipe is added.'
    return 'Unclassified explicit gap', 'Coordinator review required before final inventory disposition.'


for row in all_skips:
    row['scope'], row['effect_on_task7_and_substitute'] = skip_scope(row)
    row['reason_and_source_guard_observed'] = bool(row['reason'] and row['source'])
    row['is_canonical_audit_regression'] = False

canonical = []
ts_index = {(row['package'], row['file']): row for row in ts_file_rows}
for disposition in original_review['dispositions']:
    regression = disposition['canonical_regression']
    observed = [row for row in all_named_terminals if row['test'] == regression['name']]
    source = regression['path']
    ts_row = None
    if source.startswith('packages/views/'):
        ts_row = ts_index.get(('@multica/views', source[len('packages/views/'):]))
    canonical.append({'id': disposition['id'], 'regression': regression, 'actual_current_Go_named_terminals': observed, 'actual_current_TS_file_terminal': ts_row, 'current_TS_leaf_terminal_claimed': False, 'historical_ID_red_green_and_accepted_exceptions': 'Preserved in original review.json; current full-file pass is observed and no leaf terminal is fabricated.'})
L7_names = ['TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated', 'TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit', 'TestHostLifecycleStartedFailureStillWaitsAndBlocksFallback']
L7 = [row for row in all_named_terminals if row['test'] in L7_names]
assert len(L7) == 3 and all(row['action'] == 'pass' for row in L7)

contracts = []
for relative in ['.trellis/tasks/10-30-collaboration-fix-closure/prd.md', '.trellis/tasks/10-30-collaboration-fix-closure/implement.md', '.trellis/tasks/10-30-collaboration-fix-closure/verification-continue-2026-11-02.md', '.trellis/tasks/10-30-collaboration-fix-closure/research/continuation-current-session/execution-plan.md', 'docs/plans/2026-11-03-collaboration-fix-closure.md', '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/prd.md']:
    path = ROOT / relative
    contracts.append({'path': str(path), 'sha256': digest(path), 'relevant_lines': [{'line': index, 'text': line} for index, line in enumerate(path.read_text().splitlines(), 1) if re.search(r'具名|named inventory|TypeScript|TS|Go/race|AC8|final|原始 JSON|版本证据', line)]})
script_results = [{'name': line, 'scope': 'Actual script-level/printed control pass only; no unprinted leaf cases invented.'} for line in clean[3].splitlines() if line.endswith(': PASS') or 'dev-env.sh registry behaviour verified' in line]
vet = {'command': 'go vet -p 2 ./...', 'source': 'scripts/check.sh:160', 'scope': 'Actual source-bound phase3 command under fail-fast shell, reached phase4; stdout empty.', 'passed_by_phase_transition': 4 in sections, 'standalone_go_build_full_invocation': False, 'Go_compile_scope': 'All 82 actual go test -race package invocations compiled the current package/test source; 9 contain no tests.'}

go_complete = len(go_packages) == 82 and all(row['state'] == 'complete' and row['exit_code'] == 0 and row['recorded_raw_sha256_matches'] and row['recorded_counts_matches'] and row['recorded_allocation_policy_matches'] for row in go_packages) and not named_errors and counts['run'] == counts['pass']+counts['skip'] and counts['fail'] == 0
ts_file_complete = all(row['file_inventory_complete'] for row in ts_packages) and len(ts_file_rows) == 1069 and sum(row['tests_reported'] for row in ts_file_rows) == 13244
go_inventory_path = write_new('v14-go-inventory.json', {'scope': 'Completed actual v14 phase3 default -race; parents/subtests are separate named events', 'counts': {key: counts[key] for key in ('run', 'pass', 'fail', 'skip')}, 'packages': go_packages, 'named_terminal_inventory': all_named_terminals, 'named_completeness_errors': named_errors, 'skips': all_skips, 'inventory_complete': go_complete, 'distinct_database_count': len({row['database'] for row in go_packages}), 'tested_packages': sum(not row['compiled_without_tests'] for row in go_packages), 'compile_without_tests_packages': [row['package'] for row in go_packages if row['compiled_without_tests']], 'live_SQL_performed_by_reviewer': False})
ts_inventory_path = write_new('v14-ts-file-inventory.json', {'scope': 'Actual v14 stdout file terminals with test counts; explicitly NOT complete leaf case inventory', 'packages': ts_packages, 'files': ts_file_rows, 'actual_limited_slow_test_output': slow_rows, 'file_inventory_complete': ts_file_complete, 'leaf_inventory_complete': False, 'files_total': len(ts_file_rows), 'reported_tests_total': sum(row['tests_reported'] for row in ts_file_rows), 'skips_reported': 0, 'source_raw': raw_files[2]})
skip_path = write_new('v14-go-skip-review.json', {'scope': 'All 22 actual completed v14 Go skips reviewed individually against source/condition; none counted as pass', 'rows': all_skips, 'count': len(all_skips), 'coordinator_final_inventory_disposition_still_required': True})

report = {'schema_version': 1, 'reviewed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'reviewer': os.environ.get('CODEX_THREAD_ID', '/root/audit7_current_evidence'), 'scope': 'Completed static/TS/Go phases only, not whole acceptance; no test/build/service/DB commands.', 'active_task': str(TASK), 'candidate_sha': head, 'state_at_observation': run['state'], 'run_snapshot': run_snapshot, 'wholecheck_accepted': False, 'v13_original_whole_exit': 2, 'frozen_binding_checks': binding_checks, 'tools': tool_checks, 'source_snapshot': {'tracked_file_count': len(binding['source']['tracked_files']), 'mismatches': tracked_mismatches, 'mode_mismatches': mode_mismatches, 'declared_Next_import_only_verified': allowed_next, 'unexpected_mismatches': unexpected_source_changes, 'qualification': 'Current phase-time snapshot; actual final auditor remains required after complete Make.'}, 'source_log_observation': {'path': str(log_path), 'bytes_observed': len(log_bytes), 'prefix_sha256_at_observation': log_snapshot_sha, 'immutable_completed_phase_copies': raw_files, 'whole_log_is_still_mutable': run['state'] == 'running'}, 'static': {'actual_tasks': static_rows, 'count': len(static_rows), 'all_successful_summary': static_summary, 'extra_ui_exports': static_extra}, 'TS': {'reported_pass': 13244, 'file_count': 1069, 'file_inventory_complete': ts_file_complete, 'leaf_inventory_complete': False, 'inventory': ts_inventory_path, 'reporter_gap': 'Default stdout reports every file and aggregate cases; caches contain file duration/failed only. A limited slow-test subset is not a complete qualified leaf inventory.'}, 'Go': {'counts': {key: counts[key] for key in ('run', 'pass', 'fail', 'skip')}, 'inventory_complete': go_complete, 'inventory': go_inventory_path, 'skip_review': skip_path, 'vet': vet, 'script_results': script_results, 'affected_package_results': [row for row in go_packages if row['package'].split('/server/')[1] in affected], 'observed_mode': 'Current complete default package selection with -race; no distinct current non-race full invocation', 'race_selection_review': {'race_or_not_race_build_tags': race_tags, 'race_conditional_code_hits': race_condition_rows, 'all_build_tag_scope_rows': tag_rows, 'race_conditioned_named_skips': [], 'default_set_omission_found': bool(race_tags or race_condition_rows), 'qualification': 'No race/!race tags or runtime race omission found. Mode distinction does not create a new normal full-suite requirement. knowledgeintegration, agentintegration and platform tags remain separately registered gates.'}}, 'canonical_21_dispositions': canonical, 'L7_three_current_named_passes': L7, 'contract_readback': contracts, 'inventory_contract_observation': 'Original PRD/implement/verification-continue expressly use versioned TS results, static15, TS aggregate and per-Go-package/per-E2E actual results; later plans require complete named inventory without explicitly defining TS leaf level. This report does not redefine or relax that requirement: current TS file inventory is complete, leaf inventory is absent.', 'minimum_if_TS_leaf_inventory_required': 'After sole complete-run finish/resource release, record supplementary current-source five-package Vitest executions with JSON reporter/outputFile in task-owned new paths. Keep those supplementary observations separate from original v14; never manufacture original leaf terminals or launch another make check. This reviewer performs none.', 'remaining_actual_gates': ['Sole v14 final Make exit0', 'Actual hash-bound final identity audit pass', 'Final browser/named/individual-skip inventory and root live resource proof', 'Final coordinator disposition of inventory level and any required separate supplementary observation', 'Current CCG1 normal/race and CCG2 OFF/ON after shared run release', 'Independent #7 final review; #6 model/platform/release remain independent'], 'operations': {'tests': False, 'builds': False, 'lint_typecheck_execution': False, 'services': False, 'DB_SQL': False, 'claim_or_source_edits': False, 'task_metadata_edits': False, 'provider_calls': 0, 'knowledge_budget_allocated': 0, 'production_knowledge_default': False, 'write_scope': str(OUT)}, 'old_review_files_preserved': old_hashes}
report_path = write_new('v14-phase-review.json', report)
md = '# Independent v14 completed-phase evidence review\n\n'
md += 'Candidate `d0b132fff` has current completed static, TypeScript and Go race evidence. **The whole run remains running and unaccepted.** Original v13 exit2 and every historical failure remain unchanged. This review starts no tests/builds/services/SQL and does not change task status or execution ownership.\n\n'
md += 'Static: 15 named forced tasks completed successful; the fail-fast UI-export check completed before phase2. TS: all 1,069 actual file pass terminals reconcile with file summaries, and their reported case counts sum exactly to 13,244. Five actual Vitest result caches contain file duration/failed only. The raw log prints a limited slow-test subset, so **TS leaf-case inventory is absent**, not fabricated from source.\n\n'
md += 'The original shared PRD/implement/verification-continue use versioned TS evidence and aggregate counts; later plans require complete named inventory without defining TS leaf granularity. This report changes no gate. If leaf terminals are required, the minimum truthful addition is a separately recorded JSON-reporter observation of only the five current-source TS package suites after the sole run releases resources. It cannot be relabeled as original v14 leaf evidence.\n\n'
md += 'Go: independently parsed all 82 actual raw/metadata pairs: 20,985 run, 20,963 pass, 0 fail, 22 skip, 73 tested packages and 9 packages without tests. Every named run has exactly one terminal; every raw hash/count matches its metadata; all package records complete exit0 and use 82 distinct recorded clone names. Each command uses the pinned Go and -race. The 10 affected packages and all three L7 names have current pass terminals. Source-bound go vet reached phase4 without error.\n\n'
md += 'No race/!race build tags or runtime race-dependent omission was found in affected source, and no actual skip is conditioned on race. Current -race tests therefore do not reveal an omitted default set. No normal full-suite gate is added merely from the mode distinction. Separately tagged knowledgeintegration CCG1 normal/race, agentintegration/model and Linux/Windows gates retain their original scope.\n\n'
md += 'All 17,536 tracked files/modes and pinned tools were checked at this observation. The sole content mismatch is the declared exact Next dev-to-production routes import; every other tracked byte/mode and binding/adapter/launcher/config hash matches. This is phase-time evidence, not the final auditor or root live resource witness.\n\n'
md += '| Actual named skip | Verified reason / scope | #7 effect and remaining evidence |\n| --- | --- | --- |\n'
for row in all_skips:
    md += '| `' + row['test'] + '` | ' + row['scope'] + ' | ' + row['effect_on_task7_and_substitute'] + ' |\n'
md += '\nAll 22 skip reasons were observed in raw logs and checked against exact current source guards. None is counted as pass or a canonical H/M/L regression. The coordinator still owns final skip dispositions and any separately required platform/integration proof.\n\n'
md += 'Remaining: complete Make exit0, actual final identity audit, final browser/inventory/resource evidence, inventory-level disposition without gate changes, current CCG1 normal/race and separately deployed OFF/ON, then independent #7 closure. Knowledge budget/calls stay0/0 and injection stays off. JSON files contain all actual Go names, all TS file terminals, every skip source excerpt, 21 canonical mappings and three exact L7 observations.\n'
md_path = OUT / 'v14-phase-review.md'
with md_path.open('x') as output:
    output.write(md)
digest(md_path)
preserved = {name: digest(OUT / name) == old_hashes[name] for name in old_names}
assert all(preserved.values())
digest(Path(__file__))
write_new('v14-phase-hashes.json', {'created_at': report['reviewed_at'], 'hashes': hashes, 'old_review_preserved': preserved, 'source_whole_log_prefix_at_observation': log_snapshot_sha, 'mutable_observation_only_sources': [str(PRIVATE / 'make-check-v14.json')], 'immutable_run_snapshot': run_snapshot, 'whole_log_final_binding_pending': True})
print(json.dumps({'report': report_path, 'Go_counts': report['Go']['counts'], 'Go_inventory_complete': go_complete, 'TS_file_inventory_complete': ts_file_complete, 'TS_leaf_inventory_complete': False, 'skips_individually_reviewed': len(all_skips), 'race_default_omission_found': report['Go']['race_selection_review']['default_set_omission_found'], 'binding_checks': binding_checks, 'unexpected_source_mismatches': unexpected_source_changes, 'old_files_preserved': preserved, 'wholecheck_accepted': False}, ensure_ascii=False, indent=2))
