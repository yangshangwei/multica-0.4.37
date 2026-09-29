import { randomUUID } from 'node:crypto';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { test, expect, _electron as electron, type Page } from '@playwright/test';
import { TestApiClient } from './fixtures';
const provider=process.env.ACTOR_RECOMMENDATION_PROVIDER_URL;
const actorPath='/api/issues/recommend-creators';
test.use({locale:'en-US',trace:'retain-on-failure'});
test.setTimeout(120_000);
async function fixture(language="en") {
 const suffix=randomUUID().slice(0,10);const api=new TestApiClient();
 await api.login(`creator-p2-${suffix}@example.invalid`,'Creator phase2');
 const workspace=await api.ensureWorkspace('Creator phase2',`creator-p2-${suffix}`);await api.markUserOnboarded();
 await api.requestJSON('/api/me',{method:'PATCH',body:{language}});
 const runtime=await api.seedProjectRuntime();
 const create=(name:string,description:string)=>api.requestJSON<{id:string;name:string}>('/api/agents',{method:'POST',body:{name,description,runtime_id:runtime.id,visibility:'workspace'}});
 const diagnostic=await create('Refund diagnostician','Diagnose refund failures and verify fixes.');
 const writer=await create('Documentation writer','Write technical documentation and examples.');
 const squad=await api.requestJSON<{id:string;name:string}>('/api/squads',{method:'POST',body:{name:'Incident crew',leader_id:diagnostic.id,description:'Coordinate incident diagnosis and regression checks.'}});
 const project=await api.requestJSON<{id:string;title:string}>('/api/projects',{method:'POST',body:{title:'Payments project',execution_squads:[{squad_id:squad.id}]}});
 return {api,workspace,diagnostic,writer,squad,project};
}
async function authenticate(page:Page,token:string,language="en") {
 const origin = page.url().startsWith("http") ? new URL(page.url()).origin : process.env.PLAYWRIGHT_BASE_URL || process.env.FRONTEND_ORIGIN || "http://localhost:3000";
 await page.context().addCookies([{name:"multica-locale",value:language,url:origin}]);
 await page.addInitScript(({token,language})=>{
  localStorage.setItem('multica_token',token);localStorage.setItem('multica-locale',language);localStorage.setItem('multica:chat:isOpen','false');
  localStorage.setItem('multica_create_mode',JSON.stringify({state:{lastMode:'agent'},version:0}));
 },{token,language});
}
async function open(page:Page,slug?:string) {
 if(slug) await page.goto(`/${slug}/issues`,{waitUntil:'domcontentloaded'});
 await page.getByRole('button',{name:'New Issue',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'Quick create issue',exact:true});await expect(dialog).toBeVisible();
 return {dialog,editor:dialog.locator('.ProseMirror.rich-text-editor[contenteditable="true"]'),creator:dialog.getByRole('button',{name:/^Creation assistant/})};
}
async function choose(page:Page,creator:ReturnType<Page['getByRole']>,type:string,id:string) {
 await creator.click();await page.locator(`[data-actor-key="${type}:${id}"] button[data-picker-item]`).click();
}
async function preference(page:Page,slug:string) {
 return page.evaluate((slug)=>JSON.parse(localStorage.getItem(`multica_quick_create:${slug}`)||'{"state":{}}').state,slug);
}

