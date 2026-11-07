"""Read retained evidence and current source; write only this review directory."""

from pathlib import Path
import base64
import collections
import datetime
import hashlib
import json
import os
import re
import subprocess


ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
CANDIDATE = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
TASK = ROOT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes'
OUT = Path(__file__).resolve().parent
SHA = 'd0b132fff0c477eb2d744933e13db730240231aa'
proof_hashes = {}


def digest(path):
    path = Path(path)
    result = hashlib.sha256(path.read_bytes()).hexdigest()
    proof_hashes[str(path)] = result
    return result


def read(path):
    path = Path(path)
    digest(path)
    return json.loads(path.read_text())


def git(*args):
    return subprocess.check_output(['git', '-C', str(CANDIDATE), *args], text=True).strip()


def named_history(path):
    text = re.sub(r'\x1b\[[0-9;]*m', '', Path(path).read_text(errors='replace'))
    terminals = []
    for line in text.splitlines():
        if line.lstrip().startswith('#'):
            continue
        try:
            value = json.loads(line)
        except ValueError:
            value = None
        if isinstance(value, dict):
            if value.get('Test') and value.get('Action') in ('pass', 'fail', 'skip'):
                terminals.append({'name': value['Test'], 'action': value['Action']})
            continue
        match = re.match(r'\s*--- (PASS|FAIL|SKIP): ([^(]+)', line)
        if match:
            terminals.append({'name': match[2].strip(), 'action': match[1].lower()})
        elif line.lstrip().startswith('FAIL ') and ' > ' in line:
            terminals.append({'name': line.split(' > ', 1)[1].strip(), 'action': 'fail'})
        elif re.match(r'\s*✓ ', line) and ' > ' in line:
            terminals.append({'name': re.sub(r' \d+ms$', '', line.split(' > ', 1)[1]), 'action': 'pass'})
    return {'named_terminals': terminals, 'counts': dict(collections.Counter(x['action'] for x in terminals))}


legacy = read(TASK / 'research/continuation-20261102/knowledge-closure-readiness.json')
previous = read(TASK / 'research/execution-20261103/knowledge-gate-rebind/source-evidence.json')
rebind_review = read(TASK / 'research/execution-20261103/knowledge-gate-rebind/review.json')
go_inventory = read(TASK / 'research/execution-20261103/fullcheck-v13/go-inventory.json')
browser_inventory = read(TASK / 'research/execution-20261103/fullcheck-v13/playwright-inventory.json')
v13_summary = read(TASK / 'research/execution-20261103/fullcheck-v13/result-summary.json')
v14 = read(CANDIDATE / '.omx/private/final-check-v14/make-check-v14.json')
partial_path = OUT.parent / 'v14-explicit-skips-partial.json'
partial = read(partial_path) if partial_path.exists() else {'named_skips': [], 'partial': True}
sqlc_dir = CANDIDATE / '.omx/private/final-check-v12/sqlc-resume-20261103-01'
sqlc = read(sqlc_dir / 'result.json')
source_inputs = read(sqlc_dir / 'source-before.json')
generated = read(sqlc_dir / 'generated-before.json')
sqlc_mismatches = {
    'input': [p for p, h in source_inputs.items() if digest(CANDIDATE / p) != h],
    'generated': [p for p, h in generated.items() if digest(CANDIDATE / p) != h],
}
sqlc_verified = sqlc['exit_code'] == 0 and sqlc['generated_unchanged'] and not any(sqlc_mismatches.values()) and digest(sqlc_dir / 'raw.log') == sqlc['log_sha256']

reviews = []
for reference in previous['independent_reviews'] + previous['followup_reviews']:
    path = ROOT / reference['path']
    value = read(path)
    reviews.append({'path': str(path), 'sha256': digest(path), 'hash_matches_prior': digest(path) == reference['sha256'], 'verdict': value.get('verdict'), 'blocking_findings': value.get('blocking_product_findings', value.get('blocking_findings', []))})
source_comparisons = []
for row in previous['source_comparisons']:
    actual = digest(CANDIDATE / row['path'])
    source_comparisons.append({**row, 'actual_current_sha256': actual, 'current_matches_prior_current_candidate': actual == row['candidate_sha256']})

