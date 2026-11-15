from pathlib import Path
import json,subprocess,datetime,hashlib
root=Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
out=root/'.omx/private/final-check-v12'
env=json.loads((out/'preflight-process-environment.json').read_text())
command=['make','up','ENV_FILE='+str(out/'preflight-source.env'),'C=api,web','ARGS=--name closure-v12-preflight --ephemeral --web-mode production']
record={'command':command,'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'state':'running'}
with (out/'preflight-up.log').open('x') as log:
 p=subprocess.Popen(command,cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT);record['pid']=p.pid;(out/'preflight-up.json').write_text(json.dumps(record,indent=2)+'\n');code=p.wait()
record.update(state='complete',exit_code=code,finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),log_sha256=hashlib.sha256((out/'preflight-up.log').read_bytes()).hexdigest());(out/'preflight-up.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record));raise SystemExit(code)
