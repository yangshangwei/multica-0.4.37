#!/usr/bin/env python3
"""Read-only source/evidence reconciliation; writes this report directory only."""
import collections
import datetime
import hashlib
import json
import subprocess
from pathlib import Path

ROOT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
CANDIDATE = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = Path(__file__).resolve().parent
OLD = '64a9994ec549e8dff73ab14456dc9a9f4b63f7c7'
EXPECTED = '85ef5d20578f63c7c4f879f035acbd414e80f306'
TASK = Path('.trellis/tasks/11-01-collaboration-knowledge-audit-fixes')
PARENT = Path('.trellis/tasks/10-30-collaboration-context-handoff/research')

def sha(data):
    return hashlib.sha256(data).hexdigest()

def read(path):
    p = Path(path)
    return json.loads((p if p.is_absolute() else ROOT / p).read_text())

def git(*args):
    return subprocess.check_output(['git', *args], cwd=CANDIDATE)

def bound(path, expected=None):
    p = Path(path)
    actual = p if p.is_absolute() else ROOT / p
    row = {'path': str(path), 'exists': actual.is_file()}
    if row['exists']:
        row['sha256'] = sha(actual.read_bytes())
    if expected:
        row.update(expected_sha256=expected, hash_match=row.get('sha256') == expected)
    return row

def source(path):
    before = git('show', OLD + ':' + path)
    after = git('show', EXPECTED + ':' + path)
    disk = CANDIDATE / path
    return {'path': path, 'old_sha256': sha(before), 'candidate_sha256': sha(after),
            'unchanged_from_final64': before == after,
            'worktree_matches_candidate': disk.is_file() and disk.read_bytes() == after}

def json_events(path):
    result = []
    for line in Path(path).read_text().splitlines():
        try:
            row = json.loads(line)
        except ValueError:
            continue
        if isinstance(row, dict) and 'Action' in row:
            result.append(row)
    return result

head = git('rev-parse', 'HEAD').decode().strip()
assert head == EXPECTED
legacy = read(TASK / 'research/continuation-20261102/knowledge-closure-readiness.json')
reviews = []
for name, row in legacy['independent_lane_reviews'].items():
    current = read(row['path'])
    reviews.append({**bound(row['path'], row['sha256']), 'name': name,
                    'verdict': current['verdict'], 'audits': current.get('audits', []),
                    'blocking_findings': current.get('blocking_product_findings', [])})
followups = [{**bound(row['path'], row['sha256']), 'verdict': row['verdict'],
              'scope': row['scope']} for row in legacy['reviewed_followups']]
go_meta_path = PARENT / 'final-candidate-20261102/attempt-3-Go-complete.json'
go_meta = read(go_meta_path)
raw_path = go_meta['prefix_private_path']
events = json_events(raw_path)
counts = {kind: sum(r.get('Action') == kind and bool(r.get('Test')) for r in events)
          for kind in ['pass', 'fail', 'skip']}
canonical = {row['canonical_regression']['name'] for row in legacy['audit_dispositions']}
l7 = ['TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated',
      'TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit',
      'TestHostLifecycleStartedFailureStillWaitsAndBlocksFallback']
canonical.update(l7)
terminals = collections.defaultdict(list)
for row in events:
    if row.get('Test') in canonical and row['Action'] in ['pass', 'fail', 'skip']:
        terminals[row['Test']].append({key: row.get(key) for key in ['Action', 'Package', 'Time', 'Elapsed']})
skips = []
for row in events:
    if row['Action'] == 'skip' and row.get('Test'):
        outputs = [r.get('Output', '').strip() for r in events if r.get('Package') == row['Package']
                   and r.get('Test') == row['Test'] and r['Action'] == 'output']
        skips.append({'package': row['Package'], 'test': row['Test'], 'reason_output': outputs[-4:]})
audits = []
for row in legacy['audit_dispositions']:
    test = row['canonical_regression']
    audits.append({'id': row['id'], 'disposition': row['disposition'], 'behavior': row['behavior'],
                   'canonical_regression': test, 'canonical_source': source(test['path']),
                   'historical_default_race_terminals': terminals.get(test['name'], []),
                   'reviews': row['reviews'], 'commits_or_followups': row['commits_or_followups'],
                   'evidence': [bound(e['path'], e['sha256']) for e in row['evidence']]})
