import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope } from "@multica/core/collaboration";
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
