import { _electron as electron, expect } from '@playwright/test';
import { createServer } from 'node:http';
import { readFile, writeFile, mkdir, mkdtemp } from 'node:fs/promises';
import { dirname, resolve, extname, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
const dir=dirname(fileURLToPath(import.meta.url)),mode=process.argv[2]??'baseline';
const build=JSON.parse(await readFile(resolve(dir,`${mode}-build.json`),'utf8'));
const out=resolve(dir,`../native-${mode}`);await mkdir(out,{recursive:true});
const server=createServer(async(req,res)=>{try{const name=new URL(req.url,'http://127.0.0.1').pathname;const path=resolve(build.output,`.${name==='/'?'/index.html':name}`);if(!path.startsWith(resolve(build.output)+sep)){res.writeHead(403);res.end();return;}res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css'})[extname(path)]??'application/octet-stream');res.end(await readFile(path));}catch{res.writeHead(404);res.end();}});
await new Promise(r=>server.listen(0,'127.0.0.1',r));let app;
const result={mode,build,method:'Owned hiddenInset Electron; actual shared composer with mock APIs and underlying48px drag strip; OS HID via Quartz when trusted',attempts:[]};
try{
 app=await electron.launch({args:[resolve(dir,'electron-main.cjs')],env:{...process.env,DRAG_PROOF_URL:`http://127.0.0.1:${server.address().port}/?locale=en`,DRAG_PROOF_PROFILE:await mkdtemp('/tmp/multica-drag-proof-')}});
 const page=await app.firstWindow();await page.waitForFunction(()=>window.fixture);const t=await page.evaluate(()=>window.fixture.translations);
 result.trusted=await app.evaluate(()=>global.dragProof.trust);result.pid=app.process().pid;
 await page.evaluate(()=>{window.domReceipts=[];for(const type of ['pointerdown','mousedown','click'])document.addEventListener(type,e=>window.domReceipts.push({type,target:e.target.closest('button')?.getAttribute('aria-label')??e.target.tagName,x:e.clientX,y:e.clientY}),true)});
 const open=async()=>{if(!await page.getByRole('dialog').count())await page.getByRole('button',{name:t.creation.entry,exact:true}).click();await page.getByRole('button',{name:t.creation.expand,exact:true}).click();await page.waitForTimeout(400);};
 const measure=async()=>({bounds:await app.evaluate(()=>global.dragProof.win.getBounds()),content:await app.evaluate(()=>global.dragProof.win.getContentBounds()),dialogCount:await page.getByRole('dialog').count(),receipts:await page.evaluate(()=>window.domReceipts)});
 for(const action of ['click','drag']){
  await open();await app.evaluate(()=>{global.dragProof.win.show();global.dragProof.win.focus()});await page.waitForTimeout(250);
  const close=page.getByRole('button',{name:t.creation.close,exact:true});const box=await close.boundingBox();const before=await measure();
  const css=await close.evaluate(el=>({element:getComputedStyle(el).getPropertyValue('-webkit-app-region'),popup:getComputedStyle(el.closest('[role=dialog]')).getPropertyValue('-webkit-app-region'),strip:getComputedStyle(document.querySelector('[data-native-drag-strip]')).getPropertyValue('-webkit-app-region')}));
  await page.evaluate(()=>window.domReceipts=[]);
  const focused=await app.evaluate(()=>global.dragProof.win.isFocused());
  let pointer={skipped:'untrusted accessibility or owned window not focused'};
  if(result.trusted&&focused)pointer=JSON.parse(execFileSync('python3',[resolve(dir,'native-pointer.py'),String(before.content.x+box.x+box.width/2),String(before.content.y+box.y+box.height/2),action],{encoding:'utf8'}));
  await page.waitForTimeout(500);const after=await measure();await page.screenshot({path:resolve(out,`native-${action}.png`)});
  result.attempts.push({action,box,css,focused,pointer,before,after});
  if(await page.getByRole('dialog').count())await close.click();await page.waitForTimeout(180);
 }
 await open();await page.evaluate(()=>window.domReceipts=[]);await page.getByRole('button',{name:t.creation.close,exact:true}).click();await page.waitForTimeout(180);result.playwrightClick={...(await measure()),limitation:'CDP renderer input; success does not prove native hit-testing'};
}finally{if(app)await app.close();await new Promise(r=>server.close(r));result.closedOwnedApp=true;result.closedOwnedListener=true;await writeFile(resolve(out,'results.json'),JSON.stringify(result,null,2)+'\n');}
console.log(JSON.stringify({out,trusted:result.trusted,attempts:result.attempts.map(a=>({action:a.action,before:a.before.bounds,after:a.after.bounds,dialog:a.after.dialogCount,receipts:a.after.receipts})),playwrightDialog:result.playwrightClick?.dialogCount}));
