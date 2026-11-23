// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

const uploads = [
  { name: "attachment", send: (client: ApiClient) => client.uploadFile(new File(["test"], "test.txt")) },
  { name: "plugin package", send: (client: ApiClient) => client.publishPluginPackage("workspace-a", new File(["test"], "test.zip")) },
];
afterEach(() => vi.unstubAllGlobals());

it.each(uploads)("blocks new $name uploads after endpoint cleanup", async ({ send }) => {
  const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json({}));
  vi.stubGlobal("fetch", request);
  const client = new ApiClient("https://server-a.test");
  client.invalidateSession();
  await expect(send(client)).rejects.toThrow("Server session is switching");
  expect(request).not.toHaveBeenCalled();
});

it.each(uploads)("cancels pending $name uploads and rejects their late responses", async ({ send }) => {
  let finish!: (response: Response) => void;
  const request = vi.fn<typeof fetch>().mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.stubGlobal("fetch", request);
  const client = new ApiClient("https://server-a.test");
  const pending = send(client);
  const signal = request.mock.calls[0]?.[1]?.signal;
  client.invalidateSession();
  finish(Response.json({}));
  await expect(pending).rejects.toThrow("Server session changed");
  expect(signal?.aborted).toBe(true);
});

it.each(uploads)("rejects $name bodies that finish parsing after endpoint cleanup", async ({ send }) => {
  let finish!: (data: unknown) => void;
  const response = Response.json({});
  const parsing = vi.spyOn(response, "json").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(response));
  const client = new ApiClient("https://server-a.test");
  const pending = send(client);
  await vi.waitFor(() => expect(parsing).toHaveBeenCalled());
  client.invalidateSession();
  finish({});
  await expect(pending).rejects.toThrow("Server session changed");
});

it.each(uploads)("discards $name errors parsed after endpoint cleanup", async ({ send }) => {
  let finish!: (data: unknown) => void;
  const response = Response.json({ error: "Old server upload failure" }, { status: 500 });
  const parsing = vi.spyOn(response, "json").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(response));
  const client = new ApiClient("https://server-a.test");
  const pending = send(client);
  await vi.waitFor(() => expect(parsing).toHaveBeenCalled());
  client.invalidateSession();
  finish({ error: "Old server upload failure" });
  await expect(pending).rejects.toThrow("Server session changed");
});
