import fs from 'node:fs';
import path from 'node:path';
import { chromium, expect as baseExpect } from '@playwright/test';

const expect = baseExpect.configure({ timeout: 20000 });
const session = JSON.parse(fs.readFileSync('/tmp/multica-ux-audit-session.json', 'utf8'));
const phase = process.argv[2] ?? 'after';
const out = path.resolve('.trellis/tasks/archive/2026-11/11-05-collaboration-ux-audit/evidence', phase);
fs.mkdirSync(out, { recursive: true });
const issueId = session.created[0].id;
const result = { source: session.web, api: session.api, engine: 'Playwright Chromium', harnessCorsShim: false,
  realTestOwnedApiData: true, productionBuild: true, nativeElectron: false, physicalTouch: false, checks: [], captures: [], errors: [] };
const browser = await chromium.launch();
const variants = [
  { name: 'wide-en-light', width: 1440, height: 1000, locale: 'en', theme: 'light', touch: false },
  { name: 'narrow-zh-dark', width: 390, height: 740, locale: 'zh-Hans', theme: 'dark', touch: true },
  { name: 'compact-en-light', width: 320, height: 568, locale: 'en', theme: 'light', touch: true },
  { name: 'medium-en-dark', width: 1024, height: 768, locale: 'en', theme: 'dark', touch: false },
];
const write = () => fs.writeFileSync(path.join(out, 'browser-results.json'), JSON.stringify(result, null, 2) + '\n');
try {
  for (const variant of variants) {
    const strings = JSON.parse(fs.readFileSync(`packages/views/locales/${variant.locale}/collaboration.json`, 'utf8'));
    const context = await browser.newContext({ viewport: { width: variant.width, height: variant.height }, locale: 'en-US', colorScheme: variant.theme, hasTouch: variant.touch });
    await context.addCookies([{ name: 'multica-locale', value: variant.locale, url: session.web }]);
    await context.addInitScript(({ token, theme }) => { localStorage.setItem('multica_token', token); localStorage.setItem('theme', theme); localStorage.setItem('multica:chat:isOpen', 'false'); }, { token: session.token, theme: variant.theme });
    let memoryFailure = false;
    let posts = 0;
    await context.route(session.api + '/**', async route => {
      const req = route.request();
      if (req.method() === 'POST') posts++;
      if (memoryFailure && new URL(req.url()).pathname === `/api/collaboration/issues/${issueId}/memory`) {
        return route.fulfill({ status: 503, headers: { 'access-control-allow-origin': session.web, 'access-control-allow-credentials': 'true' }, contentType: 'application/json', body: JSON.stringify({ error: 'Audit-injected read failure' }) });
      }
      await route.continue();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(20000); page.setDefaultNavigationTimeout(60000);
    page.on('pageerror', error => result.errors.push({ variant: variant.name, message: error.message }));
    async function capture(name, target) {
      if (target) await target.scrollIntoViewIfNeeded();
      await page.evaluate(async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); });
      const geometry = await page.evaluate(() => {
        const roots = [...document.querySelectorAll('[data-tab-scroll-root], [role="dialog"]')].filter(el => el.getBoundingClientRect().width > 0);
        return { width: innerWidth, height: innerHeight, documentWidth: document.documentElement.scrollWidth,
          headerHeight: document.querySelector('main > header')?.getBoundingClientRect().height,
          coarse: matchMedia('(pointer: coarse)').matches,
          roots: roots.map(el => ({ name: el.getAttribute('data-tab-scroll-root') || el.getAttribute('role'), width: el.clientWidth, scrollWidth: el.scrollWidth })) };
      });
      expect(geometry.documentWidth).toBeLessThanOrEqual(variant.width + 1);
      for (const root of geometry.roots) expect(root.scrollWidth).toBeLessThanOrEqual(root.width + 1);
      const filename = `${variant.name}-${name}.png`;
      await page.screenshot({ path: path.join(out, filename) });
      result.captures.push({ name: filename, geometry }); write();
    }
    async function goDetail() {
      await page.goto(`${session.web}/account?view=participating&issue=${issueId}`, { waitUntil: 'domcontentloaded' });
      await expect(page.getByRole('heading', { name: '跨工作空间协作体验验收', exact: true })).toBeVisible();
    }
    async function openDisclosure(label) {
      const summary = page.locator('summary').filter({ hasText: label }).first();
      await summary.scrollIntoViewIfNeeded();
      if (!(await summary.evaluate(el => el.parentElement.open))) await summary.click();
      return summary;
    }
    await page.goto(`${session.web}/account?view=participating`);
    await expect(page.getByRole('textbox', { name: strings.page.search, exact: true })).toBeVisible();
    await capture('list');
    await page.getByRole('textbox', { name: strings.page.search, exact: true }).fill('  Review release  ');
    await expect(page.getByRole('link', { name: /Review release handoff/ })).toBeVisible();
    if (phase !== 'before') {
      await page.getByRole('button', { name: strings.page.clearSearch, exact: true }).click();
      await expect(page.getByRole('textbox', { name: strings.page.search, exact: true })).toHaveValue('');
    }
    await goDetail();
    await capture('detail');
    await openDisclosure(strings.delivery.more);
    await page.getByRole('button', { name: strings.workflow.edit, exact: true }).focus();
    await page.keyboard.press('Enter');
    const editForm = page.getByRole('form', { name: strings.workflow.edit, exact: true });
    await expect(editForm).toBeVisible();
    if (phase !== 'before') expect(await editForm.evaluate(el => el.contains(document.activeElement))).toBe(true);
    await capture('edit-form', editForm);
    await editForm.getByRole('button', { name: strings.workflow.dismiss, exact: true }).click();
    const useResult = page.getByRole('button', { name: strings.delivery.useComment, exact: true }).last();
    await useResult.scrollIntoViewIfNeeded();
    const beforePosts = posts;
    await useResult.click();
    const resultForm = page.getByRole('form', { name: strings.delivery.selfComplete, exact: true });
    await expect(resultForm).toBeVisible();
    if (phase !== 'before') {
      expect(await resultForm.evaluate(el => el.contains(document.activeElement))).toBe(true);
      const rect = await resultForm.boundingBox();
      expect(rect.y).toBeGreaterThanOrEqual(0);
      expect(rect.y).toBeLessThan(variant.height - 40);
    }
    expect(posts).toBe(beforePosts);
    await capture('prepared-result');
    await resultForm.getByRole('button', { name: strings.workflow.dismiss, exact: true }).click();
    if (phase !== 'before') await expect(useResult).toBeFocused();
    result.checks.push({ variant: variant.name, workflowOpenFocus: phase !== 'before', commentPreparationNoWrite: true, commentCancelReturnsFocus: phase !== 'before' });
    await capture('discussion', page.locator('#discussion'));
    await openDisclosure(strings.memory.title);
    await expect(page.getByRole('button', { name: strings.memory.edit, exact: true })).toBeVisible();
    await page.getByRole('button', { name: strings.memory.edit, exact: true }).click();
    await capture('context-editor', page.getByRole('textbox', { name: strings.memory.editLabel, exact: true }));
    await page.getByRole('form', { name: strings.memory.edit, exact: true }).getByRole('button', { name: strings.memory.cancel, exact: true }).click();
    await openDisclosure(strings.knowledge.title);
    const propose = page.getByRole('button', { name: strings.knowledge.propose, exact: true });
    await expect(propose).toBeVisible();
    await propose.click();
    const knowledge = page.getByRole('form', { name: strings.knowledge.propose, exact: true });
    await expect(knowledge).toBeVisible();
    await capture('knowledge-proposal', knowledge.getByRole('textbox', { name: strings.knowledge.summary, exact: true }));
    if (variant.touch && phase !== 'before') {
      const box = await knowledge.getByRole('textbox', { name: strings.knowledge.summary, exact: true }).boundingBox();
      expect(box.height).toBeGreaterThanOrEqual(44);
      result.checks.push({ variant: variant.name, knowledgeSummaryTouchHeight: box.height });
    }
    await knowledge.getByRole('button', { name: strings.knowledge.cancel, exact: true }).click();
    await capture('attachments', await openDisclosure(strings.attachments.title));
    const aiSummary = page.locator('summary').filter({ hasText: strings.execution.title }).first();
    if (await aiSummary.count()) await capture('execution-resources', await openDisclosure(strings.execution.title));
    await capture('activity', await openDisclosure(strings.activity.title));
    await page.getByRole('button', { name: new RegExp(strings.page.notifications) }).click();
    await capture('notifications');
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: strings.creation.entry, exact: true }).click();
    const dialog = page.getByRole('dialog', { name: strings.creation.title, exact: true });
    await expect(dialog.getByRole('textbox', { name: strings.creation.titleLabel, exact: true })).toBeVisible();
    await capture('creation');
    if (variant.touch && phase !== 'before') {
      const box = await dialog.getByRole('button', { name: strings.creation.close, exact: true }).boundingBox();
      expect(box.height).toBeGreaterThanOrEqual(44); expect(box.width).toBeGreaterThanOrEqual(44);
    }
    await dialog.getByRole('button', { name: strings.creation.close, exact: true }).click();
    if (phase !== 'before') {
      memoryFailure = true;
      await page.reload();
      await expect(page.getByRole('heading', { name: '跨工作空间协作体验验收', exact: true })).toBeVisible();
      await openDisclosure(strings.memory.title);
      const memory = page.locator('#memory');
      await expect(memory.getByRole('alert')).toBeVisible();
      await capture('context-read-failure', memory);
      memoryFailure = false;
      await memory.getByRole('button', { name: strings.page.refresh, exact: true }).click();
      await expect(memory.getByRole('button', { name: strings.memory.edit, exact: true })).toBeVisible();
      result.checks.push({ variant: variant.name, injectedMemoryFailureRetry: true });
    }
    if (variant.name === 'wide-en-light') {
      await page.goto(`${session.web}/${session.workspace.slug}/issues/${issueId}`);
      await expect(page.getByRole('heading', { name: '跨工作空间协作体验验收', exact: true })).toBeVisible();
      await capture('source-workspace-detail');
      await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
      await capture('source-enlarged-text');
      result.checks.push({ variant: variant.name, sourceWorkspaceSharedDetail: true, enlargedText: '200% root font size, not browser zoom' });
    }
    await context.close();
  }
} catch (error) {
  result.failure = { message: error.message, stack: error.stack };
  const current = browser.contexts().flatMap(context => context.pages()).at(-1);
  if (current) { result.failure.url = current.url(); result.failure.body = (await current.locator('body').innerText()).slice(0,4000); await current.screenshot({ path: path.join(out, 'failure.png') }); }
  throw error;
} finally {
  write(); await browser.close();
}
console.log(JSON.stringify({ captures: result.captures.length, checks: result.checks, errors: result.errors, output: out }));
