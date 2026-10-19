import { afterEach, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope } from "@multica/core/collaboration";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationNotifications } from "./notifications";

const id = "00000000-0000-4000-8000-000000000001";
const runId = "00000000-0000-4000-8000-000000000002";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); vi.unstubAllGlobals(); });

it("opens an AI group result on its representative run and retains issue links for older notifications", async () => {
  const notification = { id, issue_id: id, event_seq: "10", kind: "ai_failed", is_mention: false, source_kind: "issue", source_id: null,
    issue_title: "Recorded group", can_open: true, created_at: "2026-10-19T00:00:00Z", read_at: null, archived_at: null };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [
    { ...notification, run_id: runId },
    { ...notification, id: runId, event_seq: "9", issue_title: "Legacy group" },
    { ...notification, id: "00000000-0000-4000-8000-000000000003", event_seq: "8", can_open: false, issue_title: null },
  ], unread_count: 3, next_cursor: null }))));
  setApiInstance(new ApiClient("https://server.example"));
  const push = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "", push, replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => `https://server.example${path}` }}><CollaborationNotifications scope={collaborationScope(id)} view="current" /></NavigationProvider></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: /Notifications.*3 unread/ }));
  const link = await screen.findByRole("link", { name: "Recorded group" });
  expect(link).toHaveAttribute("href", `/account?view=participating&issue=${id}#run-${runId}`);
  expect(within(link.closest("li")!).getByText("AI execution failed")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Legacy group" })).toHaveAttribute("href", `/account?view=current&issue=${id}`);
  expect(screen.getByText("Issue unavailable").closest("li")!.querySelector("a")).toBeNull();
  await userEvent.click(link);
  expect(push).toHaveBeenCalledWith(`/account?view=participating&issue=${id}#run-${runId}`);
  expect(screen.queryByRole("link", { name: "Recorded group" })).not.toBeInTheDocument();
});

it("distinguishes human handoffs and assignments from content-free removal notifications", async () => {
  const base = { issue_id: id, is_mention: false, source_kind: "issue", source_id: null, can_open: true,
    created_at: "2026-10-19T00:00:00Z", read_at: null, archived_at: null };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [
    { ...base, id, event_seq: "12", kind: "assigned", issue_title: "Assigned issue" },
    { ...base, id: runId, event_seq: "11", kind: "transferred", issue_title: "Transferred issue" },
    { ...base, id: "00000000-0000-4000-8000-000000000003", event_seq: "10", kind: "removed", can_open: false, issue_title: null },
    { ...base, id: "00000000-0000-4000-8000-000000000004", event_seq: "9", kind: "future_event", issue_title: "Other issue" },
  ], unread_count: 4, next_cursor: null }))));
  setApiInstance(new ApiClient("https://server.example"));
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => `https://server.example${path}` }}><CollaborationNotifications scope={collaborationScope(id)} view="current" /></NavigationProvider></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: /Notifications.*4 unread/ }));
  expect(within(screen.getByRole("link", { name: "Assigned issue" }).closest("li")!).getByText("Assigned to you")).toBeInTheDocument();
  expect(within(screen.getByRole("link", { name: "Transferred issue" }).closest("li")!).getByText("Transferred")).toBeInTheDocument();
  const removed = screen.getByText("You were removed from this collaboration issue.").closest("li")!;
  expect(within(removed).queryByRole("link")).not.toBeInTheDocument();
  expect(within(removed).getByRole("button", { name: "Archive" })).toBeEnabled();
  expect(within(screen.getByRole("link", { name: "Other issue" }).closest("li")!).getByText("Activity recorded")).toBeInTheDocument();
  expect(screen.queryByText("future_event")).not.toBeInTheDocument();
});