raw_counts = collections.Counter()
raw_mismatches = []
packages = []
for row in go_inventory['packages']:
    digest(row['metadata'])
    actual_hash = digest(row['raw'])
    counts = collections.Counter()
    for line in Path(row['raw']).read_text(errors='replace').splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get('Test') and event.get('Action') in ('run', 'pass', 'fail', 'skip'):
            counts[event['Action']] += 1
    raw_counts.update(counts)
    matches = actual_hash == row['raw_sha256'] and digest(row['metadata']) == row['metadata_sha256'] and all(counts[key] == row['named_counts'].get(key, 0) for key in ('run', 'pass', 'fail', 'skip'))
    if not matches:
        raw_mismatches.append(row['package'])
    packages.append({'package': row['package'], 'database': row['database'], 'exit_code': row['exit_code'], 'command': row['command'], 'named_counts': dict(counts), 'raw': row['raw'], 'raw_sha256': actual_hash, 'integrity_matches': matches, 'compile_only': row['compiled_without_tests']})

dispositions = []
for item in legacy['audit_dispositions']:
    canonical = dict(item['canonical_regression'])
    if item['id'] == 'L12':
        canonical['name'] = 'lets a human remove a stored identity only after a check finds no result, without any request'
    if item['id'] == 'L13':
        canonical['name'] = 'collaboration: zh-Hans covers every EN key'
        canonical['source_template'] = '${ns}: ${locale} covers every EN key'
    path = CANDIDATE / canonical['path']
    evidence = []
    for reference in item['evidence']:
        origin = ROOT / reference['path']
        evidence.append({'path': str(origin), 'sha256': digest(origin), 'expected_hash_matches': digest(origin) == reference['sha256'], **named_history(origin)})
    terminals = [event for event in go_inventory['named_terminal_inventory'] if event['test'] == canonical['name']]
    red_named = sorted({event['name'] for reference in evidence if '-red.' in reference['path'] for event in reference['named_terminals'] if event['action'] == 'fail'})
    dispositions.append({'id': item['id'], 'disposition': item['disposition'], 'behavior': item['behavior'], 'canonical_regression': canonical, 'canonical_exists_in_current_source': canonical.get('source_template', canonical['name']) in path.read_text(), 'source_sha256': digest(path), 'actual_v13_go_terminals': terminals, 'red_named_failures': red_named, 'red_canonical_matches': any(canonical['name'] in name for name in red_named) if red_named else None, 'green_scope': 'Actual v13 -race named terminal' if terminals else 'Original named/focused green and current full knowledge-panel (34 tests) / locale parity (66 tests) in v13; v14 core/views full pass now recorded by root', 'evidence': evidence, 'reviews': item['reviews'], 'exception': 'Existing correct lifecycle; no fabricated baseline red' if item['id'] == 'L7' else 'Locale parity/terminology exception' if item['id'] == 'L13' else None, 'L12_auxiliary_canonical': item['canonical_regression'] if item['id'] == 'L12' else None})

en_knowledge = read(CANDIDATE / 'packages/views/locales/en/collaboration.json')['knowledge']
zh_knowledge = read(CANDIDATE / 'packages/views/locales/zh-Hans/collaboration.json')['knowledge']


def locale_keys(value, prefix=''):
    if not isinstance(value, dict):
        return {re.sub(r'_(one|other)$', '_count', prefix)}
    keys = set()
    for key, child in value.items():
        keys.update(locale_keys(child, prefix + '.' + key if prefix else key))
    return keys


locale_findings = [value for value in zh_knowledge.values() if isinstance(value, str) and ('协作单' in value or re.search(r'(?<!工作)空间', value))]
locale_check = {'normalized_knowledge_keys_equal': locale_keys(en_knowledge) == locale_keys(zh_knowledge), 'knowledge_key_count': len(locale_keys(en_knowledge)), 'nonconforming_terminology': locale_findings, 'required_current_values': {key: zh_knowledge[key] for key in ('derive', 'workspaceTarget', 'sourceWorkspace', 'historyTruncated', 'origin', 'openSource')}}

