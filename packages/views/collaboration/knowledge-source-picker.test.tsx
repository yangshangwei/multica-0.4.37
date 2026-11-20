// Core owns exact-source/hash and lease matrices; this suite owns optional discovery observer visibility.
import { useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen } from "@testing-library/react";
import { focusManager, onlineManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationKeys, collaborationKnowledgeKeys, collaborationScope, CollaborationExecutionSchema, parseCollaborationComments, parseCollaborationIssue,
  parseCollaborationKnowledgeDetail, parseCollaborationMemory, parseCollaborationMemoryVersions, parseCollaborationRunRecords, type KnowledgeSourceSelection } from "@multica/core/collaboration";
import { collaborationExecutionFixture, collaborationIssueFixture, knowledgeCandidateFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { KnowledgeSourcePicker } from "./knowledge-source-picker";

vi.mock("@multica/core/auth", () => ({ useAuthStore: Object.assign(
  (selector: (state: unknown) => unknown) => selector({ user: { id: "00000000-0000-4000-8000-000000000001" } }),
  { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) },
) }));
const issue = { ...parseCollaborationIssue(collaborationIssueFixture)!, execution: CollaborationExecutionSchema.parse(collaborationExecutionFixture) };
const run = issue.execution.runs[1]!;
const commentId = "00000000-0000-4000-8000-000000000098";
const author = { kind: "member", member: issue.creator, agent: null, resource_owner: null };
const sourceKinds = [
  { kind: "memory", label: "Shared context", source: /Shared context.*Version 7/ },
  { kind: "comment", label: "Comment", source: /Readable source comment/ },
  { kind: "run_record", label: "Run record", source: /Observed record output/ },
  { kind: "delivery", label: "Delivery", source: /Delivery/ },
] as const;
let api: ApiClient, client: QueryClient, scope: ReturnType<typeof collaborationScope>;
const currentLease = () => { const now = Date.now(); return { protocol_version: 2, projection_checked_at: new Date(now).toISOString(),
  valid_until: new Date(now + 30_000).toISOString(), next_expiry_at: null }; };

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval", "Date", "performance"] });
  focusManager.setFocused(true); onlineManager.setOnline(true);
  api = new ApiClient("https://source-discovery.example"); setApiInstance(api); scope = collaborationScope(issue.creator.user_id);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  vi.spyOn(api, "getCollaborationMemory").mockResolvedValue(parseCollaborationMemory({ issue_id: issue.id, version: "7", content: "Current source text", bytes: 19,
    max_bytes: 131072, author, created_at: "2026-11-01T00:00:00Z", restored_from_version: null, can_edit: true, can_purge: true }));
  vi.spyOn(api, "getCollaborationMemoryVersions").mockResolvedValue(parseCollaborationMemoryVersions({ issue_id: issue.id, current_version: "7", max_bytes: 131072, can_purge: true,
    items: [{ version: "7", bytes: 19, author, created_at: "2026-11-01T00:00:00Z", restored_from_version: null, purged: false,
      purged_at: null, purged_by: null, purged_by_admin: false }], next_cursor: null }));
  vi.spyOn(api, "getCollaborationComments").mockResolvedValue(parseCollaborationComments({ issue_id: issue.id, items: [{ id: commentId,
    issue_id: issue.id, author_type: "member", author: issue.creator, content: "Readable source comment", type: "comment", parent_id: null, comment_revision: "3",
    attachment_ids: [], created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-01T00:00:00Z", deleted_at: null, resolved_at: null,
    resolved_by: null, can_edit: false, can_resolve: false, reactions: [] }], next_cursor: null }));
  vi.spyOn(api, "getCollaborationRunRecords").mockResolvedValue(parseCollaborationRunRecords({ protocol_version: 1, issue_id: issue.id, run,
    resource: { agent_id: run.agent_id, runtime_id: commentId, resource_owner_user_id: issue.creator.user_id, execution_workspace_id: issue.source_workspace.id,
      grant_id: commentId, provider: "recorded-provider", model: "recorded-model", account_source: "runtime_configuration" },
    recording: { status: "complete", inputs_recorded: true }, usage: null, items: [{ id: `event:${commentId}:message-0`, kind: "message", role: "assistant", body: "Observed record output" }], next_cursor: null }));
  vi.spyOn(api, "getCollaborationKnowledgeCandidate").mockImplementation(async () => parseCollaborationKnowledgeDetail({ ...currentLease(), history: [], history_truncated: false,
    candidate: { ...knowledgeCandidateFixture, version: "1", state: "approved", state_revision: "2", source_issue_id: issue.id,
      source_workspace_id: issue.source_workspace.id, target: { kind: "workspace", workspace_id: issue.source_workspace.id },
      proposed_by_user_id: null, approved_by_user_id: null, proposed_at: "2026-11-01T00:00:00Z", approved_at: "2026-11-01T00:00:01Z", published_target_revision: "1",
      proposal_origin: "automation", decision_origin: "automation", usage_status: "active", automation: { job_id: commentId, policy_revision: "0", rule_version: "project-knowledge-v1" },
      decision: { actor_user_id: null, at: "2026-11-01T00:00:01Z", reason: null }, sources: [{ kind: "delivery", issue_id: issue.id, submission_id: commentId,
        delivery_event_seq: "10", accepted_event_seq: "11", sha256: "c".repeat(64) }], capabilities: { can_approve: false, can_reject: false, can_withdraw: true } } }));
});
afterEach(() => { cleanup(); client.clear(); focusManager.setFocused(true); onlineManager.setOnline(true); vi.useRealTimers(); vi.restoreAllMocks(); });
async function paintQueries() { await act(async () => { await vi.advanceTimersByTimeAsync(20); }); }
function SourceDraft({ active, delivery }: { active: boolean; delivery: boolean }) {
  const [selected, setSelected] = useState<KnowledgeSourceSelection[]>([]);
  return <><button>Outside sources</button><div hidden={!active} inert={!active}><KnowledgeSourcePicker scope={scope} issue={issue} active={active}
    selected={selected} onChange={setSelected} disabled={false} deliveryCandidateId={delivery ? knowledgeCandidateFixture.candidate_id : undefined} /></div></>;
}

