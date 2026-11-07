const { app, BrowserWindow, session } = require("electron");
const fs = require("node:fs/promises");
const path = require("node:path");

// Native acceptance boundary: real Chromium windows/storage/cookies and the
// production coordinator/probe/cookie helpers. Renderer cleanup and daemon
// teardown are injected callbacks; this does not boot the product or a daemon.
const originA = process.env.PASSWORD_SWITCH_ORIGIN_A;
const originB = process.env.PASSWORD_SWITCH_ORIGIN_B;
const profile = process.env.PASSWORD_SWITCH_PROFILE;
const helperPath = process.env.PASSWORD_SWITCH_HELPERS;
if (!profile || !helperPath) throw new Error("An isolated profile and production helpers are required");
for (const origin of [originA, originB]) {
  if (!origin || new URL(origin).hostname !== "127.0.0.1") throw new Error("Only local fixture origins are allowed");
}
const { ServerSwitchCoordinator, clearServerCookies, probeServer } = require(helperPath);
app.setName("Multica Password Switch Acceptance");
app.setPath("userData", profile);

app.whenReady().then(async () => {
  const nativeSession = session.defaultSession;
  nativeSession.webRequest.onBeforeRequest((details, done) => {
    const url = new URL(details.url);
    done({ cancel: ["http:", "https:"].includes(url.protocol) && url.hostname !== "127.0.0.1" });
  });
  const main = new BrowserWindow({ show: true, width: 800, height: 600, webPreferences: { contextIsolation: true, sandbox: true } });
  await main.loadURL(`${originA}/renderer`);
  const coordinator = new ServerSwitchCoordinator();
  let issue;
  let daemonClears = 0;
  let daemonStops = 0;
  let reload;
  const storageKeys = ["multica_token", "workspace_id", "workspace_slug", "csrf_token"];
  globalThis.passwordSwitchAcceptance = {
    async seed() {
      await main.webContents.executeJavaScript(`localStorage.setItem("multica_token", "A-bearer-secret"); localStorage.setItem("workspace_id", "A-workspace-id"); localStorage.setItem("workspace_slug", "A-workspace-slug"); localStorage.setItem("csrf_token", "A-csrf-secret");`);
      for (const cookiePath of ["/", "/api/private"]) {
        for (const name of ["multica_auth", "multica_csrf"]) {
          await nativeSession.cookies.set({ url: originA + cookiePath, name, value: "A-cookie-secret", path: cookiePath, httpOnly: name === "multica_auth" });
        }
      }
      await nativeSession.cookies.set({ url: "http://unrelated.invalid", name: "multica_auth", value: "unrelated-cookie", path: "/", httpOnly: true });
      issue = new BrowserWindow({ show: false, webPreferences: { contextIsolation: true, sandbox: true } });
      await issue.loadURL(`${originA}/issue`);
      return { cookies: await nativeSession.cookies.get({}), windows: BrowserWindow.getAllWindows().length };
    },
    async request(origin, suffix) {
      if (![originA, originB].includes(origin)) throw new Error("Unknown fixture origin");
      // Deliberately include browser cookies: this canary would expose a stale
      // same-host cookie even when ordinary Desktop bearer fetches omit them.
      return main.webContents.executeJavaScript(`(async () => {
        const headers = {};
        for (const [key, header] of [["multica_token", "Authorization"], ["workspace_id", "X-Workspace-ID"], ["workspace_slug", "X-Workspace-Slug"], ["csrf_token", "X-CSRF-Token"]]) {
          const value = localStorage.getItem(key);
          if (value) headers[header] = key === "multica_token" ? "Bearer " + value : value;
        }
        return (await fetch(${JSON.stringify(origin + "/api/private/" + suffix)}, { headers, credentials: "include" })).status;
      })()`);
    },
    async switchServer() {
      await coordinator.run({
        probe: () => probeServer(originB),
        freeze: () => { if (issue && !issue.isDestroyed()) issue.destroy(); },
        cleanup: async () => {
          await main.webContents.executeJavaScript(`${JSON.stringify(storageKeys)}.forEach(key => localStorage.removeItem(key))`);
          daemonClears += 1;
          daemonStops += 1;
          await clearServerCookies(nativeSession.cookies, [originA, originB]);
        },
        save: async () => {
          const configPath = path.join(profile, "fixture-server.json");
          await fs.writeFile(configPath + ".tmp", JSON.stringify({ apiUrl: originB }));
          await fs.rename(configPath + ".tmp", configPath);
        },
        reload: () => {
          reload = new Promise(resolve => main.webContents.once("did-finish-load", resolve));
          main.webContents.reload();
        },
      });
      await reload;
      coordinator.rendererReady();
      return {
        cookies: await nativeSession.cookies.get({}),
        windows: BrowserWindow.getAllWindows().length,
        issueDestroyed: issue.isDestroyed(),
        daemonClears, daemonStops,
        storedToken: await main.webContents.executeJavaScript('localStorage.getItem("multica_token")'),
        saved: JSON.parse(await fs.readFile(path.join(profile, "fixture-server.json"), "utf8")),
      };
    },
  };
});
app.on("window-all-closed", () => app.quit());
