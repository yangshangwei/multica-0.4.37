import { _electron as electron } from '@playwright/test';
import fs from 'node:fs/promises';
import {createServer} from 'node:http';
import path from 'node:path';
const dir=path.dirname(import.meta.filename);
const root=process.cwd();
const profile=(await fs.readFile(path.join(dir,'profile-path.txt'),'utf8')).trim();
const app=await electron.launch({
 executablePath:path.join(root,'apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),
 args:[path.join(dir,'harness.cjs'),'--remote-debugging-port=9247'],
 env:{...process.env,CHANGELOG_ELECTRON_PROFILE:profile,CHANGELOG_E2E_API_URL:'http://localhost:18572',CHANGELOG_ELECTRON_RENDERER_URL:'http://localhost:5666',CHANGELOG_ELECTRON_SYSTEM_LOCALE:'zh-CN'}
});
const page=await app.firstWindow();
await page.locator('#root > *').first().waitFor({state:'attached'});
console.log(JSON.stringify({ready:true,url:page.url(),electron:await app.evaluate(()=>process.versions.electron)}));
createServer(async(req,res)=>{
 const u=new URL(req.url,'http://127.0.0.1');
 if(u.pathname==='/zoom'){const zoom=Number(u.searchParams.get('factor'));if([1,2].includes(zoom))await app.evaluate(({BrowserWindow},z)=>BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(z),zoom);}
 res.end(JSON.stringify(await app.evaluate(({BrowserWindow})=>({zoom:BrowserWindow.getAllWindows()[0].webContents.getZoomFactor()}))));
}).listen(9248,'127.0.0.1');
const close=async()=>{await app.close();process.exit(0)};
process.on('SIGTERM',close);process.on('SIGINT',close);
await new Promise(()=>{});
