#!/usr/bin/env python3
"""Manage only the PIDs recorded by the task-private QA allocation."""
import hashlib,json,os,signal,subprocess,sys,time,urllib.request
from pathlib import Path
repo=next(p for p in Path(__file__).resolve().parents if (p/'artifacts/qa/collaboration-multi-account-20261109/environment/allocation.json').is_file())
allocation=repo/'artifacts/qa/collaboration-multi-account-20261109/environment/allocation.json'
a=json.loads(allocation.read_text());p=Path(a['private_root']);env=json.loads(Path(a['env_file']).read_text());source=Path(a['snapshot']);state=p/'state/services.json'
records=json.loads(state.read_text()) if state.exists() else {}
def get(url):
 with urllib.request.urlopen(url,timeout=5) as r:return r.read()
def start(name,cmd,cwd):
 if name in records:
  try:os.kill(records[name]['pid'],0);return
  except ProcessLookupError:pass
 log=(p/'logs'/f'{name}.log').open('ab')
 process=subprocess.Popen(cmd,cwd=cwd,env=env,stdout=log,stderr=subprocess.STDOUT,stdin=subprocess.DEVNULL,start_new_session=True);log.close()
 records[name]={'pid':process.pid,'command':cmd,'cwd':str(cwd),'started_at':time.time()};state.write_text(json.dumps(records,indent=2)+'\n')
if sys.argv[1]=='start':
 if json.loads((p/'state/migrate.json').read_text())['exit_code']!=0:raise SystemExit('Migration did not pass')
 for name in ['api','web','renderer']:
  build={'api':'api-build','web':'web-build','renderer':'renderer-build'}[name]
  if json.loads((p/'state'/f'{build}.json').read_text())['exit_code']!=0:raise SystemExit('Build did not pass: '+name)
 start('api',[str(p/'bin/api')],source/'server')
 start('web',['pnpm','--filter','@multica/web','exec','next','start','--hostname','127.0.0.1','--port',str(a['web_port'])],source)
 start('renderer',['node',str(source/'scripts/check-desktop-renderer.mjs'),str(source/'apps/desktop/out/renderer'),str(a['renderer_port']),str(p/'state/renderer.running.json')],source)
 print('Started task-owned processes', {n:r['pid'] for n,r in records.items()})
elif sys.argv[1]=='verify':
 health=json.loads(get(a['api_url']+'/health'));config=json.loads(get(a['api_url']+'/api/config'))
 if health.get('pid')!=records['api']['pid'] or health.get('commit')!=a['candidate_sha']:raise SystemExit('API identity mismatch')
 if config.get('auth_mode')!='password' or config.get('deployment_id')!=a['deployment_id'] or config.get('managed_installation_supported') is not True:raise SystemExit('API configuration mismatch')
 html=get(a['web_url']+'/login').decode();rendered=get(a['renderer_url']+'/')
 if rendered!=(source/'apps/desktop/out/renderer/index.html').read_bytes():raise SystemExit('Renderer source mismatch')
 if a['api_url'] not in html or env['NEXT_PUBLIC_WS_URL'] not in html:raise SystemExit('Web runtime API/WS URL mismatch')
 for name,port in [('api',a['api_port']),('web',a['web_port']),('renderer',a['renderer_port'])]:
  listening=subprocess.check_output(['lsof','-nP',f'-iTCP:{port}','-sTCP:LISTEN','-t'],text=True).splitlines()
  if len(set(listening))!=1:raise SystemExit('Unexpected listener count')
  pid=int(listening[0]);pgid=os.getpgid(pid)
  if pgid!=records[name]['pid']:raise SystemExit('Unowned listener')
  records[name]['listener_pid']=pid
 a['api_config']={k:config.get(k) for k in ['auth_mode','deployment_id','managed_installation_supported']};a['web_runtime_urls']={'apiBaseUrl':a['api_url'],'wsUrl':env['NEXT_PUBLIC_WS_URL'],'observed_in_login_html':True};a['state']='ready';a['services']=records;a['health']=health;a['web_build_id']=(source/'apps/web/.next/BUILD_ID').read_text().strip();a['binary_hashes']={b.name:hashlib.sha256(b.read_bytes()).hexdigest() for b in (p/'bin').iterdir() if b.is_file()};a['env_sha256']=hashlib.sha256(Path(a['env_file']).read_bytes()).hexdigest()
 state.write_text(json.dumps(records,indent=2)+'\n');allocation.write_text(json.dumps(a,indent=2)+'\n');print(json.dumps({'state':a['state'],'health':health,'web_build_id':a['web_build_id'],'services':{n:r['listener_pid'] for n,r in records.items()}}))
elif sys.argv[1]=='stop':
 for name,r in records.items():
  try:
   if os.getpgid(r['pid'])!=r['pid']:raise SystemExit('Process group ownership mismatch')
   current=subprocess.check_output(['ps','-p',str(r['pid']),'-o','command='],text=True).strip()
   if str(r['command'][0]) not in current and name!='web':raise SystemExit('PID command identity changed')
   os.killpg(r['pid'],signal.SIGTERM)
  except (ProcessLookupError,subprocess.CalledProcessError):pass
 a['state']='stop_requested';allocation.write_text(json.dumps(a,indent=2)+'\n');print('Stop requested for recorded task processes only')
else:raise SystemExit('Expected start, verify, or stop')
