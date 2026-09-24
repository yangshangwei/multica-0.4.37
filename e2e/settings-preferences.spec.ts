import { randomUUID } from "node:crypto";
import { test as base, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

interface PreferencesFixture {
  api: TestApiClient;
  slug: string;
}

const test = base.extend<{ preferences: PreferencesFixture }>({
  preferences: async ({ page, baseURL }, runFixture, testInfo) => {
    const slug = `e2e-preferences-${randomUUID().slice(0, 12)}`;
    const api = new TestApiClient();
    await api.login(`${slug}@multica.ai`, "Preferences regression owner");
    const workspace = await api.ensureWorkspace("Preferences regression", slug);
    expect(workspace.slug).toBe(slug);

    try {
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
      const token = api.getToken();
      if (!token || !baseURL) throw new Error("Preferences fixture authentication is incomplete");
      await page.context().addCookies([
        { name: "multica-locale", value: "en", url: baseURL },
        { name: "multica_logged_in", value: "1", url: baseURL },
      ]);
      await page.addInitScript((value) => {
        localStorage.setItem("multica_token", value);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, token);
      await page.goto(`/${slug}/issues`);
      await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
      await runFixture({ api, slug });
    } finally {
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach("preferences-failure", {
          body: await page.screenshot({ fullPage: true }), contentType: "image/png",
        });
      }
      await api.deleteFeatureWorkspace(workspace.id);
      expect((await api.getWorkspaces()).some((item) => item.id === workspace.id)).toBe(false);
    }
  },
});

test.use({ viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure" });

async function openPreferences(page: Page, slug: string) {
  await page.getByRole("link", { name: "Settings", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/settings`);
  await expect(page.getByRole("tab", { name: "Profile", exact: true })).toHaveAttribute("aria-selected", "true");
  await page.getByRole("tab", { name: "Preferences", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/settings?tab=preferences`);
  await expect(page.getByRole("tab", { name: "Preferences", exact: true })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("heading", { name: "Preferences", exact: true })).toBeVisible();
}

test("grouped settings remain reachable from the sidebar and floating chat survives navigation and reload", async ({ page, preferences }) => {
  const { slug } = preferences;
  await page.getByRole("link", { name: "Settings", exact: true }).click();
  await expect(page.getByRole("tab", { name: "Profile", exact: true })).toHaveAttribute("aria-selected", "true");
  const navigation = page.getByRole("tablist");
  for (const group of ["Personal", "Workspace", "Issues", "Connections"]) {
    await expect(navigation.getByText(group, { exact: true })).toBeVisible();
  }

  // Visit one real page from each non-personal group before returning to
  // Preferences. The navigation definition's exhaustive matrix is unit tested.
  for (const [label, tab] of [["General", "workspace"], ["Issue Statuses", "issue-statuses"], ["Repositories", "repositories"]]) {
    await page.getByRole("tab", { name: label, exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`tab=${tab}(?:&|$)`));
    await expect(page.getByRole("tabpanel").getByRole("heading", { name: label, exact: true })).toBeVisible();
  }
  await page.getByRole("tab", { name: "Preferences", exact: true }).click();
  const floating = page.getByRole("switch", { name: "Floating chat window", exact: true });
  const launcher = page.getByRole("button", { name: "Ask Multica", exact: true });
  await expect(floating).toBeChecked();
  await floating.uncheck();
  await expect(launcher).toHaveCount(0);

  await page.getByRole("link", { name: "Issues", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/issues`);
  await page.reload();
  await expect(page.getByRole("button", { name: "New Issue", exact: true })).toBeVisible();
  await expect(launcher).toHaveCount(0);
  await openPreferences(page, slug);
  await expect(floating).not.toBeChecked();

  await floating.check();
  await expect(launcher).toBeVisible();
  await page.getByRole("link", { name: "Issues", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/issues`);
  await page.reload();
  await expect(launcher).toBeVisible();
  await launcher.click();
  await expect(page.getByText("Chat with your agents", { exact: true })).toBeVisible();

  // The dedicated Chat route must remain usable and own the sole chat surface.
  await page.getByRole("link", { name: "Chat", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/chat`);
  await expect(launcher).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Chat", exact: true })).toBeVisible();
  await expect(page.getByText("Pick a conversation, or start a new one with +", { exact: true })).toBeVisible();
  await expect(page.getByText("Chat with your agents", { exact: true })).toHaveCount(0);
});

test("hidden manual issue fields persist and remain usable through the overflow menu", async ({ page, preferences }) => {
  const { api, slug } = preferences;
  await openPreferences(page, slug);
  const manual = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Create manually", exact: true }),
  });
  const priority = manual.getByRole("switch", { name: "Priority", exact: true });
  const quickPriority = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Create with agent", exact: true }),
  }).getByRole("switch", { name: "Priority", exact: true });
  await expect(priority).toBeChecked();
  await expect(quickPriority).not.toBeChecked();
  await quickPriority.check();
  await priority.uncheck();

  await page.getByRole("link", { name: "Issues", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/issues`);
  await page.reload();
  await openPreferences(page, slug);
  await expect(priority).not.toBeChecked();
  await expect(quickPriority).toBeChecked();
  await page.getByRole("link", { name: "Issues", exact: true }).click();
  await page.getByRole("button", { name: "New Issue", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Quick create issue", exact: true }).getByRole("button", { name: "No priority", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Switch to Manual", exact: true }).click();

  const dialog = page.getByRole("dialog", { name: "New Issue", exact: true });
  await expect(dialog.getByRole("button", { name: "No priority", exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "More options", exact: true }).click();
  await page.getByRole("menuitem", { name: "Set priority...", exact: true }).click();
  await page.getByRole("button", { name: "High", exact: true }).click();
  await expect(dialog.getByRole("button", { name: "High", exact: true })).toBeVisible();
  const title = "Priority selected through the hidden-field menu";
  await dialog.getByRole("textbox", { name: "Issue title", exact: true }).fill(title);
  const created = page.waitForResponse((response) =>
    new URL(response.url()).pathname === "/api/issues" && response.request().method() === "POST",
  );
  await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
  const response = await created;
  expect(response.status()).toBe(201);
  const issue: { id: string; identifier: string; title: string; priority: string } = await response.json();
  expect(issue).toMatchObject({ title, priority: "high" });
  expect(await api.requestJSON(`/api/issues/${issue.id}`)).toMatchObject({ title, priority: "high" });
  await page.getByRole("button", { name: "View issue", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/issues/${issue.identifier}`);
  await expect(page.getByText(title, { exact: true }).first()).toBeVisible();
  await expect(page.getByRole("button", { name: "High", exact: true })).toBeVisible();

  await openPreferences(page, slug);
  await expect(priority).not.toBeChecked();
  await priority.check();
  await page.getByRole("link", { name: "Issues", exact: true }).click();
  await expect(page).toHaveURL(`/${slug}/issues`);
  await page.reload();
  await page.getByRole("button", { name: "New Issue", exact: true }).click();
  await expect(dialog.getByRole("button", { name: "No priority", exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
});
