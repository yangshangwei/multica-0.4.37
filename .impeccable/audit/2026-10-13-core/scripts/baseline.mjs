import {chromium} from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import {scanContrast} from '../../2026-10-11-desktop-core/scripts/contrast.mjs';
const dir=path.resolve(import.meta.dirname,'..');
const browser=await chromium.connectOverCDP('http://127.0.0.1:9257');
const context=browser.contexts()[0],page=context.pages()[0];
page.setDefaultTimeout(9000);
const cdp=await context.newCDPSession(page);
const routes=['issues','my-issues','projects','iterations','triage','inbox','chat','agents','squads','skills','mcp','runtimes','settings'];
const results=[];
for(const variant of [{name:'light-wide',width:1440,zoom:1,dark:false},{name:'dark-wide',width:1440,zoom:1,dark:true},{name:'light-200pct',width:900,zoom:2,dark:false}]){
 await fetch(`http://127.0.0.1:9258/resize?width=${variant.width}&height=1000`);
 await fetch(`http://127.0.0.1:9258/zoom?factor=${variant.zoom}`);
 await page.evaluate(dark=>{document.documentElement.classList.toggle('dark',dark);document.documentElement.style.colorScheme=dark?'dark':'light';},variant.dark);
 for(const route of routes){
  const name=`${route}-${variant.name}`;
  try{
   const link=page.locator(`[data-sidebar="menu-button"][href$="/${route}"]`);
   if(!await link.isVisible()){
    const toggles=page.getByRole('button',{name:'切换左侧边栏',exact:true});
    await toggles.first().click();
   }
   await link.click();
   await page.waitForTimeout(650);
   // The real window zoom is controlled by Electron, not viewport emulation.
   const metrics=await page.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth,headings:[...document.querySelectorAll('h1')].map(e=>e.textContent),dialogs:[...document.querySelectorAll('[role="dialog"]')].map(e=>({label:e.getAttribute('aria-label'),labelledby:e.getAttribute('aria-labelledby')})),elements:document.querySelectorAll('*').length}));
   const ax=await page.locator('body').ariaSnapshot();
   await fs.writeFile(path.join(dir,name+'.txt'),ax);
   const capture=await cdp.send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false,fromSurface:true});
   await fs.writeFile(path.join(dir,'screenshots',name+'.png'),Buffer.from(capture.data,'base64'));
   const contrast=await scanContrast(page);
   results.push({name,...metrics,contrast});
   console.log(JSON.stringify({name,...metrics,contrastChecked:contrast.checked,contrastFailures:contrast.failures.length}));
  }catch(error){results.push({name,error:error.message});console.log(name,error.message.slice(0,200));}
 }
 await fs.writeFile(path.join(dir,'baseline-metrics.json'),JSON.stringify(results,null,2));
}
await fetch('http://127.0.0.1:9258/resize?width=1440&height=1000');
await fetch('http://127.0.0.1:9258/zoom?factor=1');
await cdp.detach();process.exit();
