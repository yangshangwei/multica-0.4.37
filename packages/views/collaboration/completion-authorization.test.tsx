import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationWorkflow } from "./workflow";

// Core owns the variant/schema/receipt matrix; this suite drives the shared human flow.
const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ status: "authenticated", user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const actor = { user_id: id(1), name: "Creator", avatar_url: "", external: false };
const submission = { id: id(5), issue_id: id(1), event_seq: "4", lifecycle_epoch: "1", author_user_id: id(2), author: { ...actor, user_id: id(2), name: "Developer" },
  created_at: "2099-01-01T00:00:00Z", result: "Delivered result", basis: "Verified artifact", attachment_ids: [], source_comment_id: null, evidence_state: "available", decision: null };
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, status: "in_review", status_category: "in_review", creator: actor, handler: actor, ai_policy: "manual",
  collaboration_revision: "5", write_context: { participant_epoch: "1", account_epoch: "1" }, capabilities: { ...collaborationIssueFixture.capabilities, complete: true },
  ordinary_children: { items: [{ id: id(3), identifier: "SRC-3", title: "Build child", can_open: true }], source_workspace_slug: "source" },
  delivery: { current: submission, latest: submission, has_history: true } })!;
const until = "2099-01-01T01:00:00Z", checked = "2099-01-01T00:00:00Z";
const candidate = { approval_id: id(4), execution_id: id(6), source_revision: "a".repeat(40), spec_digest: "b".repeat(64), artifact_digest: "c".repeat(64), receipt_digest: "d".repeat(64),
  created_at: checked, expires_at: until, summary: "Reviewed build", status: "succeeded", result_code: null, selectable: true };
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

