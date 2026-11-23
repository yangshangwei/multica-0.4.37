from pathlib import Path
import subprocess,os,json,collections
out=Path('/tmp/f3-codex-20261120');wt=Path('/Volumes/artisan/code/2026/mc-f3-completion-20261120');inv=json.loads((out/'acceptance-inventory.json').read_text());env=os.environ.copy();env['DATABASE_URL']='postgres://invalid:invalid@127.0.0.1:1/f3_no_database?sslmode=disable';env['TMPDIR']=os.path.realpath(env.get('TMPDIR','/tmp'))
runs=[]
for mode in ['race']:
 for pkg in ['./internal/middleware','./internal/reportsnapshot','./internal/daemon']:
  tag='final-r3-'+mode+'-'+pkg.rsplit('/',1)[1]
  cmd=['bash',str(wt/'scripts/go-test-with-agent-cli-guard.sh'),'--','go','test',pkg,'-run',inv[pkg]['regex'],'-count=1','-json','-timeout=10m']
  if mode=='race':cmd+=['-race']
  log=out/(tag+'.jsonl')
  with log.open('x') as f:r=subprocess.run(cmd,cwd=wt/'server',env=env,stdout=f,stderr=subprocess.STDOUT)
  c=collections.Counter();passed=set()
  for line in log.read_text().splitlines():
   try:e=json.loads(line)
   except ValueError:continue
   if e.get('Test') and e.get('Action') in ['pass','fail','skip']:
    c[e['Action']]+=1
    if e['Action']=='pass':passed.add(e['Test'])
  missing=sorted(set(inv[pkg]['f3']+inv[pkg]['b54'])-passed)
  result={'tag':tag,'exit':r.returncode,'counts':dict(c),'missing':missing};runs.append(result);print(json.dumps(result),flush=True)
  (out/'final-r3-nodb-summary.json').write_text(json.dumps(runs,indent=2)+'\n')
  if r.returncode or missing or c['skip']:raise SystemExit(1)
