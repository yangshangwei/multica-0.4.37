// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseCollaborationIssue, parseCollaborationOperation, parseCollaborationStatusOptions } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { parseCollaborationActivity } from "./activity-schema";
import { CollaborationInformationAffectedSchema, CollaborationInformationBlockSchema, CollaborationInformationCursorSchema, CollaborationInformationEstimateSchema,
  CollaborationInformationHistorySchema, CollaborationInformationRepairBasisSchema, CollaborationInformationRepairConfirmationSchema,
  CollaborationInformationWorkflowSchema, collaborationInformationTextValid, compareCollaborationInformationTuple } from "./information-schema";

const id = collaborationIssueFixture.id;
const context = {
  protocol_version: 1, context_version: "a".repeat(64), active_request_id: null,
  historical_origin: null, pending_submission: null, human_selection_allowed: true,
};
const inactiveWorkflow = {
  protocol_version: 1, managed: false, context_version: "a".repeat(64), active_request_id: null, record_state: "none",
  actions: {
    request: { available: true, code: null },
    return: { available: false, code: "information_request_not_active" },
    repair: { available: false, code: "information_repair_not_applicable" },
  },
};

describe("information public contract integration", () => {
  it.each([false, true])("parses the actual inactive return action on an initial issue with managed=%s", (managed) => {
    // The server's initial workflow has no active request and refuses a return.
    const workflow = { ...inactiveWorkflow, managed };
    const issue = parseCollaborationIssue({ ...collaborationIssueFixture, current_information_request: null, information_workflow: workflow });
    expect(issue?.information_workflow).toEqual(workflow);
    expect(issue?.current_information_request).toBeNull();
  });

  it("keeps an unknown inactive action code unavailable", () => {
    const workflow = { ...inactiveWorkflow, actions: { ...inactiveWorkflow.actions,
      return: { available: false, code: "unknown_information_action" } } };
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, current_information_request: null, information_workflow: workflow })).toBeNull();
  });

  it("projects only content-free repair fields in generic activity and fails malformed detail closed", () => {
    const repair = { handoff_id: id, previous_status: "blocked", restored_status: "todo", target_user_id: id, basis: { kind: "human_selected" } };
    const entry = { id, issue_id: id, event_seq: "2", kind: "information_repaired", actor: null, created_at: "2026-11-18T08:00:00Z",
      details_available: true, target: null, handoff: null, comment_id: null, changes: [], information_repair: repair };
    const page = { issue_id: id, items: [entry], next_cursor: null };
    expect(parseCollaborationActivity(page)?.items[0]?.information_repair).toEqual(repair);
    const malformed = parseCollaborationActivity({ ...page, items: [{ ...entry, information_repair: { ...repair, note: "Private repair prose" } }] });
    expect(malformed?.items[0]).toMatchObject({ id, kind: "information_repaired", details_available: false });
    expect(malformed?.items[0]?.information_repair).toBeUndefined();
    expect(JSON.stringify(malformed)).not.toContain("Private repair prose");
  });

  it("accepts deliberate empty default only for the closed repair context", () => {
    const options = { action: "repair_information", status_options_version: "b".repeat(64), default_status: "",
      options: [{ key: "todo", name: "To do", category: "todo" }], information_repair_context: context };
    expect(parseCollaborationStatusOptions(options)).toEqual(options);
    expect(parseCollaborationStatusOptions({ ...options, information_repair_context: undefined })).toBeNull();
    expect(parseCollaborationStatusOptions({ action: "change_status", status_options_version: "b".repeat(64),
      default_status: "", options: options.options })).toBeNull();
  });

  it("does not discard a malformed present workflow as old-server absence", () => {
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, information_workflow: null })).toBeNull();
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, information_workflow: { protocol_version: 1, managed: false } })).toBeNull();
    expect(parseCollaborationIssue(collaborationIssueFixture)).not.toBeNull();
  });

  it("requires the exact content-free confirmation for a repair success", () => {
    const receipt = { code: "information_repaired", collaboration_revision: "2" };
    expect(parseCollaborationOperation(receipt)).toBeNull();
    const confirmation = { repair_handoff_id: id, prior_active_request_id: null, status: "todo",
      handler_user_id: id, superseded_count: "0" };
    expect(parseCollaborationOperation({ ...receipt, information_repair_confirmation: confirmation })).toEqual({
      ...receipt, information_repair_confirmation: confirmation,
    });
  });

  it("does not attach a repair success confirmation to a retained refusal", () => {
    expect(parseCollaborationOperation({ code: "information_repair_not_applicable", status: 409, collaboration_revision: "2",
      information_repair_confirmation: { repair_handoff_id: id, prior_active_request_id: null,
        status: "todo", handler_user_id: id, superseded_count: "0" } })).toBeNull();
  });
});

