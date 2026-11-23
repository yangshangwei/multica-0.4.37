// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { collaborationStageProposalActions, collaborationStageScopeSelection, deriveCollaborationStageState } from "./stage-presentation";
import { parseCollaborationStageProposalDetail, parseCollaborationStageView, type CollaborationStageRead, type CollaborationStageView } from "./stage-schema";
import {
  stageCandidateV1Fixture, stageCandidateV2Fixture, stageDigests, stageIds, stageIssueContextFixture, stageProposalAcceptedDetailFixture, stageProposalDetailFixture,
  stageViewManagedFixture, stageViewNeedsConfirmationFixture, stageViewUnmanagedFixture, stagePullRequestFixture,
} from "./stage-test-fixtures";

const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture })!;
const managed = parseCollaborationStageView(stageViewManagedFixture)!;
const available = (view: CollaborationStageView): CollaborationStageRead<CollaborationStageView> => ({ state: "available", value: view });
const none = { configureRoles: false, createProposal: false, decideProposal: false, confirmScope: false };

describe("deriveCollaborationStageState", () => {
  it("offers exactly the hinted actions on a managed, in-sync, fully known view", () => {
    expect(deriveCollaborationStageState(available(managed), issue)).toMatchObject({
      availability: "available", known: true, managed: true, inSync: true, policyState: "enabled", stage: "development", missing: [],
      can: { configureRoles: true, createProposal: true, decideProposal: true, confirmScope: true },
    });
    const first = deriveCollaborationStageState(available(parseCollaborationStageView(stageViewNeedsConfirmationFixture)!), issue);
    expect(first.missing).toEqual(["roles", "scope"]);
    expect(first.can).toEqual({ configureRoles: true, createProposal: true, decideProposal: false, confirmScope: false });
  });
  it.each<[string, CollaborationStageRead<CollaborationStageView> | undefined, string]>([
    ["no data yet", undefined, "loading"],
    ["an older server", { state: "unsupported" }, "unsupported"],
    ["an unready schema", { state: "unavailable", reason: "schema" }, "unavailable"],
    ["a malformed response", { state: "unavailable", reason: "malformed" }, "unavailable"],
    ["an unexpected state", { state: "not_found" }, "unavailable"],
  ])("offers nothing and claims nothing for %s", (_name, read, availability) => {
    expect(deriveCollaborationStageState(read, issue)).toEqual({ availability, known: false, managed: null, inSync: false, policyState: null, stage: null, missing: [], can: none });
  });
  it("offers nothing on an unmanaged root while still reporting its policy state", () => {
    const state = deriveCollaborationStageState(available(parseCollaborationStageView(stageViewUnmanagedFixture)!), issue);
    expect(state).toMatchObject({ availability: "available", managed: false, policyState: "disabled", stage: null, can: none });
  });
  it.each<[string, Partial<CollaborationStageView>]>([
    ["an unknown stage", { stage: { current: "unknown", stage_revision: "1" } }],
    ["an unknown scope state", { scope_state: "unknown" }],
    ["an unknown policy state", { policy: { ...managed.policy, state: "unknown" } }],
  ])("disables every action for %s", (_name, change) => {
    const state = deriveCollaborationStageState(available({ ...managed, ...change }), issue);
    expect(state.known).toBe(false);
    expect(state.can).toEqual(none);
  });
  it("disables every action while the view and the detail observe different versions, or without a write context", () => {
    expect(deriveCollaborationStageState(available({ ...managed, collaboration_revision: "8" }), issue)).toMatchObject({ inSync: false, can: none });
    expect(deriveCollaborationStageState(available({ ...managed, lifecycle_epoch: "2" }), issue)).toMatchObject({ inSync: false, can: none });
    expect(deriveCollaborationStageState(available(managed), { ...issue, write_context: null })).toMatchObject({ inSync: true, can: none });
    expect(deriveCollaborationStageState(available({ ...managed, issue_id: stageIds.child1 }), issue)).toMatchObject({ availability: "unavailable", can: none });
  });
  it("needs an explicit true hint for each action", () => {
    const hints = { configure_roles: false, create_proposal: true, decide_proposal: false, confirm_scope: false };
    expect(deriveCollaborationStageState(available({ ...managed, capabilities: hints }), issue).can).toEqual({ ...none, createProposal: true });
  });
});

