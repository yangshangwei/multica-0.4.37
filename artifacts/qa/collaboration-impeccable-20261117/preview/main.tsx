import { useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { I18nProvider } from "@multica/core/i18n/react";
import { collaborationKeys, collaborationScope, collaborationMemoryReasonValid, collaborationKnowledgeKeys, expireKnowledgeProjections,
  parseCollaborationIssue, parseCollaborationMemory, parseCollaborationMemoryVersions, parseCollaborationKnowledgeCandidates, parseCollaborationKnowledgeDetail,
  type CollaborationKnowledgeDetail, type CollaborationPropertyDefinition, type CollaborationStatusOptions } from "@multica/core/collaboration";
import type { IssuePropertyValue } from "@multica/core/types";
import { collaborationIssueFixture, knowledgeCandidateFixture } from "@multica/core/collaboration/test-fixtures";
import { defaultStorage } from "@verification/core/platform/storage";
import { CollaborationLabelInput, CollaborationPropertyInputs } from "@verification/views/collaboration/field-inputs";
import { CollaborationMemoryPanel } from "@verification/views/collaboration/memory-panel";
import { CollaborationKnowledgePanel } from "@verification/views/collaboration/knowledge-panel";
import { CollaborationCreationEntry } from "@verification/views/collaboration/creation-form";
import { CollaborationWorkflow } from "@verification/views/collaboration/workflow";
import { Button } from "@multica/ui/components/ui/button";
import { NavigationProvider } from "@multica/views/navigation";
import { RESOURCES } from "@multica/views/locales";
import "./style.css";

// The audit uses original components, a fresh in-memory account, and no network.
const actor = "00000000-0000-4000-8000-000000000001";
const source = "00000000-0000-4000-8000-000000000002";
const stored = new Map<string, string>();
const storage = { getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => { stored.set(key, value); },
  removeItem: (key: string) => { stored.delete(key); }, keys: () => [...stored.keys()] };
Object.assign(defaultStorage, storage);
const api = new ApiClient("https://audit-fixture.invalid");
setApiInstance(api);
const auth = createAuthStore({ api, storage });
registerAuthStore(auth);
auth.setState({ user: { id: actor, name: "Isolated audit actor" }, status: "authenticated", isLoading: false });
const scope = collaborationScope(actor);
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
const params = new URLSearchParams(location.search);
const locale = params.get("locale") === "zh-Hans" ? "zh-Hans" : "en";
const translations = RESOURCES[locale].collaboration;
const author = { kind: "member", member: issue.creator, agent: null, resource_owner: null };
const requests: string[] = [];
api.getCollaborationMemory = async () => {
  requests.push("memory");
  return parseCollaborationMemory({ issue_id: issue.id, version: "2", content: "Audit fixture context", bytes: 21,
    max_bytes: 131072, author, created_at: "2026-11-17T00:00:00Z", restored_from_version: null, can_edit: true, can_purge: true });
};
api.getCollaborationMemoryVersions = async () => {
  requests.push("memory/versions");
  return parseCollaborationMemoryVersions({ issue_id: issue.id, current_version: "2", max_bytes: 131072, can_purge: true,
    items: ["2", "1"].map(version => ({ version, bytes: 21, author, created_at: "2026-11-17T00:00:00Z",
      restored_from_version: null, purged: false, purged_at: null, purged_by: null, purged_by_admin: false })), next_cursor: null });
};
const candidate = { ...knowledgeCandidateFixture, source_issue_id: issue.id, source_workspace_id: issue.source_workspace.id,
  target: { kind: "workspace" as const, workspace_id: issue.source_workspace.id },
  sources: [{ kind: "memory" as const, issue_id: issue.id, version: "1", sha256: "a".repeat(64) }] };
const lease = () => { const start = Date.now(); return { protocol_version: 1 as const,
  projection_checked_at: new Date(start).toISOString(), valid_until: new Date(start + 30_000).toISOString(), next_expiry_at: null }; };
let holdCandidate = false;
let resolveCandidate: ((value: CollaborationKnowledgeDetail) => void) | null = null;
const detail = () => parseCollaborationKnowledgeDetail({ ...lease(), candidate, history: [], history_truncated: false })!;
api.getCollaborationKnowledgeCandidates = async () => {
  requests.push("knowledge/list");
  const { content: _content, ...summary } = candidate;
  return parseCollaborationKnowledgeCandidates({ ...lease(), issue_id: issue.id, items: [summary], next_cursor: null, capabilities: { can_propose: true } });
};
api.getCollaborationKnowledgeCandidate = async () => {
  requests.push("knowledge/candidate");
  if (holdCandidate) return new Promise<CollaborationKnowledgeDetail>(resolve => { resolveCandidate = resolve; });
  return detail();
};
const sourceChoice = { id: source, name: "Audit source", slug: "audit-source" };
const parentChoice = { id: "00000000-0000-4000-8000-000000000010", identifier: "AUD-10", name: "Parent audit task" };
api.getCollaborationCreationOptions = async sourceId => {
  requests.push("creation/options");
  return { creation_enabled: true, sources: [sourceChoice], source: sourceId ? sourceChoice : null, max_attachment_bytes: 104857600,
    ...(sourceId ? { fields: { labels: [], properties: [], statuses: [{ key: "todo", name: "Todo", category: "todo" as const }], default_status: "todo",
      source_options: { projects: [], parents: [parentChoice], parents_truncated: false, iterations: [], iterations_enabled: false } } } : {}) };
};
api.getCollaborationDraftAttachments = async draftId => ({ draft_id: draftId, attachments: [] });
let statusRequests = 0;
let resolveStatus: ((value: CollaborationStatusOptions) => void) | null = null;
api.getCollaborationStatusOptions = async (_issueId, action) => {
  requests.push("workflow/status");
  if (++statusRequests === 1) throw new TypeError("Audit transient status read failure");
  return new Promise<CollaborationStatusOptions>(resolve => { resolveStatus = resolve; });
};
const submission = { id: "00000000-0000-4000-8000-000000000009", issue_id: issue.id, event_seq: "8", lifecycle_epoch: "1",
  author_user_id: actor, author: issue.handler, created_at: "2026-11-17T00:00:00Z", result: "Delivered audit result",
  basis: "Audit fixture basis", attachment_ids: [], source_comment_id: null, evidence_state: "available" as const, decision: null };
const workflowIssue = { ...issue, ai_policy: "manual" as const, status: "in_review", status_category: "in_review" as const,
  write_context: { participant_epoch: "2", account_epoch: "3" }, delivery: { current: submission, latest: submission, has_history: true },
  capabilities: { ...issue.capabilities, return_acceptance: true, change_ai_policy: true } };
window.fetch = async () => { throw new Error("Audit fixture prohibits real API transport"); };
const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
client.setQueryData(collaborationKeys.issue(scope, issue.id), issue);
const navigation = { push: () => {}, replace: () => {}, back: () => {}, pathname: "/account",
  searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path: string) => path };
