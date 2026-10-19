// @vitest-environment node
import { expect, it } from "vitest";
import { CollaborationActionSchema } from "./action";
import { CollaborationCreationInputSchema } from "./creation-schema";
import { prepareCollaborationCreation } from "./creation";
import { newCollaborationDeliveryForm } from "./delivery-draft";

const id = "00000000-0000-4000-8000-000000000001";
const other = "00000000-0000-4000-8000-000000000002";
const context = { requestId: id, expectedCollaborationRevision: "9007199254740993", expectedLifecycleEpoch: "1", expectedParticipantEpoch: "2", expectedAccountEpoch: "3" };

it("retains label replacement and partial property clears in the original edit intent", () => {
  const input = { ...context, action: "edit", title: "Preserved", description: "Body", startDate: null, dueDate: null, priority: "high",
    labelIds: [], properties: { [id]: null, [other]: false } };
  expect(CollaborationActionSchema.parse(input)).toEqual(input);
});

it("keeps source structure separate from iteration assignment and preserves exact revisions", () => {
  const structure = { ...context, action: "edit_source_structure", projectId: null, parentIssueId: other, stage: 2 };
  const iteration = { ...context, action: "change_iteration", iterationId: other, expectedIssueRevision: "9007199254740995", expectedIterationRevision: "9", iterationReason: "Move to the delivery period" };
  expect(CollaborationActionSchema.parse(structure)).toEqual(structure);
  expect(CollaborationActionSchema.parse(iteration)).toEqual(iteration);
  expect(CollaborationActionSchema.safeParse({ ...iteration, stage: 1 }).success).toBe(false);
});

it("captures every initial field and the observed iteration revision in one creation", () => {
  const input = { source_workspace_id: id, title: "A planned collaboration", status: "planned-custom", priority: "urgent", start_date: "2026-10-19", due_date: "2026-10-25",
    label_ids: [other], properties: { [id]: false }, project_id: other, parent_issue_id: other, stage: 1,
    current_iteration_id: other, expected_iteration_revision: "11" };
  expect(CollaborationCreationInputSchema.parse(input)).toEqual(input);
  expect(CollaborationCreationInputSchema.safeParse({ ...input, expected_iteration_revision: undefined }).success).toBe(false);
  expect(CollaborationCreationInputSchema.safeParse({ ...input, current_iteration_id: null }).success).toBe(false);
});

it("freezes initial fields in the creation request independently of later form edits", () => {
  const form = newCollaborationDeliveryForm({ server: "https://example.invalid", userId: id, context: { kind: "creation", sourceWorkspaceId: other } }, "creation");
  const values = { [id]: false };
  const prepared = prepareCollaborationCreation({ ...form, title: "Original title", initialFields: {
    priority: "high", properties: values, current_iteration_id: id, expected_iteration_revision: "9007199254740993",
  } }, other);
  values[id] = true;
  expect(prepared.input).toMatchObject({ title: "Original title", priority: "high", properties: { [id]: false }, current_iteration_id: id, expected_iteration_revision: "9007199254740993" });
  expect(prepareCollaborationCreation({ ...form, title: "Later title", initialFields: { priority: "low" } }, id).input).toMatchObject({ priority: "low" });
  expect(prepared.input?.priority).toBe("high");
});
