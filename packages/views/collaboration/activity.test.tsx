import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope } from "@multica/core/collaboration";
import type { AccountDestination } from "@multica/core/paths";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationActivity } from "./activity";

const id = "00000000-0000-4000-8000-000000000001";
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
afterEach(() => { client.clear(); vi.unstubAllGlobals(); });

it("reports activity read failures to the drawer entry and clears attention after retry", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Offline"))
    .mockResolvedValue(new Response(JSON.stringify({ issue_id: id, items: [], next_cursor: null })));
  vi.stubGlobal("fetch", fetch);
  const status = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationActivity scope={collaborationScope(id)}
    destination={{ view: "participating", issueId: id, commentId: null }} onStatusChange={status} /></QueryClientProvider>);
  expect(await screen.findByRole("alert")).toHaveTextContent("Could not load collaboration. Try again.");
  await waitFor(() => expect(status).toHaveBeenLastCalledWith({ attention: true }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(status).toHaveBeenLastCalledWith({ attention: false }));
  expect(screen.queryByRole("alert")).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(2);
});

it.each([
  [{ view: "participating", issueId: id, commentId: null, runId: id }, `/account?view=participating&issue=${id}#comment-${id}`],
  [{ view: "notifications", notificationView: "archived", notificationId: id, issueId: id, commentId: null, runId: id }, `/account?view=notifications&filter=archived&notification=${id}&issue=${id}#comment-${id}`],
] satisfies [AccountDestination, string][])("keeps the originating page when opening a linked activity comment", async (destination, href) => {
  setApiInstance(new ApiClient("https://server.example"));
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ issue_id: id, next_cursor: null, items: [{
    id, issue_id: id, event_seq: "1", kind: "comment_created", actor: null, created_at: "2026-11-21T10:00:00Z",
    details_available: true, target: null, handoff: null, comment_id: id, changes: [],
  }] }))));
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: path => path }}>
    <CollaborationActivity scope={collaborationScope(id)} destination={destination} />
  </NavigationProvider></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: "Open comment" })).toHaveAttribute("href", href);
});