const options = Array.from({ length: 10 }, (_, index) => ({ id: `choice-${index + 1}`, name: `Choice ${index + 1}`, color: "#666666" }));
const definitions: CollaborationPropertyDefinition[] = [
  { id: "select", name: "Delivery type", type: "select", icon: null, config: { options }, archived: false },
  { id: "multi", name: "Review scopes", type: "multi_select", icon: null, config: { options }, archived: false },
  { id: "text", name: "Acceptance criterion", type: "text", icon: null, config: {}, archived: false },
  { id: "number", name: "Estimated days", type: "number", icon: null, config: {}, archived: false },
  { id: "url", name: "Reference URL", type: "url", icon: null, config: {}, archived: false },
];

function Fields() {
  const [labels, setLabels] = useState<string[]>(["design"]);
  const [values, setValues] = useState<Record<string, IssuePropertyValue>>({});
  return <section aria-label="Field audit" className="space-y-6">
    <CollaborationLabelInput labels={[{ id: "design", name: "Design", color: "#666666" }, { id: "engineering", name: "Engineering", color: "#666666" }]}
      selectedIds={labels} onChange={setLabels} disabled={false} />
    <CollaborationPropertyInputs scope={scope} sourceId={source} definitions={definitions} values={values}
      onChange={(id, value) => setValues(current => {
        const next = { ...current };
        if (value === undefined) delete next[id]; else next[id] = value;
        return next;
      })} disabled={false} />
    <output aria-label="Selected custom values">{JSON.stringify(values)}</output>
  </section>;
}

function Knowledge() {
  const [active, setActive] = useState(true);
  return <>
    <Button type="button" onClick={() => setActive(current => !current)}>{active ? "Hide knowledge" : "Show knowledge"}</Button>
    <button type="button" data-background-focus>Outside audit focus</button>
    <div hidden={!active} inert={!active}><CollaborationKnowledgePanel scope={scope} issue={issue} active={active} /></div>
  </>;
}

function App() {
  const mode = params.get("surface");
  return <QueryClientProvider client={client}><I18nProvider locale={locale} resources={RESOURCES}><NavigationProvider value={navigation}>
    <main className="min-w-0 space-y-6 p-4 text-foreground">
      {mode === "memory" ? <CollaborationMemoryPanel scope={scope} issue={issue} /> : mode === "knowledge" ? <Knowledge />
        : mode === "creation" ? <CollaborationCreationEntry scope={scope} sourceWorkspaceId={source} />
          : mode === "workflow" ? <CollaborationWorkflow scope={scope} issue={workflowIssue} /> : <Fields />}
    </main>
  </NavigationProvider></I18nProvider></QueryClientProvider>;
}
Object.assign(window, { auditFixture: { requests, reasonValid: collaborationMemoryReasonValid, translations, ready: true,
  holdCandidateRead: () => { holdCandidate = true; },
  refreshCandidate: () => { void client.invalidateQueries({ queryKey: collaborationKnowledgeKeys.candidate(scope, candidate.candidate_id), exact: true }); },
  eraseKnowledge: () => { expireKnowledgeProjections(client, scope, true); },
  releaseCandidate: () => { holdCandidate = false; const resolve = resolveCandidate; resolveCandidate = null; resolve?.(detail()); },
  candidatePending: () => resolveCandidate !== null,
  candidateCached: () => client.getQueryData(collaborationKnowledgeKeys.candidate(scope, candidate.candidate_id)) !== undefined,
  statusPending: () => resolveStatus !== null,
  releaseStatus: () => { const resolve = resolveStatus; resolveStatus = null; resolve?.({ action: "return_acceptance", status_options_version: "a".repeat(64),
    default_status: "in_progress", options: [{ key: "in_progress", name: "In progress", category: "in_progress" }] }); },
} });
createRoot(document.getElementById("root")!).render(<App />);
