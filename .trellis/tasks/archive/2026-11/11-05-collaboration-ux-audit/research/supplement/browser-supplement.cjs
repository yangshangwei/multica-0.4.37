const fs = require('node:fs');
const path = require('node:path');
const { chromium, expect } = require('@playwright/test');
const out = __dirname;
const session = JSON.parse(fs.readFileSync('/tmp/multica-ux-audit-session.json', 'utf8'));
const issueId = session.created[0].id;
const issuePath = '/account?view=participating&issue=' + issueId;
const evidence = { date: new Date().toISOString(), browser: 'Playwright Chromium', headless: true, viewport: { width: 390, height: 740 }, coarsePointer: null, directCors: true, cases: [], pageErrors: [], fixtures: [], writes: [] };
function persist() { fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify(evidence, null, 2) + '\n'); }
function sanitized(error) { return String(error.message || error).split('Call log:')[0].replaceAll(session.token, '[REDACTED]'); }
async function run(name, source, work) {
  const result = { name, source, status: 'running' }; evidence.cases.push(result);
  try { await work(result); result.status = 'passed'; }
  catch (error) { result.status = 'unverified'; result.reason = sanitized(error); }
  persist();
}
async function measure(page, locator, label) {
  await locator.scrollIntoViewIfNeeded();
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(180);
  const geometry = await locator.evaluate(element => { const box = element.getBoundingClientRect(); return { label: element.getAttribute('aria-label') || element.textContent.trim(), x: box.x, y: box.y, width: box.width, height: box.height, visible: box.width > 0 && box.height > 0, inViewport: box.x >= 0 && box.y >= 0 && box.right <= innerWidth + 1 && box.bottom <= innerHeight + 1, coarse: matchMedia('(pointer: coarse)').matches, documentOverflow: document.documentElement.scrollWidth > innerWidth }; });
  geometry.target44 = geometry.width >= 43.99 && geometry.height >= 43.99;
  await page.screenshot({ path: path.join(out, label + '.png') });
  expect(geometry.coarse).toBe(true); expect(geometry.target44).toBe(true); expect(geometry.inViewport).toBe(true);
  return geometry;
}
async function setup(browser, fixture = false) {
  const context = await browser.newContext({ viewport: evidence.viewport, hasTouch: true, isMobile: true, locale: 'en-US', colorScheme: 'light' });
  await context.addCookies([{ name: 'multica_logged_in', value: '1', url: session.web }]);
  await context.addInitScript(({ token }) => { localStorage.setItem('multica_token', token); localStorage.setItem('multica-locale', 'en'); localStorage.setItem('multica:chat:isOpen', 'false'); }, { token: session.token });
  const page = await context.newPage(); page.setDefaultTimeout(6500);
  page.on('pageerror', error => evidence.pageErrors.push({fixture,message:sanitized(error)}));
  page.on('request', request => { if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method()) && !request.url().includes('/recipients/search')) evidence.writes.push({ method: request.method(), path: new URL(request.url()).pathname }); });
  return {context,page};
}
(async () => {
  const browser = await chromium.launch({ headless: true });
  try {
    const live = await setup(browser);
    const page = live.page;
    await run('live-source-filter', 'real API, production web, coarse pointer', async result => {
      await page.goto(session.web + '/' + session.workspace.slug + '/issues', {waitUntil:'domcontentloaded'});
      const filter = page.getByRole('button', {name:'External collaborators',exact:true}); await filter.waitFor();
      result.geometry = await measure(page,filter,'live-source-filter'); evidence.coarsePointer = result.geometry.coarse;
      result.sourceDialog = 'Not exercised: collaboration row context menu only exposes open/copy links; no direct edit action. Source dialog close is not claimed verified.';
    });
    await run('live-recipient-query-commit', 'real API discovery; response delivery delayed 450 ms, payload unchanged', async result => {
      await page.goto(session.web + issuePath,{waitUntil:'domcontentloaded'});
      await page.getByRole('button',{name:'New collaboration issue',exact:true}).click();
      const dialog = page.getByRole('dialog'); const trigger = dialog.getByRole('button',{name:'Recipient',exact:true});
      await trigger.waitFor(); const before = await trigger.innerText();
      const requests=[];
      await page.route('**/api/collaboration/recipients/search', async route => {
        try {const query = route.request().postDataJSON()?.q || ''; requests.push(query); const response = await route.fetch(); await new Promise(resolve=>setTimeout(resolve,450)); await route.fulfill({response});} catch { /* Context close cancels remaining reads. */ }
      });
      await trigger.tap();
      const search = page.getByRole('textbox',{name:'Filter options',exact:true}); await search.waitFor();
      await page.getByRole('option').first().waitFor();
      const noMatch='supplement-no-match-5fd9'; await search.fill(noMatch); await search.press('Enter');
      await expect(search).toBeVisible(); await expect(page.getByRole('option')).toHaveCount(0);
      result.rapidEnter = { popupStayedOpen:true, optionsDuringPending:0, unchangedSelection:before === await trigger.innerText() };
      await page.waitForTimeout(800); await expect(page.getByRole('option')).toHaveCount(0);
      result.settledNoMatch = {options:0, unchangedSelection:before === await trigger.innerText()};
      await page.screenshot({path:path.join(out,'live-recipient-no-match.png')});
      await search.fill(session.userId); await search.press('Enter');
      await expect(search).toBeVisible(); await expect(page.getByRole('option')).toHaveCount(0);
      await page.getByRole('option').first().waitFor(); await expect(page.getByRole('option')).toHaveCount(1);
      const option=page.getByRole('option').first(); result.currentResult={label:await option.getAttribute('aria-label'),id:session.userId};
      expect(result.currentResult.label).toContain(session.userId);
      await search.press('ArrowDown'); await search.press('Enter'); await expect(search).not.toBeVisible();
      result.afterCommit=await trigger.innerText(); expect(result.afterCommit).toContain(session.userId);
      result.searches=requests; result.limit='Only self exists in this task source; distinct-recipient selection is covered by component tests, not this live supplement.';
      await page.screenshot({path:path.join(out,'live-recipient-current-result.png')});
      await dialog.getByRole('button',{name:'Cancel',exact:true}).tap();
    });
    await live.context.close();
    const fixture = await setup(browser,true);
    const p=fixture.page;
    const now='2026-11-05T00:00:00Z';
    const fileId='00000000-0000-4000-8000-000000001119', candidateId='00000000-0000-4000-8000-000000001120';
    const author={kind:'member',member:{user_id:session.userId,name:'Supplement fixture author',avatar_url:'',external:false},agent:null,resource_owner:null};
    const content='Controlled supplement fixture; this text is not a stored product record.';
    const metadata={version:'1',bytes:Buffer.byteLength(content),author,created_at:now,restored_from_version:null,purged:false,purged_at:null,purged_by:null,purged_by_admin:false};
    const memory={issue_id:issueId,version:'2',content,bytes:Buffer.byteLength(content),max_bytes:131072,author,created_at:now,restored_from_version:null,can_edit:false,can_purge:false};
    const file={id:fileId,draft_id:fileId,issue_id:issueId,intended_issue_id:null,source_workspace_id:session.workspace.id,uploader_user_id:session.userId,name:'SUPPLEMENT-FIXTURE.txt',content_type:'text/plain',size_bytes:Buffer.byteLength(content),version:'1',created_at:now,delivered_at:now};
    const candidate={candidate_id:candidateId,lineage_id:candidateId,version:'1',state:'pending',state_revision:'1',availability:'available',source_workspace_id:session.workspace.id,source_issue_id:issueId,target:{kind:'workspace',workspace_id:session.workspace.id},proposed_by_user_id:session.userId,proposed_at:now,expires_at:null,supersedes_candidate_id:null,derived_from_candidate_id:null,approved_at:null,approved_by_user_id:null,published_target_revision:null,decision:null,capabilities:{can_approve:false,can_reject:false,can_withdraw:false},summary:'SUPPLEMENT FIXTURE knowledge candidate',content,content_sha256:'b'.repeat(64),sources:[{kind:'memory',issue_id:issueId,version:'1',sha256:'a'.repeat(64)}],evidence:[],applicability:{surfaces:['collaboration']}};
    const lease=()=>({protocol_version:1,projection_checked_at:new Date().toISOString(),valid_until:new Date(Date.now()+25000).toISOString(),next_expiry_at:null});
    await fixture.context.route('**/api/collaboration/**', async route=>{
      const pathname=new URL(route.request().url()).pathname; const prefix='/api/collaboration/issues/'+issueId; let body;
      if(pathname===prefix+'/attachments')body={issue_id:issueId,attachments:[file]};
      else if(pathname===prefix+'/attachments/'+fileId+'/content'){await route.fulfill({status:200,contentType:'text/plain',body:content});return;}
      else if(pathname===prefix+'/memory')body=memory;
      else if(pathname===prefix+'/memory/versions')body={issue_id:issueId,current_version:'2',max_bytes:131072,can_purge:false,items:[{...metadata,version:'2'},metadata],next_cursor:null};
      else if(pathname===prefix+'/memory/versions/1')body={...metadata,issue_id:issueId,current_version:'2',max_bytes:131072,content};
      else if(pathname===prefix+'/knowledge-candidates'){const {content:omit,...summary}=candidate;body={...lease(),issue_id:issueId,items:[summary],next_cursor:null,capabilities:{can_propose:false}};}
      else if(pathname==='/api/collaboration/knowledge-candidates/'+candidateId)body={...lease(),candidate,history:[],history_truncated:false};
      if(body){evidence.fixtures.push(pathname);await route.fulfill({status:200,contentType:'application/json',json:body});}else await route.continue();
    });
    await p.goto(session.web+issuePath,{waitUntil:'domcontentloaded'});
    await p.locator('[data-tab-scroll-root="account-detail"] h2').waitFor();
    await p.evaluate(()=>{const label=document.createElement('div');label.textContent='READ-ONLY SUPPLEMENT FIXTURE';label.style.cssText='position:fixed;bottom:0;right:0;z-index:9999;background:#fff;color:#111;padding:3px;font:10px sans-serif;pointer-events:none';document.body.append(label)});
    await run('fixture-attachment-preview-close','actual UI with controlled attachment metadata and text bytes',async result=>{
      await p.locator('#collaboration-files > summary').tap(); await p.getByRole('button',{name:'Preview SUPPLEMENT-FIXTURE.txt',exact:true}).tap();
      const dialog=p.getByRole('dialog');await dialog.getByText(content).waitFor();result.geometry=await measure(p,dialog.locator('[data-slot="dialog-close"]'),'fixture-attachment-close');await dialog.locator('[data-slot="dialog-close"]').tap();
    });
    await run('fixture-memory-version-targets','actual UI with controlled memory current/version metadata and content',async result=>{
      const outer=p.locator('details').filter({has:p.locator('#memory')}); const summary=outer.locator(':scope > summary');await summary.tap();
      const panel=p.locator('#memory');await panel.getByRole('button',{name:'Version history',exact:true}).tap();
      const version=panel.getByRole('button',{name:'View v1',exact:true});result.version=await measure(p,version,'fixture-memory-version');await version.tap();
      const dialog=p.getByRole('dialog');await dialog.getByText(content).waitFor();result.close=await measure(p,dialog.locator('[data-slot="dialog-close"]'),'fixture-memory-close');await dialog.locator('[data-slot="dialog-close"]').tap();
    });
    await run('fixture-knowledge-review-summaries','actual UI with controlled candidate/list/history projections',async result=>{
      const outer=p.locator('details').filter({has:p.locator('#knowledge')});await outer.locator(':scope > summary').tap();
      await p.getByRole('button',{name:/SUPPLEMENT FIXTURE knowledge candidate/}).tap();const review=p.getByRole('region',{name:'Review candidate',exact:true});await review.waitFor();
      result.digest=await measure(p,review.locator('summary').filter({hasText:'Content SHA-256'}),'fixture-knowledge-digest');
      result.history=await measure(p,review.locator('summary').filter({hasText:'Version history'}),'fixture-knowledge-history');
    });
    await fixture.context.close();
  } finally { await browser.close(); evidence.fixtures=[...new Set(evidence.fixtures)];persist(); }
  console.log(JSON.stringify({cases:evidence.cases.map(x=>({name:x.name,status:x.status,reason:x.reason})),pageErrors:evidence.pageErrors,writes:evidence.writes},null,2));
})().catch(error=>{evidence.fatal=sanitized(error);persist();console.error('Supplement harness failed: '+evidence.fatal);process.exit(1)});
