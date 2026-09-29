const { chromium, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');
const out = path.join(__dirname, 'screenshots');
fs.mkdirSync(out, { recursive: true });
const stamp = new Date().toISOString();
const user = { id: '10000000-0000-4000-8000-000000000001', name: 'Visual review', email: 'visual@example.test', onboarded_at: stamp, starter_content_state: 'imported', language: 'zh-Hans' };
const ws = { id: '20000000-0000-4000-8000-000000000001', name: '北落师门', slug: 'visual-review', description: null, context: null, settings: {}, repos: [], issue_prefix: 'VIS', avatar_url: null, created_at: stamp, updated_at: stamp };
const member = { id: 'member-1', workspace_id: ws.id, user_id: user.id, role: 'owner', created_at: stamp, name: user.name, email: user.email, avatar_url: null };
const runtime = { id: '30000000-0000-4000-8000-000000000001', workspace_id: ws.id, daemon_id: '40000000-0000-4000-8000-000000000001', name: 'Claude (yangshangweideMac-mini.local)', custom_name: null, provider: 'claude', runtime_mode: 'local', owner_id: user.id, status: 'online', visibility: 'private', metadata: {}, launch_header: '', device_info: 'macOS', last_seen_at: stamp, created_at: stamp, updated_at: stamp };
const templates = [
 { key: 'feature-delivery', title: '特性交付小队', description: '从规划到验证，协作完成项目任务。', leader: { template_key: 'coordinator' }, members: [] },
 { key: 'bug-fix', title: '缺陷修复小队', description: '定位问题、修复缺陷并检查结果。', leader: { template_key: 'coordinator' }, members: [] }
];
(async () => {
 const browser = await chromium.launch({ headless: true });
 const context = await browser.newContext({ viewport: {width: 1280, height: 900}, locale: 'zh-CN' });
 await context.addCookies([{ name: 'multica_logged_in', value: '1', url: 'http://localhost:13494' }, {name:'multica-locale',value:'zh-Hans',url:'http://localhost:13494'}]);
 await context.addInitScript(() => { if (!localStorage.getItem('theme')) localStorage.setItem('theme','light'); localStorage.setItem('multica:chat:isOpen','false'); });
 const page = await context.newPage();
 const errors=[]; const unknown=new Set();
 page.on('pageerror',e=>errors.push(e.message));
 await page.route(/\/(api|auth)\//, async route => {
  const p=new URL(route.request().url()).pathname;
  let json=[];
  if (p==='/api/me') json=user;
  else if (p==='/api/config') json={ cloud_hosted:false, device_auth_available:false };
  else if (p==='/api/workspaces') json=[ws];
  else if (p.endsWith('/members') || p==='/api/members') json=[member];
  else if (p.startsWith('/api/workspaces/') && !p.slice('/api/workspaces/'.length).includes('/')) json=ws;
  else if (p==='/api/projects') json={projects:[],total:0};
  else if (p==='/api/runtimes') json=[runtime];
  else if (p==='/api/squads/templates') json={templates};
  else if (p==='/api/issues') json={issues:[],total:0};
  else if (p.endsWith('/github/installations')) json={installations:[]};
  else if (p==='/api/inbox/unread-count') json={count:0};
  else if (p==='/api/notification-preferences') json={preferences:{}};
  else if (p==='/api/agents' || p==='/api/squads' || p==='/api/views' || p==='/api/labels') json=[];
  else {unknown.add(p);}
  await route.fulfill({json});
 });
 try {
  await page.goto('http://localhost:13494/visual-review/projects',{waitUntil:'domcontentloaded'});
  await page.getByRole('button',{name:'新建项目',exact:true}).first().click({timeout:30000});
  const dialog=page.getByRole('dialog').first();
  await expect(dialog.getByRole('button',{name:'选择执行AI小队'})).toContainText('特性交付小队');
  await expect(dialog.getByText('在线',{exact:true})).toBeVisible();
  await page.screenshot({path:path.join(out,'desktop.png'),animations:'disabled'});
  await page.setViewportSize({width:390,height:844});
  await expect(dialog.getByRole('button',{name:'创建项目',exact:true})).toBeInViewport();
  await page.screenshot({path:path.join(out,'narrow.png'),animations:'disabled'});
  const box=await dialog.boundingBox();
  if (box.x<0 || box.x+box.width>391) throw new Error('Dialog overflows narrow viewport');
  await dialog.getByRole('button',{name:'选择执行AI小队'}).click();
  await page.getByRole('checkbox',{name:/缺陷修复小队/}).check();
  await page.keyboard.press('Escape');
  await expect(dialog.getByRole('button',{name:'选择执行AI小队'})).toContainText('已选 2 个');
  await page.screenshot({path:path.join(out,'narrow-multiple.png'),animations:'disabled'});
  await dialog.getByRole('button',{name:'稍后连接',exact:true}).click();
  await expect(dialog.getByText('在线',{exact:true})).toHaveCount(0);
  await expect(dialog.getByRole('button',{name:'选择执行AI小队'})).toContainText('已选 2 个');
  await page.setViewportSize({width:1280,height:900});
  await page.screenshot({path:path.join(out,'deferred.png'),animations:'disabled'});
  await page.setViewportSize({width:390,height:568});
  await expect(dialog.getByRole('button',{name:'创建项目',exact:true})).toBeInViewport();
  await dialog.getByRole('button',{name:'选择执行AI小队'}).scrollIntoViewIfNeeded();
  await page.screenshot({path:path.join(out,'short-window.png'),animations:'disabled'});
  await page.setViewportSize({width:1280,height:900});
  await dialog.getByRole('textbox').first().fill('官网改版');
  await expect(dialog.getByRole('button',{name:'创建项目',exact:true})).toBeEnabled();
  await dialog.getByRole('button',{name:/展开|放大/}).click();
  await expect(dialog.getByRole('button',{name:'创建项目',exact:true})).toBeInViewport();
  await page.screenshot({path:path.join(out,'expanded.png'),animations:'disabled'});
  await page.evaluate(()=>localStorage.setItem('theme','dark'));
  await page.reload({waitUntil:'domcontentloaded'});
  await page.getByRole('button',{name:'新建项目',exact:true}).first().click();
  await expect(page.locator('html')).toHaveClass(/dark/);
  await page.screenshot({path:path.join(out,'dark.png'),animations:'disabled'});
  user.language='en';
  await context.addCookies([{name:'multica-locale',value:'en',url:'http://localhost:13494'}]);
  await page.setViewportSize({width:390,height:844});
  await page.reload({waitUntil:'domcontentloaded'});
  await page.getByRole('button',{name:/create.*project/i}).first().click();
  await expect(page.getByRole('dialog').first().getByRole('button',{name:'Create Project',exact:true})).toBeInViewport();
  await page.screenshot({path:path.join(out,'english-narrow.png'),animations:'disabled'});
  fs.writeFileSync(path.join(out,'verification.json'),JSON.stringify({errors,unknown:[...unknown],narrowBounds:box,assertions:'desktop/narrow, multi-selection, runtime deferral, fixed footer'},null,2));
  console.log(JSON.stringify({out,errors,unknown:[...unknown]}));
 } catch(e) {
  await page.screenshot({path:path.join(out,'failure.png'),animations:'disabled'});
  console.log(JSON.stringify({error:e.message,url:page.url(),text:(await page.locator('body').innerText()).slice(0,5000),errors,unknown:[...unknown]}));
  process.exitCode=1;
 } finally {await browser.close();}
})();
