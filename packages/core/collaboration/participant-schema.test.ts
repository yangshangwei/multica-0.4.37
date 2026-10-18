// @vitest-environment node
import { expect, it } from "vitest";
import { parseCollaborationParticipants } from "./participant-schema";
import { collaborationIssueFixture } from "./test-fixtures";

const value = { protocol_version: 1, issue_id: collaborationIssueFixture.id, collaboration_revision: "9007199254740993",
  participants: [{ ...collaborationIssueFixture.creator, account_active: false }] };
it("retains disabled participants and exact management revisions", () => {
  expect(parseCollaborationParticipants(value)).toEqual(value);
});
it("does not infer availability from malformed or incomplete participant data", () => {
  for (const accountActive of [undefined, null, 1, "true"]) {
    expect(parseCollaborationParticipants({ ...value, participants: [{ ...value.participants[0], account_active: accountActive }] })).toBeNull();
  }
  expect(parseCollaborationParticipants({ ...value, participants: [value.participants[0], value.participants[0]] })).toBeNull();
  expect(parseCollaborationParticipants({ ...value, collaboration_revision: 1 })).toBeNull();
});
