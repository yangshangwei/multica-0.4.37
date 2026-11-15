// @vitest-environment node
import { expect, it } from "vitest";
import { CollaborationCommentInputSchema, parseCollaborationTriggerPreview } from "./content-schema";
import { parseCollaborationCommentDraft, prepareCollaborationComment } from "./comment";
import { stageIds } from "./stage-test-fixtures";
import { stageTestSubjectFixture as subject } from "./stage-test-report-fixtures";
const input = { execution_view_version: 6, content: `[@Agent](mention://agent/${stageIds.agent}) test this`, parent_id: null, suppressed_target_ids: [],
  expected_lifecycle_epoch: "1", expected_participant_epoch: "1", expected_account_epoch: "1",
  target_versions: [{ target_id: stageIds.agent, expected_target_epoch: "1", expected_config_version: "1" }], test_subject: subject };
it("preserves a strict create-only test subject in the original request", () => {
  expect(CollaborationCommentInputSchema.parse(input).test_subject).toEqual(subject);
});
it.each([null, { ...subject, passed: true }, { ...subject, candidate_version: 1 }])("refuses malformed or fabricated test subjects", (test_subject) => {
  expect(CollaborationCommentInputSchema.safeParse({ ...input, test_subject }).success).toBe(false);
});
it.each([{ execution_view_version: undefined }, { expected_account_epoch: null }, { expected_participant_epoch: null }, { target_versions: [] }])("requires existing v6 target observations for tests", (change) => {
  expect(CollaborationCommentInputSchema.safeParse({ ...input, ...change }).success).toBe(false);
});
it("does not attach a subject to plain prose after a target-free preview", () => {
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, execution_view_version: 6, issue_id: stageIds.issue, lifecycle_epoch: "1",
    write_context: { participant_epoch: "1", account_epoch: "1" }, targets: [], recipients: [] })!;
  const scope = { server: "server", apiScope: "session", userId: stageIds.product };
  expect(() => prepareCollaborationComment(scope, stageIds.issue, stageIds.request, "plain test request", [], preview, undefined, null, subject)).toThrow();
  expect(parseCollaborationCommentDraft(prepareCollaborationComment(scope, stageIds.issue, stageIds.request, "normal comment", [], preview))).not.toHaveProperty("test_subject");
});