test('explicit default outranks accepted history and project candidates do not replace it',async({page},info)=>{
 const f=await fixture();try {
  await authenticate(page,f.api.getToken()!);let ui=await open(page,f.workspace.slug);
  await ui.editor.fill('Investigate refund failures.');await choose(page,ui.creator,'agent',f.writer.id);
  await ui.creator.click();await page.getByRole('button',{name:'Set current as default',exact:true}).click();await page.keyboard.press('Escape');
  expect((await preference(page,f.workspace.slug)).defaultActor).toEqual({type:'agent',id:f.writer.id});
  await choose(page,ui.creator,'agent',f.diagnostic.id);
  const accepted=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/issues/quick-create'&&r.request().method()==='POST');
  await ui.dialog.getByRole('button',{name:'Create',exact:true}).click();expect((await accepted).status()).toBe(202);await expect(ui.dialog).toBeHidden();
  ui=await open(page);await expect(ui.creator).toContainText(f.writer.name);
  await ui.dialog.getByRole('button',{name:'No project',exact:true}).click();
  await page.locator('button[data-picker-item]').filter({hasText:f.project.title}).click();
  await expect(page.locator('[data-slot=popover-content]')).toHaveCount(0);
  await expect(ui.creator).toContainText(f.writer.name);
  await ui.creator.click();await expect(page.getByRole('heading',{name:'Project squads',exact:true})).toBeVisible();
  await expect(page.locator(`[data-actor-key="squad:${f.squad.id}"]`)).toBeVisible();
  await expect(page.locator("[data-slot=popover-content]").filter({has:page.getByRole("heading",{name:"Project squads",exact:true})})).toHaveCSS("opacity","1");
  await page.screenshot({path:info.outputPath("default-project-picker.png"),animations:"disabled"});
  await page.getByRole('button',{name:'View all project squads (1)',exact:true}).click();await page.keyboard.press('Escape');await expect(ui.dialog).toBeVisible();
 } finally {await f.api.deleteFeatureWorkspace(f.workspace.id);}
});

test('recommendations require a click, adoption stays explicit, and changed text discards late results',async({page},info)=>{
 test.skip(!provider,'Requires the deterministic local recommendation provider');
 const f=await fixture();try {
  await authenticate(page,f.api.getToken()!);const ui=await open(page,f.workspace.slug);
  await choose(page,ui.creator,'agent',f.writer.id);await ui.creator.click();await page.getByRole('button',{name:'Set current as default',exact:true}).click();await page.keyboard.press('Escape');
  const before=await preference(page,f.workspace.slug);
  const calls=await page.request.get(`${provider}/health`).then(r=>r.json());
  await ui.editor.fill('Diagnose refund failures and verify fixes.');await page.waitForTimeout(200);
  expect((await page.request.get(`${provider}/health`).then(r=>r.json())).calls).toBe(calls.calls);
  const response=page.waitForResponse(r=>new URL(r.url()).pathname===actorPath&&r.request().method()==='POST');
  await ui.dialog.getByRole('button',{name:'Help me choose',exact:true}).click();expect((await response).status()).toBe(200);
  await expect(ui.creator).toContainText(f.writer.name);await expect(ui.dialog.getByRole('button',{name:`Use ${f.diagnostic.name}`,exact:true})).toBeVisible();
  await page.screenshot({path:info.outputPath('recommendations-1280.png')});
  await ui.dialog.getByRole('button',{name:`Use ${f.diagnostic.name}`,exact:true}).click();await expect(ui.creator).toContainText(f.diagnostic.name);
  await expect(ui.editor).toHaveText('Diagnose refund failures and verify fixes.');
  const after=await preference(page,f.workspace.slug);expect(after.defaultActor).toEqual(before.defaultActor);expect(after.recentActors).toEqual(before.recentActors);
  await ui.editor.fill('Diagnose refund failures CREATORSLOW');await page.waitForTimeout(200);
  await ui.dialog.getByRole('button',{name:'Help me choose',exact:true}).click();await expect(ui.dialog.getByText('Finding suitable assistants...')).toBeVisible();
  await ui.editor.fill('Write technical documentation instead.');await page.waitForTimeout(1800);
  await expect(ui.dialog.getByRole('button',{name:/^Use /})).toHaveCount(0);await expect(ui.creator).toContainText(f.diagnostic.name);
  await ui.editor.fill('CREATORFAIL diagnose failures');await page.waitForTimeout(200);await ui.dialog.getByRole('button',{name:'Help me choose',exact:true}).click();
  await expect(ui.dialog.getByRole('alert')).toBeVisible();await expect(ui.dialog.getByRole('button',{name:'Create',exact:true})).toBeEnabled();
  await ui.editor.fill('Diagnose refund failures');await page.waitForTimeout(200);await ui.dialog.getByRole('button',{name:'Help me choose',exact:true}).click();
  await expect(ui.dialog.getByRole('button',{name:`Use ${f.diagnostic.name}`,exact:true})).toBeVisible();
  await page.setViewportSize({width:375,height:812});await expect(ui.dialog.getByRole('button',{name:`Use ${f.diagnostic.name}`,exact:true})).toBeInViewport();
  await expect(ui.dialog.getByRole('button',{name:'Create',exact:true})).toBeInViewport({ratio:1});
  await expect(ui.editor).toBeInViewport({ratio:0.5});
  await page.screenshot({path:info.outputPath('recommendations-375.png')});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 } finally {await f.api.deleteFeatureWorkspace(f.workspace.id);}
});