function mount(behavior: "success" | "lost" | "stage_blocked" = "success", self = false, reference?: "available" | "denied") {
  const currentIssue = self ? { ...issue, status: "todo", status_category: "todo" as const, delivery: { current: null, latest: null, has_history: false } } : issue;
  const shownIssue = reference ? { ...currentIssue, ordinary_children: { ...currentIssue.ordinary_children!, items: [] } } : currentIssue;
  vi.spyOn(Date, "now").mockReturnValue(Date.parse(checked));
  const writes: { path: string; body: Record<string, unknown>; key: string | null }[] = [];
  let grant: Record<string, unknown> | null = reference ? { authorization_id: id(8), operation_id: id(9), issue_id: issue.id, issue_revision: 42,
    target_status: "done", contract_version: "delivery-completion-enforcement-v1", created_at: checked, expires_at: until,
    state: "issued", applicability: "not_checked", replayed: false } : null;
  let completed = false;
  const base = `/api/collaboration/issues/${issue.id}`;
  const fetch = vi.fn(async (url: string, init: RequestInit = {}) => {
    const path = new URL(url).pathname, body = init.body ? JSON.parse(String(init.body)) : null;
    const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
    if (path === `${base}/status-options`) return json({ action: "complete", status_options_version: "e".repeat(64), default_status: "done", options: [{ key: "done", name: "Done", category: "done" }] });
    if (path === `${base}/delivery-evidence/candidates`) return json({ protocol_version: 1, issue_id: issue.id, issue_revision: 42, source_issue_id: id(3), source_issue_revision: 17,
      project_id: id(7), checked_at: checked, items: [candidate], next_cursor: null, has_more: false });
    if (path === `${base}/delivery-evidence/completion-preview`) {
      const blocked = behavior === "stage_blocked";
      return json({ protocol_version: 1, issue_id: issue.id, issue_revision: 42, source_issue_id: id(3), source_issue_revision: 17,
        current_status: shownIssue.status, current_category: shownIssue.status_category, target_status: "done", target_category: "done", project_id: id(7), project_revision: 2, policy_mode: "enforce", policy_revision: 1,
        checked_at: checked, reason: blocked ? "stage_incomplete" : "authorization_available", max_expires_at: blocked ? null : until, enforced: false, usable_for_completion: false, skip_supported: false,
        stage_blockers: blocked ? [{ code: "stage_execution_fact_missing", stage: "verification" }] : [], stage_blockers_truncated: false,
        observation: { mode: "observe", policy_version: "delivery-artifact-observe-v1", verdict: "pass", reason: "artifact_matches", issue_id: id(3), issue_revision: 17,
          project_id: id(7), project_revision: 2, approval_id: candidate.approval_id, execution_id: candidate.execution_id, source_revision: candidate.source_revision,
          observed_head_sha: candidate.source_revision, revision_provenance: "database_observed_pr_head_not_remote_verified", spec_digest: candidate.spec_digest, artifact_digest: candidate.artifact_digest,
          receipt_digest: candidate.receipt_digest, producer_task_generation: 0, observed_at: checked, valid_until: until },
        authorization_binding: blocked ? null : { contract_version: "delivery-completion-enforcement-v1", source_workspace_id: issue.source_workspace.id, target: body.target,
          project: { id: id(7), expected_revision: 2, expected_policy_revision: 1 }, evidence: { issue_id: id(3), expected_issue_revision: 17, approval_id: candidate.approval_id,
            expected_source_revision: candidate.source_revision, expected_spec_digest: candidate.spec_digest, expected_artifact_digest: candidate.artifact_digest }, consumer: { kind: "human", user_id: id(1) } } });
    }
    if (init.method === "POST") {
      writes.push({ path, body, key: new Headers(init.headers).get("Idempotency-Key") });
      if (path === "/api/delivery-completion-authorizations") {
        grant = { authorization_id: id(8), operation_id: body.operation_id, issue_id: issue.id, issue_revision: 42, target_status: "done", contract_version: "delivery-completion-enforcement-v1",
          created_at: checked, expires_at: body.expires_at, state: "issued", applicability: "not_checked", replayed: false };
        return json(grant, 201);
      }
      if (path === `/api/delivery-completion-authorizations/${id(8)}/revocations`) {
        grant = { ...grant, state: "revoked", applicability: "revoked" }; return json(grant);
      }
      if (path === `${base}/lifecycle`) {
        if (behavior === "lost") throw new TypeError("Lost completion response");
        completed = true;
        return json({ code: "completed", collaboration_revision: "6", delivery_confirmation: { submission_id: submission.id, attachment_ids: [] } });
      }
    }
    if (path === `/api/delivery-completion-authorizations/${id(8)}` && reference === "denied") return json({ code: "delivery_completion_denied" }, 403);
    if (path === `/api/delivery-completion-authorizations/${id(8)}`) return json({ ...grant, ...(completed ? { state: "consumed", applicability: "consumed",
      completion_receipt: { authorization_id: id(8), operation_id: grant?.operation_id, issue_id: issue.id, completed_revision: 43, target_status: "done", completed_at: checked,
        collaboration_revision: "6", submission_id: submission.id, replayed: false } } : {}) });
    if (path.startsWith(`${base}/operations/`)) return json({ code: "not_found" }, 404);
    if (path === base) return json(shownIssue);
    throw new Error(`Unexpected completion test request ${path}`);
  });
  vi.stubGlobal("fetch", fetch); setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(id(1))} issue={shownIssue} /></QueryClientProvider>);
  return { writes, fetch };
}
async function review(self = false) {
  const title = self ? "Record result and complete" : "Accept and complete";
  await userEvent.click(await screen.findByRole("button", { name: title }));
  const form = await screen.findByRole("dialog", { name: title });
  if (self) { await userEvent.type(within(form).getByRole("textbox", { name: "Result" }), "Original self result"); await userEvent.type(within(form).getByRole("textbox", { name: "Basis" }), "Original self checks"); }
  await userEvent.click(within(form).getByRole("checkbox", { name: "Use completion authorization" }));
  await userEvent.selectOptions(within(form).getByRole("combobox", { name: "Source sub-issue" }), id(3));
  await userEvent.selectOptions(await within(form).findByRole("combobox", { name: "Artifact evidence" }), id(4));
  return form;
}
async function authorize(self = false) {
  const form = await review(self);
  await userEvent.type(within(form).getByRole("textbox", { name: "Authorization reason" }), "I reviewed this exact artifact and result");
  await userEvent.click(await within(form).findByRole("button", { name: "Use latest expiry" }));
  await userEvent.click(within(form).getByRole("button", { name: "Authorize exact completion" }));
  expect(await within(form).findByText("The selected authorization is ready for this completion.")).toBeVisible();
  return form;
}
it("authorizes one exact child artifact and completes through the original shared workflow and operation key", async () => {
  const { writes, fetch } = mount(); const form = await authorize();
  expect(writes).toHaveLength(1);
  await userEvent.click(within(form).getByRole("button", { name: "Accept and complete" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes).toHaveLength(2);
  expect(writes[0]?.body).toMatchObject({ target: { kind: "collaboration_submission", issue_id: issue.id, expected_issue_revision: 42, submission_id: submission.id }, evidence: { issue_id: id(3), expected_issue_revision: 17 } });
  expect(writes[1]?.key).toBe(writes[0]?.body.operation_id);
  expect(writes[1]?.body).toMatchObject({ action: "complete", submission_id: submission.id, completion: { authorization_id: id(8), operation_id: writes[0]?.body.operation_id, expected_issue_revision: 42 } });
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname === `/api/delivery-completion-authorizations/${id(8)}`)).toBe(true);
});
it("keeps a lost completion pending after original GET404 without another authorization or completion POST", async () => {
  const { writes } = mount("lost"); const form = await authorize();
  await userEvent.click(within(form).getByRole("button", { name: "Accept and complete" }));
  await waitFor(() => expect(writes).toHaveLength(2));
  await userEvent.click(await screen.findByRole("button", { name: "Check original result" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Check original result" })).toBeEnabled());
  expect(writes).toHaveLength(2);
});
it("does not authorize or complete from a stage-blocked M1 preview", async () => {
  const { writes } = mount("stage_blocked"); const form = await review();
  await waitFor(() => expect(within(form).getByRole("button", { name: "Authorize exact completion" })).toBeDisabled());
  expect(within(form).getByRole("button", { name: "Accept and complete" })).toBeDisabled();
  expect(writes).toEqual([]);
});

it("binds self result and basis to the same explicit authorization and original completion", async () => {
  const { writes } = mount("success", true); const form = await authorize(true);
  expect(writes[0]?.body).toMatchObject({ target: { kind: "collaboration_self", result: "Original self result", basis: "Original self checks", draft_id: null, attachment_ids: [], source_comment_id: null } });
  await userEvent.click(within(form).getByRole("button", { name: "Record result and complete" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes).toHaveLength(2);
  expect(writes[1]?.key).toBe(writes[0]?.body.operation_id);
  expect(writes[1]?.body).toMatchObject({ note: "Original self result", basis: "Original self checks", completion: { authorization_id: id(8) } });
});

it("uses a new explicit operation after revocation and a new authorization review", async () => {
  const { writes } = mount(); const form = await authorize();
  await userEvent.type(within(form).getByRole("textbox", { name: "Authorization revocation reason" }), "Review again");
  await userEvent.click(within(form).getByRole("button", { name: "Revoke completion authorization" }));
  await userEvent.click(await within(form).findByRole("button", { name: "Start a new authorization review" }));
  await userEvent.click(within(form).getByRole("checkbox", { name: "Use completion authorization" }));
  await waitFor(() => expect(within(form).getByRole("button", { name: "Authorize exact completion" })).toBeEnabled());
  await userEvent.click(within(form).getByRole("button", { name: "Authorize exact completion" }));
  await waitFor(() => expect(writes.filter((write) => write.path === "/api/delivery-completion-authorizations")).toHaveLength(2));
  const grants = writes.filter((write) => write.path === "/api/delivery-completion-authorizations");
  expect(grants[0]?.body.operation_id).not.toBe(grants[1]?.body.operation_id);
  expect(writes.filter((write) => write.path.endsWith("/lifecycle"))).toEqual([]);
});

it.each(["available", "denied"] as const)("reads an externally issued authorization reference without artifact reads: %s", async (reference) => {
  const { writes, fetch } = mount("success", false, reference);
  await userEvent.click(await screen.findByRole("button", { name: "Accept and complete" }));
  const form = await screen.findByRole("dialog", { name: "Accept and complete" });
  await userEvent.click(within(form).getByRole("checkbox", { name: "Use completion authorization" }));
  await userEvent.selectOptions(within(form).getByRole("combobox", { name: "Authorization method" }), "existing");
  await userEvent.type(within(form).getByRole("textbox", { name: "Authorization ID" }), id(8));
  await userEvent.click(within(form).getByRole("button", { name: "Read authorization" }));
  if (reference === "available") {
    expect(await within(form).findByText("The selected authorization is ready for this completion.")).toBeVisible();
    await userEvent.click(within(form).getByRole("button", { name: "Accept and complete" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(writes).toEqual([expect.objectContaining({ path: `/api/collaboration/issues/${issue.id}/lifecycle`, key: id(9),
      body: expect.objectContaining({ completion: { authorization_id: id(8), operation_id: id(9), expected_issue_revision: 42 } }) })]);
    expect(fetch.mock.calls.filter(([url]) => new URL(url).pathname === `/api/delivery-completion-authorizations/${id(8)}`)).toHaveLength(2);
  } else {
    expect(await within(form).findByText("Completion evidence or authorization is unavailable. Refresh current evidence or query the original request.")).toBeVisible();
    expect(within(form).getByRole("button", { name: "Accept and complete" })).toBeDisabled();
    expect(writes).toEqual([]);
  }
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname.includes("/delivery-evidence/"))).toBe(false);
});
