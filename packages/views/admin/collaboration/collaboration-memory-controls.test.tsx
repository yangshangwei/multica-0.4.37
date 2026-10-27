// Schema and request identity matrices belong to core/admin and core/collaboration tests.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "@multica/core/api";
import { EMPTY_USER } from "@multica/core/api/schemas";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { adminApiScope, configureAdminControlStorage, parseAdminCollaborationMemoryResult, readAdminCollaborationMemoryDraft,
  type AdminScope } from "@multica/core/admin";
import { adminCollaborationDetailFixture as detail, adminCollaborationTestId as id } from "@multica/core/admin/test-fixtures";
import { renderWithI18n } from "../../test/i18n";
import { AdminCollaborationMemoryControls } from "./collaboration-memory-controls";

let api: ApiClient;
let client: QueryClient;
let scope: AdminScope;
const baseUrl = "https://admin-memory.example";
const storageScope = { server: baseUrl, userId: id(1), organizationId: detail.scope };
const metadata = ["2", "1"].map(version => ({ version, bytes: 12, author: null, created_at: "2026-10-26T00:00:00Z", restored_from_version: null,
  purged: false, purged_at: null, purged_by: null, purged_by_admin: false }));
beforeEach(() => {
  api = new ApiClient(baseUrl); setApiInstance(api);
  const store = createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(store); store.setState({ user: { ...EMPTY_USER, id: id(1), name: "Administrator" }, status: "authenticated", isLoading: false });
  scope = { apiScope: adminApiScope(), userId: id(1), organizationId: detail.scope };
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const values = new Map<string, string>();
  configureAdminControlStorage({ getItem: key => values.get(key) ?? null, setItem: (key, value) => { values.set(key, value); },
    removeItem: key => { values.delete(key); }, keys: () => [...values.keys()] });
  vi.spyOn(api, "getAdminCollaborationMemoryVersions").mockResolvedValue({ scope: detail.scope, issueId: detail.issueId, currentVersion: "2", items: metadata, nextCursor: null });
  vi.spyOn(api, "getAdminCollaborationMemoryOperation").mockRejectedValue(new ApiError("Missing", 404, "Not found", { code: "admin_not_found" }));
});
afterEach(() => { client.clear(); vi.restoreAllMocks(); });
const result = (requestId: string) => parseAdminCollaborationMemoryResult({
  operation: { id: id(20), organization_id: detail.scope, target_id: detail.issueId, actor_id: id(1), target_kind: "collaboration_issue",
    kind: "collaboration.purge_memory", state: "applied", result_code: "memory_purged", confirmation: "not_required", reconciliation_state: "complete", version: 1,
    applied_at: "2026-10-26T00:00:00Z" },
  result: { issue_id: detail.issueId, request_id: requestId, action: "purge_memory", code: "memory_purged", version: "1", current_version: "2", bytes: 12,
    creator_user_id: detail.creator.userId, purged_at: "2026-10-26T00:00:00Z" },
})!;
function content(current: typeof detail | null = detail) {
  return <QueryClientProvider client={client}><AdminCollaborationMemoryControls scope={scope} issueId={detail.issueId} detail={current} refreshing={false} /></QueryClientProvider>;
}

it("uses current disabled-creator detail to permit historical purge with a validated reason", async () => {
  const purge = vi.spyOn(api, "purgeAdminCollaborationMemoryVersion").mockImplementation(async (_id, _body, key) => result(key));
  renderWithI18n(content());
  expect(await screen.findByRole("button", { name: "Purge v1" })).toBeEnabled();
  expect(screen.queryByRole("button", { name: "Purge v2" })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Purge v1" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Purge reason" }), { target: { value: "界".repeat(334) } });
  await userEvent.click(screen.getByRole("button", { name: "Purge text" }));
  expect(await screen.findByText("The reason is too long or contains unsupported characters. Edit it and try again.")).toBeVisible();
  expect(purge).not.toHaveBeenCalled();
  fireEvent.change(screen.getByRole("textbox", { name: "Purge reason" }), { target: { value: "Reported credential" } });
  await userEvent.click(screen.getByRole("button", { name: "Purge text" }));
  expect(await screen.findByRole("region", { name: "Recorded shared context purge" })).toBeVisible();
  expect(purge).toHaveBeenCalledWith(detail.issueId, { version: "1", reason: "Reported credential" }, expect.any(String));
});

it("restores an uncertain purge and retries the unchanged original even without current detail", async () => {
  const purge = vi.spyOn(api, "purgeAdminCollaborationMemoryVersion").mockRejectedValueOnce(new TypeError("Response lost"))
    .mockImplementationOnce(async (_id, _body, key) => result(key));
  const view = renderWithI18n(content());
  await userEvent.click(await screen.findByRole("button", { name: "Purge v1" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Purge reason" }), "Reported credential");
  await userEvent.click(screen.getByRole("button", { name: "Purge text" }));
  await screen.findByRole("region", { name: "Original purge request" });
  const original = readAdminCollaborationMemoryDraft(storageScope, detail.issueId)!.input;
  view.unmount(); renderWithI18n(content(null));
  await screen.findByRole("region", { name: "Original purge request" });
  expect(purge).toHaveBeenCalledTimes(1);
  await userEvent.click(await screen.findByRole("button", { name: "Retry original request" }));
  await waitFor(() => expect(purge).toHaveBeenCalledTimes(2));
  expect(purge.mock.calls[1]).toEqual(purge.mock.calls[0]);
  expect(purge.mock.calls[1]?.[2]).toBe(original.key);
  await screen.findByRole("region", { name: "Recorded shared context purge" });
  await userEvent.click(screen.getByRole("button", { name: "Close receipt" }));
  expect(readAdminCollaborationMemoryDraft(storageScope, detail.issueId)).toBeNull();
});

it("removes a new purge affordance when the current creator has been restored", async () => {
  const view = renderWithI18n(content());
  await screen.findByRole("button", { name: "Purge v1" });
  view.rerender(content({ ...detail, creator: { ...detail.creator, disabled: false } }));
  expect(screen.queryByRole("button", { name: "Purge v1" })).not.toBeInTheDocument();
});
