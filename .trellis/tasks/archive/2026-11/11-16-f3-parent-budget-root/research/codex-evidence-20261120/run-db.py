import collections,fcntl,json,os,pathlib,subprocess,sys,time
root=pathlib.Path('/tmp/f3-codex-20261120')
c=json.loads((root/'db-private.json').read_text())
key,tag,regex=sys.argv[1:4]
extra=sys.argv[4:]
a=c['allocations'][key]
if any(x in extra for x in ['-overlay','-modfile','-C']): raise SystemExit('Unsupported test override')
lock=(root/'db.lock').open('w')
try: fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
except BlockingIOError: raise SystemExit('DB lane occupied; do other work before retry')
url=c['base_url']+a['database']+'?sslmode=disable'
q=subprocess.run([c['psql'],url,'-AtX','-c','select current_database(); select system_identifier from pg_control_system()'],text=True,capture_output=True)
if q.returncode or q.stdout.splitlines()!=[a['database'],c['system_identifier']]: raise SystemExit('Database identity mismatch')
wt=a.get('worktree','/Volumes/artisan/code/2026/mc-f3-completion-20261120')
guard='/Volumes/artisan/code/2026/mc-f3-completion-20261120/scripts/go-test-with-agent-cli-guard.sh'
env=os.environ.copy();env.update(DATABASE_URL=url,JWT_SECRET=c['jwt_secret'],TMPDIR=os.path.realpath(os.environ.get('TMPDIR','/tmp')))
for k in ['DATABASE_MAX_CONNS','DATABASE_MIN_CONNS','MULTICA_RESOURCE_PUBLISH_DIR','MULTICA_TASK_CONFIG_ROOT','MULTICA_TASK_WORKSPACES_ROOT'] : env.pop(k,None)
cmd=['bash',guard,'--','go','test',a['package'],'-run',regex,'-count=1','-json','-timeout=10m',*extra]
log=root/(tag+'.jsonl')
with log.open('x') as f:r=subprocess.run(cmd,cwd=wt+'/server',env=env,stdout=f,stderr=subprocess.STDOUT)
counts=collections.Counter(); names=[]
for line in log.read_text().splitlines():
 try:e=json.loads(line)
 except ValueError:continue
 if e.get('Test') and e.get('Action') in ['pass','fail','skip']:
  counts[e['Action']]+=1;names.append({'test':e['Test'],'action':e['Action']})
summary={'database':a['database'],'package':a['package'],'exit':r.returncode,'counts':dict(counts),'tests':names,'log':str(log)}
(root/(tag+'.summary.json')).write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps({k:v for k,v in summary.items() if k!='tests'}))
raise SystemExit(r.returncode)
