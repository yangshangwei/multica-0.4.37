// @vitest-environment node
import { expect, it } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { collaborationRunRecordsOptions } from "/Volumes/artisan/code/2026/mc-runtime-integration-20261031/packages/core/collaboration/run-records-queries";
import { parseCollaborationRunRecords } from "/Volumes/artisan/code/2026/mc-runtime-integration-20261031/packages/core/collaboration/run-records-schema";
import { collaborationRunRecordsFixture as fixture } from "/Volumes/artisan/code/2026/mc-runtime-integration-20261031/packages/core/collaboration/run-records-test-fixtures";
it("rejects changing recording stream and coverage within one immutable page chain", () => {
  const streamA = "00000000-0000-4000-8000-0000000000aa";
  const streamB = "00000000-0000-4000-8000-0000000000bb";
  const page = (stream: string, next: string | null) => parseCollaborationRunRecords({ ...fixture,
    recording: { status: "partial", inputs_recorded: true, retention: { version: 1, stream_id: stream, retained_through: 1, observed_through: null, sealed: false, reason: "incomplete" } },
    items: [{ id: `record:${stream}:0`, kind: "message", role: "assistant", body: "Retained message" }], next_cursor: next });
  const first = page(streamA, "page-two"); const second = page(streamB, null);
  expect(first).not.toBeNull(); expect(second).not.toBeNull();
  const client = new QueryClient();
  const options = collaborationRunRecordsOptions(client, {server: "https://example.test", apiScope: "scope", userId: fixture.issue_id}, fixture.issue_id, fixture.run.id);
  expect(() => (options.structuralSharing as Function)(undefined, { pages: [first, second], pageParams: [undefined, "page-two"] })).toThrow("Collaboration response is unavailable");
  client.clear();
});