sql_migrations = []
for path in sorted((CANDIDATE / 'server/migrations').glob('76[123]*.sql')):
    text = path.read_text()
    statements = [x for x in text.split(';') if x.strip()]
    sql_migrations.append({'path': str(path), 'sha256': digest(path), 'statements': len(statements), 'concurrent_single_statement': len(statements) == 1 and text.startswith('CREATE INDEX CONCURRENTLY') if path.name.endswith('.up.sql') else None, 'no_foreign_key': re.search(r'\b(?:REFERENCES|FOREIGN KEY|CASCADE)\b', text, re.I) is None, 'down_guard_retained': text.startswith('SELECT collaboration_assert_knowledge_empty();') if path.name.endswith('.down.sql') else None})
cleanup_source = CANDIDATE / 'server/cmd/migrate/main.go'
cleanup_text = cleanup_source.read_text()
cleanup_registered = all(name in cleanup_text for name in ['collaboration_knowledge_agent_history_idx', 'collaboration_knowledge_runtime_history_idx', 'collaboration_knowledge_delivery_history_idx'])
digest(cleanup_source)

browser_raw = read(browser_inventory['source_report'])
browser_report_hash_match = digest(browser_inventory['source_report']) == browser_inventory['source_sha256']
knowledge_tests = [x for x in browser_inventory['tests'] if x['file'] == 'collaboration-knowledge.spec.ts']
native_attachment = None


def native_walk(value):
    global native_attachment
    if isinstance(value, dict):
        if value.get('name') == 'native-execution-boundary' and 'body' in value:
            decoded = base64.b64decode(value['body'])
            native_attachment = {'encoded_sha256': hashlib.sha256(value['body'].encode()).hexdigest(), 'decoded_sha256': hashlib.sha256(decoded).hexdigest(), 'value': json.loads(decoded)}
        for child in value.values():
            native_walk(child)
    elif isinstance(value, list):
        for child in value:
            native_walk(child)


native_walk(browser_raw)
v13_log = Path(v13_summary['artifacts']['log']['path'])
digest(v13_log)
ts_lines = [line for line in v13_log.read_text(errors='replace').splitlines() if 'knowledge-panel.test.tsx (' in line or 'locales/parity.test.ts (' in line or re.search(r'Tasks:\s+\d+ successful', line) or ('Tests ' in line and ('@multica/core:' in line or '@multica/views:' in line))]
v13_product_delta = git('diff', '--name-only', v13_summary['candidate_sha'], SHA, '--', 'server', 'packages', 'apps', 'scripts', 'Makefile', 'package.json', 'pnpm-lock.yaml').splitlines()
baseline_product_delta = git('diff', '--name-only', '6aed33aad', '4e3b61500', '--', 'server', 'packages', 'apps').splitlines()

skip_reviews = []
for row in partial['named_skips']:
    name = row['test']
    if 'Linux' in name:
        classification = 'Requires actual Linux containment and, for route tests, MULTICA_RUN_COLLAB_RUNTIME_E2E=1. macOS stand-ins cannot establish this. Keep parent platform/runtime gate open; CCG1/CCG2 separately cover #7 current knowledge physical-input chain.'
    elif name == 'TestIsDriveRoot':
        classification = 'Windows filepath semantics; macOS skip verified. Retain Windows platform gate; no #7 changed drive-root path.'
    elif 'PasswordLimiter' in name:
        classification = 'Shared Redis limiter integration is untested in this run, not a pass. Requires isolated MULTICA_TEST_REDIS_URL for auth shared-mode acceptance; unrelated to #7 knowledge repair and CCG password single-mode deployments.'
    elif 'PGBigm' in name:
        classification = 'Real pg_bigm opclass remains untested. Retain extension integration gap; #7 migrations 761–763 use btree/GIN jsonb_path_ops and do not depend on pg_bigm.'
    else:
        classification = 'Unconditional documented CLI JSON round-trip limitation remains; do not count as pass. Not a #7 knowledge wire/ledger change.'
    source_proof = []
    pkg = row['package'].split('/server/')[1]
    for context in row['output_context']:
        match = re.search(r'([^ /]+\.go):(\d+):', context)
        if match:
            source = CANDIDATE / 'server' / pkg / match[1]
            line = int(match[2])
            lines = source.read_text().splitlines()
            source_proof.append({'path': str(source), 'sha256': digest(source), 'line': line, 'guard_excerpt': lines[max(0, line-6):line+2]})
    raw_path = row['source_log']
    skip_reviews.append({**row, 'reason_and_source_verified': bool(source_proof), 'source_proof': source_proof, 'classification': classification, 'counted_as_pass': False, 'is_canonical_H_M_L_regression': False, 'raw_sha256': digest(raw_path)})

