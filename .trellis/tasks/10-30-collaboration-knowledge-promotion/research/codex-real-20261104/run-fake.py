#!/usr/bin/env python3
"""Run a compiled fake-only knowledge suite on one recorded database."""
import argparse, hashlib, json, os, pathlib, subprocess, sys, time
from urllib.parse import urlsplit, unquote, parse_qs

p=argparse.ArgumentParser()
p.add_argument('--environment',required=True)
p.add_argument('--binary',required=True)
p.add_argument('--cli',required=True)
p.add_argument('--cli-sha256',required=True)
p.add_argument('--output',required=True)
p.add_argument('--compile-record',required=True)
a=p.parse_args()
root=pathlib.Path.cwd();out=pathlib.Path(a.output).resolve();out.mkdir(parents=True,exist_ok=False)
env=dict(os.environ);env.update(json.loads(pathlib.Path(a.environment).read_text()))
for key in ['MULTICA_RUN_REAL_AGENT_SMOKE','RK1K_CODEX_REGISTRATION','RK1K_CODEX_REGISTRATION_SHA256','DATABASE_MAX_CONNS','DATABASE_MIN_CONNS','MULTICA_TASK_CONFIG_ROOT','MULTICA_TASK_WORKSPACES_ROOT']:
 env.pop(key,None)
url=urlsplit(env['DATABASE_URL']); pg_env=dict(env)
pg_env.update(PGHOST=url.hostname or '',PGPORT=str(url.port or 5432),PGUSER=unquote(url.username or ''),PGPASSWORD=unquote(url.password or ''),PGDATABASE=unquote(url.path.lstrip('/')))
for key,value in parse_qs(url.query).items():
 if key=='sslmode':pg_env['PGSSLMODE']=value[0]
actual=subprocess.check_output(['/opt/homebrew/opt/libpq/bin/psql','-X','-Atqc','SELECT current_database()'],env=pg_env,text=True).strip()
assert actual==env['TASK_DATABASE_NAME'] and actual.startswith('multica_rk1k_'), 'database identity mismatch'
sha=lambda f:hashlib.sha256(pathlib.Path(f).read_bytes()).hexdigest()
assert sha(a.cli)==a.cli_sha256,'CLI hash mismatch'
env.update(RK1K_SOURCE_CLI=str(pathlib.Path(a.cli).resolve()),RK1K_SOURCE_CLI_SHA256=a.cli_sha256,RK1K_FAKE_PROOF_OUT=str(out/'proof'))
def manifest():

 names=subprocess.check_output(['git','ls-files','-co','--exclude-standard','-z','--','server'],cwd=root).split(b'\0')
 return {name.decode():sha(root/name.decode()) for name in sorted(set(names)) if name and (root/name.decode()).is_file()}
before=manifest()
compile_record=json.loads(pathlib.Path(a.compile_record).read_text())
assert compile_record['source_stable'] and compile_record['source']==before and compile_record['binary_sha256']==sha(a.binary), 'compiled source binding mismatch'
(out/'source-before.json').write_text(json.dumps(before,indent=2)+'\n')
cmd=['bash',str(root/'scripts/go-test-with-agent-cli-guard.sh'),'go','tool','test2json','-t','-p','github.com/multica-ai/multica/server/cmd/server',str(pathlib.Path(a.binary).resolve()),'-test.v=test2json','-test.run=^TestRK1KnowledgeCodex','-test.count=1','-test.timeout=20m']
start=time.time()
with (out/'tests.jsonl').open('xb') as raw:
 status=subprocess.run(cmd,cwd=root/'server',env=env,stdout=raw,stderr=subprocess.STDOUT).returncode
after=manifest();(out/'source-after.json').write_text(json.dumps(after,indent=2)+'\n')
terminals=[];invalid=0
for line in (out/'tests.jsonl').read_text(errors='replace').splitlines():
 try:d=json.loads(line)
 except ValueError:
  invalid+=1;continue
 if d.get('Action') in ['pass','fail','skip'] and d.get('Test'):terminals.append({'name':d['Test'],'action':d['Action']})
summary_path=out/'proof/summary.json'
summary=json.loads(summary_path.read_text()) if summary_path.exists() else None
expected={'TestRK1KnowledgeCodexSampleAcknowledgment','TestRK1KnowledgeCodexRPCControls','TestRK1KnowledgeCodexLedgerStopsAfterIncomplete','TestRK1KnowledgeCodexRealPlanDoesNotCarryFakeAnswers','TestRK1KnowledgeCodexPhysicalBudget','TestRK1KnowledgeCodexRegistrationIntegrity','TestRK1KnowledgeCodexRegistrationGuard','TestRK1KnowledgeCodexFakeProof'}
actual_top={t['name'] for t in terminals if '/' not in t['name'] and t['action']=='pass'}
accepted=status==0 and invalid==0 and before==after and actual_top==expected and all(t['action']=='pass' for t in terminals) and summary is not None and summary.get('measured_fake_starts')==15 and summary.get('fake_seed_starts')==3 and summary.get('paid_starts')==0
result={'accepted':accepted,'invalid_json_lines':invalid,'summary_present':summary is not None,'expected_top_matches':actual_top==expected,'exit':status,'database':actual,'duration_seconds':time.time()-start,'source_stable':before==after,'compile_record_sha256':sha(a.compile_record),'binary_sha256':sha(a.binary),'cli_sha256':sha(a.cli),'terminals':terminals,'real_calls':0}
(out/'result.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({'exit':status,'source_stable':before==after,'passed':sum(t['action']=='pass' for t in terminals),'failed':[t['name'] for t in terminals if t['action']=='fail'],'skipped':sum(t['action']=='skip' for t in terminals),'output':str(out)},indent=2))
sys.exit(status if status else 0 if accepted else 3)
