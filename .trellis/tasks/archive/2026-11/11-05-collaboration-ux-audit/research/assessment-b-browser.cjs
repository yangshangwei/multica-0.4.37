const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('@playwright/test');
const out = __dirname;
const session = JSON.parse(fs.readFileSync('/tmp/multica-ux-audit-session.json', 'utf8'));
const issuePath = '/account?view=participating&issue=01a1a9a7-53d0-777a-9305-01532d06cf17';
(async () => {
  const browser = await chromium.launch({ headless: true });
  const evidence = { engine: 'Playwright Chromium', headless: true, source: session.web, api: session.api, harnessCorsShim: true, pages: [], console: [] };
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'en-US', colorScheme: 'light' });
    await context.addCookies([{ name: 'multica_logged_in', value: '1', url: session.web }]);
    await context.addInitScript(({token}) => {localStorage.setItem('multica_token',token);localStorage.setItem('multica-locale','en');localStorage.setItem('multica:chat:isOpen','false');}, {token:session.token});
    await context.route(session.api + '/**', async route => {
      const origin = session.web;
      const cors = { 'access-control-allow-origin': origin, 'access-control-allow-credentials': 'true', 'access-control-allow-methods': 'GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS', 'access-control-allow-headers': 'authorization,content-type,x-workspace-id,x-csrf-token' };
      if (route.request().method() === 'OPTIONS') return route.fulfill({status:204,headers:cors});
      try { const response = await route.fetch(); await route.fulfill({response,headers:{...response.headers(),...cors}}); } catch { /* Closing this isolated context aborts outstanding reads. */ }
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('console', msg => {if (msg.text().includes('impeccable')) evidence.console.push({type:msg.type(),text:msg.text()});});
    await page.goto(session.web + issuePath, {waitUntil:'domcontentloaded'});
    await page.locator('[data-tab-scroll-root="account-detail"] h2').waitFor();
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(1800);
    evidence.body = (await page.locator('body').innerText()).slice(0,14000);
    evidence.injection = await page.evaluate(() => {document.title='[Human] Impeccable audit';const s=document.createElement('script');s.textContent='window.__impeccableAuditProbe = true';document.head.append(s);return {titleChanged:document.title==='[Human] Impeccable audit',scriptRan:window.__impeccableAuditProbe===true};});
    async function measure(name) {
      evidence.pages.push({name, geometry:await page.evaluate(()=>({width:innerWidth,height:innerHeight,bodyOverflow:document.documentElement.scrollWidth>innerWidth,roots:[...document.querySelectorAll('[data-tab-scroll-root]')].filter(e=>e.getBoundingClientRect().width).map(e=>({name:e.getAttribute('data-tab-scroll-root'),width:e.clientWidth,scrollWidth:e.scrollWidth})),overflow:[...document.querySelectorAll('main *')].filter(e=>{const b=e.getBoundingClientRect();return b.width>0&&(b.right>innerWidth+1||b.left<-1)}).slice(0,20).map(e=>({tag:e.tagName,text:e.textContent.slice(0,90),right:e.getBoundingClientRect().right})),smallControls:[...document.querySelectorAll('main button,main a,main input,main select,main textarea,main summary,[role=dialog] button')].filter(e=>{const b=e.getBoundingClientRect();return b.width>0&&b.height>0&&b.top>=0&&b.bottom<=innerHeight&&(b.height<44||b.width<44)}).map(e=>({tag:e.tagName,text:e.getAttribute('aria-label')||e.textContent.slice(0,80),height:e.getBoundingClientRect().height,width:e.getBoundingClientRect().width}))}))});
      await page.screenshot({path:path.join(out,`assessment-b-${name}.png`)});
      fs.writeFileSync(path.join(out,'assessment-b-browser-evidence.json'),JSON.stringify(evidence,null,2));
    }
    async function overlay(name) {
      try { await page.addScriptTag({url:'http://localhost:8400/detect.js'}); await page.waitForTimeout(2200); evidence.pages.push({name:name+'-overlay',injected:true}); await page.screenshot({path:path.join(out,`assessment-b-${name}-overlay.png`)}); } catch { evidence.pages.push({name:name+'-overlay',injected:false}); }
    }
    await measure('desktop-detail');
    const more=page.locator('summary').filter({hasText:'More actions'}); await more.click();
    const edit=page.getByRole('button',{name:'Edit issue',exact:true}); await edit.focus(); await page.keyboard.press('Enter');
    evidence.workflowFocus=await page.evaluate(()=>({tag:document.activeElement.tagName,text:document.activeElement.textContent,form:document.activeElement.closest('form')?.getAttribute('aria-label')}));
    await page.getByRole('button',{name:'Close',exact:true}).click();
    await overlay('desktop-detail');
    await page.goto(session.web+issuePath); await page.locator('[data-tab-scroll-root="account-detail"] h2').waitFor(); await page.waitForTimeout(1000);
    await page.setViewportSize({width:390,height:740}); await measure('narrow-detail');
    await page.getByRole('button',{name:'New collaboration issue',exact:true}).click();
    const dialog=page.getByRole('dialog'); await dialog.getByLabel('Title',{exact:true}).waitFor();
    await page.waitForTimeout(350); await measure('narrow-create'); await overlay('narrow-create');
    await page.keyboard.press('Escape');
    await page.goto(session.web+'/account?view=participating'); await page.getByRole('textbox',{name:'Search loaded issues'}).waitFor().catch(()=>{});
    await page.waitForTimeout(800); await measure('narrow-list'); await overlay('narrow-list');
    await page.goto(session.web+issuePath); await page.locator('[data-tab-scroll-root="account-detail"] h2').waitFor();
    await page.evaluate(()=>{document.documentElement.classList.remove('light');document.documentElement.classList.add('dark');});
    await page.waitForTimeout(500); await measure('narrow-dark-detail');
    await page.setViewportSize({width:320,height:568}); await measure('compact-detail');
    fs.writeFileSync(path.join(out,'assessment-b-browser-evidence.json'),JSON.stringify(evidence,null,2));
    console.log('Evidence captured',evidence.pages.length,'surface measurements and overlays.');
  } finally { await browser.close(); }
})().catch(error => {console.error(error.message.split('Call log:')[0]);process.exit(1);});
