// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { iterationKeys } from "../iterations";
import { useProjectAccessStore } from "./access";
import { projectKeys } from "./queries";
import { projectP1Keys } from "./p1-queries";
import { useProjectPlanningTimezone } from "./p1-mutations";

beforeEach(() => useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} }));

it("refreshes project and iteration planning after a workspace timezone save", async () => {
  const saved = { workspace_id: "ws", planning_timezone: "UTC", effective_timezone: "UTC", configured: true };
  setApiInstance({ updateProjectPlanningTimezone: vi.fn().mockResolvedValue(saved) } as unknown as ApiClient);
  const client = new QueryClient();
  const affected = [projectKeys.list("ws"), [...iterationKeys.all("ws"), "settings"]];
  const other = [...iterationKeys.all("other"), "settings"];
  for (const key of [...affected, other]) client.setQueryData(key, {});
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  const { result } = renderHook(() => useProjectPlanningTimezone("ws"), { wrapper });
  await act(async () => { await result.current.mutateAsync("UTC"); });
  expect(client.getQueryData(projectP1Keys.timezone("ws"))).toEqual(saved);
  for (const key of affected) expect(client.getQueryState(key)?.isInvalidated).toBe(true);
  expect(client.getQueryState(other)?.isInvalidated).toBe(false);
  client.clear();
});
