import { test, expect } from "@playwright/test";
import { seedRuntimeHandoff, assertRuntimeHandoff } from "../../../../../../e2e/fixtures/collaboration-runtime-handoff";
import { writeFileSync } from "node:fs";
test("diagnose source navigation against initial production snapshot", async ({ page }) => {
 const fixture = await seedRuntimeHandoff();
 await fixture.recipient.authenticateWeb(page);
 await page.addInitScript(() => {
  const events: unknown[] = [];
  Object.assign(window, { navigationDiagnosis: events });
  for (const method of ["pushState", "replaceState"] as const) {
   const original = history[method].bind(history);
   history[method] = function(data, unused, url) { events.push({ method, url: String(url), before: location.href, stack: new Error().stack }); return original(data, unused, url); };
  }
  for (const type of ["hashchange", "popstate"]) addEventListener(type, () => events.push({ type, url: location.href }));
 });
 await page.goto(fixture.path);
 const { handoff } = await assertRuntimeHandoff(page, fixture);
 const link = handoff.getByRole("link", { name: "Open source run", exact: true });
 await link.click();
 await page.waitForTimeout(2000);
 const result = await page.evaluate(() => ({ url: location.href, events: Reflect.get(window, "navigationDiagnosis") }));
 writeFileSync(new URL("./navigation-initial-events.json", import.meta.url), JSON.stringify(result, null, 2));
 expect(result.url).toBe(`http://localhost:13000${fixture.accountPath}#run-${fixture.priorRunId}`);
});
