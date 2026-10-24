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
import { AdminCollaborationEntry, AdminCollaborationPage } from "./collaboration-page";

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
afterEach(() => { cleanup(); client.clear(); vi.restoreAllMocks(); });
function content(issueId: string | null = detail.issueId, entry = false) {
  return <QueryClientProvider client={client}><I18nProvider locale="en" resources={{ en: { admin: en } }}>
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
