// Query, identity and original-key matrices live in core/admin/collaboration-queries.test.ts.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "@multica/core/api";
import { EMPTY_USER } from "@multica/core/api/schemas";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { I18nProvider } from "@multica/core/i18n/react";
import { adminApiScope, adminKeys, adminMeOptions, configureAdminControlStorage, readAdminCollaborationDraft, saveAdminControlDraft,
  type AdminCollaborationChange, type AdminIdentity } from "@multica/core/admin";
import { adminCollaborationCandidatesFixture as candidates, adminCollaborationDetailFixture as detail,
  adminCollaborationInputFixture as original, adminCollaborationResultFixture as receipt, adminCollaborationTestId as id } from "@multica/core/admin/test-fixtures";
import type { AnchorHTMLAttributes } from "react";
import en from "../../locales/en/admin.json";
import { RESOURCES } from "../../test/i18n";
import { AdminCollaborationEntry, AdminCollaborationPage } from "./collaboration-page";
import { CollaborationCompletionPreviewSchema } from "@multica/core/collaboration";
import { clearCompletionDrafts, type CompletionAuthorizationRequest } from "@multica/core/issues";

vi.mock("../../navigation", () => ({ AppLink: (props: AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} /> }));
const user = { ...EMPTY_USER, id: id(1), name: "Administrator" };
const identity: AdminIdentity = { userId: user.id, organizationId: id(2), role: "super_admin", supported: true, allowedActions: [] };
const baseUrl = "https://s5.example.test";
const storageScope = { server: baseUrl, userId: user.id, organizationId: identity.organizationId };
let api: ApiClient;
let store: ReturnType<typeof createAuthStore>;
let client: QueryClient;
let values: Map<string, string>;
const hrefForIssue = (issueId: string | null) => issueId ? `/admin/collaboration?issue=${issueId}` : "/admin/collaboration";
beforeEach(() => {
  api = new ApiClient(baseUrl); setApiInstance(api);
  store = createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(store); store.setState({ user, status: "authenticated", isLoading: false });
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  values = new Map();
  configureAdminControlStorage({ getItem: key => values.get(key) ?? null, setItem: (key, value) => { values.set(key, value); },
    removeItem: key => { values.delete(key); }, keys: () => [...values.keys()] });
  vi.spyOn(api, "getAdminMe").mockResolvedValue(identity);
  vi.spyOn(api, "getAdminCollaborationIssue").mockResolvedValue(detail);
  vi.spyOn(api, "getAdminCollaborationIssues").mockResolvedValue({ scope: id(2), items: [detail], nextCursor: null });
  vi.spyOn(api, "getAdminCollaborationCandidates").mockResolvedValue(candidates);
  vi.spyOn(api, "getAdminCollaborationOperation").mockRejectedValue(new ApiError("No result", 404, "Not found", { code: "admin_not_found" }));
  vi.spyOn(api, "changeAdminCollaborationLifecycle").mockImplementation(async (_issueId, body, key) => ({ ...receipt,
    result: { ...receipt.result, requestId: key, status: body.targetStatus, handlerUserId: body.successorUserId ?? receipt.result.previousHandlerUserId } }));
});
afterEach(() => { cleanup(); client.clear(); clearCompletionDrafts(); vi.restoreAllMocks(); });
function content(issueId: string | null = detail.issueId, entry = false, locale: "en" | "zh-Hans" = "en") {
  return <QueryClientProvider client={client}><I18nProvider locale={locale} resources={RESOURCES}>
    {entry ? <AdminCollaborationEntry href="/account?view=current&administration=collaboration" /> : <AdminCollaborationPage issueId={issueId} hrefForIssue={hrefForIssue} />}
  </I18nProvider></QueryClientProvider>;
}
async function open() {
  await userEvent.click(await screen.findByRole("button", { name: "Reopen task" }));
  await screen.findByRole("radio", { name: /First participant/ });
}
async function fillAndSubmit() {
  await open();
  await userEvent.click(screen.getByRole("radio", { name: /First participant/ }));
  await userEvent.type(screen.getByRole("textbox", { name: "Reason" }), "Restore responsibility");
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
}

