from pathlib import Path
from datetime import datetime, timezone
import argparse
import hashlib
import json
import runpy
import subprocess
from urllib.parse import urlsplit
from urllib.request import urlopen

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/failure-repair-preflight-v13'
EVIDENCE = Path(__file__).resolve().parent
OLD_EVIDENCE = EVIDENCE.parents[1] / 'execution-20261103'
SPEC = 'e2e/collaboration-preparation-records-web.spec.ts'
m = runpy.run_path(str(OUT / 'start-environment.py'), run_name='preflight_definitions')
require, digest, save = m['require'], m['digest'], m['save']
parser = argparse.ArgumentParser()
parser.add_argument('label')
parser.add_argument('--sha256', required=True)
parser.add_argument('--full', action='store_true')
parser.add_argument('--manifest-sha256', required=True)
args = parser.parse_args()
require(args.label.startswith('assertions-') and args.label.replace('-', '').isalnum(), 'Invalid numbered run label')
RUN = OUT / 'runs' / args.label
require(not RUN.exists(), 'Never overwrite run evidence')
binding = json.loads((OUT.parent / 'final-check-v13/owned-bindings-v13-provenance.json').read_text())
applied = json.loads((OLD_EVIDENCE / 'applied-failure-repairs.json').read_text())
baseline = json.loads((OUT / 'runs/round1/source-before.json').read_text())
require(digest(EVIDENCE / 'source-before.ts') == baseline['approved_overlay'][SPEC], 'Original spec snapshot changed')
require(digest(ROOT / SPEC) == args.sha256, 'Spec bytes do not match this invocation')
manifest_path = EVIDENCE / 'assertions/after.json'
require(digest(manifest_path) == args.manifest_sha256, 'Assertion restoration manifest changed')
restored = json.loads(manifest_path.read_text())
require(set(restored) == {'e2e/collaboration-workbench-web.spec.ts', 'e2e/collaboration-memory-web.spec.ts', 'e2e/fixtures/collaboration-mention-journeys.ts', 'e2e/collaboration-preview-web.spec.ts'}, 'Unexpected assertion restoration scope')
for row in applied['files']:
    if row['path'] == SPEC:
        row['proposed_sha256'] = args.sha256
    elif row['path'] in restored:
        require(row['proposed_sha256'] == restored[row['path']]['before_sha256'], 'Prior reviewed assertion source changed')
        row['proposed_sha256'] = restored[row['path']]['after_sha256']
