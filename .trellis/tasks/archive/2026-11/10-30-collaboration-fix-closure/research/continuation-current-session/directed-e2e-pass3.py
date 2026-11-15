from pathlib import Path
import json,subprocess,datetime,hashlib,importlib.util,urllib.request
ROOT=Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103');OUT=ROOT/'.omx/private/final-check-v12'
spec=importlib.util.spec_from_file_location('launcher',OUT/'run-make-check-v12-provenance.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
env=json.loads((OUT/'preflight-process-environment.json').read_text());env.update(m.parse_generated_environment((OUT/'preflight-source.env').read_text()))
env.update(COLLABORATION_E2E_API_URL=env['NEXT_PUBLIC_API_URL'],COLLABORATION_ELECTRON_SOURCE_ROOT=str(ROOT),PLAYWRIGHT_BROWSERS_PATH='/Users/artisan/Library/Caches/ms-playwright',PLAYWRIGHT_JSON_OUTPUT_FILE=str(OUT/'directed-e2e-pass3-results.json'))
for url in [env['NEXT_PUBLIC_API_URL']+'/health',env['PLAYWRIGHT_BASE_URL']+'/account',env['COLLABORATION_ELECTRON_RENDERER_URL']]:
 with urllib.request.urlopen(url,timeout=20) as response:assert response.status==200
node='/Users/artisan/.nvm/versions/node/v22.23.2/bin/node';pnpm=str(ROOT/'.omx/private/owned-workspace-links/pnpm-10.28.2/bin/pnpm.cjs')
command=['bash',str(ROOT/'scripts/go-test-with-agent-cli-guard.sh'),'--',node,pnpm,'exec','playwright','test','e2e/collaboration-account-desktop.spec.ts','e2e/collaboration-editing-web.spec.ts','e2e/collaboration-delivery-web.spec.ts','--grep','Desktop authors initial fields|Web explicitly resubmits rejected creation|Web source creation, field edits','--project=chromium','--workers=1','--retries=0','--reporter=line,json','--output='+str(OUT/'directed-e2e-pass3-artifacts')]
record={'candidate_sha':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'command':command,'state':'running','started_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'web_url':env['PLAYWRIGHT_BASE_URL'],'api_url':env['NEXT_PUBLIC_API_URL'],'renderer_url':env['COLLABORATION_ELECTRON_RENDERER_URL']}
with (OUT/'directed-e2e-pass3.log').open('x') as log:
 p=subprocess.Popen(command,cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT);record['pid']=p.pid;(OUT/'directed-e2e-pass3.json').write_text(json.dumps(record,indent=2)+'\n');code=p.wait()
record.update(state='complete',exit_code=code,finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),log_sha256=hashlib.sha256((OUT/'directed-e2e-pass3.log').read_bytes()).hexdigest())
if (OUT/'directed-e2e-pass3-results.json').exists():record['stats']=json.loads((OUT/'directed-e2e-pass3-results.json').read_text())['stats']
(OUT/'directed-e2e-pass3.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record));raise SystemExit(code)
