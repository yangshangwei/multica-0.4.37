// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => vi.unstubAllGlobals());

it.each(["desktop", "web"])("rejects redirects for %s API reads and both multipart upload paths", async (platform) => {
  const request = vi.fn<typeof fetch>().mockImplementation(async () => Response.json({}));
  vi.stubGlobal("fetch", request);
  const client = new ApiClient("https://server-a.test", { identity: { platform } });
  client.setToken("server-a-credential");
  await client.listMembers("workspace-a");
  await client.uploadFile(new File(["attachment"], "attachment.txt"));
  await client.publishPluginPackage("workspace-a", new File(["bundle"], "plugin.zip"));
  expect(request).toHaveBeenCalledTimes(3);
  for (const [, options] of request.mock.calls) {
    expect(options).toMatchObject({
      redirect: "error",
      headers: { Authorization: "Bearer server-a-credential" },
    });
  }
});
