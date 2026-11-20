from pathlib import Path
import subprocess, os, json, hashlib, collections, datetime, re, sys
out=Path('/tmp/f3-takeover-20261120')
wt=Path('/Volumes/artisan/code/2026/mc-f3-completion-20261120')
invpath=out/'integrated-acceptance-inventory.json'
inv=json.loads(invpath.read_text())
private=out/'nodb-private-home'; private.mkdir(exist_ok=True)
real_tmp=os.path.realpath(os.environ.get('TMPDIR','/tmp'))
go='/opt/homebrew/bin/go'
cache=subprocess.check_output([go,'env','GOCACHE'],text=True).strip()
env={'PATH':'/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin','HOME':str(private),'TMPDIR':real_tmp,'GOMAXPROCS':'2','GOPATH':'/Users/artisan/go','GOMODCACHE':'/Users/artisan/go/pkg/mod','GOCACHE':cache,'GOTOOLCHAIN':'local','GOPROXY':'off','GOSUMDB':'off','GOFLAGS':'-mod=readonly','CGO_ENABLED':'1','DATABASE_URL':'postgres://invalid:invalid@127.0.0.1:1/f3_no_database?sslmode=disable'}
source_paths=subprocess.check_output(['git','ls-files','server','scripts/go-test-with-agent-cli-guard.sh','scripts/agent-cli-command-names.txt'],cwd=wt,text=True).splitlines()
def source():
 return {p:hashlib.sha256((wt/p).read_bytes()).hexdigest() for p in source_paths if (wt/p).is_file()}
before=source()
meta={'cwd':str(wt/'server'),'env':env,'go_version':subprocess.check_output([go,'version'],env=env,text=True).strip(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=wt,text=True).strip(),'merge_head':subprocess.check_output(['git','rev-parse','MERGE_HEAD'],cwd=wt,text=True).strip(),'inventory_sha256':hashlib.sha256(invpath.read_bytes()).hexdigest(),'source_files':before}
(out/'nodb-source-start.json').write_text(json.dumps(meta,indent=2)+'\n')
guard=['bash',str(wt/'scripts/go-test-with-agent-cli-guard.sh'),'--',go]
runs=[]
def run(tag,args,expected=None):
 log=out/(tag+('.jsonl' if expected is not None else '.txt'))
 command=guard+args
 start=datetime.datetime.now(datetime.timezone.utc).isoformat()
 with log.open('x') as f:
  p=subprocess.run(command,cwd=wt/'server',env=env,stdout=f,stderr=subprocess.STDOUT)
 raw=log.read_bytes(); record={'tag':tag,'command':command,'cwd':str(wt/'server'),'env':env,'started_at':start,'finished_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'exit':p.returncode,'log':log.name,'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw),'agent_cli_guard_violations':[]}
 text=raw.decode(errors='replace')
 record['agent_cli_guard_violations']=[line for line in text.splitlines() if 'unexpected agent CLI invocation:' in line]
 bad=p.returncode!=0 or bool(record['agent_cli_guard_violations'])
 if expected is not None:
  counts=collections.Counter();top=collections.Counter();passed=set();failed=[];skipped=[];terminal=[]
  for line in text.splitlines():
   try:e=json.loads(line)
   except ValueError:continue
   action=e.get('Action');test=e.get('Test')
   if test and action in ['pass','fail','skip']:
    counts[action]+=1
    if '/' not in test:top[action]+=1
    if action=='pass':passed.add(test)
    elif action=='fail':failed.append(test)
    else:skipped.append(test)
   if not test and action in ['pass','fail','skip']:terminal.append({'package':e.get('Package'),'action':action})
  record.update(counts=dict(counts),top_level_counts=dict(top),expected_names=expected,missing_names=sorted(set(expected)-passed),failed_names=failed,skipped_names=skipped,package_terminal=terminal)
  bad=bad or bool(record['missing_names']) or bool(failed) or bool(skipped) or len(terminal)!=1 or terminal[0]['action']!='pass'
 record['valid']=not bad;runs.append(record)
 (out/'nodb-summary.json').write_text(json.dumps(runs,indent=2)+'\n')
 (out/(tag+'.summary.json')).write_text(json.dumps(record,indent=2)+'\n')
 print(json.dumps({k:record.get(k) for k in ['tag','exit','counts','top_level_counts','missing_names','package_terminal','valid']}),flush=True)
 if bad:
  print('STOPPED FAILED LANE: '+tag,flush=True);sys.exit(1)
run('nodb-build',['build','-p=1','./...'])
run('nodb-vet',['vet','-p=1','./...'])
metrics=sorted(set(n for f in (wt/'server/internal/metrics').glob('*_test.go') for n in re.findall(r'^func (Test\w+)\(',f.read_text(),re.M)))
packages=['./internal/budget','./cmd/multica','./internal/middleware','./internal/reportsnapshot','./internal/daemon','./internal/metrics']
for mode in ['normal','race']:
 for pkg in packages:
  expected=metrics if pkg=='./internal/metrics' else sorted(set(inv[pkg]['f3']+inv[pkg]['b54']))
  regex='^Test' if pkg=='./internal/metrics' else inv[pkg]['regex']
  args=['test','-p=1',pkg,'-run',regex,'-count=1','-json','-timeout=10m']
  if mode=='race':args.append('-race')
  run('nodb-'+mode+'-'+pkg.rsplit('/',1)[1],args,expected)
after=source();changed=[p for p in before if before[p]!=after.get(p)]
(out/'nodb-source-end.json').write_text(json.dumps({'source_files':after,'changed_since_start':changed,'inventory_unchanged':hashlib.sha256(invpath.read_bytes()).hexdigest()==meta['inventory_sha256']},indent=2)+'\n')
print('ALL COMPLETE; source drift='+str(changed),flush=True)
