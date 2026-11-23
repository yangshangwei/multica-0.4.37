import fs from 'node:fs';import { chromium, expect } from '@playwright/test';
const s=JSON.parse(fs.readFileSync('/tmp/multica-ux-audit-session.json'));const out='.trellis/tasks/archive/2026-11/11-05-collaboration-ux-audit/evidence';const result={source:'Production UI, real API; 450ms artificial search request latency',checks:[],errors:[]};const b=await chromium.launch();
try{
 const c=await b.newContext({viewport:{width:390,height:740},hasTouch:true});await c.addCookies([{name:'multica-locale',value:'en',url:s.web}]);await c.addInitScript(t=>localStorage.setItem('multica_token',t),s.token);
 await c.route('**/api/collaboration/recipients/search',async r=>{await new Promise(done=>setTimeout(done,450));await r.continue().catch(()=>{})});
 const p=await c.newPage();p.on('pageerror',e=>result.errors.push(e.message));await p.goto(s.web+'/account');await p.getByRole('button',{name:'New collaboration issue',exact:true}).click();
 const dialog=p.getByRole('dialog',{name:'New collaboration issue',exact:true});const trigger=dialog.locator('[data-recipient-picker] button[aria-label="Recipient"]');await trigger.click();
 const list=p.getByRole('listbox',{name:'Recipient',exact:true});await expect(p.getByRole('textbox',{name:'Filter options',exact:true})).toBeVisible();
 const search=p.getByRole('textbox',{name:'Filter options',exact:true});await expect(search).toBeVisible();
 await search.fill('no-such-ux-recipient');await search.press('Enter');await expect(p.getByRole('textbox',{name:'Filter options',exact:true})).toBeVisible();await expect(trigger).toContainText('Assign to me');
 await expect(p.getByText('No available accounts found.',{exact:true})).toBeVisible({timeout:5000});result.checks.push('rapid no-match Enter cannot commit self/old row');
 await search.fill(s.userId);await search.press('Enter');await expect(p.getByRole('textbox',{name:'Filter options',exact:true})).toBeVisible();await expect(list.getByRole('option')).toHaveCount(1,{timeout:5000});await expect(list).toHaveAttribute('aria-busy','false');await search.press('ArrowDown');await search.press('Enter');await expect(list).toBeHidden();await expect(trigger).toContainText(s.userId);result.checks.push('settled exact self-ID search selects current intended account');
 await p.screenshot({path:out+'/recipient-current-selection.png'});
}catch(e){result.failure=e.message;throw e;}finally{await b.close();fs.writeFileSync(out+'/recipient-browser.json',JSON.stringify(result,null,2)+'\n');}
console.log(JSON.stringify(result));
