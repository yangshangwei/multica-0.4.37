import { test, expect } from "@playwright/test";
import { loginAsDefault, waitForPageText } from "./helpers";

test.describe("Settings", () => {
  test("updating workspace name reflects in sidebar immediately", async ({
    page,
  }) => {
    const workspaceSlug = await loginAsDefault(page);

    await page.goto(`/${workspaceSlug}/settings?tab=workspace`, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, "General");

    const nameInput = page.getByRole("textbox", { name: "Name", exact: true });
    const originalName = await nameInput.inputValue();

    async function renameWorkspace(name: string) {
      const saved = page.waitForResponse((response) =>
        response.request().method() === "PATCH" &&
        /^\/api\/workspaces\/[^/]+$/.test(new URL(response.url()).pathname) &&
        response.request().postDataJSON()?.name === name,
      );
      await nameInput.fill(name);
      // Text settings auto-save; leaving the field flushes the pending change.
      await nameInput.blur();
      const response = await saved;
      expect(response.ok()).toBe(true);
      expect(await response.json()).toMatchObject({ name });
      await expect(page.getByText("Workspace settings saved", { exact: true })).toBeVisible({ timeout: 5000 });
    }

    try {
      const newName = "Renamed WS " + Date.now();
      await renameWorkspace(newName);

      // Sidebar should reflect the new name WITHOUT page refresh.
      await expect(page.getByRole("button", { name: new RegExp(newName) }).first()).toBeVisible();
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(nameInput).toHaveValue(newName);
    } finally {
      // Restore the shared workspace even if an outcome assertion fails.
      await renameWorkspace(originalName);
      await expect(page.getByRole("button", { name: new RegExp(originalName) }).first()).toBeVisible();
    }
  });

  // Composio connect flow, fully mocked at the network boundary so it runs
  // without a configured COMPOSIO_API_KEY or a live Composio project. The
  // backend redirect is simulated by pointing the init endpoint's redirect_url
  // straight back at the settings page with ?connected=<slug> — exercising the
  // frontend's callback toast + connections refresh (MUL-3718) end to end.
  test("connecting a Composio toolkit shows a toast and refreshes the list", async ({
    page,
  }) => {
    await page.route("**/api/config", async (route) => {
      const response = await route.fetch();
      const config = await response.json();
      await route.fulfill({
        response,
        json: {
          ...config,
          feature_flags: { ...config.feature_flags, composio_mcp_apps: true },
        },
      });
    });

    const workspaceSlug = await loginAsDefault(page);
    const settingsUrl = `/${workspaceSlug}/settings?tab=integrations`;

    // Stateful: connections is empty until the (mocked) connect flow lands.
    let connected = false;
    let connectRequest: { method: string; body: unknown } | undefined;

    await page.route("**/api/integrations/composio/toolkits", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          { slug: "notion", name: "Notion", connectable: true },
        ]),
      }),
    );

    await page.route("**/api/integrations/composio/connections", (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          connected
            ? [
                {
                  id: "conn-notion-1",
                  toolkit_slug: "notion",
                  status: "active",
                  connected_at: new Date().toISOString(),
                  last_used_at: null,
                },
              ]
            : [],
        ),
      });
    });

    await page.route("**/api/integrations/composio/connect/init", (route) => {
      connectRequest = {
        method: route.request().method(),
        body: route.request().postDataJSON(),
      };
      // Composio would 302 through its hosted consent and back to our callback,
      // which emits CallbackRedirect's slug-less shape:
      // `/settings?tab=integrations&connected=<slug>`. The web proxy's
      // legacy-route redirect then prepends the last workspace slug, landing on
      // the real settings route. Mock that exact backend shape (NOT the final
      // slugged URL) so the test exercises the same redirect path real users hit.
      connected = true;
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          redirect_url: `/settings?tab=integrations&connected=notion`,
        }),
      });
    });

    await page.goto(settingsUrl, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("tab", { name: "Integrations", exact: true })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("heading", { name: "Composio", exact: true })).toBeVisible();

    // Notion starts disconnected → click Connect.
    await page.getByRole("button", { name: /^Connect$/ }).first().click();
    await expect.poll(() => connectRequest).toEqual({
      method: "POST",
      body: { toolkit_slug: "notion" },
    });

    // Success toast from the simulated callback redirect.
    await expect(
      page.getByRole("region", { name: /Notifications/ }).getByText("Connected", { exact: true }),
    ).toBeVisible({ timeout: 10000 });

    // List refreshed without a manual reload: the Notion card now offers
    // Disconnect, and the one-shot ?connected param has been stripped.
    await expect(
      page.getByRole("button", { name: /Disconnect/ }).first(),
    ).toBeVisible({ timeout: 10000 });
    await expect(page).not.toHaveURL(/connected=notion/);
  });
});