it("reveals the entry only after current complete super-admin access", async () => {
  let resolve!: (value: AdminIdentity) => void;
  vi.mocked(api.getAdminMe).mockImplementationOnce(() => new Promise(ready => { resolve = ready; }));
  render(content(null, true));
  expect(screen.queryByRole("link", { name: "Manage collaborations" })).not.toBeInTheDocument();
  await act(async () => resolve(identity));
  expect(await screen.findByRole("link", { name: "Manage collaborations" })).toHaveAttribute("href", "/account?view=current&administration=collaboration");
  await act(async () => { client.setQueryData(adminKeys.me({ apiScope: adminApiScope(), userId: user.id, organizationId: null }), { ...identity, role: "platform_observer" }); });
  await waitFor(() => expect(screen.queryByRole("link")).not.toBeInTheDocument());
});

it.each(["en", "zh-Hans"] as const)("localizes status in the %s metadata list and detail without ordinary issue reads", async locale => {
  vi.mocked(api.getAdminCollaborationIssues).mockResolvedValue({ scope: id(2), items: [{ ...detail, status: "in_progress" }], nextCursor: null });
  vi.mocked(api.getAdminCollaborationIssue).mockResolvedValue({ ...detail, status: "in_progress" });
  const ordinaryRead = vi.spyOn(api, "getCollaborationIssue");
  const view = render(content(null, false, locale));
  const label = locale === "en" ? "In progress" : "进行中";
  expect(await screen.findByText(label)).toBeVisible();
  expect(screen.queryByText("in_progress")).not.toBeInTheDocument();
  view.rerender(content(detail.issueId, false, locale));
  await screen.findByRole("heading", { name: detail.identifier });
  expect(screen.getByText(label)).toBeVisible();
  expect(ordinaryRead).not.toHaveBeenCalled();
});

it("self-guards the page against observers without reading any collaboration metadata", async () => {
  vi.mocked(api.getAdminMe).mockResolvedValue({ ...identity, role: "platform_observer" });
  render(content());
  expect(await screen.findByText(en.collaboration.accessRequired)).toBeVisible();
  expect(api.getAdminCollaborationIssue).not.toHaveBeenCalled();
  expect(api.getAdminCollaborationCandidates).not.toHaveBeenCalled();
  expect(screen.queryByText(detail.identifier)).not.toBeInTheDocument();
});

it("keeps every minimal list page reachable through the injected route builder", async () => {
  vi.mocked(api.getAdminCollaborationIssues).mockResolvedValueOnce({ scope: id(2), items: [detail], nextCursor: detail.issueId })
    .mockResolvedValueOnce({ scope: id(2), items: [{ ...detail, issueId: id(30), identifier: "SOURCE-99" }], nextCursor: null });
  render(content(null));
  expect(await screen.findByRole("link", { name: detail.identifier })).toHaveAttribute("href", hrefForIssue(detail.issueId));
  await userEvent.click(screen.getByRole("button", { name: "Load more collaborations" }));
  expect(await screen.findByRole("link", { name: "SOURCE-99" })).toHaveAttribute("href", hrefForIssue(id(30)));
  expect(screen.getAllByText(detail.sourceName)).toHaveLength(2);
  expect(api.getAdminCollaborationIssues).toHaveBeenLastCalledWith({ limit: 50, cursor: detail.issueId }, { signal: expect.any(AbortSignal) });
});