const uuid = (value: number) => `00000000-0000-4000-8000-${value.toString(16).padStart(12, "0")}`;
const legacyRequest = (value: number, time = "2026-11-18T08:00:00.000001Z") => ({
  kind: "information_request", handoff_id: uuid(value), created_at: time,
  provenance: { state: "legacy_unlinked", event_id: null, event_sequence: null },
  actor_user_id: id, note: "Original legacy reason", from_user_id: null, to_user_id: id,
  previous_status: "todo", resolution: "open", requester_user_id: null, parent_request_id: null,
  block: { protocol_version: 1, state: "legacy" }, superseded_by_repair_handoff_id: null,
});
const page = (items: ReturnType<typeof legacyRequest>[]) => ({ protocol_version: 1, issue_id: id,
  collaboration_revision: "2", lifecycle_epoch: "1", upper_handoff: items[0] ? { created_at: items[0].created_at, handoff_id: items[0].handoff_id } : null,
  items, next_cursor: null });

describe("information metadata and history schemas", () => {
  it("keeps cursors opaque and enforces their UTF-8 byte and control-character bounds", () => {
    expect(CollaborationInformationCursorSchema.parse("a".repeat(2048))).toBe("a".repeat(2048));
    expect(CollaborationInformationCursorSchema.parse("界".repeat(682))).toBe("界".repeat(682));
    for (const cursor of ["", "a".repeat(2049), "界".repeat(683), "cursor\0tail", "cursor\n", "cursor\u007f"]) {
      expect(CollaborationInformationCursorSchema.safeParse(cursor).success).toBe(false);
    }
  });

  it("validates decoded UTF-8 without changing the original reason or condition", () => {
    expect(collaborationInformationTextValid("  original reason  ")).toBe(true);
    expect(collaborationInformationTextValid("😀".repeat(1024))).toBe(true);
    expect(collaborationInformationTextValid("😀".repeat(1025))).toBe(false);
    for (const value of ["", " \n ", "hidden\0text", "\ud800", "\udc00"]) expect(collaborationInformationTextValid(value)).toBe(false);
  });

  it.each(["2026-11-18T08:00:00Z", "2026-11-18T08:00:00.123456Z", "2026-11-18T16:00:00.001+08:00"])("preserves a zoned estimate %s", (value) => {
    expect(CollaborationInformationEstimateSchema.parse(value)).toBe(value);
  });
  it.each(["2026-11-18T08:00:00.1234567Z", "2026-11-18T08:00:00", "2026-02-30T08:00:00Z", "2026-11-18T08:00:00+25:00"])("rejects an invalid estimate %s", (value) => {
    expect(CollaborationInformationEstimateSchema.safeParse(value).success).toBe(false);
  });

  it("keeps complete, legacy and unavailable records distinct", () => {
    const complete = { protocol_version: 1, state: "complete", unblock_condition: "Receipt verified", estimated_resolution_at: "2026-11-18T08:00:00.123456Z" };
    expect(CollaborationInformationBlockSchema.parse(complete)).toEqual(complete);
    expect(CollaborationInformationBlockSchema.parse({ protocol_version: 1, state: "legacy" })).toEqual({ protocol_version: 1, state: "legacy" });
    expect(CollaborationInformationBlockSchema.safeParse({ ...complete, state: "legacy" }).success).toBe(false);
    expect(CollaborationInformationBlockSchema.safeParse({ protocol_version: 1, state: "unavailable", reason_code: "information_metadata_invalid" }).success).toBe(true);
    expect(CollaborationInformationBlockSchema.safeParse(null).success).toBe(false);
  });

  it("retains a dangling raw pointer as unavailable rather than none", () => {
    const value = { protocol_version: 1, managed: true, context_version: "a".repeat(64), active_request_id: uuid(3), record_state: "unavailable",
      actions: { request: { available: false, code: "information_repair_required" }, return: { available: false, code: "information_context_unavailable" }, repair: { available: true, code: null } } };
    expect(CollaborationInformationWorkflowSchema.parse(value)).toEqual(value);
    expect(CollaborationInformationWorkflowSchema.safeParse({ ...value, record_state: "none" }).success).toBe(false);
    expect(CollaborationInformationWorkflowSchema.safeParse({ ...value, actions: { ...value.actions, repair: { available: true, code: "forbidden" } } }).success).toBe(false);
  });

  it("requires one closed exact repair basis", () => {
    expect(CollaborationInformationRepairBasisSchema.parse({ kind: "human_selected" })).toEqual({ kind: "human_selected" });
    expect(CollaborationInformationRepairBasisSchema.safeParse({ kind: "historical_information", origin_request_id: uuid(3), origin_event_id: uuid(4) }).success).toBe(true);
    for (const value of [{ kind: "historical_information", origin_request_id: uuid(3) },
      { kind: "human_selected", origin_event_id: uuid(4) }, { kind: "pending_submission", submission_id: uuid(5), submission_event_id: null }]) {
      expect(CollaborationInformationRepairBasisSchema.safeParse(value).success).toBe(false);
    }
  });

  it("preserves exact nonnegative int64 decimal affected counts", () => {
    const value = { repair_handoff_id: uuid(3), prior_active_request_id: null, status: "todo", handler_user_id: id, superseded_count: "9223372036854775807" };
    expect(CollaborationInformationRepairConfirmationSchema.parse(value)).toEqual(value);
    for (const count of ["-1", "01", "9223372036854775808", 1, null]) expect(CollaborationInformationRepairConfirmationSchema.safeParse({ ...value, superseded_count: count }).success).toBe(false);
  });

  it("checks authoritative microseconds before UUID tie ordering", () => {
    const first = legacyRequest(2, "2026-11-18T08:00:00.000999Z"), second = legacyRequest(9, "2026-11-18T08:00:00.000001Z");
    expect(compareCollaborationInformationTuple(first, second)).toBeGreaterThan(0);
    expect(CollaborationInformationHistorySchema.safeParse(page([first, second])).success).toBe(true);
    expect(CollaborationInformationHistorySchema.safeParse(page([second, first])).success).toBe(false);
    expect(CollaborationInformationHistorySchema.safeParse(page([first, first])).success).toBe(false);
  });

  it("includes legacy-unlinked and unavailable identities instead of dropping them", () => {
    const unavailable = { kind: "information_request", handoff_id: uuid(3), created_at: "2026-11-18T08:00:00.000000Z",
      provenance: { state: "unavailable", event_id: null, event_sequence: null, reason_code: "information_context_unavailable" } };
    const value = { ...page([legacyRequest(4)]), items: [legacyRequest(4), unavailable] };
    expect(CollaborationInformationHistorySchema.parse(value).items).toHaveLength(2);
    expect(CollaborationInformationHistorySchema.safeParse({ ...value, items: [{ ...unavailable, note: "Cannot invent details" }] }).success).toBe(false);
  });

  it("keeps every affected relation in the count with explicit null event provenance", () => {
    const item = { request_id: uuid(2), request_event_id: null, provenance: { state: "legacy_unlinked", event_id: null, event_sequence: null },
      resolution: "superseded", superseded_by_repair_handoff_id: uuid(3) };
    const value = { ...page([legacyRequest(2)]), repair_handoff_id: uuid(3), superseded_request_count: "1", items: [item] };
    expect(CollaborationInformationAffectedSchema.parse(value)).toEqual(value);
    expect(CollaborationInformationAffectedSchema.safeParse({ ...value, items: [{ ...item, request_event_id: uuid(9) }] }).success).toBe(false);
    expect(CollaborationInformationAffectedSchema.safeParse({ ...value, superseded_request_count: "0" }).success).toBe(false);
  });
});
