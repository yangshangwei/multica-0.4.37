import { expect, test } from "@playwright/test";
import { buildSurfaceFrameDocument } from "../packages/views/plugins/surface-document";

/**
 * Real Chromium coverage for behavior jsdom does not implement: executing a
 * hosted sandboxed document, reporting errors from a dynamically inserted
 * script, and firing page lifecycle events when the host replaces its wrapper.
 */

test.describe("plugin surface document (real Chromium, hosted sandbox)", () => {
  test("a host-authored launch replacement is not reported as hostile navigation", async ({ page }) => {
    const first = {
      url: "https://plugin-content.example.test/plugin-surfaces/first",
      bridgeToken: "first-proof",
    };
    const replacement = {
      url: "https://plugin-content.example.test/plugin-surfaces/replacement",
      bridgeToken: "replacement-proof",
    };
    for (const launch of [first, replacement]) {
      await page.route(launch.url, async (route) => {
        await route.fulfill({
          contentType: "text/html",
          headers: { "Content-Security-Policy": "default-src 'none'; script-src 'unsafe-inline'" },
          body: `<!doctype html><script>
            window.addEventListener("pagehide", () => {
              parent.postMessage({ type: "multica:plugin-surface-navigated" }, "*");
            });
            const channel = new MessageChannel();
            parent.postMessage({
              type: "multica:plugin-bridge-connect",
              version: 2,
              challenge: ${JSON.stringify(launch.bridgeToken)}
            }, "*", [channel.port1]);
          </script>`,
        });
      });
    }

    await page.setContent("<!doctype html><body></body>");
    await page.evaluate((srcdoc) => {
      const state = { connections: [] as string[], navigated: 0 };
      (window as unknown as { __surfaceState: typeof state }).__surfaceState = state;
      const frame = document.createElement("iframe");
      frame.id = "surface";
      frame.sandbox.add("allow-scripts", "allow-same-origin");
      window.addEventListener("message", (event) => {
        if (event.source !== frame.contentWindow) return;
        const data = event.data as { type?: string; challenge?: string } | null;
        if (data?.type === "multica:plugin-bridge-connect" && data.challenge && event.ports[0]) {
          state.connections.push(data.challenge);
          event.ports[0].close();
        }
        if (data?.type === "multica:plugin-surface-navigated" ||
            data?.type === "multica:plugin-surface-navigation-blocked") state.navigated++;
      });
      frame.srcdoc = srcdoc;
      document.body.appendChild(frame);
    }, buildSurfaceFrameDocument(first));

    await expect.poll(() => page.evaluate(() =>
      (window as unknown as { __surfaceState: { connections: string[] } }).__surfaceState.connections,
    )).toEqual([first.bridgeToken]);

    await page.evaluate((srcdoc) => {
      document.querySelector<HTMLIFrameElement>("#surface")!.srcdoc = srcdoc;
    }, buildSurfaceFrameDocument(replacement));
    await expect.poll(() => page.evaluate(() =>
      (window as unknown as { __surfaceState: { connections: string[] } }).__surfaceState.connections,
    )).toEqual([first.bridgeToken, replacement.bridgeToken]);

    expect(await page.evaluate(() =>
      (window as unknown as { __surfaceState: { navigated: number } }).__surfaceState.navigated,
    )).toBe(0);
  });

  test("reports a synchronous hosted plugin error to the pre-armed host listener", async ({ page }) => {
    const url = "https://plugin-content.example.test/plugin-surfaces/failing";
    await page.route(url, async (route) => {
      await route.fulfill({
        contentType: "text/html",
        headers: { "Content-Security-Policy": "default-src 'none'; script-src 'unsafe-inline'" },
        body: `<!doctype html><body><script>
          window.addEventListener("error", () => {
            parent.postMessage({ type: "multica:plugin-surface-error" }, "*");
          });
          const plugin = document.createElement("script");
          plugin.textContent = "throw new Error('plugin failed during bootstrap');";
          document.body.appendChild(plugin);
        </script>`,
      });
    });
    const pluginErrors: string[] = [];
    page.on("pageerror", (error) => pluginErrors.push(error.message));

    await page.setContent("<!doctype html><body></body>");
    // Hosted guests report errors once. Like PluginSurfaceFrame, arm the host
    // listener before assigning srcdoc so bootstrap cannot outrun the listener.
    await page.evaluate((srcdoc) => {
      const frame = document.createElement("iframe");
      frame.id = "surface";
      frame.sandbox.add("allow-scripts", "allow-same-origin");
      (window as unknown as { __surfaceErrors: number }).__surfaceErrors = 0;
      window.addEventListener("message", (event) => {
        if ((event.data as { type?: string } | null)?.type !== "multica:plugin-surface-error") return;
        if (event.source !== frame.contentWindow) return;
        (window as unknown as { __surfaceErrors: number }).__surfaceErrors++;
      });
      frame.srcdoc = srcdoc;
      document.body.appendChild(frame);
    }, buildSurfaceFrameDocument({ url, bridgeToken: "error-proof" }));

    await expect.poll(() => page.evaluate(() =>
      (window as unknown as { __surfaceErrors: number }).__surfaceErrors,
    )).toBe(1);
    expect(pluginErrors).toEqual(["plugin failed during bootstrap"]);
  });
});
