import {chromium,expect} from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
const dir=path.resolve(import.meta.dirname,'..');
const browser=await chromium.connectOverCDP('http://127.0.0.1:9257');
const context=browser.contexts()[0];
const page=context.pages().find(p=>p.url().startsWith('http://localhost:5666'));
if(!page) throw new Error('Audit renderer unavailable');
page.setDefaultTimeout(6000);
await page.evaluate(() => {
 const el=document.getElementById('root');
 const key=Object.keys(el).find(k=>k.startsWith('__reactContainer'));
 const stack=[el[key].stateNode.current];
 while(stack.length){const f=stack.pop(),c=f.memoizedProps?.client;
  if(c?.getQueryCache){window.__auditQC=c;break;}
  if(f.child)stack.push(f.child);if(f.sibling)stack.push(f.sibling);
 }
});
let script=''; for await (const c of process.stdin) script+=c;
const shot=async name=>{const session=await context.newCDPSession(page);try{const capture=await session.send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false,fromSurface:true});await fs.writeFile(path.join(dir,'screenshots',name+'.png'),Buffer.from(capture.data,'base64'));}finally{await session.detach();}};
const snap=async()=>console.log((await page.locator('body').ariaSnapshot()).slice(0,12000));
const cdp=await context.newCDPSession(page);
try { await new (Object.getPrototypeOf(async()=>{}).constructor)('page','context','browser','cdp','fs','path','dir','expect','shot','snap',script)(page,context,browser,cdp,fs,path,dir,expect,shot,snap); }
catch(e){console.error(e.stack);process.exitCode=1;}
await cdp.detach();
process.exit(process.exitCode??0);
