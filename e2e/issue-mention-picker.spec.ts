import { mkdirSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { _electron as electron, expect, test, type Locator, type Page, type TestInfo } from "@playwright/test";
import en from "../packages/views/locales/en/editor.json" with { type: "json" };
import zh from "../packages/views/locales/zh-Hans/editor.json" with { type: "json" };
import enIssues from "../packages/views/locales/en/issues.json" with { type: "json" };
import zhIssues from "../packages/views/locales/zh-Hans/issues.json" with { type: "json" };
import { actorId, creationPreferences, mentionRow, picker, prepareMentionPage, seedMentionWorkspace, type MentionFixture } from "./fixtures/issue-mention-picker";

test.skip(process.env.ISSUE_MENTION_E2E !== "1", "Requires an isolated mention acceptance environment");
test.setTimeout(120_000);
test.use({ actionTimeout: 15_000 });

async function typeMention(editor: Locator, query = "") {
  await editor.fill("");
  await editor.pressSequentially(`@${query}`);
}

async function inserted(page: Page, editor: Locator, fixture: MentionFixture, type: "agent" | "squad", index: number, draftKey = `new:${fixture.issue.id}`) {
  await expect(editor.locator(".mention")).toHaveText(`@${type === "agent" ? "Agent" : "Squad"} ${String(index).padStart(3, "0")}`);
  await expect.poll(() => page.evaluate(({ slug, draftKey }) => JSON.parse(localStorage.getItem(`multica_comment_drafts:${slug}`) || '{"state":{"drafts":{}}}').state.drafts[draftKey]?.content, { slug: fixture.workspace.slug, draftKey }))
    .toContain(`mention://${type}/${actorId(type, index)}`);
}

async function screenshot(page: Page, info: TestInfo, name: string) {
  const box = await picker(page).boundingBox();
  const viewport = await page.evaluate(() => ({ width: innerWidth, height: innerHeight }));
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(viewport.width);
  expect(box!.y + box!.height).toBeLessThanOrEqual(viewport.height);
  const directory = process.env.ISSUE_MENTION_EVIDENCE_DIR;
  if (directory) mkdirSync(directory, { recursive: true });
  await page.screenshot({ path: directory ? join(directory, name) : info.outputPath(name), animations: "disabled" });
}

async function discovery(page: Page, editor: Locator, fixture: MentionFixture, locale: "en" | "zh-Hans", info: TestInfo, native = false) {
  const copy = (locale === "en" ? en : zh).mention;
  const popup = picker(page);
  const category = (name: string) => popup.getByRole("button", { name, exact: true });
  const preferences = await creationPreferences(page, fixture.workspace.slug);
  await typeMention(editor);
  await expect(popup).toBeVisible();
  await screenshot(page, info, `mention-${native ? "native" : "web"}-${locale}-initial.png`);
  for (const label of [copy.category_all, copy.category_agents, copy.category_squads, copy.category_members, copy.group_issues])
    await expect(category(label)).toBeVisible();

  await category(copy.category_agents).click();
  await screenshot(page, info, `mention-${native ? "native" : "web"}-${locale}-agents.png`);
  await expect(popup.locator('[data-mention-key^="agent:"]')).toHaveCount(50);
  await category(copy.leaders).click();
  await expect(popup.locator('[data-mention-key]')).toHaveCount(1);
  await expect(mentionRow(page, "agent", actorId("agent", 0))).toBeVisible();
  await category(copy.all_agents).click();
  await popup.getByRole("button", { name: /^(Show more|显示更多)/ }).click();
  await expect(popup.locator('[data-mention-key^="agent:"]')).toHaveCount(100);
  await expect(mentionRow(page, "agent", actorId("agent", 75))).toBeAttached();

  await category(copy.category_squads).click();
  await page.keyboard.type("squad-duty-0049");
  await expect(mentionRow(page, "squad", actorId("squad", 49))).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(popup).toHaveCount(0);
  await inserted(page, editor, fixture, "squad", 49);

  await typeMention(editor, "responsibility-0498");
  await expect(mentionRow(page, "agent", actorId("agent", 498))).toBeVisible();
  await editor.press("Enter");
  await inserted(page, editor, fixture, "agent", 498);
  await typeMention(editor, "ceshigongchengshi");
  await expect(mentionRow(page, "agent", actorId("agent", 499))).toBeVisible();
  await typeMention(editor, "no-such-mention-zzzz");
  await expect(popup.getByText(copy.no_results, { exact: true })).toBeVisible();
  await typeMention(editor);
  await category(copy.category_members).click();
  await expect(mentionRow(page, "member", fixture.userId)).toBeVisible();
  await category(copy.group_issues).click();
  await page.keyboard.type(fixture.target.identifier);
  await expect(mentionRow(page, "issue", fixture.target.id)).toBeVisible();
  await typeMention(editor);

  await category(copy.category_all).click();
  await editor.press("Alt+ArrowDown");
  await expect(category(copy.category_all)).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await expect(category(copy.category_agents)).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(popup).toHaveCount(0);
  await expect(editor).toBeFocused();
  await editor.pressSequentially(" @");
  await expect(popup).toBeVisible();
  await editor.press("Alt+ArrowDown");
  const returnButton = category(copy.return_typing);
  for (let attempt = 0; attempt < 8 && !(await returnButton.evaluate((element) => element === document.activeElement)); attempt++)
    await page.keyboard.press("Tab");
  await expect(returnButton).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(editor).toBeFocused();
  await page.keyboard.type("Agent 498");
  await expect(mentionRow(page, "agent", actorId("agent", 498))).toBeVisible();

  if (native) await screenshot(page, info, `mention-native-${locale}.png`);
  else for (const width of [1280, 375]) {
    if (page.viewportSize()?.width !== width) {
      await editor.press("Escape");
      await page.setViewportSize({ width, height: 900 });
      await page.reload({ waitUntil: "domcontentloaded" });
      await typeMention(editor, "Agent 498");
    }
    await expect(mentionRow(page, "agent", actorId("agent", 498))).toBeVisible();
    await screenshot(page, info, `mention-${locale}-${width}.png`);
  }
  await editor.press("Enter");
  await expect(popup).toHaveCount(0);
  await inserted(page, editor, fixture, "agent", 498);
  await expect.poll(() => page.evaluate((slug) => localStorage.getItem(`multica_comment_drafts:${slug}`), fixture.workspace.slug))
    .toContain(`mention://agent/${actorId("agent", 498)}`);
  expect(await creationPreferences(page, fixture.workspace.slug)).toBe(preferences);
}

async function otherHosts(page: Page, fixture: MentionFixture) {
  const toggle = page.locator(`button[aria-controls="comment-body-${fixture.parent.id}"]`);
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  await page.getByTestId("reply-composer-shell").click();
  const replyEditor = page.getByRole("textbox", { name: enIssues.reply.placeholder, exact: true });
  await typeMention(replyEditor, "Agent 498");
  await expect(mentionRow(page, "agent", actorId("agent", 498))).toBeVisible();
  await replyEditor.press("Tab");
  await inserted(page, replyEditor, fixture, "agent", 498, `reply:${fixture.issue.id}:${fixture.parent.id}`);
  await replyEditor.fill("");
  for (const id of [fixture.parent.id, fixture.reply.id]) {
    const card = page.locator(`#comment-${id}`);
    await card.getByRole("button", { name: enIssues.comment.more_actions, exact: true }).first().click();
    await page.getByRole("menuitem", { name: enIssues.comment.edit_action, exact: true }).click();
    const edit = card.locator('.ProseMirror[contenteditable="true"]').first();
    await typeMention(edit, "Agent 498");
    await expect(mentionRow(page, "agent", actorId("agent", 498))).toBeVisible();
    await edit.press("Enter");
    await inserted(page, edit, fixture, "agent", 498, `edit:${fixture.issue.id}:${id}`);
    await card.getByRole("button", { name: enIssues.comment.cancel_edit, exact: true }).click();
  }
}

for (const locale of ["en", "zh-Hans"] as const) {
  test(`comment mention discovery preserves unsent drafts (${locale})`, async ({ page }, info) => {
    const fixture = await seedMentionWorkspace(locale);
    try {
      const origin = info.project.use.baseURL;
      if (typeof origin !== "string") throw new Error("Missing isolated Web baseURL");
      const diagnostics = await prepareMentionPage(page, fixture, origin, locale, locale === "en" ? "light" : "dark");
      await page.setViewportSize({ width: 1280, height: 900 });
      await page.goto(`/${fixture.workspace.slug}/issues/${fixture.issue.id}`);
      await page.getByTestId("comment-composer-shell").click();
      const editor = page.getByRole("textbox", { name: (locale === "en" ? enIssues : zhIssues).comment.field_label, exact: true });
      await discovery(page, editor, fixture, locale, info);
      if (locale === "en") {
        await page.setViewportSize({ width: 1280, height: 900 });
        await page.reload({ waitUntil: "domcontentloaded" });
        await otherHosts(page, fixture);
      }
      await info.attach("renderer diagnostics", { body: JSON.stringify(diagnostics), contentType: "application/json" });
      expect(diagnostics.writes).toEqual([]);
      expect(diagnostics.errors).toEqual([]);
      expect(diagnostics.failures).toEqual([]);
      const comments = await fixture.api.requestJSON<unknown[]>(`/api/issues/${fixture.issue.id}/comments`);
      expect(comments).toHaveLength(2);
    } finally { await fixture.api.deleteFeatureWorkspace(fixture.workspace.id); }
  });
}

test("native Electron mention discovery preserves its draft without daemon execution", async ({}, info) => {
  test.skip(!process.env.CHANGELOG_ELECTRON_RENDERER_URL || !process.env.CHANGELOG_E2E_API_URL, "Requires built renderer/preload and isolated API");
  const fixture = await seedMentionWorkspace();
  const profile = mkdtempSync(join(tmpdir(), "multica-mention-picker-"));
  const root = resolve(import.meta.dirname, "..");
  let closeMocks: (() => Promise<void>) | undefined;
  let desktop: Awaited<ReturnType<typeof electron.launch>> | undefined;
  try {
    desktop = await electron.launch({
      executablePath: join(root, "apps/desktop/node_modules/electron/dist", process.platform === "darwin" ? "Electron.app/Contents/MacOS/Electron" : process.platform === "win32" ? "electron.exe" : "electron"),
      args: [join(root, "e2e/fixtures/changelog-electron.cjs")],
      env: { ...process.env, CHANGELOG_ELECTRON_PROFILE: profile, CHANGELOG_ELECTRON_SYSTEM_LOCALE: "en" },
    });
    const page = await desktop.firstWindow();
    page.setDefaultTimeout(15_000);
    await page.waitForURL(new URL(process.env.CHANGELOG_ELECTRON_RENDERER_URL!).href);
    const diagnostics = await prepareMentionPage(page, fixture, process.env.CHANGELOG_ELECTRON_RENDERER_URL!, "en", "light", true);
    closeMocks = diagnostics.close;
    await page.reload();
    await page.getByRole("link", { name: "Workspace issues", exact: true }).click();
    await page.getByText(fixture.issue.title, { exact: true }).first().click();
    await page.getByTestId("comment-composer-shell").click();
    await discovery(page, page.getByRole("textbox", { name: enIssues.comment.field_label, exact: true }), fixture, "en", info, true);
    await info.attach("native diagnostics", { body: JSON.stringify(diagnostics), contentType: "application/json" });
    expect(diagnostics.writes).toEqual([]);
    expect(diagnostics.errors).toEqual([]);
    expect(diagnostics.failures).toEqual([]);
    expect(await desktop.evaluate(() => (globalThis as unknown as { changelogAcceptance: { daemonStarts: number; installCalls: number } }).changelogAcceptance))
      .toMatchObject({ daemonStarts: 0, installCalls: 0 });
  } catch (error) {
    const page = desktop?.windows()[0];
    if (page && !page.isClosed()) {
      await info.attach("native failure", { body: await page.screenshot({ path: info.outputPath("native-failure.png") }), contentType: "image/png" });
      await info.attach("native failure DOM", { body: await page.locator("body").innerText(), contentType: "text/plain" });
    }
    throw error;
  } finally {
    try { await desktop?.close(); await closeMocks?.(); }
    finally {
      try { await fixture.api.deleteFeatureWorkspace(fixture.workspace.id); }
      finally { rmSync(profile, { recursive: true, force: true }); }
    }
  }
});
