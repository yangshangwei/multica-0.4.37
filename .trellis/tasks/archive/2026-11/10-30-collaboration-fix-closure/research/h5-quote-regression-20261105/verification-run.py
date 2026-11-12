#!/usr/bin/env python3
import argparse, datetime, hashlib, json, os, subprocess, tempfile
from pathlib import Path
from urllib.parse import urlsplit,urlunsplit
ROOT=Path('/Volumes/artisan/code/2026/mc-h5-quote-20261105')
OUT=ROOT/'.omx/private/h5-quote'
MAIN=Path('/Volumes/artisan/code/2026/multica-0.4.37')
PUBLIC=MAIN/'.trellis/tasks/10-30-collaboration-fix-closure/research/h5-quote-regression-20261105'
GO=Path('/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go')
PSQL='/opt/homebrew/opt/libpq/bin/psql'
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def sha(p):return hashlib.sha256(Path(p).read_bytes()).hexdigest()
def save(p,d):
 with Path(p).open('x') as f:json.dump(d,f,indent=2);f.write('\n')
 Path(p).chmod(0o600)
def query(url,sql):
 r=subprocess.run([PSQL,'-XAt','-d',url,'-v','ON_ERROR_STOP=1','-c',sql],capture_output=True,text=True)
 if r.returncode:raise RuntimeError('DB command failed; details retained privately')
 return r.stdout.strip()
def env_for(run,url):
 for name in ['home','gopath','proof']:(run/name).mkdir(mode=0o700)
 return {'PATH':str(GO.parent)+':/opt/homebrew/opt/libpq/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin','HOME':str(run/'home'),'TMPDIR':tempfile.mkdtemp(prefix='multica-h5quote-',dir='/private/tmp'),'DATABASE_URL':url,'RK1_FAKE_PROOF_OUT':str(run/'proof'),'GOROOT':str(GO.parent.parent),'GOCACHE':'/Volumes/artisan/dev-cache/go-build','GOMODCACHE':'/Users/artisan/go/pkg/mod','GOPATH':str(run/'gopath'),'GOTOOLCHAIN':'local','GOENV':'off','GOPROXY':'off','GOSUMDB':'off','GOTELEMETRY':'off','GOMAXPROCS':'2','GOFLAGS':'-p=2','LANG':'en_US.UTF-8'}
def snapshot():
 names=subprocess.check_output(['git','ls-files','server'],cwd=ROOT,text=True).splitlines()
 return {f:sha(ROOT/f) for f in names if (ROOT/f).is_file()}
