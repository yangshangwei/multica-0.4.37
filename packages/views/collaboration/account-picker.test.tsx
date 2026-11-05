import { afterEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, type CollaborationRecipientAccount } from "@multica/core/collaboration";
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

function mount(fetch: typeof globalThis.fetch, options: { allowSelf?: boolean; membersOnly?: boolean; onSelect?: (value: unknown) => void;
  selected?: CollaborationRecipientAccount | readonly CollaborationRecipientAccount[] | null; multiple?: boolean; maxSelected?: number } = {}) {
  setApiInstance(new ApiClient("https://server.example"));
  vi.stubGlobal("fetch", fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const ui = (disabled = false) => <QueryClientProvider client={client}>
    <CollaborationAccountPicker scope={collaborationScope(actor)} context={{ kind: "creation", sourceWorkspaceId: source }}
      selected={options.selected ?? null} allowSelf={options.allowSelf} membersOnly={options.membersOnly} multiple={options.multiple} maxSelected={options.maxSelected}
      label="Recipient" disabled={disabled} onSelect={options.onSelect ?? vi.fn()} />
  </QueryClientProvider>;
  const view = renderWithI18n(ui());
  return { ...view, client, setDisabled: (disabled: boolean) => view.rerender(ui(disabled)),
    setSelected: (selected: CollaborationRecipientAccount) => { options.selected = selected; view.rerender(ui()); } };
}

it("distinguishes same-name accounts by full IDs and retains the chosen identity", async () => {
  const accounts = [recentId, directoryId].map((user_id) => ({ user_id, name: "李明", avatar_url: "", external: false }));
  const onSelect = vi.fn();
  const { setSelected } = mount(vi.fn(async () => new Response(JSON.stringify({ ...page(), recent: [], recipients: accounts }))), { onSelect });
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Recipient" });
  await user.click(trigger);
  const list = await screen.findByRole("listbox", { name: "Recipient" });
  const chosen = await within(list).findByRole("option", { name: new RegExp(directoryId) });
  expect(within(list).getByRole("option", { name: new RegExp(recentId) })).toHaveTextContent(recentId);
  expect(chosen).toHaveTextContent(directoryId);
  await user.click(chosen);
  expect(onSelect).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ user_id: directoryId }));
  setSelected(accounts[1]!);
  expect(trigger).toHaveTextContent(directoryId);
  expect(trigger).toHaveAccessibleDescription(new RegExp(directoryId));
});

it("connects the native filter textbox to stable selected and disabled account options", async () => {
  const onSelect = vi.fn();
  mount(vi.fn(async () => new Response(JSON.stringify(page()))), { selected: [page().recent[0]!], multiple: true, maxSelected: 1, onSelect });
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Recipient" });
  await user.click(trigger);
  await screen.findByText("Directory account");
  const input = screen.getByRole("textbox", { name: "Filter options" });
  const listbox = screen.getByRole("listbox", { name: "Recipient" });
  expect(input).toHaveAttribute("aria-controls", listbox.id);
  expect(input).toHaveAttribute("aria-haspopup", "listbox");
  expect(input).not.toHaveAttribute("aria-expanded");
  expect(listbox).toHaveAttribute("aria-multiselectable", "true");
  const selected = within(listbox).getByRole("option", { name: /Recent collaborator/ });
  const blocked = within(listbox).getByRole("option", { name: /Directory account/ });
  expect(selected).toHaveAttribute("aria-selected", "true");
  expect(selected).toHaveAttribute("aria-disabled", "false");
  expect(blocked).toHaveAttribute("aria-selected", "false");
  expect(blocked).toHaveAttribute("aria-disabled", "true");
  expect(blocked).toBeDisabled();
  expect(selected.id).not.toBe(blocked.id);
  expect(selected.id).not.toBe("");
  const selectedId = selected.id;
  await user.click(input);
  await user.keyboard("{ArrowDown}");
  expect(input).toHaveAttribute("aria-activedescendant", selectedId);
  expect(onSelect).not.toHaveBeenCalled();
  await user.keyboard("{Escape}");
  expect(trigger).toHaveFocus();
  await user.click(trigger);
  expect(await screen.findByRole("option", { name: /Recent collaborator/ })).toHaveAttribute("id", selectedId);
  expect(screen.getByRole("textbox", { name: "Filter options" })).not.toHaveAttribute("aria-activedescendant");
});

it("exposes navigation without selecting during IME and clears removed or empty active options", async () => {
  let accounts = page().recipients;
  const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    const query = JSON.parse(String(init?.body)).q;
    return new Response(JSON.stringify({ ...page(), recent: [], recipients: query === "none" ? [] : accounts }));
  });
  const onSelect = vi.fn();
  const { client } = mount(fetch, { onSelect });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  const option = await screen.findByRole("option", { name: /Directory account/ });
  const input = screen.getByRole("textbox", { name: "Filter options" });
  await user.click(input);
  await user.keyboard("{ArrowDown}");
  expect(input).toHaveAttribute("aria-activedescendant", option.id);
  expect(option).toHaveAttribute("aria-selected", "false");
  fireEvent.keyDown(input, { key: "ArrowUp", isComposing: true });
  fireEvent.keyDown(input, { key: "Enter", isComposing: true });
  fireEvent.keyDown(input, { key: "Enter", keyCode: 229 });
  expect(input).toHaveAttribute("aria-activedescendant", option.id);
  expect(onSelect).not.toHaveBeenCalled();
  accounts = [{ user_id: exactId, name: "Replacement account", avatar_url: "", external: false }];
  await act(() => client.invalidateQueries());
  await screen.findByRole("option", { name: /Replacement account/ });
  expect(input).not.toHaveAttribute("aria-activedescendant");
  await user.type(input, "none");
  await waitFor(() => expect(screen.queryByRole("option")).toBeNull());
  expect(input).not.toHaveAttribute("aria-activedescendant");
  await user.keyboard("{Enter}");
  expect(onSelect).not.toHaveBeenCalled();
  await user.clear(input);
  const replacement = await screen.findByRole("option", { name: /Replacement account/ });
  await user.keyboard("{ArrowDown}{Enter}");
  expect(onSelect).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ user_id: exactId }));
  expect(replacement).not.toBeInTheDocument();
});