test('actual Electron renderer adopts suggestions without running a daemon',async({},info)=>{
 test.skip(!provider||!process.env.CHANGELOG_ELECTRON_RENDERER_URL||!process.env.CHANGELOG_E2E_API_URL,'Requires isolated desktop renderer and provider');
 const f=await fixture();const root=resolve(import.meta.dirname,'..');const profile=mkdtempSync(join(tmpdir(),'multica-creator-phase2-'));
 const app=await electron.launch({executablePath:join(root,'apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),args:[join(root,'e2e/fixtures/changelog-electron.cjs')],env:{...process.env,CHANGELOG_ELECTRON_PROFILE:profile,CHANGELOG_ELECTRON_SYSTEM_LOCALE:'en-US'}});
 try {
  const page=await app.firstWindow();await authenticate(page,f.api.getToken()!);await page.reload({waitUntil:'domcontentloaded'});
  const ui=await open(page);await ui.editor.fill('Diagnose refund failures and verify fixes.');await page.waitForTimeout(200);
  await ui.dialog.getByRole('button',{name:'Help me choose',exact:true}).click();await expect(ui.dialog.getByRole('button',{name:`Use ${f.diagnostic.name}`,exact:true})).toBeVisible();
  await page.screenshot({path:info.outputPath('recommendations-electron.png')});
  await ui.dialog.getByRole('button',{name:`Use ${f.diagnostic.name}`,exact:true}).click();await expect(ui.creator).toContainText(f.diagnostic.name);
  const services=await app.evaluate(()=> (globalThis as unknown as {changelogAcceptance:{daemonStarts:number}}).changelogAcceptance);
  expect(services.daemonStarts).toBe(0);
 } finally {await app.close();rmSync(profile,{recursive:true,force:true});await f.api.deleteFeatureWorkspace(f.workspace.id);}
});


test('Chinese narrow-screen controls expose defaults and grounded suggestions',async({page},info)=>{
 test.skip(!provider,'Requires deterministic local provider');
 const f=await fixture('zh-Hans');try {
  await authenticate(page,f.api.getToken()!,'zh-Hans');await page.setViewportSize({width:1280,height:900});
  await page.emulateMedia({colorScheme:'dark'});await page.goto(`/${f.workspace.slug}/issues`,{waitUntil:'domcontentloaded'});
  await page.getByRole('button',{name:'新建任务',exact:true}).click();
  const dialog=page.getByRole('dialog',{name:'快速创建任务',exact:true});await expect(dialog).toBeVisible();await page.setViewportSize({width:375,height:812});
  const creator=dialog.getByRole('button',{name:/^创建助手/});await creator.click();
  await page.getByRole('button',{name:'将当前助手设为默认',exact:true}).click();await page.keyboard.press('Escape');
  const editor=dialog.locator('.ProseMirror.rich-text-editor[contenteditable="true"]');await editor.fill('请排查退款失败，并验证修复。');await page.waitForTimeout(200);
  await dialog.getByRole('button',{name:'帮我选',exact:true}).click();
  await expect(dialog.getByRole('button',{name:`使用 ${f.diagnostic.name}`,exact:true})).toBeInViewport();
  await expect(dialog.getByRole('button',{name:'创建',exact:true})).toBeInViewport({ratio:1});
  await expect(editor).toBeInViewport({ratio:0.5});
  await page.screenshot({path:info.outputPath('recommendations-zh-375.png')});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 } finally {await f.api.deleteFeatureWorkspace(f.workspace.id);}
});