it("requires a reason and an explicit reachable successor, then records the frozen lifecycle request", async () => {
  vi.mocked(api.getAdminCollaborationCandidates).mockResolvedValueOnce({ ...candidates, nextCursor: id(7) })
    .mockResolvedValueOnce({ ...candidates, items: [{ userId: id(10), name: "Later participant", avatarUrl: "", external: false }] });
  render(content());
  await open();
  expect(screen.getByRole("radio", { name: /First participant/ })).not.toBeChecked();
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
  expect(screen.getByRole("textbox", { name: "Reason" })).toHaveFocus();
  expect(screen.getByText(en.collaboration.reasonRequired)).toBeVisible();
  await userEvent.type(screen.getByRole("textbox", { name: "Reason" }), "Restore responsibility");
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
  expect(screen.getByRole("group", { name: "New handler" })).toHaveFocus();
  await userEvent.click(screen.getByRole("button", { name: "Load more participants" }));
  await userEvent.click(await screen.findByRole("radio", { name: "Later participant" }));
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "New task status" }), "in_progress");
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
  const result = await screen.findByRole("region", { name: "Recorded administration change" });
  expect(within(result).getByText("Task reopened with execution paused")).toBeVisible();
  expect(within(result).getByText(en.collaboration.lifecycleNotice)).toBeVisible();
  expect(api.changeAdminCollaborationLifecycle).toHaveBeenCalledExactlyOnceWith(detail.issueId, {
    ...original.body, targetStatus: "in_progress", successorUserId: id(10),
  }, expect.any(String));
  expect(readAdminCollaborationDraft(storageScope, detail.issueId)?.input).toMatchObject({ creatorUserId: detail.creator.userId,
    handlerUserId: detail.handler.userId, body: { successorUserId: id(10), expectedCollaborationRevision: "7" } });
  expect(screen.queryByRole("textbox", { name: "Reason" })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Close receipt" }));
  expect(readAdminCollaborationDraft(storageScope, detail.issueId)).toBeNull();
});

