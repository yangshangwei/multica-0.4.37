page.setDefaultTimeout(30000);
const checks=JSON.parse(await fs.readFile(path.join(dir,'remediation-native.json'),'utf8')).filter(x=>x.passed);
const save=()=>fs.writeFile(path.join(dir,'remediation-native.json'),JSON.stringify(checks,null,2));
const check=async(name,fn)=>{try{const evidence=await fn();checks.push({name,passed:true,evidence});console.log('PASS',name,JSON.stringify(evidence));}catch(error){checks.push({name,passed:false,error:error.message});console.log('FAIL',name,error.message.slice(0,350));await page.keyboard.press('Escape').catch(()=>{});}await save();};
const idle=()=>page.waitForFunction(()=>!window.__auditQC.getQueryCache().getAll().some(q=>q.getObserversCount()>0&&q.state.fetchStatus==='fetching'),{},{timeout:30000});
const nav=async route=>{const link=page.locator(`[data-sidebar="menu-button"][href$="/${route}"]`);if(!await link.isVisible())await page.getByRole('button',{name:'切换左侧边栏',exact:true}).first().click();await link.click();await idle();};
const {scanContrast}=await import(path.resolve(dir,'../2026-10-11-desktop-core/scripts/contrast.mjs'));
await fetch('http://127.0.0.1:9258/zoom?factor=1');await fetch('http://127.0.0.1:9258/resize?width=1440&height=1000');await page.waitForFunction(()=>innerWidth===1440);
await check('F08 notification cold failure, pending retry and recovery',async()=>{
 await nav('settings');await page.getByRole('tab',{name:'个人资料',exact:true}).click();await idle();
 let fail=true;const pattern='**/api/notification-preferences';let requests=0;
 await context.route(pattern,async route=>{requests++;if(fail)return route.fulfill({status:500,contentType:'application/json',body:'{"error":"Audit notification outage"}'});await new Promise(r=>setTimeout(r,1200));await route.continue();});
 try{
 await page.evaluate(()=>window.__auditQC.removeQueries({queryKey:['notification-preferences']}));await page.getByRole('tab',{name:'通知',exact:true}).click();const panel=page.getByRole('tabpanel',{name:'通知',exact:true});await expect(panel.getByRole('alert')).toBeVisible();expect(await panel.getByRole('switch').count()).toBe(0);await shot('fixed-notification-error');
 fail=false;const retry=panel.getByRole('alert').getByRole('button');await retry.click();await expect(retry).toBeDisabled();await expect(panel.getByRole('alert')).toBeVisible();await expect(panel.getByRole('switch').first()).toBeVisible();await expect(panel.getByRole('alert')).toHaveCount(0);return {requests,switches:await panel.getByRole('switch').count()};
 }finally{await context.unroute(pattern);}
});
await check('F09 disclosure state and retained description editor',async()=>{
 await nav('projects');await page.getByRole('link',{name:'桌面场景 · 平台体验',exact:true}).click();await page.getByRole('button',{name:'描述',exact:true}).waitFor();await idle();
 const toggle=page.getByRole('button',{name:'描述',exact:true});const id=await toggle.getAttribute('aria-controls');expect(id).toBeTruthy();await expect(toggle).toHaveAttribute('aria-expanded','true');await page.evaluate(id=>{window.__descriptionNode=document.getElementById(id)?.querySelector('[contenteditable]');},id);
 await toggle.click();await expect(toggle).toHaveAttribute('aria-expanded','false');expect(await page.evaluate(id=>document.getElementById(id).hidden,id)).toBe(true);await toggle.click();await expect(toggle).toHaveAttribute('aria-expanded','true');const retained=await page.evaluate(id=>window.__descriptionNode===document.getElementById(id)?.querySelector('[contenteditable]'),id);expect(retained).toBe(true);await shot('fixed-project-disclosure');return {id,retained};
});
await check('F03 named project Sheet at native 200% zoom',async()=>{
 await fetch('http://127.0.0.1:9258/resize?width=900&height=1000');await fetch('http://127.0.0.1:9258/zoom?factor=2');await page.getByRole('button',{name:'切换右侧边栏',exact:true}).click();const dialog=page.getByRole('dialog',{name:/桌面场景.*平台体验.*属性/});await expect(dialog).toBeVisible();await shot('fixed-project-drawer-200pct');const ax=await dialog.ariaSnapshot();await page.keyboard.press('Escape');await fetch('http://127.0.0.1:9258/zoom?factor=1');await fetch('http://127.0.0.1:9258/resize?width=1440&height=1000');return {name:ax.split('\n')[0]};
});
await check('F07 Gantt cold and cached error recovery',async()=>{
 await page.getByRole('button',{name:'任务',exact:true}).click();await idle();let fail=true;const pattern='**/api/issues?**';let requests=0;
 await context.route(pattern,async route=>{const url=new URL(route.request().url());if(url.searchParams.get('scheduled')==='true'){requests++;if(fail)return route.fulfill({status:500,contentType:'application/json',body:'{"error":"Audit Gantt outage"}'});}await route.continue();});
 try{
 await page.evaluate(()=>window.__auditQC.removeQueries({predicate:q=>q.queryKey.includes('project-gantt')}));await page.getByRole('button',{name:'看板',exact:true}).click();await page.getByText('甘特图',{exact:true}).click();await expect(page.getByRole('alert').filter({hasText:'无法加载已排期的任务'})).toBeVisible();expect(await page.getByText(/暂无已排期的任务/).count()).toBe(0);await shot('fixed-gantt-cold-error');
 fail=false;await page.getByRole('button',{name:'重试',exact:true}).click();await page.locator('[data-gantt-issue-link]').first().waitFor();await page.evaluate(()=>window.__ganttNode=document.querySelector('[data-gantt-issue-link]'));
 fail=true;await page.evaluate(()=>window.__auditQC.invalidateQueries({predicate:q=>q.queryKey.includes('project-gantt')}));await expect(page.getByRole('alert').filter({hasText:'无法刷新已排期的任务'})).toBeVisible();const retained=await page.evaluate(()=>window.__ganttNode===document.querySelector('[data-gantt-issue-link]')&&window.__ganttNode.isConnected);expect(retained).toBe(true);await shot('fixed-gantt-cached-error');
 fail=false;await page.getByRole('button',{name:'重试',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:'无法刷新已排期的任务'})).toHaveCount(0);return {requests,retained};
 }finally{await context.unroute(pattern);}
});
await save();if(checks.some(x=>!x.passed))process.exitCode=1;