paths = [
    'server/internal/service/task_knowledge.go', 'server/internal/service/task_knowledge_prepare.go',
    'server/internal/service/task_knowledge_observe.go', 'server/internal/service/task.go',
    'server/internal/handler/daemon.go', 'server/internal/daemon/daemon.go',
    'server/internal/daemon/knowledge_context.go', 'server/pkg/agent/launch_lifecycle.go',
    'server/pkg/agent/knowledge_launch_test.go', 'server/pkg/agent/launch_lifecycle_test.go',
    'server/pkg/protocol/task_knowledge.go', 'server/internal/realtime/hub.go',
    'server/internal/realtime/collaboration.go', 'server/internal/service/collaboration_knowledge.go',
    'server/internal/service/collaboration_knowledge_credentials.go',
    'server/internal/service/collaboration_knowledge_invalidation.go',
    'server/pkg/db/queries/task_knowledge.sql', 'server/pkg/db/queries/collaboration_knowledge_service.sql',
    'server/pkg/db/generated/task_knowledge.sql.go',
    'packages/core/collaboration/knowledge-cache.ts', 'packages/core/collaboration/knowledge-lease.ts',
    'packages/core/collaboration/knowledge-actions.ts', 'packages/core/collaboration/knowledge-recovery.ts',
    'packages/core/collaboration/knowledge-queries.ts', 'packages/views/collaboration/knowledge-panel.tsx',
    'packages/views/collaboration/knowledge-review.tsx', 'packages/views/collaboration/knowledge-proposal.tsx',
    'server/cmd/server/collaboration_knowledge_correction_composite_test.go',
    'server/cmd/server/collaboration_knowledge_host_composite_test.go',
    'server/internal/daemon/knowledge_server_integration_test.go',
    'server/cmd/server/router.go', 'server/internal/service/collaboration_runtime.go',
    'server/internal/service/collaboration_authority.go', 'server/internal/service/collaboration_queue.go',
    'server/internal/service/collaboration_batches.go', 'server/internal/service/collaboration_control.go',
    'server/internal/service/collaboration_mentions.go', 'server/internal/collaboration/access.go',
    'server/pkg/db/generated/collaboration_knowledge_service.sql.go',
    'server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md',
    'server/internal/service/builtin_skills/multica-working-on-issues/references/collaboration-workflow.md',
    '.trellis/spec/server/collaboration-knowledge.md',
    '.trellis/spec/server/collaboration-knowledge-execution.md',
    '.trellis/spec/views/frontend/collaboration-knowledge.md',
]
paths.extend(p for p in git('ls-tree', '-r', '--name-only', OLD, '--', 'server/migrations').decode().splitlines()
             if Path(p).name.startswith(('761_', '762_', '763_')))
source_rows = [source(path) for path in paths]
ccg2_review_path = PARENT / 'knowledge-deployed-probe-20261102/independent-review/review.json'
ccg2_review = read(ccg2_review_path)
ccg2_source = [source(path) for path in ccg2_review['source_sha256']]
composite_path = PARENT / 'final-candidate-20261102/attempt-3-knowledge-composites.json'
composites = []
for row in read(composite_path)['runs']:
    parsed = json_events(row['raw_private_log'])
    observed = {a: sum(r.get('Action') == a and bool(r.get('Test')) for r in parsed)
                for a in ['run', 'pass', 'fail', 'skip']}
    composites.append({key: row[key] for key in ['family', 'mode', 'candidate_sha', 'command', 'database', 'exit_code', 'counts']}
                      | {'raw': bound(row['raw_private_log'], row['log_sha256']),
                         'observed_counts': observed, 'counts_match': observed == row['counts']})
locale = []
for lang in ['en', 'zh-Hans']:
    path = f'packages/views/locales/{lang}/collaboration.json'
    old = json.loads(git('show', OLD + ':' + path))
    new = json.loads(git('show', EXPECTED + ':' + path))
    locale.append({'path': path, 'knowledge_subtree_unchanged': old['knowledge'] == new['knowledge']})
sqlc_path = TASK / 'research/execution-20261103/sqlc/sqlc-resume-01.json'
sqlc = read(sqlc_path)
sqlc_binding = {'record': bound(sqlc_path), 'exit_code': sqlc['exit_code'],
                'source_commit': sqlc['candidate_head'], 'generated_files': sqlc['generated_files_after'],
                'inputs_unchanged_in_run': sqlc['source_inputs_unchanged'],
                'generated_unchanged_in_run': sqlc['generated_unchanged'],
                'log': bound(sqlc['log_path'], sqlc['log_sha256']),
                'server_delta_to_candidate': git('diff', '--name-only', sqlc['candidate_head'], EXPECTED, '--', 'server').decode().splitlines()}

