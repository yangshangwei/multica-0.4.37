// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { noopLogger } from "../logger";
import type { IssueTableQuerySpec } from "../types";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";

const secret = "PRIVATE_SOURCE_COLLABORATION_SENTINEL";
const query: IssueTableQuerySpec = { scope: { kind: "workspace" }, filters: {}, sort: { field: "title", direction: "asc" } };
const cases: { name: string; read: (client: ApiClient) => Promise<unknown> }[] = [
  { name: "detail", read: (client) => client.getIssue("issue-1") },
  { name: "list", read: (client) => client.listIssues() },
  { name: "query list", read: (client) => client.listIssues({ ids: ["issue-1"] }) },
  { name: "grouped", read: (client) => client.listGroupedIssues({ group_by: "assignee" }) },
  { name: "search", read: (client) => client.searchIssues({ q: secret }) },
  { name: "table groups", read: (client) => client.listIssueTableGroups({ query, group: { kind: "status" } }) },
  { name: "table rows", read: (client) => client.listIssueTableRows({ query, group: { kind: "none" }, group_key: null, hierarchy: { enabled: true }, parent_id: null }) },
  { name: "table facets", read: (client) => client.listIssueTableFacets({ query, facets: [{ kind: "status" }] }) },
  { name: "children", read: (client) => client.listChildIssues("issue-1") },
  { name: "batched children", read: (client) => client.listChildrenByParents(["issue-1"]) },
];

afterEach(() => {
  setSchemaLogger(noopLogger);
  vi.unstubAllGlobals();
});

describe("mixed source issue response privacy", () => {
  it.each(cases)("redacts malformed $name content and account projections", async ({ read }) => {
    const warn = vi.fn();
    const info = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
      description: secret, collaboration: { handler: { email: secret }, has_external_collaborators: "bad" },
      issues: [{ description: secret, collaboration: "bad" }], groups: "bad", total: "bad",
    }), { headers: { "Content-Type": "application/json" } })));
    const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
    await Promise.allSettled([read(client)]);
    expect(warn).toHaveBeenCalled();
    expect(JSON.stringify([...warn.mock.calls, ...info.mock.calls])).not.toContain(secret);
  });

  it("does not log private source filters or server error bodies", async () => {
    const info = vi.fn(), error = vi.fn();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: secret }), {
      status: 400, headers: { "Content-Type": "application/json" },
    })));
    const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info, error } });
    await expect(client.searchIssues({ q: secret })).rejects.toThrow();
    expect(error).toHaveBeenCalled();
    expect(JSON.stringify([...info.mock.calls, ...error.mock.calls])).not.toContain(secret);
  });

  it("does not expose private bytes through JSON syntax errors", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(`invalid ${secret}`)));
    const client = new ApiClient("https://api.example.test");
    const [result] = await Promise.allSettled([client.getIssue("issue-1")]);
    expect(result.status).toBe("rejected");
    if (result.status === "rejected") expect(String(result.reason)).not.toContain(secret);
  });
});
