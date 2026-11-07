// @vitest-environment node
import { expect, it } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { projectManagementEvent } from "./realtime";
import { clearProtectedProjectContent, markProjectDeleted, protectProjectRequest, useProjectAccessStore } from "./access";
import { projectKeys } from "./queries";
import { projectP1Keys } from "./p1-queries";
it.each(["issue:updated", "issue:deleted", "triage:updated", "issue_status:updated", "member:removed", "agent:updated", "squad:deleted", "daemon:updated", "workspace:planning_timezone_updated", "project:update_corrected"])("refreshes project facts for %s", (event) => expect(projectManagementEvent(event)).toBe(true));
it("does not refresh aggregates per streamed message or heartbeat", () => { expect(projectManagementEvent("task:message")).toBe(false); expect(projectManagementEvent("daemon:heartbeat")).toBe(false); });
it("clears project description, overview and history before navigation", () => {
 const qc = new QueryClient(); qc.setQueryData(projectKeys.detail("w", "p"), { description: "secret" });
 qc.setQueryData(projectP1Keys.overview("w", "p"), { evidence: "secret" }); qc.setQueryData(projectKeys.detail("other", "p"), { title: "retained" });
 clearProtectedProjectContent(qc, "w"); expect(qc.getQueryData(projectKeys.detail("w", "p"))).toBeUndefined(); expect(qc.getQueryData(projectP1Keys.overview("w", "p"))).toBeUndefined(); expect(qc.getQueryData(projectKeys.detail("other", "p"))).toEqual({ title: "retained" }); qc.clear();
});

it("a removed project stays copy-only when a late read retries", async () => {
  const qc = new QueryClient(); useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
  markProjectDeleted(qc, "copy-w", "copy-p");
  await expect(protectProjectRequest(qc, "copy-w", "copy-p", async () => ({ description: "server" }))).rejects.toMatchObject({ status: 404 });
  expect(useProjectAccessStore.getState().deleted[JSON.stringify(["copy-w", "copy-p"])]).toEqual([]);
  expect(useProjectAccessStore.getState().denied).toEqual({}); qc.clear();
});
