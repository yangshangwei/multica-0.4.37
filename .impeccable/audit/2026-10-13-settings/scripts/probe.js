// Navigate to settings and list the visible nav groups/tabs.
const slug = new URL(page.url()).pathname.split('/').filter(Boolean)[0];
console.log('URL', page.url(), 'slug', slug);
const link = page.locator('[data-sidebar="menu-button"][href$="/settings"]');
if (await link.count()) await link.first().click();
else await page.evaluate(s => { location.hash = ''; history.pushState({}, '', `/${s}/settings`); dispatchEvent(new PopStateEvent('popstate')); }, slug);
await page.waitForTimeout(1500);
console.log('URL2', page.url());
const nav = await page.evaluate(() => [...document.querySelectorAll('[role="tablist"] [role="group"]')].map(g => ({
  group: g.querySelector('h2')?.textContent,
  tabs: [...g.querySelectorAll('[role="tab"]')].map(t => t.textContent.trim()),
})));
console.log(JSON.stringify(nav, null, 1));
await shot('probe-settings');