acs = [
    {'id': 'AC1', 'state': 'verified_in_disposition_scope', 'evidence': '19 reproduced IDs have actual named baseline failures; L12 primary mapping points to same red/green deletion regression, with ownership regression retained separately. L7 and L13 use explicit accepted exceptions. Baseline 6aed33aad and 4e3b61500 product source match.', 'remaining': 'Final current complete gate still required; embedded failures/mutants in green files retained.'},
    {'id': 'AC2', 'state': 'verified_in_disposition_scope', 'evidence': 'Current H1 safe-OFF/nil descriptor and unsafe-selector canonical source unchanged; actual v13 named -race pass.', 'remaining': 'No selector stripping or private-history downgrade introduced.'},
    {'id': 'AC3', 'state': 'verified_in_disposition_scope', 'evidence': 'Long histories, deep exposure/corruption, complete namespace and CT1 controls retain exact bytes and actual v13 named passes.', 'remaining': 'O(N) namespace cost and production-scale planner benchmark remain explicit limitations.'},
    {'id': 'AC4', 'state': 'verified_in_disposition_scope', 'evidence': 'M1 immediate requeue and M2 bounded retry tests actually pass; accepted 1.5s budget and terminal exhaustion remain.', 'remaining': 'Current changed Host admission/lock graph requires CCG1 normal/race.'},
    {'id': 'AC5', 'state': 'verified_in_disposition_scope', 'evidence': 'H2 actual daemon/HTTP rebinding plus stale/refusal controls pass in v13, preserving exposed generations and original requests.', 'remaining': 'Current operator documentation explicitly describes API-first/daemon upgrade compatibility.'},
    {'id': 'AC6', 'state': 'verified_in_current_TS_scope', 'evidence': 'Current 34-test knowledge panel / 66-test locale parity v13 pass; actual v13 product source equals d0b132fff. Current source recovery disclosure increment preserved. Root now records complete v14 TS pass.', 'remaining': '7 Web + 1 native human knowledge v13 observations may be rebound in source scope, not treated as complete candidate acceptance.'},
    {'id': 'AC7', 'state': 'verified_current_source', 'evidence': 'SQLC actual exit0, all 1631 current input hashes and 130 generated hashes match before/after success; 761–763 concurrent single-statement up/no FK/down guard and cleanup registration verified.', 'remaining': 'Old GOPROXY=off and migration failures retained; no SQLC rerun needed on these identical bytes.'},
    {'id': 'AC8', 'state': 'open_fullcheck_gate', 'evidence': 'Independently reconstructed 82 v13 -race package raw/metadata hashes: 20963 pass / 0 fail / 22 skip, 9 compile-only; all 82 unique DBs. v13 static15 and core4455/views7165 pass on current product bytes. This run still exited2.', 'remaining': 'Sole v14 make exit0 plus actual final audit, complete named/skip inventory and root live resource ownership. Partial skips individually reviewed; none counted as pass.'},
    {'id': 'AC9', 'state': 'verified_contract_sync_in_stated_scope', 'evidence': 'Server/core/views knowledge specs and runtime upgrade documentation retain current contracts; built-in issue skill/source map remains current after delivered policy/detail updates; independent sync/followup reports retain hashes.', 'remaining': 'Final full built-in template test scope, final-source binding and no unresolved review findings required.'},
]
report = {
    'schema_version': 1,
    'reviewed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'reviewer': os.environ.get('CODEX_THREAD_ID', '/root/audit7_current_evidence'),
    'active_task': str(TASK),
    'scope': 'Independent read-only readiness review, current candidate source and retained actual evidence. Not final acceptance.',
    'candidate_sha': git('rev-parse', 'HEAD'),
    'candidate_status': git('status', '--short').splitlines(),
    'candidate_matches_expected': git('rev-parse', 'HEAD') == SHA,
    'verdict': 'conditional; no new confirmed product defect; full v14 and current CCG gates remain open',
    'task7_closable_now': False,
    'task7_independent_of_task6_real_model_and_release': True,
    'new_blocking_product_findings': [],
    'shared_owner_retained': '01a1a186-9a5c-7b00-96f8-0430b47edb4c',
    'shared_executor_retained': '/root/closure_inventory',
    'v14_observation': {key: v14.get(key) for key in ('state', 'started_at', 'finished_at', 'exit_code', 'process_id')},
    'v14_observation_is_partial': v14.get('state') == 'running',
    'v13_product_delta_to_current': v13_product_delta,
    'baseline_product_delta_6aed_4e3': baseline_product_delta,
    'disposition_count': len(dispositions),
    'dispositions': dispositions,
    'AC1_AC9': acs,
    'independent_and_followup_reviews': reviews,
    'source_comparisons': source_comparisons,
    'SQLC': {'verified': sqlc_verified, 'exit_code': sqlc['exit_code'], 'source_input_count': len(source_inputs), 'generated_count': len(generated), 'mismatches': sqlc_mismatches, 'log_sha256': sqlc['log_sha256'], 'record': str(sqlc_dir / 'result.json')},
    'migrations': sql_migrations,
    'concurrent_index_cleanup_registered': cleanup_registered,
    'v13_Go_reconstruction': {'counts': {key: raw_counts[key] for key in ('run', 'pass', 'fail', 'skip')}, 'packages': packages, 'package_count': len(packages), 'mismatches': raw_mismatches, 'unique_databases': len({row['database'] for row in packages}) == len(packages), 'explicit_skips': go_inventory['skipped'], 'qualification': 'Historical actual pass scopes on unchanged v13 product bytes; entire v13 exited2 and remains unaccepted.'},
    'L13_current_locale_check': locale_check,
    'v13_TS_static_raw_lines': ts_lines,
    'v13_knowledge_human_UI': {'passed': len(knowledge_tests), 'web': 7, 'native_macOS': 1, 'report_hash_matches': browser_report_hash_match, 'tests': [{key: x[key] for key in ('id', 'title', 'outcome', 'attempts', 'retries')} for x in knowledge_tests], 'native_execution_boundary': native_attachment, 'source_rebind_allowed': not v13_product_delta, 'qualification': 'Human/stand-in boundary only; zero daemon starts does not establish model/native execution.'},
    'current_CCG1_CCG2': {'historical_proof_retained': True, 'historical_whole_chain_rebind_allowed': False, 'reason': 'AI policy, router, assisted/manual migration764, target issue_collaboration locking, queue/batch/runtime and detail graph differ from old proofs.', 'pending': rebind_review['minimum_post_full_gates'], 'current_permits_must_remain_false_before_full_gate': True},
    'v14_individual_skip_review_partial': {'source': str(partial_path), 'partial': True, 'observed_count': len(skip_reviews), 'rows': skip_reviews, 'required_final_action': 'Refresh against final inventory, inspect newly observed names and keep every untested gate separate.'},
    'closure_prerequisites': ['Sole v14 actual make exit0', 'Hash-bound actual final identity audit pass', 'Complete named and individual skip inventory; actual API/database/process ownership', 'Current CCG1 normal and race, each fresh migrated task DB with physical v2 input and zero third launch', 'Current CCG2 OFF and ON, separate task deployments, exact input/event/fake-process attachments', 'Independent final review and final-source binding; no new product blocker'],
    'proof_hashes': proof_hashes,
    'operations': {'tests_run': False, 'lint_run': False, 'typecheck_run': False, 'database_commands': False, 'service_commands': False, 'product_edits': False, 'task_metadata_edits': False, 'claim_edits': False, 'agents_spawned': 0, 'provider_calls': 0, 'knowledge_budget_allocated': 0, 'production_knowledge_default': False, 'write_scope': str(OUT)},
}
(OUT / 'review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
rows = ['| ID | Disposition | Current canonical regression | Evidence |', '| --- | --- | --- | --- |']
for item in dispositions:
    state = 'v13 -race pass' if item['actual_v13_go_terminals'] else 'Original named/focused green + current v13 full TS file pass'
    rows.append('| ' + item['id'] + ' | ' + item['disposition'] + ' | `' + item['canonical_regression']['name'].replace('|', '\\|') + '` | ' + state + ' |')
ac_rows = ['| AC | State | Remaining boundary |', '| --- | --- | --- |']
for row in acs:
    ac_rows.append('| ' + row['id'] + ' | ' + row['state'] + ' | ' + row['remaining'] + ' |')
skip_rows = ['| Named current skip | Review |', '| --- | --- |']
for row in skip_reviews:
    skip_rows.append('| `' + row['test'] + '` | ' + row['classification'] + ' |')
text = '# Independent #7 current-evidence readiness review\n\n'
text += 'Candidate `d0b132fff` has all 21 independently reviewed dispositions and no newly confirmed product defect. **#7 remains open** until the sole v14 complete check and current CCG1/CCG2 observations actually satisfy their gates. It closes independently of #6 real models/framework/releases. No tests, database/service/Provider commands, product edits or task-status changes were performed by this review.\n\n'
text += 'SQLC now has real successful evidence: exit0 at 2026-11-03 17:37 Beijing time, local file proxy only, all 1,631 current source inputs and all 130 generated files hash-identical to successful before/after inventories. Migrations 761–763 are concurrent single-statement up files, no FK, with guarded down/cleanup registration. No rerun is needed on these identical bytes.\n\n'
text += 'Independently parsed all 82 v13 -race raw logs and metadata: 20,963 named pass, 0 fail, 22 explicit skip, 9 compile-only packages and 82 different DB allocations; every raw/count/hash matched. V13 static15, core4,455, views7,165, knowledge-panel34 and locale-parity66 passed. Product bytes from v13 to d0b132fff have no delta. V13 whole make nevertheless exited2 and remains failed. Current v14 stage observations are partial; final exit/audit/inventory/resource proof is still required.\n\n'
text += 'Original red/green chronology is preserved: H1/H2/L6 green files include failed later controls, L1/L2 contain commented mutants, and focused L9–L12 runs include deliberate exclusions. Filenames do not establish pass. All 19 reproduced IDs have actual named failures. L12 primary red/green mapping is the exact local-record deletion regression; loaded-page ownership is a separate retained auxiliary regression. L7 is tested existing correct lifecycle behavior, and L13 is the accepted locale exception.\n\n'
text += '\n'.join(rows) + '\n\n' + '\n'.join(ac_rows) + '\n\n'
text += 'The retained v13 human UI has 7 Web and 1 actual macOS Electron tests, each one pass/no skip/retry. Native attachment records darwin, the owned renderer/source, auth session and zero daemon starts; it proves the human UI boundary, not model/native execution. Old CCG1/CCG2 proofs cannot be rebound as whole chains because current policy/router/migration/lock and detail source changed. Preserve original proofs, then execute current CCG1 normal/race and separately deployed OFF/ON only after successful complete-check release.\n\n'
text += 'Current partial v14 skips were inspected against their exact source guards. None is a canonical H/M/L regression or counted as pass. Linux/Windows, real pg_bigm and shared-Redis acceptance remain separate explicit gaps; current CCG password deployments use single mode. Final inventory must be reviewed again for newly observed names.\n\n'
text += '\n'.join(skip_rows) + '\n\n'
text += 'All seven lane/combined reviews and five followup review hashes still match. All 50 selected supporting source hashes match the previous current-candidate reconciliation. The JSON carries every AC/ID, raw terminal, selected source hash, SQLC input comparison, 82 package inventories, current partial skip source excerpts and exact CCG prerequisites. The source default and knowledge allocation/calls stay false/0/0.\n'
(OUT / 'review.md').write_text(text)
print(json.dumps({'review': str(OUT / 'review.json'), 'verdict': report['verdict'], 'dispositions': len(dispositions), 'sqlc_verified': sqlc_verified, 'Go_mismatches': raw_mismatches, 'selected_source_mismatches': [x['path'] for x in source_comparisons if not x['current_matches_prior_current_candidate']], 'partial_skip_reviews': len(skip_reviews)}, ensure_ascii=False, indent=2))
