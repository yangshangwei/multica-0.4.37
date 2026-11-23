import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import { iterationChoicesOptions, type PendingIterationCommand } from "@multica/core/iterations";
import { renderWithI18n } from "../test/i18n";
import { IterationRecovery } from "./iteration-recovery";
import { receipt, source, targetA, ws } from "./test-fixtures";

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: { user: { id: string } }) => unknown) => select({ user: { id: ws } }),
    { getState: () => ({ user: { id: ws } }) },
  ),
}));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: { getBaseUrl: () => "test", getSessionScope: () => "session", getIterationOperation: vi.fn(), applyIterationOperation: vi.fn() },
}));

beforeEach(() => {
  vi.resetAllMocks();
  window.localStorage.clear();
});

it("distinguishes pending operations and recovers only the selected original request", async () => {
  const endRequest = "40000000-0000-4000-8000-000000000001";
  const startRequest = "40000000-0000-4000-8000-000000000002";
  const entries: PendingIterationCommand[] = [
    { scope: `end:${source.id}`, command: { kind: "operation", body: { request_id: endRequest, preview_hash: "end-hash", draft: { operation: "end", iteration_id: source.id, expected_iteration_revision: source.revision, expected_scope_revision: source.scope_revision, expected_settings_revision: 1, reason: "Close the first period", moves: [], start: null } } } },
    { scope: `start:${targetA.id}`, command: { kind: "operation", body: { request_id: startRequest, preview_hash: "start-hash", draft: { operation: "start", iteration_id: targetA.id, expected_iteration_revision: targetA.revision, expected_scope_revision: targetA.scope_revision, expected_settings_revision: 1, reason: null, moves: [], start: { target_id: targetA.id, mode: "scheduled", terminal_choices: [] } } } } },
  ];
  for (const entry of entries) window.localStorage.setItem(
    `multica_iteration_command:test:${ws}:${ws}:${entry.scope}:${entry.command.body.request_id}`,
    JSON.stringify(entry.command),
  );
  const client = new QueryClient();
  client.setQueryData(iterationChoicesOptions(ws).queryKey, [source, targetA]);
  vi.mocked(api.getIterationOperation).mockResolvedValue({ ...receipt, request_id: startRequest, operation: "start" });
  renderWithI18n(<QueryClientProvider client={client}><IterationRecovery wsId={ws} /></QueryClientProvider>);
  const user = userEvent.setup();
  const region = screen.getByRole("region", { name: "Unconfirmed iteration requests" });
  const endAction = within(region).getByRole("button", { name: "Check request: End iteration — Source iteration" });
  const startAction = within(region).getByRole("button", { name: "Check request: Start iteration — Next A" });
  expect(endAction).toBeEnabled();
  await user.click(startAction);
  await waitFor(() => expect(api.getIterationOperation).toHaveBeenCalledExactlyOnceWith(ws, startRequest));
  expect(api.applyIterationOperation).not.toHaveBeenCalled();
  expect(within(region).queryByRole("button", { name: "Check request: Start iteration — Next A" })).not.toBeInTheDocument();
  expect(endAction).toBeEnabled();
  expect(window.localStorage.getItem(`multica_iteration_command:test:${ws}:${ws}:${entries[0]!.scope}:${endRequest}`)).not.toBeNull();
});

it("distinguishes same-operation requests with the same reason when identity names are not cached", () => {
  const requests = ["40000000-0000-4000-8000-000000000001", "40000000-0000-4000-8000-000000000002"];
  const entries: PendingIterationCommand[] = [source, targetA].map((iteration, index) => ({
    scope: `end:${iteration.id}`,
    command: { kind: "operation", body: {
      request_id: requests[index]!, preview_hash: "end-hash",
      draft: { operation: "end", iteration_id: iteration.id, expected_iteration_revision: iteration.revision, expected_scope_revision: iteration.scope_revision, expected_settings_revision: 1, reason: "Close this period", moves: [], start: null },
    } },
  }));
  for (const entry of entries) window.localStorage.setItem(
    `multica_iteration_command:test:${ws}:${ws}:${entry.scope}:${entry.command.body.request_id}`,
    JSON.stringify(entry.command),
  );
  const client = new QueryClient();
  renderWithI18n(<QueryClientProvider client={client}><IterationRecovery wsId={ws} /></QueryClientProvider>);
  const region = screen.getByRole("region", { name: "Unconfirmed iteration requests" });
  for (const iteration of [source, targetA]) {
    const action = within(region).getByRole("button", { name: `Check request: End iteration — Close this period · ${iteration.id}` });
    expect(action).toBeVisible();
    expect(action).toHaveTextContent(iteration.id);
  }
  expect(api.getIterationOperation).not.toHaveBeenCalled();
});
