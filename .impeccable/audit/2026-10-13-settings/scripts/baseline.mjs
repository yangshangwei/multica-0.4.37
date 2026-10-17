// Captures every visible settings tab in light/dark wide windows and a 900px
// window at native 200% zoom: screenshot, ARIA snapshot, contrast samples,
// unnamed interactive AX nodes and page/inner horizontal overflow.
import {chromium} from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import {scanContrast} from '../../2026-10-11-desktop-core/scripts/contrast.mjs';
const dir=path.resolve(import.meta.dirname,'..');
const browser=await chromium.connectOverCDP('http://127.0.0.1:9267');
const context=browser.contexts()[0],page=context.pages()[0];
page.setDefaultTimeout(9000);
const cdp=await context.newCDPSession(page);
const only=process.argv[2]?.split(',');
const variants=[{name:'light-wide',width:1440,zoom:1,dark:false},{name:'dark-wide',width:1440,zoom:1,dark:true},{name:'light-200pct',width:900,zoom:2,dark:false}].filter(v=>!only||only.includes(v.name));
const INTERACTIVE=new Set(['button','link','textbox','combobox','checkbox','switch','radio','tab','menuitem','slider','spinbutton','searchbox','listbox','option']);
const unnamed=async()=>{
 const {nodes}=await cdp.send('Accessibility.getFullAXTree');
 return nodes.filter(n=>!n.ignored&&INTERACTIVE.has(n.role?.value)&&!(n.name?.value||'').trim()).map(n=>({role:n.role.value,id:n.backendDOMNodeId}));
};
const describe=async ids=>{
 const out=[];
 for(const id of ids.slice(0,12)){try{const {object}=await cdp.send('DOM.resolveNode',{backendNodeId:id});const r=await cdp.send('Runtime.callFunctionOn',{objectId:object.objectId,functionDeclaration:'function(){return this.outerHTML.slice(0,260)}',returnByValue:true});out.push(r.result.value);}catch{}}
 return out;
};
const openSettings=async()=>{
 const link=page.locator('[data-sidebar="menu-button"][href$="/settings"]');
 if(!await link.isVisible().catch(()=>false)){const t=page.getByRole('button',{name:'切换左侧边栏',exact:true});if(await t.count())await t.first().click();}
 if(await link.isVisible().catch(()=>false))await link.click();
 await page.waitForTimeout(700);
};
const selectTab=async label=>{
 const tab=page.getByRole('tab',{name:label,exact:true});
 if(!await tab.isVisible().catch(()=>false)){
  const dirBtn=page.getByRole('button',{name:/^设置目录/});
  if(await dirBtn.isVisible().catch(()=>false)){await dirBtn.click();await page.waitForTimeout(400);}
 }
 await page.getByRole('tab',{name:label,exact:true}).click();
 await page.waitForTimeout(1300);
};
const results=[];
await openSettings();
const tabs=await page.evaluate(()=>[...document.querySelectorAll('[role="tablist"] [role="tab"]')].map(t=>t.textContent.trim()));
const slugs=['profile','preferences','notifications','shortcuts','tokens','workspace','members','labels','triage','iterations','issue-statuses','properties','quick-actions','repositories','integrations','mcp','daemon','server','updates','behavior'];
for(const variant of variants){
 await fetch(`http://127.0.0.1:9268/resize?width=${variant.width}&height=1000`);
 await fetch(`http://127.0.0.1:9268/zoom?factor=${variant.zoom}`);
 await page.evaluate(dark=>{document.documentElement.classList.toggle('dark',dark);document.documentElement.style.colorScheme=dark?'dark':'light';},variant.dark);
 await page.waitForTimeout(600);
 await openSettings();
 for(const [i,label] of tabs.entries()){
  const name=`${slugs[i]??i}-${variant.name}`;
  try{
   await selectTab(label);
   const metrics=await page.evaluate(()=>{
    const vw=document.documentElement.clientWidth;
    const panel=document.querySelector('[role="tabpanel"]:not([hidden])');
    const over=[];
    for(const el of (panel??document.body).querySelectorAll('*')){
     const r=el.getBoundingClientRect();
     if(r.width&&r.right>vw+1&&getComputedStyle(el).position!=='fixed'){let p=el.parentElement,clipped=false;while(p&&p!==panel){const s=getComputedStyle(p);if(/(auto|scroll|hidden|clip)/.test(s.overflowX)){clipped=true;break;}p=p.parentElement;}if(!clipped)over.push(`${el.tagName.toLowerCase()}.${String(el.className).slice(0,80)} right=${Math.round(r.right)}`);}
    }
    const scrollers=[...document.querySelectorAll('*')].filter(e=>e.scrollWidth>e.clientWidth+2&&/(auto|scroll)/.test(getComputedStyle(e).overflowX)).map(e=>`${e.tagName.toLowerCase()}.${String(e.className).slice(0,60)} ${e.scrollWidth}/${e.clientWidth}`);
    return {width:innerWidth,docScroll:document.documentElement.scrollWidth,overflow:over.slice(0,8),scrollers:scrollers.slice(0,6),headings:[...(panel??document).querySelectorAll('h1,h2,h3,h4')].map(h=>`${h.tagName}:${h.textContent.trim().slice(0,30)}`).slice(0,14),text:(panel?.innerText??'').slice(0,400)};
   });
   const ax=await page.locator('body').ariaSnapshot();
   await fs.writeFile(path.join(dir,'ax',name+'.txt'),ax);
   const capture=await cdp.send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false,fromSurface:true});
   await fs.writeFile(path.join(dir,'screenshots',name+'.png'),Buffer.from(capture.data,'base64'));
   const contrast=await scanContrast(page);
   const missing=await unnamed();
   const missingHtml=await describe(missing.map(m=>m.id));
   results.push({name,label,...metrics,contrastChecked:contrast.checked,contrastFailures:contrast.failures,unnamed:missing.map(m=>m.role),unnamedHtml:missingHtml});
   console.log(JSON.stringify({name,checked:contrast.checked,fail:contrast.failures.length,unnamed:missing.length,overflow:metrics.overflow.length,docScroll:metrics.docScroll,width:metrics.width}));
  }catch(error){results.push({name,label,error:error.message});console.log(name,'ERROR',error.message.slice(0,200));}
 }
 await fs.writeFile(path.join(dir,`baseline-${only?only.join('_'):'all'}.json`),JSON.stringify(results,null,2));
}
await fetch('http://127.0.0.1:9268/resize?width=1440&height=1000');
await fetch('http://127.0.0.1:9268/zoom?factor=1');
await page.evaluate(()=>{document.documentElement.classList.remove('dark');document.documentElement.style.colorScheme='light';});
await cdp.detach();process.exit();