result = {
    'schema_version': 1, 'reviewed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'reviewer': 'native trellis-check /root/knowledge_framework_audit',
    'scope': 'Independent read-only #7 AC1-AC9/21 dispositions and historical source reconciliation; no closure claim',
    'candidate_root': str(CANDIDATE), 'candidate_sha': head, 'comparison_sha': OLD,
    'candidate_status_at_read': git('status', '--porcelain').decode().splitlines(),
    'audit_merge_is_ancestor': subprocess.run(['git', 'merge-base', '--is-ancestor', '6cb9fcb36', head], cwd=CANDIDATE).returncode == 0,
    'legacy_readiness': bound(TASK / 'research/continuation-20261102/knowledge-closure-readiness.json'),
    'independent_reviews': reviews, 'followup_reviews': followups, 'audit_dispositions': audits,
    'source_comparisons': source_rows, 'locale_knowledge_subtrees': locale,
    'historical_default_go': {'candidate_sha': go_meta['candidate_sha'], 'metadata': bound(go_meta_path),
        'raw': bound(raw_path, go_meta['prefix_log_sha256']), 'observed_counts': counts,
        'package_count': go_meta['package_count'], 'compile_only_packages': go_meta['compile_only_package_count'],
        'all_package_databases_unique': go_meta['all_package_databases_unique'],
        'all_executed_packages_used_race': all('-race' in p['command'] for p in go_meta['packages'] if not p['compiled_without_tests']),
        'L7_exact_test_terminals': {name: terminals[name] for name in l7}, 'explicit_skips': skips,
        'qualification': 'Retained prior race evidence only; historical full make check stayed failed. No new whole-candidate pass is inferred.'},
    'historical_composites': composites,
    'CCG1': {'test_source': source('server/cmd/server/collaboration_knowledge_correction_composite_test.go'),
             'review': bound(PARENT / 'knowledge-crosschain-correction-20261102/independent-review/review.json', '592969bbd1cc06039d14224b1808a60379d3bd06c3e096bf9e1d8672ee5a2604'),
             'historical_normal_race_pass': True, 'current_whole_chain_rebound': False,
             'reason': 'Current runtime/queue/target input admission adds AI policy checks; target authorization additionally locks issue_collaboration. Migration764 and generated scans alter the real DB-backed chain. Unchanged CCG1 test bytes do not rebind the old complete execution.'},
    'CCG2': {'harness_sources': ccg2_source,
             'review': bound(ccg2_review_path, 'e442d8b24c93e0c2f9281760fd7f7dcb83d180cc7ea6e60d802c5c0b46a64272'),
             'original_results': bound(PARENT / 'knowledge-deployed-probe-20261102/root-deployed-results.json', '4b741252a8a98745515d760577626ae6aea9e83715a5e31dc460e3d7c7881f1e'),
             'historical_OFF_ON_pass': True, 'current_whole_chain_rebound': False,
             'reason': 'Four main harness files remain byte-identical, but policy-dependent admission, deployed schema/API and current detail UI are different. Old OFF/ON startup observations stay valid only for their original deployments.'},
    'sqlc': sqlc_binding,
    'operations': {'tests_run': False, 'database_operations': False, 'service_operations': False,
                   'provider_calls': 0, 'source_edits': False, 'metadata_edits': False,
                   'old_evidence_edits': False, 'write_scope': str(OUT)},
}
assert len(audits) == 21
assert all(r['hash_match'] for r in reviews + followups)
assert all(e['hash_match'] for a in audits for e in a['evidence'])
assert all(r['raw']['hash_match'] and r['counts_match'] for r in composites)
assert counts == {'pass': 20852, 'fail': 0, 'skip': 22}
assert all(terminals[n] and all(r['Action'] == 'pass' for r in terminals[n]) for n in l7)
(OUT / 'source-evidence.json').write_text(json.dumps(result, indent=2, ensure_ascii=False) + '\n')
print(json.dumps({'audits': len(audits), 'review_hashes': 'match', 'evidence_hashes': 'match',
                  'default_go_counts': counts, 'L7': 'three exact retained pass terminals',
                  'source_comparisons': len(source_rows), 'CCG1_CCG2_current_rebind': False,
                  'sqlc_exit': sqlc_binding['exit_code'], 'sqlc_server_delta': sqlc_binding['server_delta_to_candidate']}))