it.each(["keyboard", "pointer"] as const)("blocks stale %s selection during debounce and while replacement results are pending", async (method) => {
  let finish!: (response: Response) => void;
  const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    const query = JSON.parse(String(init?.body)).q;
    if (query) return new Promise<Response>((resolve) => { finish = resolve; });
    return new Response(JSON.stringify({ ...page(), recent: [] }));
  });
  const onSelect = vi.fn();
  mount(fetch, { onSelect });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  const oldOption = await screen.findByRole("option", { name: /Directory account/ });
  const input = screen.getByRole("textbox", { name: "Filter options" });
  const attemptSelection = () => method === "keyboard" ? fireEvent.keyDown(input, { key: "Enter" }) : fireEvent.click(oldOption);

  fireEvent.change(input, { target: { value: "replacement" } });
  attemptSelection();
  expect(onSelect).not.toHaveBeenCalled();
  expect(screen.getByText("Loading accounts...")).toBeVisible();
  expect(input).not.toHaveAttribute("aria-activedescendant");
  for (const option of screen.queryAllByRole("option")) expect(option).toBeDisabled();

  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  attemptSelection();
  expect(onSelect).not.toHaveBeenCalled();
  expect(screen.getByText("Loading accounts...")).toBeVisible();
  for (const option of screen.queryAllByRole("option")) expect(option).toBeDisabled();

  await act(async () => finish(new Response(JSON.stringify({ ...page(), recent: [], recipients: [
    { user_id: exactId, name: "Replacement account", avatar_url: "", external: false },
  ] }))));
  const current = await screen.findByRole("option", { name: /Replacement account/ });
  expect(current).toBeEnabled();
  expect(screen.queryByText("Loading accounts...")).toBeNull();
  if (method === "keyboard") await user.keyboard("{ArrowDown}{Enter}");
  else await user.click(current);
  expect(onSelect).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ user_id: exactId }));
});

it("filters pinned accounts during search while preserving the selected identity and server matches", async () => {
  const selected = { user_id: exactId, name: "Chosen handler", avatar_url: "", external: false };
  const fetch = vi.fn(async () => new Response(JSON.stringify(page())));
  mount(fetch, { allowSelf: true, selected });
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Recipient" });
  await user.click(trigger);
  await waitFor(() => expect(screen.getByRole("option", { name: /Assign to me/ })).toBeEnabled());
  await screen.findByRole("option", { name: /Recent collaborator/ });
  const input = screen.getByRole("textbox", { name: "Filter options" });
  await user.type(input, "directory@example.test");
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  await waitFor(() => expect(screen.getAllByRole("option")).toHaveLength(1));
  expect(screen.getByRole("option", { name: /Directory account/ })).toBeEnabled();
  expect(trigger).toHaveTextContent(selected.name);
  expect(trigger).toHaveTextContent(exactId);

  fireEvent.change(input, { target: { value: "CHOSEN" } });
  await waitFor(() => expect(screen.getByRole("option", { name: /Chosen handler/ })).toBeEnabled());
  expect(screen.queryByRole("option", { name: /Recent collaborator|Assign to me/ })).toBeNull();
  await user.clear(input);
  await waitFor(() => expect(screen.getByRole("option", { name: /Assign to me/ })).toBeEnabled());
});

it("keeps the active account through append-only paging while footer actions stay outside navigation", async () => {
  const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body));
    return new Response(JSON.stringify(body.cursor ? { ...page(), recent: [], recipients: [{ user_id: exactId, name: "Later account", avatar_url: "", external: false }] }
      : { ...page(), next_cursor: "next" }));
  });
  const onSelect = vi.fn();
  mount(fetch, { onSelect });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  const account = await screen.findByRole("option", { name: /Directory account/ });
  const input = screen.getByRole("textbox", { name: "Filter options" });
  await user.click(input);
  await user.keyboard("{ArrowDown}{ArrowDown}");
  expect(input).toHaveAttribute("aria-activedescendant", account.id);
  await user.click(screen.getByRole("button", { name: "Load more" }));
  await screen.findByRole("option", { name: /Later account/ });
  expect(input).toHaveAttribute("aria-activedescendant", account.id);
  expect(input).toHaveFocus();
  await user.keyboard("{Enter}");
  expect(onSelect).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ user_id: directoryId }));
});

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
  expect(await screen.findByText(new RegExp(exactId))).toBeVisible();
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
  expect(screen.queryByRole("option", { name: /Directory account/ })).toBeNull();
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