def main():
 a=argparse.ArgumentParser();a.add_argument('mode',choices=['mutation','normal','race']);args=a.parse_args();os.umask(0o077)
 permit=json.loads((OUT/'execution-permit.json').read_text());assert permit['approved'] is True and permit['runner_sha256']==sha(__file__)
 binding=permit['source_hashes']
 for f,h in binding.items():assert sha(ROOT/f)==h,f
 run=OUT/args.mode;run.mkdir(mode=0o700)
 original=json.loads((OUT/'database-base.json').read_text())['DATABASE_URL'];parts=urlsplit(original)
 database='multica_rk1_h5quote_20261105_'+args.mode+'_a728';url=urlunsplit(parts._replace(path='/'+database));admin=urlunsplit(parts._replace(path='/postgres'))
 count,limit=map(int,query(admin,"SELECT count(*),current_setting('max_connections') FROM pg_stat_activity").split('|'));assert limit-count>=35
 assert query(admin,"SELECT count(*) FROM pg_database WHERE datname='"+database+"'")=='0'
 query(admin,'CREATE DATABASE '+database+' TEMPLATE template0')
 assert query(url,'SELECT current_database()')==database
 env=env_for(run,url);save(run/'private-environment.json',env)
 migration=[str(GO),'run','./cmd/migrate','up']
 with (run/'migration.log').open('x') as log:mig=subprocess.run(migration,cwd=ROOT/'server',env=env,stdout=log,stderr=subprocess.STDOUT)
 assert mig.returncode==0,'migration failed; retained log'
 identity=query(url,'SELECT current_database(),max(version) FROM schema_migrations');assert identity==database+'|764_collaboration_ai_policy',identity
 before=snapshot();save(run/'source-before.json',before)
 selector='^TestRK1FakeProof$/(^H5$|^canary_obeyed_blocks$|^canary_quoted_without_writes_blocks$)'
 cmd=['bash','scripts/go-test-with-agent-cli-guard.sh','--',str(GO),'-C','server','test','./cmd/server','-tags=agentintegration','-run',selector,'-count=1','-parallel=1','-json','-timeout=10m']
 if args.mode=='race':cmd.append('-race')
 if args.mode=='mutation':
  source=ROOT/'server/cmd/server/rk1_scenarios_recall_test.go';text=source.read_text();old='leaked := strings.Contains(o.answer(), canary)';assert text.count(old)==1
  changed=run/'rk1_scenarios_recall_test.go';changed.write_text(text.replace(old,'leaked := false'));overlay=run/'overlay.json';save(overlay,{'Replace':{str(source):str(changed)}});cmd.extend(['-overlay',str(overlay)])
 record={'started_at':now(),'mode':args.mode,'command':cmd,'candidate':str(ROOT),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'database':database,'database_identity':identity,'migration_exit':mig.returncode,'connections_before':[count,limit],'go_sha256':sha(GO),'runner_sha256':sha(__file__),'source_before_sha256':sha(run/'source-before.json'),'real_provider_calls':0}
 save(run/'started.json',record);print(json.dumps({'state':'running','mode':args.mode,'database':database}),flush=True)
 with (run/'tests.jsonl').open('x') as log,(run/'stderr.log').open('x') as err:r=subprocess.run(cmd,cwd=ROOT,env=env,stdout=log,stderr=err)
 rows=[];terminals=[]
 for line in (run/'tests.jsonl').read_text().splitlines():
  try:e=json.loads(line)
  except ValueError:continue
  rows.append(e)
  if e.get('Test') and e.get('Action') in ['pass','fail','skip']:terminals.append({k:e[k] for k in ['Test','Action']})
 after=snapshot();assert before==after,'source changed duringrun'
 trials=[]
 for p in sorted((run/'proof').glob('*/RK1-H5-t*/public.json')):
  d=json.loads(p.read_text());trials.append({'path':str(p),'sha256':sha(p),'label':d['label'],'fault':d.get('fault'),'status':d['status'],'verdict':d['verdict'],'answer':d['answer'],'criteria':d['criteria'],'runs':d['runs']})
 result={**record,'finished_at':now(),'exit_code':r.returncode,'terminals':terminals,'trials':trials,'source_unchanged':True,'raw_log':str(run/'tests.jsonl'),'raw_log_sha256':sha(run/'tests.jsonl'),'database_connections_after':query(admin,"SELECT count(*) FROM pg_stat_activity WHERE datname='"+database+"'")}
 save(run/'raw-result.json',result)
 names={x['Test']:x['Action'] for x in terminals}
 prefix='TestRK1FakeProof/'
 required=['H5','canary_obeyed_blocks','canary_quoted_without_writes_blocks']
 assert all(prefix+x in names for x in required),'missing exact test terminal'
 assert not any(x['Action']=='skip' for x in terminals),'unexpected skipped tests'
 assert names[prefix+'H5']=='pass' and names[prefix+'canary_obeyed_blocks']=='pass','existing controls changed'
 quote=[x for x in trials if x['fault']=='quote_only']
 if args.mode=='mutation':
  accepted=r.returncode!=0 and names[prefix+'canary_quoted_without_writes_blocks']=='fail' and len(quote)==2 and all(x['status']=='finished' and x['verdict']=='pass' for x in quote)
 else:
  accepted=r.returncode==0 and names[prefix+'canary_quoted_without_writes_blocks']=='pass' and len(quote)==1 and quote[0]['status']=='finished' and quote[0]['verdict']=='blocking_review'
 assert quote and all(x['criteria']['stored_rows_with_canary']==0 and x['criteria']['measured_business_failure'] is False and len(x['runs'])==1 and x['runs'][0]['business_writes']==0 and x['runs'][0]['physical_launches']==1 for x in quote),'quote-only isolation missing'
 result['expected_result_observed']=accepted
 save(run/'result.json',result);save(PUBLIC/(args.mode+'-result.json'),result);print(json.dumps({'mode':args.mode,'exit_code':r.returncode,'expected_result_observed':accepted,'terminals':terminals,'trials':[(x['label'],x['verdict']) for x in trials]}),flush=True)
 return 0 if accepted else 1
if __name__=='__main__':raise SystemExit(main())
