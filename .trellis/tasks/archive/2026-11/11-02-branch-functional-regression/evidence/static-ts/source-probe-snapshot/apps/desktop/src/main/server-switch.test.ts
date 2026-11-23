// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { BrowserLoginAttempt, ServerSwitchCoordinator, clearServerCookies, probeServer } from "./server-switch";

describe("server switch", () => {
  it("validates before cleanup and persists only after all cleanup", async () => {
    const order: string[] = [];
    const coordinator = new ServerSwitchCoordinator();
    await coordinator.run({ probe: async () => { order.push("probe"); }, freeze: () => { order.push("freeze"); }, cleanup: async () => { order.push("cleanup"); }, save: async () => { order.push("save"); }, reload: () => { order.push("reload"); } });
    expect(order).toEqual(["probe", "freeze", "cleanup", "save", "reload"]);
    expect(coordinator.blocked).toBe(true);
  });
  it("retains A on failed probe and rejects overlapping switches", async () => {
    const c = new ServerSwitchCoordinator();
    let reject!: (e: Error) => void;
    const actions = { probe: () => new Promise<void>((_, r) => { reject = r; }), freeze: vi.fn(), cleanup: vi.fn(), save: vi.fn(), reload: vi.fn() };
    const pending = c.run(actions);
    await expect(c.run(actions)).rejects.toThrow("progress");
    reject(new Error("unreachable"));
    await expect(pending).rejects.toThrow("unreachable");
    expect(c.blocked).toBe(false);
    expect(actions.freeze).not.toHaveBeenCalled();
  });
  it("fails closed when local cleanup fails", async () => {
    const c = new ServerSwitchCoordinator();
    const save = vi.fn();
    await expect(c.run({ probe: async () => {}, freeze: () => {}, cleanup: async () => { throw new Error("disk"); }, save, reload: () => {} })).rejects.toThrow("disk");
    expect(save).not.toHaveBeenCalled();
    expect(c.blocked).toBe(true);
  });
  it.each(["save", "reload"] as const)("keeps the transition blocked after %s fails", async (stage) => {
    const c = new ServerSwitchCoordinator();
    const reload = vi.fn(() => { if (stage === "reload") throw new Error("reload"); });
    await expect(c.run({ probe: async () => {}, freeze: () => {}, cleanup: async () => {}, save: async () => { if (stage === "save") throw new Error("save"); }, reload })).rejects.toThrow(stage);
    expect(c.blocked).toBe(true);
    if (stage === "save") expect(reload).not.toHaveBeenCalled();
  });
  it("accepts declared password and historical legacy capabilities", async () => {
    for (const config of [{ auth_mode: "password", device_auth_available: false }, { device_auth_available: true }]) {
      const request = vi.fn().mockResolvedValueOnce(new Response("ok")).mockResolvedValueOnce(Response.json(config));
      await expect(probeServer("https://b.test", request)).resolves.toBeUndefined();
    }
  });
  it.each([null, [], {auth_mode: "password", device_auth_available: true}])("rejects malformed or contradictory capabilities %j", async (config) => {
    const request = vi.fn().mockResolvedValueOnce(new Response("ok")).mockResolvedValueOnce(Response.json(config));
    await expect(probeServer("https://b.test", request)).rejects.toThrow();
  });
  it("propagates cookie deletion failures so persistence cannot proceed", async () => {
    const cookies = { get: vi.fn().mockResolvedValue([{name: "multica_auth", domain: "localhost", path: "/"}]), remove: vi.fn().mockRejectedValue(new Error("cookie store unavailable")) };
    await expect(clearServerCookies(cookies, ["http://localhost:8080", "http://localhost:9080"])).rejects.toThrow("cookie store unavailable");
  });
  it("clears authentication cookies at every path including parent-domain cookies shared across ports", async () => {
    const cookies = { get: vi.fn().mockResolvedValue([{name:"multica_auth",domain:".example.test",path:"/api",secure:true},{name:"multica_csrf",domain:"api.example.test",path:"/",secure:false},{name:"theme",domain:"api.example.test",path:"/",secure:false},{name:"multica_auth",domain:"other.test",path:"/",secure:false}]), remove: vi.fn().mockResolvedValue(undefined) };
    await clearServerCookies(cookies, ["https://api.example.test:8443"]);
    expect(cookies.remove.mock.calls).toEqual([["https://example.test/api", "multica_auth"], ["http://api.example.test/", "multica_csrf"]]);
  });
  it("probes health and capabilities without credentials and rejects unknown auth modes", async () => {
    const request = vi.fn().mockResolvedValueOnce(new Response("ok")).mockResolvedValueOnce(Response.json({auth_mode:"future"}));
    await expect(probeServer("https://b.test", request)).rejects.toThrow("authentication");
    expect(request.mock.calls.map(call => call[0])).toEqual(["https://b.test/health", "https://b.test/api/config"]);
    for (const [, init] of request.mock.calls) expect(init).toMatchObject({credentials:"omit",redirect:"error"});
  });
});

describe("browser login callbacks", () => {
  it("rejects replay, missing state and callbacks from the previous server", () => {
    const login = new BrowserLoginAttempt();
    const old = login.begin();
    login.clear();
    const current = login.begin();
    expect(login.consume(old)).toBe(false);
    expect(login.consume(null)).toBe(false);
    expect(login.consume(current)).toBe(true);
    expect(login.consume(current)).toBe(false);
  });
});
