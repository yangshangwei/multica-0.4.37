import pathlib,os,json,subprocess,hashlib,time,signal,sys
p=pathlib.Path(__file__).parent
role=sys.argv[1]
assert role in ['reporter','reviewer','leader']
ledger=p/'model-trials.json'
runs=json.loads(ledger.read_text()) if ledger.exists() else []
assert len(runs)<3 and not any(r.get('stop_remaining') for r in runs), 'model budget stopped'
t=json.loads((p/(role+'-trial.json')).read_text())
assert not (p/(role+'-codex-events.jsonl')).exists(), 'retain original run'
env={k:v for k,v in t['env'].items()}
env['CODEX_HOME']=str(p/'codex-private');env['PATH']=str(p/'bin')+':'+os.environ['PATH'];env['TMPDIR']=str(pathlib.Path(t['cwd'])/'tmp');pathlib.Path(env['TMPDIR']).mkdir(exist_ok=True)
env['LANG']='en_US.UTF-8'
cmd=['/Users/artisan/.nvm/versions/node/v22.23.2/bin/codex','exec','--ephemeral','--ignore-rules','--skip-git-repo-check','--sandbox','workspace-write','--json','--color','never','-C',t['cwd'],'--output-last-message',str(p/(role+'-codex-final.txt')),'-']
prompt=pathlib.Path(t['prompt_file']).read_bytes()
record={'role':role,'argv':cmd,'prompt_sha256':hashlib.sha256(prompt).hexdigest(),'started_at':time.time(),'timeout_seconds':180,'status':'started'};runs.append(record);ledger.write_text(json.dumps(runs,indent=2)+'\n')
start=time.monotonic()
with (p/(role+'-codex-events.jsonl')).open('wb') as out,(p/(role+'-codex-stderr.log')).open('wb') as err:
 proc=subprocess.Popen(cmd,env=env,stdin=subprocess.PIPE,stdout=out,stderr=err,start_new_session=True)
 try:proc.communicate(prompt,timeout=180);record['exit']=proc.returncode
 except subprocess.TimeoutExpired:
  os.killpg(proc.pid,signal.SIGTERM)
  try:proc.wait(timeout=8)
  except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);proc.wait()
  record['exit']=124;record['timeout']=True
record['elapsed_seconds']=round(time.monotonic()-start,3)
text=(p/(role+'-codex-events.jsonl')).read_text(errors='replace')+'\n'+(p/(role+'-codex-stderr.log')).read_text(errors='replace')
record['stop_remaining']=record['exit']!=0 and any(s in text.lower() for s in ['403','401','unauthorized','forbidden','authentication','gateway','429'])
record['status']='passed_process' if record['exit']==0 else 'execution_failed'
record['event_sha256']=hashlib.sha256((p/(role+'-codex-events.jsonl')).read_bytes()).hexdigest()
ledger.write_text(json.dumps(runs,indent=2)+'\n');print(json.dumps(record));sys.exit(0)
