// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { squadMembersOptions, workspaceKeys } from "./queries";

const listSquadMembers = vi.hoisted(() => vi.fn(async () => []));
vi.mock("../api", () => ({ api: { listSquadMembers } }));

describe("squadMembersOptions", () => {
  it("shares detail's workspace cache and invalidation subtree", async () => {
    const queryClient = new QueryClient();
    const options = squadMembersOptions("ws-1", "squad-1");
    expect(options.queryKey).toEqual(["workspaces", "ws-1", "squads", "squad-1", "members"]);
    queryClient.setQueryData(options.queryKey, []);
    await queryClient.invalidateQueries({ queryKey: workspaceKeys.squads("ws-1") });
    expect(queryClient.getQueryState(options.queryKey)?.isInvalidated).toBe(true);
    await queryClient.fetchQuery(options);
    expect(listSquadMembers).toHaveBeenCalledWith("squad-1");
    queryClient.clear();
  });

  it("does not enable an unscoped request", () => {
    expect(squadMembersOptions("", "squad-1").enabled).toBe(false);
    expect(squadMembersOptions("ws-1", "").enabled).toBe(false);
  });
});
