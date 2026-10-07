import { useCallback, useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { setApiInstance, ApiError } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { useProjectAccessStore, projectCapabilitiesOptions, projectOverviewOptions } from "@multica/core/projects";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { memberListOptions, agentListOptions } from "@multica/core/workspace/queries";
import { useProjectAccessGuard } from "./use-project-access-guard";

// The idempotent cleanup rule is canonical in
// packages/core/projects/access-lifecycle.test.ts; this suite keeps the
// mounted-page wiring that turned a single revocation into a request loop.
const WS = "11111111-1111-4111-8111-111111111111";
const PROJECT = "22222222-2222-4222-8222-222222222222";
let qc: QueryClient;
let revoked = false;
const reply = <T,>(value: T) => () => revoked
  ? Promise.reject(new ApiError("workspace not found or access denied", 404, "Not Found", { code: "workspace_access_denied" }))
  : Promise.resolve(value);
const api = {
  getBaseUrl: () => "https://source.test",
  getSessionScope: () => "session-1",
  getProject: vi.fn(reply({ id: PROJECT, workspace_id: WS, title: "Project" })),
  getProjectCapabilities: vi.fn(reply({ overview: true })),
  getProjectOverview: vi.fn(reply({ id: PROJECT })),
  listMembers: vi.fn(reply([])),
  listAgents: vi.fn(reply([])),
};

// Mirrors ProjectDetail: the page guard and workspace reads stay mounted above
// a child section whose own guard sees the revocation first.
function Overview() {
  const overview = useQuery(projectOverviewOptions(WS, PROJECT));
  useProjectAccessGuard(overview.error, WS, PROJECT, () => {});
  return <p>Overview</p>;
}
function Detail() {
  const detail = useQuery(projectDetailOptions(WS, PROJECT));
  const capabilities = useQuery(projectCapabilitiesOptions(WS));
  const [accessLost, setAccessLost] = useState(false);
  const denied = useProjectAccessStore((state) => state.denied[JSON.stringify([WS, "*"])] || state.denied[JSON.stringify([WS, PROJECT])]);
  const hide = useCallback(() => setAccessLost(true), []);
  useProjectAccessGuard(detail.error ?? capabilities.error, WS, PROJECT, hide);
  useQuery(memberListOptions(WS));
  useQuery(agentListOptions(WS));
  if (accessLost || denied) return <div role="alert">Permission lost</div>;
  return <Overview />;
}

beforeEach(() => {
  revoked = false;
  qc = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
  setApiInstance(api as unknown as ApiClient);
  for (const fn of Object.values(api)) if (vi.isMockFunction(fn)) fn.mockClear();
});
afterEach(() => qc.clear());

it("P1-L14 a workspace revocation refetches mounted workspace reads once, then settles", async () => {
  render(<QueryClientProvider client={qc}><Detail /></QueryClientProvider>);
  expect(await screen.findByText("Overview")).toBeInTheDocument();
  const members = api.listMembers.mock.calls.length;
  const agents = api.listAgents.mock.calls.length;
  revoked = true;
  await act(async () => { await qc.invalidateQueries({ queryKey: projectOverviewOptions(WS, PROJECT).queryKey }); });
  expect(await screen.findByRole("alert")).toHaveTextContent("Permission lost");
  // Before the fix every render re-ran the cleanup, so these counts climbed
  // into the hundreds within this window.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 200)); });
  expect(api.listMembers.mock.calls.length - members).toBeLessThanOrEqual(1);
  expect(api.listAgents.mock.calls.length - agents).toBeLessThanOrEqual(1);
  expect(useProjectAccessStore.getState().denied).toEqual({ [JSON.stringify([WS, "*"])]: true });
});
