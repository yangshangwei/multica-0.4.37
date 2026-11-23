// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationKeys, collaborationScope, collaborationIssueAccessDenied } from "./access";
import { collaborationRecipientOptions, collaborationWorkbenchOptions } from "./queries";
import { collaborationIssueFixture } from "./test-fixtures";

const id = "00000000-0000-4000-8000-000000000001";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); vi.unstubAllGlobals(); });

it("keeps issue access when an exact recipient lookup has no match", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  client.setQueryData(collaborationKeys.issue(scope, id), { title: "Authorized issue" });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: "recipient_not_found" }), { status: 404 })));
  await expect(client.fetchQuery(collaborationRecipientOptions(client, scope, id, "missing-account"))).rejects.toThrow();
  expect(collaborationIssueAccessDenied(client, scope, id)).toBe(false);
  expect(client.getQueryData(collaborationKeys.issue(scope, id))).toEqual({ title: "Authorized issue" });
});

it("discards a denied account list rather than showing its warm private titles", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const options = collaborationWorkbenchOptions(client, scope, "current");
  client.setQueryData(options.queryKey, { pages: [{ items: [{ ...collaborationIssueFixture, title: "Old private title", updated_at: "2026-10-18T00:00:00Z" }], total: 1, next_cursor: null }], pageParams: [undefined] });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: "forbidden" }), { status: 403 })));
  await expect(client.fetchInfiniteQuery(options)).rejects.toThrow();
  expect(client.getQueryData(options.queryKey)).toBeUndefined();
});
