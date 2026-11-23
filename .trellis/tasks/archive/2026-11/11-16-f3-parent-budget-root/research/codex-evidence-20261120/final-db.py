from pathlib import Path
import json,subprocess,sys,time
out=Path('/tmp/f3-codex-20261120'); inv=json.loads((out/'acceptance-inventory.json').read_text())
run_id=sys.argv[1]
all_results=[]
for mode in ['normal','race']:
 for key,pkg in [('service','./internal/service'),('handler','./internal/handler'),('router','./cmd/server'),('migrations','./internal/migrations'),('migrate','./cmd/migrate')]:
  tag=run_id+'-'+mode+'-'+key
  regex=inv[pkg]['regex']
  args=['python3',str(out/'run-db.py'),'final_'+mode+'_'+key,tag,regex]
  if mode=='race':args+=['-race']
  r=subprocess.run(args)
  result=json.loads((out/(tag+'.summary.json')).read_text())
  expected=set(inv[pkg]['f3']+inv[pkg]['b54']); passed={x['test'] for x in result['tests'] if x['action']=='pass'}
  result['missing_expected']=sorted(expected-passed)
  all_results.append({k:v for k,v in result.items() if k!='tests'})
  (out/(run_id+'-summary.json')).write_text(json.dumps(all_results,indent=2)+'\n')
  if r.returncode or result['missing_expected'] or result['counts'].get('skip',0):
   print('FINAL CHECK STOPPED',tag,'missing',result['missing_expected'])
   raise SystemExit(1)
print('FINAL DB ACCEPTANCE PASS')
