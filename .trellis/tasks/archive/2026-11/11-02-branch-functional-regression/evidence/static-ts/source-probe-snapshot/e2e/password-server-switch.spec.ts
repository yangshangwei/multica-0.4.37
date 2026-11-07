import { test, expect, _electron as electron } from "@playwright/test";
import { createServer, type IncomingHttpHeaders } from "node:http";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import ts from "typescript";

interface RequestRecord { path: string; headers: IncomingHttpHeaders }
interface CookieSnapshot { name: string; domain: string; path: string; httpOnly: boolean; value: string }
interface SwitchFixture {
  seed(): Promise<{ cookies: CookieSnapshot[]; windows: number }>;
  request(origin: string, suffix: string): Promise<number>;
  switchServer(): Promise<{ cookies: CookieSnapshot[]; windows: number; issueDestroyed: boolean; daemonClears: number; daemonStops: number; storedToken: string | null; saved: { apiUrl: string } }>;
}

test("Electron server switch clears same-host credentials and closes issue windows", async ({}, info) => {
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Opt-in isolated password acceptance");
  const root = resolve(import.meta.dirname, "..");
  const profile = mkdtempSync(join(tmpdir(), "multica-password-switch-"));
  const received: RequestRecord[][] = [[], []];
  const servers = received.map(records => createServer((request, response) => {
    records.push({ path: request.url ?? "", headers: request.headers });
    if (request.headers.origin) {
      response.setHeader("Access-Control-Allow-Origin", request.headers.origin);
      response.setHeader("Access-Control-Allow-Credentials", "true");
      response.setHeader("Access-Control-Allow-Headers", "Authorization, X-Workspace-ID, X-Workspace-Slug, X-CSRF-Token");
    }
    if (request.method === "OPTIONS") { response.writeHead(204).end(); return; }
    if (request.url === "/api/config") { response.setHeader("Content-Type", "application/json"); response.end(JSON.stringify({ auth_mode: "password", device_auth_available: false })); return; }
    if (request.url === "/renderer" || request.url === "/issue") { response.setHeader("Content-Type", "text/html"); response.end("<!doctype html><title>Password switch fixture</title><main>Isolated server switch acceptance</main>"); return; }
    response.end("ok");
  }));
  let desktop: Awaited<ReturnType<typeof electron.launch>> | undefined;
  try {
    const origins = await Promise.all(servers.map(server => new Promise<string>((resolveOrigin, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", () => {
        const address = server.address();
        if (!address || typeof address === "string") { reject(new Error("Missing local port")); return; }
        resolveOrigin(`http://127.0.0.1:${address.port}`);
      });
    })));
    expect(new URL(origins[0]!).hostname).toBe(new URL(origins[1]!).hostname);
    expect(new URL(origins[0]!).port).not.toBe(new URL(origins[1]!).port);
    const helpers = join(profile, "server-switch.cjs");
    const source = readFileSync(join(root, "apps/desktop/src/main/server-switch.ts"), "utf8");
    writeFileSync(helpers, ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText);
    const executablePath = process.platform === "darwin"
      ? join(root, "apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron")
      : join(root, "apps/desktop/node_modules/electron/dist", process.platform === "win32" ? "electron.exe" : "electron");
    desktop = await electron.launch({ executablePath, args: [join(root, "e2e/fixtures/password-switch-electron.cjs")], env: { ...process.env, PASSWORD_SWITCH_PROFILE: profile, PASSWORD_SWITCH_HELPERS: helpers, PASSWORD_SWITCH_ORIGIN_A: origins[0]!, PASSWORD_SWITCH_ORIGIN_B: origins[1]! } });
    await desktop.firstWindow();
    await expect.poll(() => desktop!.evaluate(() => Boolean((globalThis as unknown as { passwordSwitchAcceptance?: SwitchFixture }).passwordSwitchAcceptance))).toBe(true);
    const before = await desktop.evaluate(() => (globalThis as unknown as { passwordSwitchAcceptance: SwitchFixture }).passwordSwitchAcceptance.seed());
    expect(before.windows).toBe(2);
    expect(before.cookies.filter(cookie => cookie.domain === "127.0.0.1")).toHaveLength(4);
    expect(before.cookies.some(cookie => cookie.name === "multica_auth" && cookie.path === "/api/private" && cookie.httpOnly)).toBe(true);
    await desktop.evaluate((_, origin) => (globalThis as unknown as { passwordSwitchAcceptance: SwitchFixture }).passwordSwitchAcceptance.request(origin, "before"), origins[0]!);
    const aRequest = received[0]!.find(request => request.path === "/api/private/before")!;
    expect(aRequest.headers.authorization).toBe("Bearer A-bearer-secret");
    expect(aRequest.headers.cookie).toContain("multica_auth=A-cookie-secret");
    expect(aRequest.headers["x-workspace-id"]).toBe("A-workspace-id");

    const after = await desktop.evaluate(() => (globalThis as unknown as { passwordSwitchAcceptance: SwitchFixture }).passwordSwitchAcceptance.switchServer());
    expect(after).toMatchObject({ windows: 1, issueDestroyed: true, daemonClears: 1, daemonStops: 1, storedToken: null, saved: { apiUrl: origins[1] } });
    expect(after.cookies.filter(cookie => cookie.domain === "127.0.0.1")).toEqual([]);
    expect(after.cookies).toEqual([expect.objectContaining({ domain: "unrelated.invalid", name: "multica_auth", value: "unrelated-cookie" })]);
    expect(await desktop.evaluate((_, origin) => (globalThis as unknown as { passwordSwitchAcceptance: SwitchFixture }).passwordSwitchAcceptance.request(origin, "after"), origins[1]!)).toBe(200);
    expect(received[1]!.map(request => request.path)).toEqual(expect.arrayContaining(["/health", "/api/config", "/api/private/after"]));
    for (const request of received[1]!) {
      for (const header of ["authorization", "cookie", "x-workspace-id", "x-workspace-slug", "x-csrf-token"]) expect(request.headers[header], `${request.path}: ${header}`).toBeUndefined();
    }
    await info.attach("server-switch-evidence", { body: JSON.stringify({ origins, beforeCookieCount: before.cookies.length, after, serverBRequests: received[1] }, null, 2), contentType: "application/json" });
  } finally {
    if (desktop) await desktop.close();
    await Promise.all(servers.map(server => new Promise<void>(done => { server.closeAllConnections(); server.close(() => done()); })));
    rmSync(profile, { recursive: true, force: true });
  }
});
