import { afterEach, expect, it, vi } from "vitest";
import { act, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope } from "@multica/core/collaboration";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationNotifications } from "./notifications";

const id = "00000000-0000-4000-8000-000000000001";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); vi.unstubAllGlobals(); });

function mount(locale: "en" | "zh-Hans" = "en") {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const push = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "", push, replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => path }}>
    <CollaborationNotifications scope={collaborationScope(id)} />
  </NavigationProvider></QueryClientProvider>, { locale });
  return push;
}

it("opens the default unread page with a native link while its count is unknown", async () => {
  let resolve!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((ready) => { resolve = ready; })));
  const push = mount();
  const link = screen.getByRole("link", { name: "Collaboration notifications" });
  expect(link).toHaveAttribute("href", "/account?view=notifications&filter=unread");
  expect(link).not.toHaveTextContent("0");
  await act(async () => resolve(new Response(JSON.stringify({ items: [], unread_count: 2, next_cursor: null, view: "all" }))));
  expect(await screen.findByRole("link", { name: "Collaboration notifications 2 unread" })).toBe(link);
  await userEvent.click(link);
  expect(push).toHaveBeenCalledExactlyOnceWith("/account?view=notifications&filter=unread");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.queryByText("No notifications.")).not.toBeInTheDocument();
});

it("keeps a translated page entry without presenting a known zero as a badge", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [], unread_count: 0, next_cursor: null, view: "all" }))));
  mount("zh-Hans");
  const link = await screen.findByRole("link", { name: "协作通知" });
  expect(link).toHaveAttribute("href", "/account?view=notifications&filter=unread");
  expect(link).not.toHaveTextContent("0");
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});
