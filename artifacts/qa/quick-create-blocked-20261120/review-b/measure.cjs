const { chromium, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');
const fixture = JSON.parse(fs.readFileSync(process.env.REVIEW_AUTH_FILE, 'utf8'));
const out = __dirname;
const labels = {
  en: { open: 'New Issue', submit: 'Send to agent', suggest: 'Suggest an agent', refine: 'Help me refine', task: 'Task description', setup: 'Set up an agent', none: 'No agent or squad can take this' },
  'zh-Hans': { open: '新建任务', submit: '交给智能体', suggest: '帮我选智能体', refine: '帮我完善', task: '任务描述', setup: '去配置智能体', none: '没有可以交给的智能体或AI小队' },
};

async function inspect(page, language, state) {
  const l = labels[language];
  return page.getByRole('dialog').evaluate((dialog, { l, language, state }) => {
    const rect = element => {
      if (!element) return null;
      const b = element.getBoundingClientRect();
      return { x: b.x, y: b.y, width: b.width, height: b.height, right: b.right, bottom: b.bottom };
    };
    const name = element => (element.getAttribute('aria-label') || element.textContent || '').trim();
    const buttons = [...dialog.querySelectorAll('button')];
    const rec = dialog.querySelector('[data-testid="creator-recommendations"]');
    const suggest = rec.querySelector('button[data-slot="button"]');
    const refine = buttons.find(button => name(button) === l.refine);
    const submit = buttons.find(button => name(button).startsWith(l.submit));
    const picker = rec.querySelector('button[data-slot="popover-trigger"]');
    const hint = [...rec.querySelectorAll('p')].find(p => /Assigned to your selection|默认由你选/.test(p.textContent));
    const task = [...dialog.querySelectorAll('p')].find(p => p.textContent === l.task);
    const footer = submit.parentElement;
    const canvas = document.createElement('canvas'); canvas.width = 1; canvas.height = 1;
    const context = canvas.getContext('2d', { willReadFrequently: true });
    const rgba = css => {
      context.clearRect(0, 0, 1, 1); context.fillStyle = css; context.fillRect(0, 0, 1, 1);
      return [...context.getImageData(0, 0, 1, 1).data].map((v, i) => i === 3 ? v / 255 : v);
    };
    const compose = (fg, bg) => [0, 1, 2].map(i => fg[i] * fg[3] + bg[i] * (1 - fg[3]));
    const luminance = color => color.map(v => v / 255).map(v => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
    const tone = button => {
      if (!button) return null;
      const chain = []; for (let node = button; node; node = node.parentElement) chain.unshift(node);
      let background = [255, 255, 255]; let opacity = 1;
      const layers = chain.map(node => { const s = getComputedStyle(node); return { tag: node.tagName, color: s.backgroundColor, opacity: Number(s.opacity) }; });
      for (const layer of layers) { const color = rgba(layer.color); color[3] *= layer.opacity; background = compose(color, background); opacity *= layer.opacity; }
      const style = getComputedStyle(button); const foreground = rgba(style.color); foreground[3] *= opacity;
      const rendered = compose(foreground, background); const a = luminance(rendered); const b = luminance(background);
      return { text: name(button), disabled: button.disabled, cssColor: style.color, opacity, rendered: rendered.map(v => +v.toFixed(2)), background: background.map(v => +v.toFixed(2)), contrast: +( (Math.max(a,b)+0.05)/(Math.min(a,b)+0.05) ).toFixed(4), layers: layers.filter(v => v.opacity !== 1 || rgba(v.color)[3] > 0) };
    };
    const sb = rect(submit); const fb = rect(footer); const db = rect(dialog); const pb = rect(picker); const hb = rect(hint); const tb = rect(task);
    const point = sb && document.elementFromPoint(sb.x + sb.width / 2, sb.y + sb.height / 2);
    const descriptionId = submit.getAttribute('aria-describedby');
    return {
      language, state, viewport: { width: innerWidth, height: innerHeight }, theme: document.documentElement.className,
      bodyWidth: document.documentElement.scrollWidth, dialog: db, footer: fb, submit: sb,
      footerFits: fb.x >= db.x && fb.right <= db.right && fb.bottom <= db.bottom && fb.bottom <= innerHeight,
      submitFullyVisible: sb.x >= 0 && sb.right <= innerWidth && sb.y >= 0 && sb.bottom <= innerHeight,
      submitHitTarget: point === submit || submit.contains(point),
      submitDisabled: submit.disabled, submitAriaDisabled: submit.getAttribute('aria-disabled'),
      submitDescription: descriptionId ? document.getElementById(descriptionId)?.textContent : null,
      picker: pb, hint: hb, task: tb,
      pickerToHint: pb && hb ? +(hb.y-pb.bottom).toFixed(2) : null,
      hintToTask: hb && tb ? +(tb.y-hb.bottom).toFixed(2) : null,
      suggest: tone(suggest), refine: tone(refine),
      visibleStatuses: [...dialog.querySelectorAll('[role="status"]')].map(el => el.textContent.trim()).filter(Boolean),
      visibleButtons: buttons.filter(button => button.getBoundingClientRect().width > 0).map(name),
    };
  }, { l, language, state });
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const measurements = [];
  try {
    for (const noRecipients of [false, true]) for (const language of ['en', 'zh-Hans']) for (const theme of ['light', 'dark']) for (const width of [1280, 375]) {
      const key = `${language}-${theme}-${width}-${noRecipients ? 'no-recipients' : 'available'}`;
      const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: language === 'en' ? 'en-US' : 'zh-CN', colorScheme: theme });
      const network = { recommendations: 0, creates: 0 };
      await context.addCookies([{ name: 'multica-locale', value: language, url: 'http://localhost:60677' }, { name: 'multica_logged_in', value: '1', url: 'http://localhost:60677' }]);
      await context.addInitScript(({ token, language, theme }) => {
        localStorage.setItem('multica_token', token); localStorage.setItem('multica-locale', language);
        localStorage.setItem('multica:chat:isOpen', 'false'); localStorage.setItem('theme', theme);
        localStorage.setItem('multica_create_mode', JSON.stringify({ state: { lastMode: 'agent' }, version: 0 }));
      }, { token: fixture.token, language, theme });
      await context.route('**/api/me', async route => {
        if (route.request().method() !== 'GET') return route.continue();
        const response = await route.fetch(); const body = await response.json();
        await route.fulfill({ response, json: { ...body, language } });
      });
      if (noRecipients) {
        await context.route(/\/api\/agents(?:\?|$)/, route => route.fulfill({ json: [] }));
        await context.route(/\/api\/squads(?:\?|$)/, route => route.fulfill({ json: [] }));
      }
      await context.route('**/api/issues/recommend-creators', route => { network.recommendations++; return route.fulfill({ status: 503, json: { code: 'ai_unavailable' } }); });
      await context.route('**/api/issues/quick-create', route => { network.creates++; return route.fulfill({ status: 503, json: { error: 'review-only' } }); });
      const page = await context.newPage();
      await page.goto(fixture.url, { waitUntil: 'domcontentloaded' });
      await page.getByRole('button', { name: labels[language].open, exact: true }).click({ timeout: 10000 });
      const dialog = page.getByRole('dialog'); const submit = dialog.getByRole('button', { name: labels[language].submit, exact: true });
      if (noRecipients) await expect(dialog.getByText(labels[language].none, { exact: true }).first()).toBeVisible();
      else await expect(dialog.locator('[data-testid="creator-recommendations"] button[data-slot="popover-trigger"]')).toContainText(fixture.agent.name);
      await page.setViewportSize({ width, height: width === 375 ? 812 : 900 });
      await page.mouse.move(0, 0); await page.waitForTimeout(350);
      const empty = await inspect(page, language, 'empty');
      await page.screenshot({ path: path.join(out, `${key}-empty.png`), animations: 'disabled' });
      await submit.focus(); await page.waitForTimeout(350);
      empty.focusedReasonVisible = await page.getByRole('tooltip').isVisible().catch(() => false);
      const submitBox = await submit.boundingBox();
      await page.mouse.click(submitBox.x + submitBox.width / 2, submitBox.y + submitBox.height / 2);
      empty.focusAfterBlocked = await page.evaluate(() => ({ tag: document.activeElement.tagName, text: document.activeElement.textContent, contenteditable: document.activeElement.getAttribute('contenteditable') }));
      await dialog.locator('.ProseMirror.rich-text-editor[contenteditable="true"]').fill(language === 'en' ? 'Review the refund failure and verify the fix.' : '排查退款失败并验证修复。');
      await page.mouse.move(0, 0); await page.waitForTimeout(400);
      const filled = await inspect(page, language, 'filled');
      await page.screenshot({ path: path.join(out, `${key}-filled.png`), animations: 'disabled' });
      measurements.push({ key, noRecipients, empty, filled, network });
      fs.writeFileSync(path.join(out, 'measurements.json'), JSON.stringify(measurements, null, 2));
      console.log(JSON.stringify({ key, gaps: [empty.pickerToHint, empty.hintToTask], tones: [empty.suggest?.contrast, empty.refine?.contrast], fits: empty.footerFits && filled.footerFits }));
      await context.close();
    }
  } finally { await browser.close(); }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
