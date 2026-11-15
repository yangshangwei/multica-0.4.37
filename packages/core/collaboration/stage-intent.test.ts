// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { clearCollaborationDrafts, retainCollaborationDraftScope } from "./draft";
import {
  CollaborationStageIntentStorageError, clearCollaborationStageIntents, collaborationStageFrozenRequest, collaborationStageIntentRevision,
  freezeCollaborationStageIntent, readCollaborationStageIntents, retainCollaborationStageIntentScope, retireCollaborationStageIntent,
  saveCollaborationStageIntent, subscribeCollaborationStageIntents, type CollaborationStageIntent,
} from "./stage-intent";
import { stageDigests, stageIds, stageObservationsFixture, stageProposalContentFixture } from "./stage-test-fixtures";
import type { StorageAdapter } from "../types/storage";

const saved = vi.hoisted(() => new Map<string, string>());
vi.mock("../platform/storage", () => ({ defaultStorage: {
  keys: () => [...saved.keys()], getItem: (key: string) => saved.get(key) ?? null,
  setItem: (key: string, value: string) => saved.set(key, value), removeItem: (key: string) => saved.delete(key),
} }));
afterEach(() => { saved.clear(); });

const scope = { server: "https://server.example", userId: stageIds.product };
const issueTarget = { kind: "issue", issueId: stageIds.issue } as const;
const projectTarget = { kind: "project", workspaceId: stageIds.workspace, projectId: stageIds.project } as const;
const candidate = { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 };
const decideBody = { ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", candidate, decision: "accept", reason: "" };
function decide(requestId: string = stageIds.request, createdAt = 1): CollaborationStageIntent {
  return freezeCollaborationStageIntent({ scope, target: issueTarget, action: "decide_proposal", requestId, proposalId: stageIds.proposal, body: decideBody, createdAt });
}
const policy = () => freezeCollaborationStageIntent({ scope, target: projectTarget, action: "revise_policy", requestId: stageIds.request2,
  body: { expected_policy_revision: "0", enabled: false, reason: "Pause" }, createdAt: 2 });

it("freezes the key and the exact canonical body, and nothing else from the server", () => {
  const intent = decide();
  expect(Object.isFrozen(intent)).toBe(true);
  expect(JSON.parse(intent.bodyJSON)).toEqual(decideBody);
  expect(Object.keys(intent).sort()).toEqual(["action", "bodyJSON", "createdAt", "proposalId", "requestId", "scope", "target", "version"]);
  expect(collaborationStageFrozenRequest(intent)).toEqual({ action: "decide_proposal", issueId: stageIds.issue, proposalId: stageIds.proposal, body: decideBody });
  expect(collaborationStageFrozenRequest(policy())).toMatchObject({ action: "revise_policy", workspaceId: stageIds.workspace, projectId: stageIds.project, body: { enabled: false } });
});

it("refuses to freeze an intent whose route identity, body or size is not the action's", () => {
  expect(() => freezeCollaborationStageIntent({ scope, target: issueTarget, action: "decide_proposal", requestId: stageIds.request, body: decideBody, createdAt: 1 })).toThrow();
  expect(() => freezeCollaborationStageIntent({ scope, target: projectTarget, action: "decide_proposal", requestId: stageIds.request, proposalId: stageIds.proposal, body: decideBody, createdAt: 1 })).toThrow();
  expect(() => freezeCollaborationStageIntent({ scope, target: issueTarget, action: "revise_policy", requestId: stageIds.request, body: { expected_policy_revision: "0", enabled: false, reason: "x" }, createdAt: 1 })).toThrow();
  expect(() => freezeCollaborationStageIntent({ scope, target: issueTarget, action: "configure_roles", requestId: stageIds.request, proposalId: stageIds.proposal, body: decideBody, createdAt: 1 })).toThrow();
  expect(() => freezeCollaborationStageIntent({ scope, target: issueTarget, action: "decide_proposal", requestId: "not-a-key", proposalId: stageIds.proposal, body: decideBody, createdAt: 1 })).toThrow();
  expect(() => freezeCollaborationStageIntent({ scope, target: issueTarget, action: "decide_proposal", requestId: stageIds.request, proposalId: stageIds.proposal, body: { ...decideBody, reason: "r".repeat(2001) }, createdAt: 1 })).toThrow();
  // 2 MiB applies to proposal bodies; a 64 KiB action cannot carry one.
  const children = Array.from({ length: 200 }, (_unused, index) => ({ issue_id: `00000000-0000-4000-8000-00000000b${String(index).padStart(3, "0")}`, content_digest: stageDigests.child1, code: null }));
  expect(() => freezeCollaborationStageIntent({ scope, target: issueTarget, action: "confirm_scope", requestId: stageIds.request,
    body: { ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_scope_revision: "0", children, reason: "界".repeat(2000) }, createdAt: 1 })).not.toThrow();
  expect(() => freezeCollaborationStageIntent({ scope, target: issueTarget, action: "create_proposal", requestId: stageIds.request,
    body: { ...stageObservationsFixture, expected_policy_revision: "1", content: stageProposalContentFixture, source: { kind: "manual" } }, createdAt: 1 })).not.toThrow();
});

it("stores one record per key, reports an earlier copy and refuses a different body under the same key", () => {
  const intent = decide();
  expect(saveCollaborationStageIntent(intent)).toBe(false);
  expect(saveCollaborationStageIntent(intent)).toBe(true);
  expect(saved.size).toBe(1);
  const changed = freezeCollaborationStageIntent({ scope, target: issueTarget, action: "decide_proposal", requestId: stageIds.request, proposalId: stageIds.proposal,
    body: { ...decideBody, decision: "reject", reason: "No" }, createdAt: 1 });
  expect(() => saveCollaborationStageIntent(changed)).toThrow(CollaborationStageIntentStorageError);
  expect(readCollaborationStageIntents(scope, issueTarget)).toEqual([intent]);
});

it("reads by target in creation order and keeps other accounts, servers, issues and projects apart", () => {
  saveCollaborationStageIntent(decide(stageIds.request2, 5));
  saveCollaborationStageIntent(decide(stageIds.request, 3));
  saveCollaborationStageIntent(policy());
  expect(readCollaborationStageIntents(scope, issueTarget).map((intent) => intent.requestId)).toEqual([stageIds.request, stageIds.request2]);
  expect(readCollaborationStageIntents(scope, projectTarget).map((intent) => intent.action)).toEqual(["revise_policy"]);
  expect(readCollaborationStageIntents(scope, { kind: "issue", issueId: stageIds.child1 })).toEqual([]);
  expect(readCollaborationStageIntents({ ...scope, userId: stageIds.owner }, issueTarget)).toEqual([]);
  expect(readCollaborationStageIntents({ ...scope, server: "https://other.example" }, issueTarget)).toEqual([]);
  expect(readCollaborationStageIntents(scope, { ...projectTarget, projectId: stageIds.child1 })).toEqual([]);
});

it("treats a corrupt, relocated or edited record as a storage error instead of dropping it", () => {
  const intent = decide();
  saveCollaborationStageIntent(intent);
  const [key, value] = [...saved.entries()][0]!;
  saved.set(key, value.replace('\\"decision\\":\\"accept\\"', '\\"decision\\":\\"reject\\"'));
  expect(() => readCollaborationStageIntents(scope, issueTarget)).toThrow(CollaborationStageIntentStorageError);
  saved.set(key, "{not json");
  expect(() => readCollaborationStageIntents(scope, issueTarget)).toThrow(CollaborationStageIntentStorageError);
  saved.clear();
  saved.set(key.replace(stageIds.request, stageIds.request2), value);
  expect(() => readCollaborationStageIntents(scope, issueTarget)).toThrow(CollaborationStageIntentStorageError);
});

it("fails closed when storage cannot enumerate or does not keep the record", () => {
  const blind: StorageAdapter = { getItem: () => null, setItem: () => undefined, removeItem: () => undefined };
  expect(() => saveCollaborationStageIntent(decide(), blind)).toThrow(CollaborationStageIntentStorageError);
  expect(() => readCollaborationStageIntents(scope, issueTarget, blind)).toThrow(CollaborationStageIntentStorageError);
  const lossy: StorageAdapter = { keys: () => [], getItem: () => null, setItem: () => undefined, removeItem: () => undefined };
  let error: unknown;
  try { saveCollaborationStageIntent(decide(), lossy); } catch (caught) { error = caught; }
  expect(error).toBeInstanceOf(CollaborationStageIntentStorageError);
  expect((error as CollaborationStageIntentStorageError).previouslyStored).toBe(false);
});

it("retires exactly one request and notifies subscribers", () => {
  const listener = vi.fn();
  const unsubscribe = subscribeCollaborationStageIntents(listener);
  const before = collaborationStageIntentRevision();
  const first = decide(stageIds.request, 1), second = decide(stageIds.request2, 2);
  saveCollaborationStageIntent(first); saveCollaborationStageIntent(second);
  retireCollaborationStageIntent(first);
  expect(readCollaborationStageIntents(scope, issueTarget)).toEqual([second]);
  expect(listener).toHaveBeenCalledTimes(3);
  expect(collaborationStageIntentRevision()).toBe(before + 3);
  unsubscribe();
  retireCollaborationStageIntent(second);
  expect(listener).toHaveBeenCalledTimes(3);
});

it("strips an issue's private intent on access loss, leaving other issues and project requests", () => {
  saveCollaborationStageIntent(decide());
  saveCollaborationStageIntent(freezeCollaborationStageIntent({ scope, target: { kind: "issue", issueId: stageIds.child1 }, action: "decide_proposal",
    requestId: stageIds.child2, proposalId: stageIds.proposal, body: decideBody, createdAt: 1 }));
  saveCollaborationStageIntent(policy());
  clearCollaborationDrafts(undefined, { ...scope, issueId: stageIds.issue });
  expect(readCollaborationStageIntents(scope, issueTarget)).toMatchObject([{ recoveryOnly: true, requestId: stageIds.request }]);
  expect(readCollaborationStageIntents(scope, { kind: "issue", issueId: stageIds.child1 })).toHaveLength(1);
  expect(readCollaborationStageIntents(scope, projectTarget)).toHaveLength(1);
  clearCollaborationStageIntents();
  expect(saved.size).toBe(0);
});

it("discards another identity's intents when the endpoint or account changes", () => {
  saveCollaborationStageIntent(decide());
  saveCollaborationStageIntent(policy());
  const other = { server: scope.server, userId: stageIds.owner };
  saveCollaborationStageIntent(freezeCollaborationStageIntent({ scope: other, target: issueTarget, action: "decide_proposal", requestId: stageIds.child1,
    proposalId: stageIds.proposal, body: decideBody, createdAt: 1 }));
  saved.set("multica_collaboration_stage:not-a-scope:x", "{}");
  saved.set("unrelated", "kept");
  retainCollaborationDraftScope(scope);
  expect(readCollaborationStageIntents(scope, issueTarget)).toHaveLength(1);
  expect(readCollaborationStageIntents(scope, projectTarget)).toHaveLength(1);
  expect(readCollaborationStageIntents(other, issueTarget)).toEqual([]);
  expect([...saved.keys()].filter((key) => key.startsWith("multica_collaboration_stage:"))).toHaveLength(2);
  expect(saved.get("unrelated")).toBe("kept");
  retainCollaborationStageIntentScope(other);
  expect([...saved.keys()].filter((key) => key.startsWith("multica_collaboration_stage:"))).toHaveLength(0);
});

it("C01 account switch and logout clear stripped recovery records as well as full payloads", () => {
  saveCollaborationStageIntent(decide());
  clearCollaborationDrafts(undefined, { ...scope, issueId: stageIds.issue });
  expect(readCollaborationStageIntents(scope, issueTarget)).toHaveLength(1);
  retainCollaborationStageIntentScope({ ...scope, userId: stageIds.owner });
  expect(saved.size).toBe(0);
  saveCollaborationStageIntent(decide());
  clearCollaborationDrafts(undefined, { ...scope, issueId: stageIds.issue });
  clearCollaborationDrafts();
  expect(saved.size).toBe(0);
});
