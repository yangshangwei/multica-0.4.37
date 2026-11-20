from pathlib import Path
import subprocess,json,time,hashlib,os
root=Path('/Volumes/artisan/code/2026/mc-f3-completion-20261120');out=Path('/tmp/f3-takeover-20261120');runs=[]
for package,kind,args in [('core','typecheck',['node','node_modules/typescript/bin/tsc','--noEmit']),('core','lint',['node',str(root/'node_modules/eslint/bin/eslint.js'),'.']),('core','tests',['node','node_modules/vitest/vitest.mjs','run','--maxWorkers=2']),('views','typecheck',['node','node_modules/typescript/bin/tsc','--noEmit']),('views','tests',['node','node_modules/vitest/vitest.mjs','run','--maxWorkers=2'])]:
 name='integrated-'+package+'-'+kind;path=out/(name+'.txt');start=time.time()
 with path.open('x') as f:r=subprocess.run(args,cwd=root/'packages'/package,stdout=f,stderr=subprocess.STDOUT)
 row={'name':name,'command':args,'cwd':str(root/'packages'/package),'exit':r.returncode,'seconds':round(time.time()-start,2),'raw_sha256':hashlib.sha256(path.read_bytes()).hexdigest()};runs.append(row);(out/'integrated-frontend-summary.json').write_text(json.dumps(runs,indent=2)+'\n');print(json.dumps(row),flush=True)
 if r.returncode:raise SystemExit(r.returncode)
