import fs from 'node:fs';
import path from 'node:path';
import { chromium, expect as baseExpect } from '@playwright/test';
const expect = baseExpect.configure({ timeout: 20000 });
const s = JSON.parse(fs.readFileSync('/tmp/multica-ux-audit-session.json', 'utf8'));
const out = path.resolve('.trellis/tasks/archive/2026-11/11-05-collaboration-ux-audit/evidence/after-admin');fs.mkdirSync(out,{recursive:true});
const uuid=n=>`00000000-0000-4000-8000-${String(n).padStart(12,'0')}`;
const detail = {scope:uuid(2), issue_id:uuid(3),identifier:'SOURCE-42',source_workspace_id:uuid(9),source_name:'跨工作空间协作来源',status:'done',status_category:'done',collaboration_revision:'7',queue_state:'closed',unsettled_runs:2,
 creator:{user_id:uuid(5),name:'原始发起人',avatar_url:'',disabled:true},handler:{user_id:uuid(6),name:'原处理人',avatar_url:'',disabled:true},successor_required:true,
 actions:[{action:'reopen',status_options_version:'a'.repeat(64),default_status:'todo',options:[{key:'todo',name:'Todo',category:'todo'},{key:'in_progress',name:'In progress',category:'in_progress'}]}]};
const evidence={mockedAdminProjection:true,realAuthentication:true,nativeElectron:false,checks:[],errors:[]};
const b=await chromium.launch();
try {
 for(const v of [{locale:'en',width:1440,height:1000,theme:'light'},{locale:'zh-Hans',width:390,height:740,theme:'dark'}]){
  const t=JSON.parse(fs.readFileSync(`packages/views/locales/${v.locale}/admin.json`,'utf8'));
  const c=await b.newContext({viewport:{width:v.width,height:v.height},hasTouch:v.width<500,colorScheme:v.theme});
  await c.addCookies([{name:'multica-locale',value:v.locale,url:s.web}]);
  await c.addInitScript(({token,theme})=>{localStorage.setItem('multica_token',token);localStorage.setItem('theme',theme)}, {token:s.token,theme:v.theme});
  await c.route(s.api+'/**',async r=>{
   const req=r.request(),url=new URL(req.url());const cors={'access-control-allow-origin':s.web,'access-control-allow-credentials':'true','access-control-allow-methods':'GET,POST,PATCH,DELETE,PUT,OPTIONS','access-control-allow-headers':req.headers()['access-control-request-headers']||'*'};
   if(req.method()==='OPTIONS')return r.fulfill({status:204,headers:cors});
   let body;
   if(url.pathname==='/api/admin/me')body={user_id:s.userId,organization_id:uuid(2),role:'super_admin',allowed_actions:[],supported:true};
   if(url.pathname==='/api/admin/collaboration/issues')body={scope:uuid(2),items:[detail],next_cursor:null};
   if(url.pathname===`/api/admin/collaboration/issues/${uuid(3)}`)body=detail;
   if(url.pathname.endsWith('/reopen-candidates'))body={scope:uuid(2),issue_id:uuid(3),collaboration_revision:'7',items:[{user_id:uuid(7),name:'接任协作者',avatar_url:'',external:true}],next_cursor:null};
   if(url.pathname.endsWith('/memory/versions'))body={scope:uuid(2),issue_id:uuid(3),current_version:'0',items:[],next_cursor:null};
   if(body)return r.fulfill({status:200,headers:cors,contentType:'application/json',body:JSON.stringify(body)});
   try {const response=await r.fetch();await r.fulfill({response,headers:{...response.headers(),...cors}});} catch {await r.abort().catch(()=>{});}
  });
  const p=await c.newPage();p.setDefaultTimeout(20000);p.on('pageerror',e=>evidence.errors.push(e.message));
  for(const [name,url] of [['list','/account?view=current&administration=collaboration'],['detail',`/account?view=current&administration=collaboration&issue=${uuid(3)}`]]){
   await p.goto(s.web+url);await expect(p.getByText('SOURCE-42',{exact:true})).toBeVisible();await p.evaluate(()=>document.fonts.ready);
   const width=await p.evaluate(()=>document.documentElement.scrollWidth);expect(width).toBeLessThanOrEqual(v.width);
   await p.screenshot({path:path.join(out,`${v.locale}-${name}.png`)});
   evidence.checks.push({locale:v.locale,name,width,viewport:v.width,rawDoneHidden:await p.getByText('done',{exact:true}).count()===0});
  }
  await c.close();
 }
}finally{await b.close();fs.writeFileSync(path.join(out,'admin-results.json'),JSON.stringify(evidence,null,2)+'\n');}
console.log(JSON.stringify(evidence));
