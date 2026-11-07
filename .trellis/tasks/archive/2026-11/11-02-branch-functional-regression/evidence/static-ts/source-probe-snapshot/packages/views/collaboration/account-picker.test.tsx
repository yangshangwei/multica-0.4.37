import { afterEach, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope } from "@multica/core/collaboration";
import { renderWithI18n } from "../test/i18n";
import { CollaborationAccountPicker } from "./account-picker";

vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: actor } }) } }));

const actor = "00000000-0000-4000-8000-000000000001";
const recentId = "00000000-0000-4000-8000-000000000002";
const directoryId = "00000000-0000-4000-8000-000000000003";
const exactId = "00000000-0000-4000-8000-000000000004";
const source = "00000000-0000-4000-8000-000000000005";
const clients: QueryClient[] = [];
const page = (sourceMember = true) => ({
  issue_id: null,
  source_workspace_id: source,
  scope: "directory",
  source_member: sourceMember,
  recent: [{ user_id: recentId, name: "Recent collaborator", avatar_url: "", external: false }],
  recipients: [{ user_id: directoryId, name: "Directory account", avatar_url: "", external: true }],
  next_cursor: null,
});

afterEach(() => {
  clients.splice(0).forEach((client) => client.clear());
  localStorage.clear();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function mount(fetch: typeof globalThis.fetch, options: { allowSelf?: boolean; membersOnly?: boolean; onSelect?: (value: unknown) => void } = {}) {
  setApiInstance(new ApiClient("https://server.example"));
  vi.stubGlobal("fetch", fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const ui = (disabled = false) => <QueryClientProvider client={client}>
    <CollaborationAccountPicker scope={collaborationScope(actor)} context={{ kind: "creation", sourceWorkspaceId: source }}
      selected={null} allowSelf={options.allowSelf} membersOnly={options.membersOnly} label="Recipient" disabled={disabled} onSelect={options.onSelect ?? vi.fn()} />
  </QueryClientProvider>;
  const view = renderWithI18n(ui());
  return { ...view, client, setDisabled: (disabled: boolean) => view.rerender(ui(disabled)) };
}

it("browses sections, uses exact lookup, and closes with Escape without leaving the trigger", async () => {
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/recipients/search")) return new Response(JSON.stringify(page()));
    if (url.pathname.endsWith("/recipients/resolve")) return new Response(JSON.stringify({ issue_id: null, source_workspace_id: source,
      recipient: { user_id: exactId, name: "Exact account", avatar_url: "", external: true } }));
    throw new Error(`Unexpected request ${url.pathname} ${String(init?.body)}`);
  });
  const onSelect = vi.fn();
  mount(fetch, { allowSelf: true, onSelect });
  const trigger = screen.getByRole("button", { name: "Recipient" });
  expect(trigger).toHaveTextContent("Assign to me");
  const user = userEvent.setup();
  await user.click(trigger);
  expect(await screen.findByText("Recent collaborator")).toBeVisible();
  expect(screen.getByText("Directory account")).toBeVisible();
  expect(screen.getByText("You")).toBeVisible();

  const search = screen.getByRole("textbox", { name: "Filter options" });
  await user.type(search, "recent");
  await user.click(await screen.findByRole("button", { name: "Find by full username, email, or account ID" }));
  const exact = screen.getByPlaceholderText("Username, email, or account ID");
  await user.type(exact, "exact@example.test");
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await user.click(await screen.findByRole("button", { name: "Use this account" }));
  expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({ user_id: exactId, name: "Exact account" }));

  await user.click(trigger);
  await screen.findByRole("listbox");
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(trigger).toHaveFocus();
});

it("keeps a members-only picker from selecting an undisclosed external account", async () => {
  const fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/recipients/search")) return new Response(JSON.stringify({ ...page(false), recent: [], recipients: [{ ...page(false).recipients[0], external: null }] }));
    if (url.pathname.endsWith("/recipients/resolve")) return new Response(JSON.stringify({ issue_id: null, source_workspace_id: source,
      recipient: { user_id: exactId, name: "Undisclosed account", avatar_url: "", external: null } }));
    throw new Error(`Unexpected request ${url.pathname}`);
  });
  const onSelect = vi.fn();
  mount(fetch, { membersOnly: true, onSelect });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  await screen.findByText("Only members of the source workspace can be selected here.");
  expect(screen.queryByRole("button", { name: /Directory account/ })).toBeNull();
  expect(onSelect).not.toHaveBeenCalled();
});

it("closes an already-open picker when its parent disables selection", async () => {
  const fetch = vi.fn(async () => new Response(JSON.stringify(page())));
  const onSelect = vi.fn();
  const { setDisabled, client } = mount(fetch, { onSelect });
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Recipient" });
  await user.click(trigger);
  await screen.findByText("Directory account");

  setDisabled(true);

  expect(trigger).toBeDisabled();
  await waitFor(() => expect(screen.queryByRole("listbox")).toBeNull());
  await act(() => client.invalidateQueries());
  await user.click(trigger);
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(onSelect).not.toHaveBeenCalled();
  expect(fetch).toHaveBeenCalledTimes(1);
});

it("retains cached rows and retries the failed next page from its original cursor", async () => {
  const cursors: Array<string | null> = [];
  let nextAttempts = 0;
  const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body));
    cursors.push(body.cursor ?? null);
    if (body.cursor === "page-two") {
      nextAttempts += 1;
      if (nextAttempts === 1) return new Response(JSON.stringify({ error: "Unavailable" }), { status: 503 });
      return new Response(JSON.stringify({ ...page(), recent: [], recipients: [{ user_id: exactId, name: "Next page account", avatar_url: "", external: false }] }));
    }
    return new Response(JSON.stringify({ ...page(), next_cursor: "page-two" }));
  });
  mount(fetch);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  await screen.findByText("Directory account");
  await user.click(screen.getByRole("button", { name: "Load more" }));

  const error = await screen.findByRole("alert");
  expect(screen.getByText("Directory account")).toBeVisible();
  await user.click(within(error).getByRole("button", { name: "Refresh" }));

  await screen.findByText("Next page account");
  expect(screen.getByText("Directory account")).toBeVisible();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(cursors).toEqual([null, "page-two", "page-two"]);
});

it("keeps cached rows visible while exposing and retrying a failed background refresh", async () => {
  let attempts = 0;
  const fetch = vi.fn(async () => {
    attempts += 1;
    if (attempts === 2) return new Response(JSON.stringify({ error: "Unavailable" }), { status: 503 });
    return new Response(JSON.stringify(attempts === 1 ? page() : { ...page(), recipients: [{ ...page().recipients[0], name: "Refreshed directory account" }] }));
  });
  const { client } = mount(fetch);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  await screen.findByText("Directory account");

  await act(() => client.invalidateQueries());

  const error = await screen.findByRole("alert");
  expect(screen.getByText("Directory account")).toBeVisible();
  await user.click(within(error).getByRole("button", { name: "Refresh" }));
  await screen.findByText("Refreshed directory account");
  expect(screen.queryByRole("alert")).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(3);
});
