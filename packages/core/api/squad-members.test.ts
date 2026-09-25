// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { SquadSchema } from "./schemas";

const baseSquad = {
  id: "squad-1", workspace_id: "ws-1", name: "Squad", leader_id: "lead",
  creator_id: "owner", created_at: "", updated_at: "",
};

afterEach(() => vi.unstubAllGlobals());

describe("squad complete membership compatibility", () => {
  it("preserves the full membership beyond preview and explicit empty membership", () => {
    expect(SquadSchema.parse({ ...baseSquad, agent_member_ids: ["a", "b", "c", "d"] }).agent_member_ids).toEqual(["a", "b", "c", "d"]);
    expect(SquadSchema.parse({ ...baseSquad, agent_member_ids: [] }).agent_member_ids).toEqual([]);
  });

  it.each([undefined, null, "invalid", [1], ["valid", null]])("keeps unreadable membership unknown (%j) without losing the squad", (agentMemberIds) => {
    const parsed = SquadSchema.parse({ ...baseSquad, agent_member_ids: agentMemberIds });
    expect(parsed.id).toBe("squad-1");
    expect(parsed.agent_member_ids).toBeUndefined();
  });
});

describe("ApiClient.listSquadMembers", () => {
  const client = new ApiClient("https://api.example.test");
  const fetchBody = (body: unknown) => vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }),
  ));

  it("parses a full roster with optional display fields omitted", async () => {
    fetchBody([{ id: "seat-1", squad_id: "squad-1", member_type: "agent", member_id: "agent-1" }]);
    await expect(client.listSquadMembers("squad-1")).resolves.toEqual([
      { id: "seat-1", squad_id: "squad-1", member_type: "agent", member_id: "agent-1", role: "", created_at: "" },
    ]);
  });

  it("accepts a confirmed empty roster", async () => {
    fetchBody([]);
    await expect(client.listSquadMembers("squad-1")).resolves.toEqual([]);
  });

  it.each([null, {}, [{ id: "broken" }], [{ id: "s", squad_id: "squad-1", member_type: "agent", member_id: 12 }]])("rejects unreadable rosters rather than claiming no members (%j)", async (body) => {
    fetchBody(body);
    await expect(client.listSquadMembers("squad-1")).rejects.toThrow(/squad members/i);
  });
});
