// @vitest-environment jsdom
import type { ReactNode } from "react";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { clearCollaborationAccess, collaborationIssueAccessDenied, collaborationKeys, collaborationScope } from "./access";
import { useCollaborationStatusOptions } from "./queries";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";

const actorId = collaborationIssueFixture.creator.user_id;
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
const clients: QueryClient[] = [];
const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const options = { action: "complete", status_options_version: "a".repeat(64), default_status: "done", options: [{ key: "done", name: "Done", category: "done" }] };

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
  vi.unstubAllGlobals();
});

function setup() {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const scope = collaborationScope(actorId);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  client.setQueryData(collaborationKeys.issue(scope, issue.id), issue);
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return { api, client, scope, wrapper };
}

it("keeps the completed issue readable when scope invalidation refetches the still-mounted completion options", async () => {
  let completed = false;
  const done = { ...issue, status: "done", status_category: "done", capabilities: { ...issue.capabilities, complete: false, reopen: true } };
  const fetch = vi.fn(async (url: string) => new URL(url).pathname.endsWith("status-options")
    ? response(completed ? { code: "forbidden" } : options, completed ? 403 : 200)
    : response({ ...done, execution: undefined }));
  vi.stubGlobal("fetch", fetch);
  const { client, scope, wrapper } = setup();
  const { result } = renderHook(() => useCollaborationStatusOptions(scope, issue.id, "complete", true), { wrapper });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));

  completed = true;
  client.setQueryData(collaborationKeys.issue(scope, issue.id), done);
  await act(async () => { await client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) }); });

  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(false);
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toEqual(done);
  await waitFor(() => expect(result.current.isError).toBe(true));
  expect(result.current.error).toBeInstanceOf(ApiError);
  expect((result.current.error as ApiError).status).toBe(403);
  expect(fetch.mock.calls.filter(([url]) => new URL(url).pathname.endsWith("status-options"))).toHaveLength(2);
  expect(fetch.mock.calls.filter(([url]) => new URL(url).pathname === `/api/collaboration/issues/${issue.id}`)).toHaveLength(1);
});

it.each([403, 404])("clears private issue data only when the authoritative re-read also denies access (%s)", async (status) => {
  const fetch = vi.fn(async (url: string) => response({ code: "forbidden" }, new URL(url).pathname.endsWith("status-options") ? 403 : status));
  vi.stubGlobal("fetch", fetch);
  const { client, scope, wrapper } = setup();
  renderHook(() => useCollaborationStatusOptions(scope, issue.id, "complete", true), { wrapper });
  await waitFor(() => expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(true));
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toBeUndefined();
  expect(fetch).toHaveBeenCalledTimes(2);
});

it("keeps private issue data and the action disabled when revalidating read access is temporarily unavailable", async () => {
  vi.stubGlobal("fetch", vi.fn(async (url: string) => response({ code: "unavailable" }, new URL(url).pathname.endsWith("status-options") ? 403 : 503)));
  const { client, scope, wrapper } = setup();
  const { result } = renderHook(() => useCollaborationStatusOptions(scope, issue.id, "complete", true), { wrapper });
  await waitFor(() => expect(result.current.isError).toBe(true));
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(false);
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toEqual(issue);
  expect(result.current.isSuccess).toBe(false);
});

it("does not restore access when a newer denial arrives while status-option revalidation is pending", async () => {
  let release: (value: Response) => void = () => undefined;
  let revalidating = false;
  vi.stubGlobal("fetch", vi.fn(async (url: string) => {
    if (new URL(url).pathname.endsWith("status-options")) return response({ code: "forbidden" }, 403);
    revalidating = true;
    return new Promise<Response>((resolve) => { release = resolve; });
  }));
  const { client, scope, wrapper } = setup();
  renderHook(() => useCollaborationStatusOptions(scope, issue.id, "complete", true), { wrapper });
  await waitFor(() => expect(revalidating).toBe(true));
  await act(async () => {
    clearCollaborationAccess(client, scope, issue.id);
    release(response({ ...issue, execution: undefined }));
  });
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(true);
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toBeUndefined();
});
