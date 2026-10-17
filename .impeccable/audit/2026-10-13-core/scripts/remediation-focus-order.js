page.setDefaultTimeout(30000);
const checks=JSON.parse(await fs.readFile(path.join(dir,'remediation-native.json'),'utf8')).filter(x=>x.passed);
const save=()=>fs.writeFile(path.join(dir,'remediation-native.json'),JSON.stringify(checks,null,2));
const check=async(name,fn)=>{try{const evidence=await fn();checks.push({name,passed:true,evidence});console.log('PASS',name,JSON.stringify(evidence));}catch(error){checks.push({name,passed:false,error:error.message});console.log('FAIL',name,error.message.slice(0,600));}await save();};
const idle=()=>page.waitForFunction(()=>!window.__auditQC.getQueryCache().getAll().some(q=>q.getObserversCount()>0&&q.state.fetchStatus==='fetching'),{},{timeout:30000});
const nav=async route=>{const link=page.locator(`[data-sidebar="menu-button"][href$="/${route}"]`);if(!await link.isVisible())await page.getByRole('button',{name:'切换左侧边栏',exact:true}).first().click();await link.click();await idle();};
await fetch('http://127.0.0.1:9258/zoom?factor=1');await fetch('http://127.0.0.1:9258/resize?width=1440&height=1000');
await check('F05 real virtual inbox arrow focus and current item',async()=>{
 await nav('inbox');await page.locator('[data-inbox-selection]').first().waitFor();
 const original=await page.evaluate(()=>{const q=window.__auditQC.getQueryCache().getAll().find(q=>q.queryKey[0]==='inbox'&&q.queryKey[2]==='list'&&Array.isArray(q.state.data));return {key:q.queryKey,data:q.state.data};});
 try{
 await page.evaluate(({key,data})=>{const base=data[0];window.__auditQC.setQueryData(key,Array.from({length:80},(_,i)=>({...base,id:`audit-notice-${i}`,issue_id:null,type:'quick_create_done',title:`Audit notification ${i}`,body:'Local UI verification fixture',details:null,actor_type:'system',actor_id:null,read:true,archived:false})));},original);
 const first=page.locator('[data-inbox-selection="audit-notice-0"]');await first.waitFor();await first.focus();const initialMounted=await page.locator('[data-inbox-selection]').count();
 for(let i=1;i<=40;i++){await page.keyboard.press('ArrowDown');await page.waitForFunction(id=>document.activeElement?.getAttribute('data-inbox-selection')===id,`audit-notice-${i}`);}
 const result=await page.evaluate(()=>({focused:document.activeElement.getAttribute('data-inbox-selection'),current:document.activeElement.getAttribute('aria-current'),position:document.activeElement.closest('[role="listitem"]').getAttribute('aria-posinset'),setSize:document.activeElement.closest('[role="listitem"]').getAttribute('aria-setsize'),mounted:document.querySelectorAll('[data-inbox-selection]').length}));
 expect(result.focused).toBe('audit-notice-40');expect(result.current).toBe('true');expect(Number(result.position)).toBe(41);expect(result.mounted).toBeLessThan(80);await shot('fixed-inbox-virtual-focus');return {initialMounted,...result};
 }finally{await page.evaluate(({key,data})=>window.__auditQC.setQueryData(key,data),original);}
});
await check('F06 native keyboard reorder, pending guard and archived protection',async()=>{
 await nav('settings');await page.getByRole('tab',{name:'任务状态',exact:true}).click();await idle();
 await page.evaluate(async()=>{
 if(window.__statusOriginal)window.__auditQC.setQueryData(window.__statusOriginal.key,window.__statusOriginal.data);const q=window.__auditQC.getQueryCache().getAll().find(q=>q.queryKey[0]==='issue-statuses'&&q.queryKey[2]==='list'&&q.state.data?.statuses);window.__statusOriginal={key:q.queryKey,data:q.state.data};const base=q.state.data.statuses[0];
 const rows=[['a','审计状态甲',1,null],['retired','审计归档状态',2,'2026-10-01T00:00:00Z'],['b','审计状态乙',3,null],['qa','审计状态丙',4,null]].map(([id,name,position,archived_at])=>({...base,id:'audit-status-'+id,key:'audit_'+id,name,position,archived_at,category:'todo',is_system:false,color:'#3b82f6',description:'Local UI verification fixture'}));
 window.__auditQC.setQueryData(q.queryKey,{...q.state.data,statuses:[...q.state.data.statuses,...rows],total:q.state.data.total+rows.length});
 const module=await import(performance.getEntriesByType('resource').map(x=>x.name).find(x=>/packages\/core\/api\/index\.ts\?/.test(x)));const api=module.getApi();window.__statusApi=api;window.__statusMethod=api.reorderIssueStatuses;window.__statusCalls=[];
 api.reorderIssueStatuses=(category,ids)=>{window.__statusCalls.push({category,ids});return new Promise(resolve=>{window.__statusResolve=()=>resolve(window.__statusOriginal.data);});};
 });
 try{
 const panel=page.getByRole('tabpanel',{name:'任务状态',exact:true});const toggle=panel.getByRole('switch');await toggle.click();await expect(panel.getByText('隐藏已归档状态后，即可调整状态顺序。')).toBeVisible();const move=panel.getByRole('button',{name:'上移 审计状态丙',exact:true});await expect(move).toHaveAttribute('aria-disabled','true');await move.focus();await page.keyboard.press('Enter');expect(await page.evaluate(()=>window.__statusCalls.length)).toBe(0);await shot('fixed-status-archived-guard');
 await toggle.click();await expect(move).toHaveAttribute('aria-disabled','false');await move.focus();await page.keyboard.press('Enter');await page.waitForFunction(()=>window.__statusCalls.length===1);await expect(move).toHaveAttribute('aria-disabled','true');await page.keyboard.press('Enter');expect(await page.evaluate(()=>window.__statusCalls.length)).toBe(1);
 await page.evaluate(()=>window.__statusResolve());await expect(move).toHaveAttribute('aria-disabled','false');await expect(move).toBeFocused();
 const result=await page.evaluate(()=>({calls:window.__statusCalls,order:window.__auditQC.getQueryData(window.__statusOriginal.key).statuses.filter(x=>x.id.startsWith('audit-status-')&&!x.archived_at).map(x=>x.id),focused:document.activeElement.getAttribute('aria-label')}));expect(result.order).toEqual(['audit-status-a','audit-status-qa','audit-status-b']);await shot('fixed-status-keyboard-order');return result;
 }finally{await page.evaluate(()=>{window.__statusResolve?.();window.__statusApi.reorderIssueStatuses=window.__statusMethod;window.__auditQC.setQueryData(window.__statusOriginal.key,window.__statusOriginal.data);});}
});
await save();if(checks.some(x=>!x.passed))process.exitCode=1;
