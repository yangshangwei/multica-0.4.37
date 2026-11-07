// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { AgentRoleTemplate, Squad, SquadMember, SquadTemplate } from "../types";
import { buildAgentRoleMetadata, buildAgentSquadMemberships, resolveAgentRole } from "./discovery";

const specialist: AgentRoleTemplate = {
  key: "reviewer", version: 1, name: "Default reviewer", title: "Code review",
  description: "Review changes", autonomy_level: "observer", avatar_emoji: "",
  max_concurrent_tasks: 1, skill_names: [], instructions: "Read only",
};
const template: SquadTemplate = {
  key: "delivery", version: 1, name: "Delivery", title: "Delivery", description: "",
  avatar_emoji: "", instructions: "",
  leader: { template_key: "delivery-lead", title: "Delivery lead", name: "Lead", autonomy_level: "coordinator", avatar_emoji: "", role: "Route work" },
  members: [{ template_key: "reviewer", title: "Review seat", name: "Reviewer", autonomy_level: "observer", avatar_emoji: "", role: "Review" }],
};

function squad(id: string, overrides: Partial<Squad> = {}): Squad {
  return {
    id, workspace_id: "workspace", name: id, description: "", instructions: "",
    avatar_url: null, leader_id: `${id}-leader`, creator_id: "owner", created_at: "",
    updated_at: "", archived_at: null, archived_by: null, ...overrides,
  };
}

function member(squadId: string, memberId: string, memberType: "agent" | "member" = "agent"): SquadMember {
  return { id: `${squadId}-${memberId}`, squad_id: squadId, member_type: memberType, member_id: memberId, role: "", created_at: "" };
}

describe("agent role provenance", () => {
  it("resolves localized specialist and unlisted leader identities from both catalogs", () => {
    const metadata = buildAgentRoleMetadata([specialist], [template]);
    expect(resolveAgentRole({ template_key: "reviewer" }, metadata)).toEqual({
      templateKey: "reviewer", title: "Code review", kind: "specialist",
    });
    expect(resolveAgentRole({ template_key: "delivery-lead" }, metadata)).toEqual({
      templateKey: "delivery-lead", title: "Delivery lead", kind: "coordinator",
    });
  });

  it("does not infer roles from editable names, unknown keys or declared autonomy", () => {
    const metadata = buildAgentRoleMetadata([specialist], [template]);
    const renamed = { template_key: "reviewer", name: "My renamed agent", autonomy_level: "operator" };
    expect(resolveAgentRole(renamed, metadata)?.kind).toBe("specialist");
    expect(resolveAgentRole({ template_key: "unknown-lead" }, metadata)).toBeNull();
    expect(resolveAgentRole({}, metadata)).toBeNull();
    expect(resolveAgentRole({ template_key: "reviewer" }, new Map())).toBeNull();
  });

  it("uses roster metadata when a specialist is absent from the role catalog, ignoring blank keys", () => {
    const metadata = buildAgentRoleMetadata([], [template, { ...template, leader: { ...template.leader, template_key: "" } }]);
    expect(resolveAgentRole({ template_key: "reviewer" }, metadata)?.title).toBe("Review seat");
    expect(metadata.has("")).toBe(false);
  });
});

describe("complete agent squad membership", () => {
  it("keeps shared specialists and members beyond the three-person preview", () => {
    const result = buildAgentSquadMemberships([
      squad("one", { agent_member_ids: ["one-leader", "a", "b", "shared"], member_preview: [] }),
      squad("two", { agent_member_ids: ["shared", "two-leader", "shared"] }),
    ]);
    expect(result.byAgent.get("shared")).toEqual([
      { squadId: "one", name: "one", isLeader: false },
      { squadId: "two", name: "two", isLeader: false },
    ]);
    expect(result.byAgent.get("one-leader")).toEqual([{ squadId: "one", name: "one", isLeader: true }]);
    expect(result.incompleteSquadIds.size).toBe(0);
  });

  it("treats legacy previews as incomplete and confirms only the leader", () => {
    const result = buildAgentSquadMemberships([squad("old", {
      member_count: 5,
      member_preview: [{ member_type: "agent", member_id: "preview-only", role: "" }],
    })]);
    expect(result.incompleteSquadIds).toEqual(new Set(["old"]));
    expect(result.byAgent.has("preview-only")).toBe(false);
    expect(result.byAgent.get("old-leader")?.[0]?.isLeader).toBe(true);
  });

  it("completes selected legacy squads from members data and excludes people and other squads", () => {
    const result = buildAgentSquadMemberships([squad("old")], new Map([
      ["old", [member("old", "shared"), member("old", "person", "member"), member("wrong", "foreign")]],
    ]));
    expect(result.incompleteSquadIds.size).toBe(0);
    expect(result.byAgent.get("shared")?.[0]?.squadId).toBe("old");
    expect(result.byAgent.has("person")).toBe(false);
    expect(result.byAgent.has("foreign")).toBe(false);
  });

  it("prefers full list data over stale fallback cache and omits archived squads", () => {
    const result = buildAgentSquadMemberships([
      squad("new", { agent_member_ids: [] }),
      squad("archived", { archived_at: "2026-01-01", agent_member_ids: ["retired"] }),
    ], new Map([["new", [member("new", "removed")]]]));
    expect(result.incompleteSquadIds.size).toBe(0);
    expect(result.byAgent.has("removed")).toBe(false);
    expect(result.byAgent.has("retired")).toBe(false);
    expect(result.byAgent.has("new-leader")).toBe(true);
  });
});
