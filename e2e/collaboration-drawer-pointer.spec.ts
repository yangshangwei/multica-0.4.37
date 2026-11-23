import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type Locator, type Page } from "@playwright/test";
import type { CollaborationIssue } from "@multica/core/collaboration";
import { TestApiClient } from "./fixtures";
import { deepLink, nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import { offlineHandoffEnvironment } from "./fixtures/collaboration-offline-handoff";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

async function pointerClick(page: Page, target: Locator) {
  await target.click({ trial: true });
  const box = await target.boundingBox();
  if (!box) throw new Error("The native pointer target has no painted bounds");
  const point = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
  expect(await target.evaluate((element, position) => element.contains(document.elementFromPoint(position.x, position.y)), point)).toBe(true);
  await page.mouse.click(point.x, point.y);
  return { box, point };
}

// Shared unit tests own panel behavior; this native case owns drag-region hit
// testing, ordinary pointer dismissal and retained draft/overlay lifetime.
// eslint-disable-next-line no-empty-pattern -- The real Electron fixture owns its page.
test("Electron resource drawer keeps its titlebar close control clickable and retains drafts across every dismissal", async ({}, info) => {
  test.setTimeout(120_000);
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1" || !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
    "Requires the task-owned password API and current production Desktop renderer");
  const { apiUrl } = offlineHandoffEnvironment();
  const configuration = await fetch(`${apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password" });
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  const actor = new TestApiClient();
  const user = await actor.registerPassword(`drawer${suffix}`, `Drawer-${randomUUID()}!`, "Drawer pointer owner");
  expect((await actor.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} })).onboarded_at).not.toBeNull();
  const workspace = await actor.ensureWorkspace("Drawer pointer source", `drawer-${suffix}`);
  const issue = await actor.requestJSON<CollaborationIssue>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: workspace.id, handler_user_id: user.id, title: `Drawer pointer ${suffix}`,
      description: "Manual collaboration used to verify native resource drawer controls.", ai_policy: "manual", attachment_ids: [],
    },
  });
  expect(issue.ai_policy).toBe("manual");
  const token = actor.getToken();
  if (!token) throw new Error("The isolated password actor has no session");
  const profile = mkdtempSync(join(tmpdir(), "multica-drawer-pointer-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  const evidence: Record<string, unknown> = {
    issue_id: issue.id, workspace_id: workspace.id, platform: process.platform,
    scope: "Real Electron renderer/preload and account routing fixture; not packaged Main or OS cursor automation",
    pointer: "Playwright mouse coordinates with ordinary actionability and hit-target checks; no forced clicks",
    provider_account_access: false, fixture_rows: "Retained in the task-owned acceptance database",
  };
  try {
    native = await openDesktop(profile, { token });
    const { page, app } = native;
    const memoryPath = `/api/collaboration/issues/${issue.id}/memory`;
    const memoryWrites: string[] = [];
    page.on("request", (request) => {
      if (new URL(request.url()).pathname === memoryPath && ["POST", "PUT", "PATCH", "DELETE"].includes(request.method())) memoryWrites.push(request.method());
    });
    await native.start();
    await deepLink(app, `/account?view=participating&issue=${issue.id}`);
    await expect(page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
    const resources = page.getByRole("group", { name: "Issue resources", exact: true });
    const memoryTrigger = resources.getByRole("button", { name: "Shared context", exact: true });
    const drawer = page.locator('[data-slot="dialog-content"]').filter({ has: page.locator("[data-collaboration-drawer-scroll]") });
    await expect(drawer).toHaveCount(1);
    await expect(drawer).toBeHidden();
    await expect(page.locator('[data-slot="dialog-overlay"]:visible')).toHaveCount(0);

    await pointerClick(page, memoryTrigger);
    await expect(drawer).toBeVisible();
    await expect(drawer).toHaveAccessibleName("Shared context");
    const popup = await drawer.elementHandle();
    const backdrop = await page.locator('[data-slot="dialog-overlay"]:visible').elementHandle();
    if (!popup || !backdrop) throw new Error("The retained drawer and backdrop must both exist");
    evidence.app_region = await drawer.evaluate((element) => getComputedStyle(element).getPropertyValue("-webkit-app-region"));
    expect(evidence.app_region, "The full-height resource drawer overlaps Electron's draggable titlebar").toBe("no-drag");

    await pointerClick(page, drawer.getByRole("button", { name: "Edit shared context", exact: true }));
    const editor = drawer.getByRole("textbox", { name: "Shared context (Markdown)", exact: true });
    const draft = `Keep this unsaved native draft ${suffix}`;
    await editor.fill(draft);
    const originalEditor = await editor.elementHandle();
    if (!originalEditor) throw new Error("The shared-context draft editor was not mounted");
    const categories = drawer.getByRole("group", { name: "Browse issue resources", exact: true });
    await pointerClick(page, categories.getByRole("button", { name: "Activity", exact: true }));
    await expect(editor).toBeHidden();
    await expect(drawer).toHaveAccessibleName("Activity");

    const assertClosed = async (opener: Locator) => {
      await expect(drawer).toBeHidden();
      await expect(page.locator('[data-slot="dialog-overlay"]:visible')).toHaveCount(0);
      await expect(page.getByRole("button", { name: "Close resources", exact: true })).toHaveCount(0);
      await expect(opener).toBeFocused();
      expect(await popup.evaluate((element) => element.isConnected)).toBe(true);
      await expect.poll(() => backdrop.evaluate((element) => element.isConnected && element.hasAttribute("hidden"))).toBe(true);
      expect(await originalEditor.evaluate((element) => element.isConnected)).toBe(true);
    };
    const assertDraft = async () => {
      await expect(editor).toHaveValue(draft);
      expect(await editor.evaluate((element, original) => element === original, originalEditor)).toBe(true);
    };
    const close = drawer.getByRole("button", { name: "Close resources", exact: true });
    const closeBox = await close.boundingBox();
    if (!closeBox) throw new Error("The titlebar close button has no bounds");
    expect(closeBox.y + closeBox.height / 2).toBeLessThan(48);
    await nativeScreenshot(app, page, info.outputPath("drawer-before-pointer-close.png"));
    evidence.close_target = await pointerClick(page, close);
    await assertClosed(memoryTrigger);

    const participants = page.getByRole("button", { name: "Participants", exact: true });
    await pointerClick(page, participants);
    await expect(participants).toHaveAttribute("aria-expanded", "true");
    await expect(page.getByRole("list", { name: "Participants", exact: true })).toContainText("Drawer pointer owner");
    await pointerClick(page, participants);
    await expect(participants).toHaveAttribute("aria-expanded", "false");

    await pointerClick(page, memoryTrigger);
    await assertDraft();
    await editor.focus();
    await page.keyboard.press("Escape");
    await assertClosed(memoryTrigger);

    await pointerClick(page, memoryTrigger);
    await assertDraft();
    const drawerBox = await drawer.boundingBox();
    if (!drawerBox || drawerBox.x < 80) throw new Error("Outside-click coverage requires the wide native drawer layout");
    const outside = { x: drawerBox.x / 2, y: Math.max(100, Math.min(drawerBox.height / 2, 300)) };
    expect(await page.evaluate((point) => Boolean(document.elementFromPoint(point.x, point.y)?.closest('[data-slot="dialog-overlay"]')), outside)).toBe(true);
    await page.mouse.click(outside.x, outside.y);
    evidence.outside_target = outside;
    await assertClosed(memoryTrigger);

    await pointerClick(page, memoryTrigger);
    await assertDraft();
    await pointerClick(page, close);
    await assertClosed(memoryTrigger);
    expect(memoryWrites).toEqual([]);
    expect(await actor.requestJSON<{ content: string; version: string }>(memoryPath)).toMatchObject({ content: "", version: "0" });
    expect(native.errors).toEqual([]);
    expect(await app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    await nativeScreenshot(app, page, info.outputPath("drawer-closed-underlying-controls.png"));
    evidence.outcome = "passed";
  } finally {
    try {
      await info.attach("native-drawer-pointer-evidence", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    } finally {
      try { if (native) await native.app.close(); }
      finally { rmSync(profile, { recursive: true, force: true }); }
    }
  }
});
