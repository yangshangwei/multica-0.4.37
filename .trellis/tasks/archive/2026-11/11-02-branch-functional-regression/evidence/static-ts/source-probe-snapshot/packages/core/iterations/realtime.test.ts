// @vitest-environment node
import { expect, it } from "vitest";
import { iterationManagementEvent } from "./realtime";
it.each([
  "issue:updated",
  "issue:created",
  "issue:deleted",
  "issue:labels-changed",
  "label:updated",
  "label:deleted",
  "task:running",
  "task:completed",
  "task:cancelled",
  "project:updated",
  "project:deleted",
  "member:removed",
  "agent:archived",
  "squad:updated",
  "issue_status:changed",
  "workspace:updated",
  "triage:updated",
])("refreshes iteration facts for %s", (type) => {
  expect(iterationManagementEvent(type)).toBe(true);
});
it.each([
  "task:message",
  "task:progress",
  "chat:message",
  "comment:created",
  "reaction:added",
  "daemon:heartbeat",
  "iteration:updated",
])("does not redundantly refresh iteration statistics for %s", (type) => {
  expect(iterationManagementEvent(type)).toBe(false);
});
