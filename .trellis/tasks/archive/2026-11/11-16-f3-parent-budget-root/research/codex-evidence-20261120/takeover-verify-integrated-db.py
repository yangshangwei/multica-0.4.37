from pathlib import Path
import json,os,subprocess,hashlib,collections,re,time
out=Path('/tmp/f3-takeover-20261120');root=Path('/Volumes/artisan/code/2026/mc-f3-completion-20261120')
c=json.loads(Path('/tmp/f3-codex-20261120/db-private.json').read_text())
inv=json.loads(Path('/tmp/f3-codex-20261120/acceptance-inventory.json').read_text())
# Include the omitted lifecycle metric and the built-in documentation contracts.
inv['./internal/metrics']['f3']=['TestBusinessMetricsRuntimeSweepStageBudgetLifecycle']
skills=['TestBuiltinSkillsConformToTemplate','TestBuiltinSkillsFrontmatterIsStrictYAML','TestWorkingOnIssuesSkillCoversIssueLoopContracts','TestBuildAgentSkillBundlesProducesRefsAndFullBundles','TestBuildAgentSkillBundlesAssignsBuiltinID']
inv['./internal/service']['f3']=sorted(set(inv['./internal/service']['f3']+skills))
for v in inv.values():v['regex']='^('+'|'.join(sorted(set(v['f3']+v['b54'])))+')$'
(out/'integrated-acceptance-inventory.json').write_text(json.dumps(inv,indent=2)+'\n')
def url(db): return c['base_url']+db+'?sslmode=disable'
def sql(db,s):
 r=subprocess.run([c['psql'],url(db),'-AtX','-v','ON_ERROR_STOP=1','-c',s],capture_output=True,text=True)
 if r.returncode: raise RuntimeError('task database SQL failed; exit '+str(r.returncode))
 return r.stdout.strip()
assert sql('postgres','select system_identifier from pg_control_system()')==c['system_identifier']
owned=[]
def create(db,template):
 assert re.fullmatch(r'f3_budget_cx_takeover_[a-z_]+',db)
 sql('postgres',f'CREATE DATABASE "{db}" TEMPLATE "{template}"')
 owned.append(db);(out/'integrated-databases.json').write_text(json.dumps({'system_identifier':c['system_identifier'],'databases':owned},indent=2)+'\n')
 assert sql(db,'select current_database()')==db
base='f3_budget_cx_takeover_template';create(base,'template0')
env=os.environ.copy();env.update(DATABASE_URL=url(base),JWT_SECRET=c['jwt_secret'],TMPDIR=os.path.realpath(os.environ.get('TMPDIR','/tmp')))
for k in ['DATABASE_MAX_CONNS','DATABASE_MIN_CONNS','MULTICA_RESOURCE_PUBLISH_DIR','MULTICA_TASK_CONFIG_ROOT','MULTICA_TASK_WORKSPACES_ROOT']:env.pop(k,None)
cmd=['bash','scripts/go-test-with-agent-cli-guard.sh','--','go','-C','server','run','./cmd/migrate','up']
with (out/'integrated-template-migrate.log').open('x') as f:r=subprocess.run(cmd,cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
assert r.returncode==0,'migration failed'
proof=sql(base,"select current_database(); show default_transaction_isolation; select count(*),max(version) from schema_migrations; select count(*) from workspace; select count(*) from pg_index where not indisvalid;")
(out/'integrated-template-proof.txt').write_text(proof+'\n');print(proof,flush=True)
results=[]
for mode in ['normal','race']:
 for key,pkg in [('service','./internal/service'),('handler','./internal/handler'),('router','./cmd/server'),('migrations','./internal/migrations'),('migrate','./cmd/migrate')]:
  db='f3_budget_cx_takeover_'+mode+'_'+key;create(db,base);env['DATABASE_URL']=url(db)
  tag='integrated-'+mode+'-'+key;cmd=['bash','scripts/go-test-with-agent-cli-guard.sh','--','go','-C','server','test',pkg,'-run',inv[pkg]['regex'],'-count=1','-p','1','-json','-timeout=10m']
  if mode=='race':cmd+=['-race']
  path=out/(tag+'.jsonl');start=time.time()
  with path.open('x') as f:r=subprocess.run(cmd,cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
  events=[];nonjson=[]
  for line in path.read_text().splitlines():
   try:events.append(json.loads(line))
   except ValueError:nonjson.append(line)
  terminal=[e for e in events if e.get('Action') in ['pass','fail','skip']]
  counts=collections.Counter(e['Action'] for e in terminal if e.get('Test'))
  expected=set(inv[pkg]['f3']+inv[pkg]['b54']);passed={e['Test'] for e in terminal if e.get('Test') and e['Action']=='pass'}
  row={'tag':tag,'package':pkg,'database':db,'exit':r.returncode,'seconds':round(time.time()-start,2),'counts':dict(counts),'missing_expected':sorted(expected-passed),'package_terminal':[e['Action'] for e in terminal if not e.get('Test')],'raw_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'command':cmd,'tests':[{'name':e['Test'],'action':e['Action']} for e in terminal if e.get('Test')]}
  (out/(tag+'.summary.json')).write_text(json.dumps(row,indent=2)+'\n');results.append({k:v for k,v in row.items() if k!='tests'});(out/'integrated-db-summary.json').write_text(json.dumps(results,indent=2)+'\n');print(json.dumps({k:v for k,v in row.items() if k not in ['tests','command']}),flush=True)
  assert r.returncode==0 and not row['missing_expected'] and not counts['skip'] and not counts['fail'] and row['package_terminal']==['pass'],tag
print('INTEGRATED DB ACCEPTANCE PASS',flush=True)