it.each(sourceKinds)("stops hidden $kind discovery and rereads current sources on reopen with the selection kept", async ({ kind, label, source }) => {
  const view = (active: boolean) => <QueryClientProvider client={client}><SourceDraft active={active} delivery={kind === "delivery"} /></QueryClientProvider>;
  const mounted = renderWithI18n(view(true));
  await paintQueries();
  if (kind !== "memory" && kind !== "delivery") {
    fireEvent.click(screen.getByRole("combobox", { name: "Sources" }));
    const option = screen.getByRole("option", { name: label });
    fireEvent.mouseMove(option); fireEvent.click(option);
    await paintQueries();
  }
  if (kind === "run_record") {
    fireEvent.click(screen.getByRole("combobox", { name: "Choose a recorded run" }));
    const option = screen.getByRole("option", { name: new RegExp(run.id.slice(-8)) });
    fireEvent.mouseMove(option); fireEvent.click(option);
    await paintQueries();
  }
  const selectedSource = screen.getByRole("checkbox", { name: source });
  fireEvent.click(selectedSource);
  expect(selectedSource).toBeChecked();
  const reads = () => kind === "memory" ? [vi.mocked(api.getCollaborationMemory).mock.calls.length, vi.mocked(api.getCollaborationMemoryVersions).mock.calls.length]
    : [vi.mocked(kind === "comment" ? api.getCollaborationComments : kind === "run_record" ? api.getCollaborationRunRecords : api.getCollaborationKnowledgeCandidate).mock.calls.length];
  const beforeHide = reads();
  mounted.rerender(view(false));
  const outside = screen.getByRole("button", { name: "Outside sources" }); outside.focus();
  await act(async () => {
    await client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) });
    await vi.advanceTimersByTimeAsync(31_000);
    focusManager.setFocused(false); focusManager.setFocused(true);
    onlineManager.setOnline(false); onlineManager.setOnline(true);
    await vi.advanceTimersByTimeAsync(20);
  });
  expect(reads()).toEqual(beforeHide);
  if (kind === "delivery") expect(client.getQueryData(collaborationKnowledgeKeys.candidate(scope, knowledgeCandidateFixture.candidate_id))).toBeUndefined();
  mounted.rerender(view(true));
  await paintQueries();
  expect(reads().every((count, index) => count > beforeHide[index]!)).toBe(true);
  expect(screen.getByRole("checkbox", { name: source })).toBeChecked();
  expect(outside).toHaveFocus();
});
