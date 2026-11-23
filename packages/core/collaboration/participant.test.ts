// @vitest-environment node
import { expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "./access";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { prepareCollaborationParticipantRemoval } from "./participant";

const creatorId = collaborationIssueFixture.creator.user_id;
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
it("requires the current creator capability and a real non-self participant", () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(creatorId);
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, write_context: { participant_epoch: "1", account_epoch: "1" } })!;
  const participant = { ...issue.creator, user_id: "00000000-0000-4000-8000-000000000002", name: "Handler", account_active: false };
  const participants = { protocol_version: 1 as const, issue_id: issue.id, collaboration_revision: issue.collaboration_revision, participants: [{ ...issue.creator, account_active: true }, participant] };
  const prepare = (value = issue, target = participant.user_id) => prepareCollaborationParticipantRemoval(scope, value, "00000000-0000-4000-8000-000000000003", participants, target);
  expect(prepare().payload).toMatchObject({ action: "remove", targetUserId: participant.user_id, expectedCollaborationRevision: "9007199254740993" });
  expect(() => prepare({ ...issue, capabilities: { ...issue.capabilities, remove_participants: false } })).toThrow();
  expect(() => prepare({ ...issue, creator: participant })).toThrow();
  expect(() => prepare(issue, creatorId)).toThrow();
  expect(() => prepare(issue, "00000000-0000-4000-8000-000000000004")).toThrow();
  expect(() => prepare({ ...issue, collaboration_revision: "9007199254740994" })).toThrow();
});