it("freezes a refused confirmation until the user explicitly closes it", async () => {
  vi.mocked(api.changeAdminCollaborationLifecycle).mockRejectedValueOnce(new ApiError("Successor changed", 409, "Conflict", { code: "successor_unavailable" }));
  render(content()); await fillAndSubmit();
  expect(await screen.findByText(en.collaboration.rejected)).toBeVisible();
  expect(screen.queryByRole("textbox", { name: "Reason" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Retry original request" })).not.toBeInTheDocument();
  expect(api.getAdminCollaborationOperation).not.toHaveBeenCalled();
  expect(readAdminCollaborationDraft(storageScope, detail.issueId)).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Close confirmation" }));
  await open();
  expect(screen.getByRole("radio", { name: /First participant/ })).not.toBeChecked();
  expect(api.changeAdminCollaborationLifecycle).toHaveBeenCalledTimes(1);
});

it("keeps an overlong multibyte reason editable before any write", async () => {
  render(content()); await open();
  await userEvent.click(screen.getByRole("radio", { name: /First participant/ }));
  const reason = "协".repeat(334);
  fireEvent.change(screen.getByRole("textbox", { name: "Reason" }), { target: { value: reason } });
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
  expect(api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
  expect(await screen.findByText(en.collaboration.reasonInvalid)).toBeVisible();
  expect(screen.getByRole("textbox", { name: "Reason" })).toHaveValue(reason);
  expect(screen.getByRole("textbox", { name: "Reason" })).toHaveFocus();
  expect(readAdminCollaborationDraft(storageScope, detail.issueId)).toBeNull();
});

it("recovers a lost response after remount even when the creator is enabled and detail reads refuse", async () => {
  vi.mocked(api.changeAdminCollaborationLifecycle).mockRejectedValueOnce(new TypeError("Lost response"));
  const view = render(content());
  await fillAndSubmit();
  await waitFor(() => expect(api.getAdminCollaborationOperation).toHaveBeenCalledTimes(1));
  const stored = readAdminCollaborationDraft(storageScope, detail.issueId)!.input;
  expect(screen.getByRole("region", { name: "Original request" })).toBeVisible();
  view.unmount();
  vi.mocked(api.getAdminCollaborationIssue).mockRejectedValue(new ApiError("Creator restored", 409, "Conflict", { code: "creator_not_disabled" }));
  vi.mocked(api.getAdminCollaborationOperation).mockResolvedValue({ ...receipt, result: { ...receipt.result, requestId: stored.key } });
  render(content());
  expect(await screen.findByRole("region", { name: "Recorded administration change" })).toBeVisible();
  expect(await screen.findByText(en.collaboration.creatorRestored)).toBeVisible();
  expect(api.changeAdminCollaborationLifecycle).toHaveBeenCalledTimes(1);
  expect(api.getAdminCollaborationOperation).toHaveBeenLastCalledWith(detail.issueId, stored.key, { signal: expect.any(AbortSignal) });
  expect(screen.queryByRole("button", { name: "Reopen task" })).not.toBeInTheDocument();
});

it("keeps an empty lookup read-only and retries only the original payload on an explicit click", async () => {
  saveAdminControlDraft(storageScope, original);
  render(content());
  expect(await screen.findByText(en.collaboration.notRecorded)).toBeVisible();
  expect(api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Refresh task details" }));
  expect(api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  expect(await screen.findByRole("region", { name: "Recorded administration change" })).toBeVisible();
  expect(api.changeAdminCollaborationLifecycle).toHaveBeenCalledExactlyOnceWith(original.id, original.body, original.key);
});

it("keeps the opened revision and returns focus after stale details force a new confirmation", async () => {
  render(content()); await open();
  await userEvent.type(screen.getByRole("textbox", { name: "Reason" }), "Original reason");
  vi.mocked(api.getAdminCollaborationIssue).mockResolvedValue({ ...detail, collaborationRevision: "8" });
  await userEvent.click(screen.getByRole("button", { name: "Refresh task details" }));
  expect(await screen.findByText(en.collaboration.snapshotChanged)).toBeVisible();
  expect(screen.getByRole("textbox", { name: "Reason" })).toHaveValue("Original reason");
  expect(screen.getByRole("button", { name: "Confirm change" })).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "Close confirmation" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Reopen task" })).toHaveFocus());
  expect(api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
});

it("clears an expired successor and never auto-selects them when a later refresh restores eligibility", async () => {
  render(content()); await open();
  await userEvent.click(screen.getByRole("radio", { name: /First participant/ }));
  vi.mocked(api.getAdminCollaborationCandidates).mockResolvedValue({ ...candidates, items: [] });
  await userEvent.click(screen.getByRole("button", { name: "Refresh participants" }));
  expect(await screen.findByText(en.collaboration.successorExpired)).toBeVisible();
  vi.mocked(api.getAdminCollaborationCandidates).mockResolvedValue(candidates);
  await userEvent.click(screen.getByRole("button", { name: "Refresh participants" }));
  expect(await screen.findByRole("radio", { name: /First participant/ })).not.toBeChecked();
  expect(api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
});

it("removes current metadata and the open form on a protected read denial", async () => {
  render(content()); await open();
  await userEvent.type(screen.getByRole("textbox", { name: "Reason" }), "Private form input");
  vi.mocked(api.getAdminCollaborationIssue).mockRejectedValue(new ApiError("No scope", 403, "Forbidden", { code: "admin_scope_forbidden" }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh task details" }));
  expect(await screen.findByText(en.collaboration.accessRequired)).toBeVisible();
  expect(screen.queryByText(detail.identifier)).not.toBeInTheDocument();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
  expect(client.getQueryCache().getAll().some(query => query.queryKey[4] === "collaboration-issue" && query.state.data)).toBe(false);
});

it("drops open actions immediately when the current administrator becomes an observer", async () => {
  render(content()); await open();
  vi.mocked(api.getAdminMe).mockResolvedValue({ ...identity, role: "platform_observer" });
  await act(async () => { await client.fetchQuery(adminMeOptions({ apiScope: adminApiScope(), userId: user.id, organizationId: null })); });
  expect(await screen.findByText(en.collaboration.accessRequired)).toBeVisible();
  expect(screen.queryByRole("radio")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Confirm change" })).not.toBeInTheDocument();
});

it("does not restore another administrator's pending request after an actor switch", async () => {
  saveAdminControlDraft(storageScope, original);
  render(content()); await screen.findByText(en.collaboration.notRecorded);
  vi.mocked(api.getAdminMe).mockResolvedValue({ ...identity, userId: id(20) });
  await act(async () => { store.setState({ user: { ...user, id: id(20) } }); });
  await screen.findByRole("button", { name: "Reopen task" });
  expect(screen.queryByRole("region", { name: "Original request" })).not.toBeInTheDocument();
  expect(api.getAdminCollaborationOperation).toHaveBeenCalledTimes(1);
  expect(api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
  expect(readAdminCollaborationDraft(storageScope, original.id)?.input).toEqual(original);
});

it.each(["complete", "cancel"] as const)("does not add successor or ordinary execution controls to a %s action", async action => {
  const targetStatus = action === "complete" ? "done" : "cancelled";
  const category = action === "complete" ? "done" : "cancelled";
  vi.mocked(api.getAdminCollaborationIssue).mockResolvedValue({ ...detail, status: "todo", statusCategory: "todo", queueState: "paused", successorRequired: false,
    actions: [{ action, default_status: targetStatus, status_options_version: "a".repeat(64), options: [{ key: targetStatus, name: targetStatus, category }] }] });
  let sent: AdminCollaborationChange | null = null;
  vi.mocked(api.changeAdminCollaborationLifecycle).mockImplementationOnce(async (_issueId, body, key) => {
    sent = body;
    return { ...receipt, operation: { ...receipt.operation, kind: `collaboration.${action}`, resultCode: action === "complete" ? "completed" : "cancelled" },
      result: { ...receipt.result, requestId: key, action, code: action === "complete" ? "completed" : "cancelled", previousStatus: "todo", status: targetStatus,
        handlerUserId: detail.handler.userId, queueState: "closed" } };
  });
  render(content());
  await userEvent.click(await screen.findByRole("button", { name: en.collaboration.actions[action] }));
  expect(screen.queryByRole("group", { name: "New handler" })).not.toBeInTheDocument();
  await userEvent.type(screen.getByRole("textbox", { name: "Reason" }), "Administrative closure");
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
  await screen.findByRole("region", { name: "Recorded administration change" });
  expect(sent).not.toHaveProperty("successorUserId");
  expect(api.getAdminCollaborationCandidates).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: /Resume|Stop AI/ })).not.toBeInTheDocument();
});

it("authorizes the exact R38 target and consumes it through the existing administrative workflow", async () => {
  const at = "2099-01-01T00:00:00Z", until = "2099-01-01T01:00:00Z";
  vi.spyOn(Date, "now").mockReturnValue(Date.parse(at));
  vi.mocked(api.getAdminCollaborationIssue).mockResolvedValue({ ...detail, status: "todo", statusCategory: "todo", queueState: "paused", successorRequired: false,
    actions: [{ action: "complete", default_status: "done", status_options_version: "a".repeat(64), options: [{ key: "done", name: "Done", category: "done" }] }] });
  const evidence = { issue_id: id(30), expected_issue_revision: 17, approval_id: id(31), expected_source_revision: "a".repeat(40), expected_spec_digest: "b".repeat(64), expected_artifact_digest: "c".repeat(64) };
  vi.spyOn(api, "listCollaborationCompletionCandidates").mockResolvedValue({ state: "available", value: { protocol_version: 1, issue_id: detail.issueId, issue_revision: 42,
    source_issue_id: evidence.issue_id, source_issue_revision: 17, project_id: id(32), checked_at: at, next_cursor: null, has_more: false,
    target_context: { expected_lifecycle_epoch: "2", submission_id: null }, items: [{ approval_id: evidence.approval_id, execution_id: id(33), source_revision: evidence.expected_source_revision,
      spec_digest: evidence.expected_spec_digest, artifact_digest: evidence.expected_artifact_digest, receipt_digest: "d".repeat(64), created_at: at, expires_at: until, summary: "Admin source artifact", status: "succeeded", result_code: null, selectable: true }] } });
  vi.spyOn(api, "previewCollaborationCompletion").mockImplementation(async (_issueId, input) => ({ state: "available", value: CollaborationCompletionPreviewSchema.parse({
    protocol_version: 1, issue_id: detail.issueId, issue_revision: 42, source_issue_id: evidence.issue_id, source_issue_revision: 17,
    current_status: "todo", current_category: "todo", target_status: "done", target_category: "done", project_id: id(32), project_revision: 2, policy_mode: "enforce", policy_revision: 1,
    checked_at: at, reason: "authorization_available", max_expires_at: until, enforced: false, usable_for_completion: false, skip_supported: false, stage_blockers: [], stage_blockers_truncated: false,
    observation: { mode: "observe", policy_version: "delivery-artifact-observe-v1", verdict: "pass", reason: "artifact_matches", issue_id: evidence.issue_id, issue_revision: 17,
      project_id: id(32), project_revision: 2, approval_id: evidence.approval_id, source_revision: evidence.expected_source_revision, observed_head_sha: evidence.expected_source_revision,
      revision_provenance: "database_observed_pr_head_not_remote_verified", spec_digest: evidence.expected_spec_digest, artifact_digest: evidence.expected_artifact_digest, producer_task_generation: 0, observed_at: at, valid_until: until },
    authorization_binding: { contract_version: "delivery-completion-enforcement-v1", source_workspace_id: detail.sourceWorkspaceId, target: input.target,
      project: { id: id(32), expected_revision: 2, expected_policy_revision: 1 }, evidence, consumer: { kind: "admin", user_id: user.id, organization_id: id(2) } } }) }));
  let request: CompletionAuthorizationRequest | null = null, consumed = false;
  const grant = (value: CompletionAuthorizationRequest) => ({ authorization_id: id(34), operation_id: value.operation_id, issue_id: detail.issueId, issue_revision: 42, target_status: "done",
    contract_version: "delivery-completion-enforcement-v1" as const, created_at: at, expires_at: value.expires_at, state: "issued" as const, applicability: "not_checked" as const, replayed: false });
  vi.spyOn(api, "createDeliveryCompletionAuthorization").mockImplementation(async (_key, value) => { request = value; return grant(value); });
  vi.spyOn(api, "getDeliveryCompletionAuthorization").mockImplementation(async () => {
    if (!request) throw new Error("No authorization");
    return consumed ? { ...grant(request), state: "consumed", applicability: "consumed", completion_receipt: { authorization_id: id(34), operation_id: request.operation_id,
      issue_id: detail.issueId, completed_revision: 43, target_status: "done", completed_at: at, collaboration_revision: "8", replayed: false } } : grant(request);
  });
  vi.mocked(api.changeAdminCollaborationLifecycle).mockImplementation(async (_issueId, _body, key) => {
    consumed = true;
    return { ...receipt, operation: { ...receipt.operation, kind: "collaboration.complete", resultCode: "completed" },
      result: { ...receipt.result, requestId: key, action: "complete", code: "completed", collaborationRevision: "8", previousStatus: "todo", status: "done", handlerUserId: detail.handler.userId, queueState: "closed" } };
  });
  render(content());
  await userEvent.click(await screen.findByRole("button", { name: "Complete task" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Reason" }), "Exact administrative closure");
  await userEvent.click(screen.getByRole("checkbox", { name: "Use completion authorization" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Source sub-issue" }), evidence.issue_id);
  await userEvent.selectOptions(await screen.findByRole("combobox", { name: "Artifact evidence" }), evidence.approval_id);
  await userEvent.type(screen.getByRole("textbox", { name: "Authorization reason" }), "I reviewed the source artifact");
  await userEvent.click(await screen.findByRole("button", { name: "Use latest expiry" }));
  await userEvent.click(screen.getByRole("button", { name: "Authorize exact completion" }));
  await screen.findByText("The selected authorization is ready for this completion.");
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
  await screen.findByRole("region", { name: "Recorded administration change" });
  expect(api.createDeliveryCompletionAuthorization).toHaveBeenCalledOnce();
  const saved = vi.mocked(api.createDeliveryCompletionAuthorization).mock.calls[0]![1];
  expect(saved.target).toMatchObject({ kind: "collaboration_admin", expected_lifecycle_epoch: "2", submission_id: null, administration_reason: "Exact administrative closure" });
  expect(api.changeAdminCollaborationLifecycle).toHaveBeenCalledWith(detail.issueId, expect.objectContaining({ completion: { authorization_id: id(34), operation_id: saved.operation_id, expected_issue_revision: 42 } }), saved.operation_id);
  expect(api.getDeliveryCompletionAuthorization).toHaveBeenCalledWith(id(34), { workspaceId: detail.sourceWorkspaceId });
});

it("S2 renders an R38 stage refusal without a completed receipt or loss of the entered reason", async () => {
  vi.mocked(api.getAdminCollaborationIssue).mockResolvedValue({ ...detail, status: "in_progress", statusCategory: "in_progress", queueState: "paused", successorRequired: false,
    actions: [{ action: "complete", status_options_version: "a".repeat(64), default_status: "done", options: [{ key: "done", name: "Done", category: "done" }] }] });
  vi.mocked(api.changeAdminCollaborationLifecycle).mockRejectedValueOnce(new ApiError("PRIVATE raw error", 409, "Conflict", {
    code: "stage_incomplete", error: "PRIVATE raw error", request_id: "transport-trace", stage_blockers: [{ code: "stage_ci_missing", stage: "code_review" }], stage_blockers_truncated: false }));
  render(content());await userEvent.click(await screen.findByRole("button", { name: "Complete task" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Reason" }), "Administrator completion reason");
  await userEvent.click(screen.getByRole("button", { name: "Confirm change" }));
  expect(await screen.findByRole("heading", { name: "Change refused" })).toBeVisible();
  expect(screen.getByText(/CI evidence is missing for this code version/)).toBeVisible();
  expect(screen.getByText(/HTTP 409.*stage_incomplete/)).toBeVisible();
  expect(screen.getByText("Administrator completion reason", { selector: "dd" })).toBeVisible();
  expect(screen.queryByText("PRIVATE raw error")).not.toBeInTheDocument();
  expect(api.getAdminCollaborationOperation).not.toHaveBeenCalled();
});
it("S2 a rejected retry stays visibly unconfirmed and preserves original administration recovery", async () => {
  const { successorUserId: _successor, ...body } = original.body;
  const input = { ...original, body: { ...body, action: "complete" as const, expectedStatus: "in_progress", targetStatus: "done" } };
  saveAdminControlDraft(storageScope, input);
  vi.mocked(api.changeAdminCollaborationLifecycle).mockRejectedValueOnce(new ApiError("Refused", 409, "Conflict", { code: "stage_incomplete", error: "Refused", request_id: "trace",
    stage_blockers: [{ code: "stage_qa_missing", stage: "qa" }], stage_blockers_truncated: false }));
  render(content());
  const retry = await screen.findByRole("button", { name: "Retry original request" });await waitFor(() => expect(retry).toBeEnabled());
  await userEvent.click(retry);
  expect(await screen.findByText("This attempt was refused, but the original request is still unconfirmed. Check its original result.")).toBeVisible();
  expect(screen.getByRole("button", { name: "Check original result" })).toBeEnabled();
  expect(screen.queryByRole("heading", { name: "Change refused" })).not.toBeInTheDocument();
  expect(readAdminCollaborationDraft(storageScope, input.id)?.input).toEqual(input);
  expect(api.changeAdminCollaborationLifecycle).toHaveBeenCalledExactlyOnceWith(input.id, input.body, input.key);
});
