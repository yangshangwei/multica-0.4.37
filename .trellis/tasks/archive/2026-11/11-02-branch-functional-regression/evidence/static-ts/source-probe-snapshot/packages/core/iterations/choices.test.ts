// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { expect, it, vi } from "vitest";
import { api } from "../api";
import { iterationChoicesOptions, type Iteration } from "./index";
vi.mock("../api", async (original) => ({ ...await original<typeof import("../api")>(), api: { listIterations: vi.fn() } }));
it("only offers supported manual destinations across all pages", async () => {
  const item = (id: string, status: string, mode: string): Iteration => ({ id, workspace_id: "w", name: id, description: null, coordinator_user_id: null, status, mode, start_date: "2026-10-01", end_date: "2026-10-14", timezone: "UTC", revision: 1, scope_revision: 1, started_at: null, logical_ended_at: null, processed_at: null });
  vi.mocked(api.listIterations).mockResolvedValueOnce({ workspace_id: "w", items: [item("unknown", "planned", "future-mode"), item("old", "completed", "manual")], next_cursor: "next" }).mockResolvedValueOnce({ workspace_id: "w", items: [item("active", "active", "manual"), item("planned", "planned", "manual")], next_cursor: null });
  const result = await new QueryClient().fetchQuery(iterationChoicesOptions("w"));
  expect(result.map(value => value.id)).toEqual(["active", "planned"]);
});
