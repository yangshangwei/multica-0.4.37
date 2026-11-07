// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationKeys, collaborationScope, collaborationIssueAccessDenied } from "./access";
import { collaborationFieldOptions } from "./field-queries";
import { parseCollaborationIssue } from "./schema";
import { collaborationFieldOptionsFixture, collaborationIssueFixture } from "./test-fixtures";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, write_context: { participant_epoch: "1", account_epoch: "1" } })!;
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); vi.unstubAllGlobals(); });
function setup() {
  setApiInstance(new ApiClient("https://example.invalid"));
  const scope = collaborationScope(issue.creator.user_id);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  client.setQueryData(collaborationKeys.issue(scope, issue.id), issue);
  return { client, scope };
}

it("never submits source catalog search for an external editor", async () => {
  const { client, scope } = setup();
  const fetch = vi.fn().mockResolvedValue(response(collaborationFieldOptionsFixture));
  vi.stubGlobal("fetch", fetch);
  await client.fetchQuery(collaborationFieldOptions(client, scope, issue, "private source search"));
  expect(fetch).toHaveBeenCalledOnce();
  expect(fetch.mock.calls[0]?.[0]).not.toContain("parent_search");
});

it.each([false, true])("revalidates read access when field editing is refused (read lost: %s)", async (denied) => {
  const { client, scope } = setup();
  const fetch = vi.fn().mockResolvedValueOnce(response({ code: "forbidden" }, 403))
    .mockResolvedValueOnce(denied ? response({ code: "not_found" }, 404) : response({ ...issue, execution: undefined }));
  vi.stubGlobal("fetch", fetch);
  await expect(client.fetchQuery(collaborationFieldOptions(client, scope, issue))).rejects.toThrow();
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(denied);
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toEqual(denied ? undefined : issue);
});
