// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { resourceIssue, resourcesFixture, resourceOptionsFixture, resourceIdentityFixture } from "./resource-test-fixtures";

afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
it("reads management scopes without ambient workspace headers and rejects crossed response identities", async () => {
  const api = new ApiClient("https://server.example");
  setCurrentWorkspace("unrelated", "00000000-0000-4000-8000-000000000077");
  const participants = { protocol_version: 1, issue_id: resourceIssue.id, collaboration_revision: resourceIssue.collaboration_revision,
    participants: [{ ...resourceIssue.creator, account_active: false }] };
  const calls = [
    { run: () => api.getCollaborationResources(resourceIssue.id), response: resourcesFixture, suffix: "/resources" },
    { run: () => api.getCollaborationResourceOptions(resourceIssue.id, resourceIssue.source_workspace.id), response: resourceOptionsFixture, suffix: "/resources/options?execution_workspace_id=" },
    { run: () => api.getCollaborationResourceIdentity(resourceIssue.id, resourceIdentityFixture.target_id, resourceIdentityFixture.agent_id, null), response: resourceIdentityFixture, suffix: "/resources/identity?target_id=" },
    { run: () => api.getCollaborationParticipants(resourceIssue.id), response: participants, suffix: "/participants" },
  ];
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  for (const item of calls) {
    fetch.mockResolvedValueOnce(json(item.response));
    await expect(item.run()).resolves.toEqual(item.response);
    const request = fetch.mock.calls.at(-1)!;
    expect(request[0]).toContain(item.suffix);
    const headers = new Headers(request[1].headers);
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
    fetch.mockResolvedValueOnce(json({ ...item.response, issue_id: "00000000-0000-4000-8000-000000000077" }));
    await expect(item.run()).resolves.toBeNull();
  }
});

it("isolates missing workspace or runtime scope without masking a real issue denial", async () => {
  const api = new ApiClient("https://server.example");
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "resource_workspace_unavailable" }, 404))
    .mockResolvedValueOnce(json({ code: "resource_not_found" }, 404)).mockResolvedValueOnce(json({ code: "not_found" }, 404));
  vi.stubGlobal("fetch", fetch);
  await expect(api.getCollaborationResourceOptions(resourceIssue.id, resourceIssue.source_workspace.id)).resolves.toBeNull();
  await expect(api.getCollaborationResourceIdentity(resourceIssue.id, resourceIdentityFixture.target_id, resourceIdentityFixture.agent_id, null)).resolves.toBeNull();
  await expect(api.getCollaborationResources(resourceIssue.id)).rejects.toMatchObject({ status: 404 });
});
