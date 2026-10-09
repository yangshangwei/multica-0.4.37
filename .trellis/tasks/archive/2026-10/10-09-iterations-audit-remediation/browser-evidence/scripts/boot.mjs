import { _electron as electron, chromium } from '@playwright/test';
import fs from 'node:fs/promises';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
const root=process.cwd();
const evidence=path.resolve('.trellis/tasks/10-09-iterations-audit-remediation/browser-evidence');
const profile=await fs.mkdtemp(path.join(os.tmpdir(),'multica-iterations-audit-profile-'));
const source=await chromium.connectOverCDP('http://127.0.0.1:9251');
const original=source.contexts()[0].pages()[0];
const storage=await original.evaluate(()=>Object.fromEntries(Object.keys(localStorage).map(key=>[key,localStorage.getItem(key)])));
if(!storage.multica_token) throw new Error('Read-only audit profile has no session');
const allocator=net.createServer(); await new Promise(resolve=>allocator.listen(0,'127.0.0.1',resolve));
const debugPort=allocator.address().port; await new Promise(resolve=>allocator.close(resolve));
const app=await electron.launch({
 executablePath:path.join(root,'apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),
 args:[path.join(evidence,'scripts/harness.cjs'),`--remote-debugging-port=${debugPort}`],
 env:{...process.env,CHANGELOG_ELECTRON_PROFILE:profile,CHANGELOG_E2E_API_URL:'http://localhost:18572',CHANGELOG_ELECTRON_RENDERER_URL:'http://localhost:5666',CHANGELOG_ELECTRON_SYSTEM_LOCALE:'zh-CN'}
});
const page=await app.firstWindow();
await page.addInitScript(storage=>{for(const [key,value] of Object.entries(storage)) if(value!==null) localStorage.setItem(key,value); document.cookie='multica_logged_in=1; path=/; SameSite=Lax';},storage);
await page.reload();
await page.getByRole('link',{name:'迭代',exact:true}).first().click();
await page.getByRole('heading',{name:'迭代',exact:true}).waitFor();
await page.evaluate(()=>document.fonts.ready);
await page.screenshot({path:path.join(evidence,'baseline-overview.png'),animations:'disabled'});
await fs.writeFile(path.join(evidence,'session.json'),JSON.stringify({debugPort,profile,api:'http://localhost:18572',renderer:'http://localhost:5666',mode:'actual Desktop renderer (Vite)',readOnly:true,createdAt:new Date().toISOString()},null,2));
await fs.writeFile(path.join(evidence,'baseline-overview.txt'),await page.locator('body').ariaSnapshot());
console.log(JSON.stringify({ready:true,debugPort,evidence,baseline:'baseline-overview.png'}));
async function close(){await app.close(); process.exit(0);}
process.on('SIGTERM',close); process.on('SIGINT',close);
await new Promise(()=>{});
