// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => vi.unstubAllGlobals());

function response(status: number, error: string) {
  return new Response(JSON.stringify({ error }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function setup() {
  const logger = { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() };
  const onUnauthorized = vi.fn();
  const client = new ApiClient("https://api.example.test", { logger, onUnauthorized });
  client.setToken("old-session");
  return { client, logger, onUnauthorized };
}

describe("missing-user session recovery", () => {
  it.each([401, 404])("rejects a missing identity from a %i response without logging an application error", async (status) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(status, "user not found")));
    const { client, logger, onUnauthorized } = setup();
    await expect(client.getMe()).rejects.toMatchObject({ status: 401 });
    expect(onUnauthorized).toHaveBeenCalledOnce();
    expect(logger.warn).toHaveBeenCalledOnce();
    expect(logger.error).not.toHaveBeenCalled();
  });

  it.each(["bearer", "cookie"])("keeps a newer %s login when an old missing-user response arrives", async (mode) => {
    let release!: (value: Response) => void;
    const fetchMock = vi.fn().mockReturnValueOnce(new Promise<Response>((resolve) => { release = resolve; }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ token: "new-session", user: { id: "new-user", email: "new@example.test" } })));
    vi.stubGlobal("fetch", fetchMock);
    const { client, onUnauthorized } = setup();
    const oldRequest = client.getMe();
    const rejected = expect(oldRequest).rejects.toMatchObject({ status: 401 });
    if (mode === "bearer") client.setToken("new-session");
    else await client.verifyCode("new@example.test", "123456");
    release(response(404, "user not found"));
    await rejected;
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it.each([
    [404, "route not found"],
    [500, "user not found"],
    [503, "database unavailable"],
  ])("does not invalidate a session for a %i identity service failure", async (status, message) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(status, message)));
    const { client, onUnauthorized } = setup();
    await expect(client.getMe()).rejects.toMatchObject({ status });
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it("does not reinterpret missing resources or profile-write errors as expired sessions", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(response(404, "user not found"))));
    const { client, onUnauthorized } = setup();
    await expect(client.listProjects()).rejects.toMatchObject({ status: 404 });
    await expect(client.updateMe({ name: "A new name" })).rejects.toMatchObject({ status: 404 });
    expect(onUnauthorized).not.toHaveBeenCalled();
  });
});
