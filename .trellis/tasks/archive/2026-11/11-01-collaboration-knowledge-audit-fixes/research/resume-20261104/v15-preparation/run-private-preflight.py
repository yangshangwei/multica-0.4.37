# Execution coordinator: 01a1a3c9-2459-7de1-8244-55640c5f6143; template only, no execution authorization.
from pathlib import Path
import json, subprocess, hashlib, os, sys
from datetime import datetime, timezone
ROOT=Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT=ROOT/'.omx/private/final-check-v15'
HERE=Path(__file__).resolve().parent
name=sys.argv[1]
paths={'launcher':OUT/'fake-preflight-v15-provenance.py','adapter-env':OUT/'adapter-env-preflight-v15.py','final-audit':HERE/'final-state-audit-v15/final_state_audit_test.py'}
source=paths[name]
PYTHON='/opt/homebrew/opt/python@3.14/bin/python3.14'
env={'PATH':'/Users/artisan/.nvm/versions/node/v22.23.2/bin:/opt/homebrew/opt/python@3.14/bin:/usr/bin:/bin:/usr/sbin:/sbin','HOME':str(OUT/'private-preflight-home'),'PYTHONDONTWRITEBYTECODE':'1','LC_ALL':'C','CHECK_LAUNCHER_SOURCE':str(OUT/'run-make-check-v15-provenance.py')}
(OUT/'private-preflight-home').mkdir(mode=0o700,exist_ok=True)
command=['/usr/bin/sandbox-exec','-f',str(OUT/'private-preflight-offline.sb'),PYTHON,str(source)]
record={'name':name,'candidate_sha':'d41573e3a4eb3bd6c36feed3f0d542a0e066ee0f','started_at':datetime.now(timezone.utc).isoformat(),'command':command,'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'scope':'private fake subprocess and filesystem controls; no repository suites/database/Provider','network':'denied'}
log=HERE/(name+'-private-preflight.log')
with log.open('x') as sink:result=subprocess.run(command,cwd=ROOT,env=env,stdout=sink,stderr=subprocess.STDOUT)
record.update(exit_code=result.returncode,finished_at=datetime.now(timezone.utc).isoformat(),log_path=str(log),log_sha256=hashlib.sha256(log.read_bytes()).hexdigest())
with (HERE/(name+'-private-preflight.json')).open('x') as f:json.dump(record,f,indent=2);f.write('\n')
print(json.dumps(record))
raise SystemExit(result.returncode)