describe("collaborationStageProposalActions", () => {
  const state = deriveCollaborationStageState(available(managed), issue);
  const pending = parseCollaborationStageProposalDetail(stageProposalDetailFixture)!;
  it("lets the newest pending version be revised and decided, pinned to that exact reference", () => {
    expect(collaborationStageProposalActions(state, pending, issue)).toEqual({
      newest: { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 }, stale: false, canRevise: true, canDecide: true,
    });
  });
  it("allows only revision of a rejected or stale newest version", () => {
    const rejected = parseCollaborationStageProposalDetail({ ...stageProposalDetailFixture, state: "rejected", versions: [{ ...stageCandidateV2Fixture, state: "rejected" }, stageCandidateV1Fixture] })!;
    expect(collaborationStageProposalActions(state, rejected, issue)).toMatchObject({ canRevise: true, canDecide: false });
    const stale = collaborationStageProposalActions(state, pending, { ...issue, lifecycle_epoch: "2" });
    expect(stale).toMatchObject({ stale: true, canRevise: true, canDecide: false });
  });
  it("offers nothing on an accepted proposal, an unknown state or without the hints", () => {
    const acceptedDetail = parseCollaborationStageProposalDetail(stageProposalAcceptedDetailFixture)!;
    expect(collaborationStageProposalActions(state, acceptedDetail, issue)).toMatchObject({ canRevise: false, canDecide: false });
    const unknown = parseCollaborationStageProposalDetail({ ...stageProposalDetailFixture, state: "archived" })!;
    expect(collaborationStageProposalActions(state, unknown, issue)).toMatchObject({ canRevise: false, canDecide: false });
    const unknownVersion = parseCollaborationStageProposalDetail({ ...stageProposalDetailFixture, versions: [{ ...stageCandidateV2Fixture, state: "frozen" }, stageCandidateV1Fixture] })!;
    expect(collaborationStageProposalActions(state, unknownVersion, issue)).toMatchObject({ canRevise: false, canDecide: false });
    const readOnly = deriveCollaborationStageState(available({ ...managed, capabilities: { ...managed.capabilities, create_proposal: false, decide_proposal: false } }), issue);
    expect(collaborationStageProposalActions(readOnly, pending, issue)).toMatchObject({ canRevise: false, canDecide: false });
    // A detail that belongs to another issue offers nothing here.
    expect(collaborationStageProposalActions(state, { ...pending, issue_id: stageIds.child1 }, issue)).toMatchObject({ canRevise: false, canDecide: false });
  });
});

describe("collaborationStageScopeSelection", () => {
  it("requires an explicit code choice and refuses unobserved or changed references without replacing them", () => {
    const selection = collaborationStageScopeSelection(managed);
    const code = { pull_request: stagePullRequestFixture };
    expect(selection.entries([stageIds.child1], {})).toBeNull();
    expect(selection.entries([stageIds.child1], { [stageIds.child1]: code })).toEqual([{ issue_id: stageIds.child1, content_digest: stageDigests.child1, code }]);
    for (const change of [{ head_sha: "b".repeat(40) }, { pull_request_id: stageIds.child2 }, { provider: "vcs" as const }]) {
      expect(selection.entries([stageIds.child1], { [stageIds.child1]: { pull_request: { ...stagePullRequestFixture, ...change } } })).toBeNull();
    }
    expect(selection.entries([], {})).toEqual([]);
  });
  it("includes governed descendants and keeps unknown code state inoperable", () => {
    const descendant = parseCollaborationStageView({ ...stageViewManagedFixture, children: { items: [{ ...stageViewManagedFixture.children.items[0], depth: 3 }], truncated: false } })!;
    expect(collaborationStageScopeSelection(descendant).candidates.map((child) => child.depth)).toEqual([3]);
    const unknown = parseCollaborationStageView({ ...stageViewManagedFixture, children: { items: [{ ...stageViewManagedFixture.children.items[0], code_state: "future" }], truncated: false } })!;
    expect(deriveCollaborationStageState(available(unknown), issue).can.confirmScope).toBe(false);
  });
  it("lists attached children with the server digest and preselects those already carried by a current approval", () => {
    const selection = collaborationStageScopeSelection(managed);
    expect(selection.candidates.map((child) => child.issue_id)).toEqual([stageIds.child1, stageIds.child2]);
    expect(selection.preselected).toEqual([stageIds.child1, stageIds.child2]);
    expect(selection.complete).toBe(true);
    expect(selection.entries([stageIds.child1], { [stageIds.child1]: null })).toEqual([{ issue_id: stageIds.child1, content_digest: stageDigests.child1, code: null }]);
  });
  it("never preselects an unapproved child, skips detached ones and refuses unknown selections", () => {
    const first = parseCollaborationStageView(stageViewNeedsConfirmationFixture)!;
    expect(collaborationStageScopeSelection(first).preselected).toEqual([]);
    const detached = { ...managed, children: { items: [{ ...managed.children.items[0]!, attached: false }, managed.children.items[1]!], truncated: true } };
    const selection = collaborationStageScopeSelection(detached);
    expect(selection.candidates.map((child) => child.issue_id)).toEqual([stageIds.child2]);
    expect(selection.complete).toBe(false);
    expect(selection.entries([stageIds.child1], { [stageIds.child1]: null })).toBeNull();
    expect(selection.entries([stageIds.child2, stageIds.child2], { [stageIds.child2]: null })).toBeNull();
  });
});
