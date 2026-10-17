// Run a targeted inspection against the disposable audit renderer.
import { chromium, expect } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
const auditDir = path.resolve(import.meta.dirname, '..');
const dir = process.env.SETTINGS_AUDIT_OUTPUT_DIR
  ? path.resolve(process.env.SETTINGS_AUDIT_OUTPUT_DIR)
  : auditDir;
const browser = await chromium.connectOverCDP('http://127.0.0.1:9367');
const context = browser.contexts()[0];
const page = context.pages().find(p => p.url().startsWith('http://localhost:5666'));
if (!page) throw new Error('Resumed audit renderer unavailable');
page.setDefaultTimeout(6000);
const cdp = await context.newCDPSession(page);
const shot = async name => {
  const capture = await cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false, fromSurface: true });
  const file = path.join(dir, 'screenshots', `${name}.png`);
  await fs.writeFile(file, Buffer.from(capture.data, 'base64'));
  return file;
};
const snap = async () => console.log((await page.locator('body').ariaSnapshot()).slice(0, 18000));
const control = async (route, values = {}) => {
  const url = new URL(route, 'http://127.0.0.1:9368');
  for (const [key, value] of Object.entries(values)) url.searchParams.set(key, String(value));
  const response = await fetch(url);
  if (!response.ok) throw new Error(await response.text());
  const result = await response.json();
  if (route === '/viewport') await page.waitForTimeout(400);
  return result;
};
const navigate = async tab => {
  const labels = {
    profile: '个人资料', preferences: '偏好设置', notifications: '通知',
    shortcuts: '快捷键', tokens: 'API Token', workspace: '常规',
    members: '成员', labels: '标签', triage: '分拣台', iterations: '迭代',
    'issue-statuses': '任务状态', properties: '属性', 'quick-actions': '快捷操作',
    repositories: '代码仓库', integrations: '集成', mcp: 'MCP',
    daemon: '守护进程', server: '服务器连接', updates: '更新', behavior: '行为',
  };
  const label = labels[tab] ?? tab;
  const target = page.getByRole('tab', { name: label, exact: true });
  if (!await target.isVisible().catch(() => false)) {
    const trigger = page.getByRole('button', { name: /^设置目录/ });
    if (await trigger.isVisible().catch(() => false)) await trigger.click();
  }
  await target.click();
  await expect(page.getByRole('tabpanel', { name: label, exact: true })).toBeVisible();
  await page.waitForTimeout(400);
};
const queryClient = async () => {
  await page.evaluate(() => {
    const root = document.getElementById('root');
    const key = Object.keys(root).find(key => key.startsWith('__reactContainer'));
    const stack = [root[key].stateNode.current];
    while (stack.length) {
      const fiber = stack.pop();
      const client = fiber.memoizedProps?.client;
      if (client?.getQueryCache) { window.__auditQC = client; return; }
      if (fiber.child) stack.push(fiber.child);
      if (fiber.sibling) stack.push(fiber.sibling);
    }
    throw new Error('QueryClient not found');
  });
};
let script = '';
for await (const chunk of process.stdin) script += chunk;
try {
  await new (Object.getPrototypeOf(async () => {}).constructor)(
    'page', 'context', 'browser', 'cdp', 'fs', 'path', 'dir', 'expect', 'shot', 'snap', 'control', 'navigate', 'queryClient', 'auditDir',
    script,
  )(page, context, browser, cdp, fs, path, dir, expect, shot, snap, control, navigate, queryClient, auditDir);
} catch (error) {
  console.error(error.stack);
  process.exitCode = 1;
}
await cdp.detach();
process.exit(process.exitCode ?? 0);