source = m['capture_source'](binding, applied)
runtime = json.loads((OUT / 'runtime-started.json').read_text())
require(source['runtime_product_source_id'] == runtime['runtime_product_source_id'], 'Runtime product changed')
environment = json.loads((OUT / 'runs/round1/test-environment.json').read_text())
environment['PLAYWRIGHT_JSON_OUTPUT_FILE'] = str(RUN / 'results.json')
require(environment['MULTICA_KNOWLEDGE_CONTEXT_ENABLED'] == 'false', 'Knowledge flag changed')
for key in ('ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'OPENAI_BASE_URL', 'CLAUDE_CODE_OAUTH_TOKEN', 'GEMINI_API_KEY', 'GOOGLE_API_KEY', 'MULTICA_LLM_API_KEY', 'MULTICA_LLM_BASE_URL', 'MULTICA_RUN_REAL_AGENT_SMOKE'):
    require(not environment.get(key), 'Paid provider configuration is forbidden')
aliases = {key: urlsplit(value).path.lstrip('/') for key, value in environment.items() if 'DATABASE_URL' in key or key.endswith('DB_URL')}
require(aliases and all(value == runtime['database'] for value in aliases.values()), 'Database alias mismatch')
require(digest(Path(runtime['source_environment_path'])) == runtime['source_environment_sha256'], 'Source environment changed')
registry = Path(runtime['registry'])
require(digest(registry / 'manifest.env') == runtime['manifest_sha256'], 'Registry changed')
live = {}
for component in ('api', 'web'):
    record = json.loads((registry / (component + '.running.json')).read_text())
    require(record == runtime[component]['running'], 'Listener record changed: ' + component)
    pid = record['listener_pid']
    details = subprocess.check_output(['ps', '-p', str(pid), '-o', 'pid=,ppid=,etime=,command='], text=True).strip()
    cwd = subprocess.check_output(['lsof', '-a', '-p', str(pid), '-d', 'cwd', '-Fn'], text=True)
    require(str(ROOT) in cwd, 'Listener cwd changed')
    live[component] = {'pid': pid, 'process': details, 'cwd': cwd}
with urlopen(runtime['api']['origin'] + '/health', timeout=10) as response:
    health = json.load(response)
require(health == runtime['api']['health'], 'API health identity changed')
with urlopen(runtime['web']['origin'] + '/account', timeout=10) as response:
    html = response.read()
require(runtime['web']['rendered_API_URL'].encode() in html and runtime['web']['rendered_WS_URL'].encode() in html, 'Web browser origins changed')
with urlopen(runtime['renderer']['origin'], timeout=10) as response:
    require(hashlib.sha256(response.read()).hexdigest() == runtime['renderer']['index_sha256'], 'Renderer identity changed')
actual_db = m['database_query'](binding['tools']['psql']['path'], environment['DATABASE_URL'], "SELECT current_database() || '|' || (SELECT max(version) FROM schema_migrations)", runtime['database'])
require(actual_db == runtime['database_identity'], 'Actual database identity changed')
retained = {str(p): digest(p) for p in (OUT / 'runs/round1').rglob('*') if p.is_file()}
inventory = json.loads((OLD_EVIDENCE / 'failure-repair-review/directed-inventory.json').read_text())
planned = inventory['prospective_file_scope_command'] if args.full else ['pnpm', 'exec', 'playwright', 'test', SPEC, '--workers=1', '--retries=0', '--reporter=line,json']
command = ['bash', str(ROOT / 'scripts/go-test-with-agent-cli-guard.sh'), '--', binding['tools']['node']['path'], binding['tools']['pnpm']['path'], *planned[1:], '--output=' + str(RUN / 'artifacts')]
RUN.mkdir()
save(RUN / 'source-before.json', source)
save(RUN / 'environment.json', environment)
(EVIDENCE / (args.label + '-source.ts')).write_bytes((ROOT / SPEC).read_bytes())
record = {'assertion_manifest_sha256': args.manifest_sha256, 'started_at': datetime.now(timezone.utc).isoformat(), 'label': args.label, 'state': 'running', 'command': command, 'runtime': live, 'health': health, 'database': actual_db, 'source_id': source['development_source_id'], 'runtime_product_source_id': source['runtime_product_source_id'], 'spec_sha256': args.sha256, 'runner_sha256': digest(Path(__file__)), 'environment_sha256': digest(RUN / 'environment.json'), 'paid_calls': 0, 'wholecheck_acceptance': False, 'original_round1_sha256': retained}
with (RUN / 'run.log').open('x') as log:
    process = subprocess.Popen(command, cwd=ROOT, env=environment, stdout=log, stderr=subprocess.STDOUT)
    record['pid'] = process.pid
    save(RUN / 'start.json', record)
    print(json.dumps({'label': args.label, 'pid': process.pid, 'path': str(RUN)}), flush=True)
    code = process.wait()
record.update(state='complete', exit_code=code, finished_at=datetime.now(timezone.utc).isoformat(), log_sha256=digest(RUN / 'run.log'))
after = m['capture_source'](binding, applied)
save(RUN / 'source-after.json', after)
record['source_unchanged'] = source['development_source_id'] == after['development_source_id']
record['old_round1_unchanged'] = all(digest(Path(p)) == h for p, h in retained.items())
if (RUN / 'results.json').exists():
    record['results_sha256'] = digest(RUN / 'results.json')
    record['stats'] = json.loads((RUN / 'results.json').read_text())['stats']
save(RUN / 'result.json', record)
save(EVIDENCE / (args.label + '-result.json'), record)
print(json.dumps({key: record.get(key) for key in ('label', 'exit_code', 'stats', 'source_unchanged', 'old_round1_unchanged')}))
raise SystemExit(code)
