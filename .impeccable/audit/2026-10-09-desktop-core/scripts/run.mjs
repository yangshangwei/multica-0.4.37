import {chromium,expect} from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
const dir=path.resolve(import.meta.dirname,'..');
const browser=await chromium.connectOverCDP('http://127.0.0.1:9247');
const context=browser.contexts()[0];
const page=context.pages().find(p=>p.url().startsWith('http://localhost:5666'));
if(!page) throw new Error('Audit renderer unavailable');
page.setDefaultTimeout(6000);
let script=''; for await (const c of process.stdin) script+=c;
const shot=async name=>{await page.screenshot({path:path.join(dir,'screenshots',name+'.png')});};
const snap=async()=>console.log((await page.locator('body').ariaSnapshot()).slice(0,12000));
const cdp=await context.newCDPSession(page);
try { await new (Object.getPrototypeOf(async()=>{}).constructor)('page','context','browser','cdp','fs','path','dir','expect','shot','snap',script)(page,context,browser,cdp,fs,path,dir,expect,shot,snap); }
catch(e){console.error(e.stack);process.exitCode=1;}
await cdp.detach();
process.exit(process.exitCode??0);
