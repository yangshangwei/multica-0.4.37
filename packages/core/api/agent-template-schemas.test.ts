// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  AgentApprovalListResponseSchema,
  AgentApprovalSchema,
  AgentRoleTemplateListResponseSchema,
  EMPTY_AGENT_APPROVAL,
  EMPTY_AGENT_APPROVAL_LIST,
  EMPTY_AGENT_ROLE_TEMPLATE_LIST,
  EMPTY_SQUAD_TEMPLATE_LIST,
  EMPTY_STAFFED_SQUAD,
  SquadTemplateListResponseSchema,
  StaffedSquadSchema,
} from "./schemas";
import { parseWithFallback } from "./schema";

// Boundary defence for the template and approval endpoints: a drifted or hostile
// payload must degrade, never throw, and an unknown enum value must survive as
// text so the UI's default branch can handle it.

const ROLE_TEMPLATE = {
  key: "implementer",
  version: 1,
  name: "Implementer",
  title: "实现工程师",
  description: "Writes the code",
  autonomy_level: "contributor",
  avatar_emoji: "🛠️",
  max_concurrent_tasks: 1,
  skill_names: ["multica-test-report"],
  instructions: "# Implementer",
};

describe("AgentRoleTemplateListResponseSchema", () => {
  it("parses a full payload", () => {
    const parsed = parseWithFallback(
      { templates: [ROLE_TEMPLATE] },
      AgentRoleTemplateListResponseSchema,
      { templates: EMPTY_AGENT_ROLE_TEMPLATE_LIST },
      { endpoint: "test" },
    );
    expect(parsed.templates[0]).toMatchObject({
      key: "implementer",
      autonomy_level: "contributor",
      skill_names: ["multica-test-report"],
    });
  });

  it("fills every optional field a leaner backend omits", () => {
    const parsed = parseWithFallback(
      { templates: [{ key: "architect" }] },
      AgentRoleTemplateListResponseSchema,
      { templates: EMPTY_AGENT_ROLE_TEMPLATE_LIST },
      { endpoint: "test" },
    );
    expect(parsed.templates[0]).toMatchObject({
      key: "architect",
      title: "",
      instructions: "",
      skill_names: [],
      max_concurrent_tasks: 1,
    });
  });

  it("keeps an autonomy level it does not recognise", () => {
    // Widening the vocabulary must not blank the picker; the UI decides what to
    // do with an unknown level.
    const parsed = parseWithFallback(
      { templates: [{ ...ROLE_TEMPLATE, autonomy_level: "supervisor" }] },
      AgentRoleTemplateListResponseSchema,
      { templates: EMPTY_AGENT_ROLE_TEMPLATE_LIST },
      { endpoint: "test" },
    );
    expect(parsed.templates[0]?.autonomy_level).toBe("supervisor");
  });

  it("falls back on a malformed payload instead of throwing", () => {
    for (const malformed of [null, "nope", { templates: "nope" }, { templates: [{}] }]) {
      const parsed = parseWithFallback(
        malformed,
        AgentRoleTemplateListResponseSchema,
        { templates: EMPTY_AGENT_ROLE_TEMPLATE_LIST },
        { endpoint: "test" },
      );
      expect(parsed.templates).toEqual([]);
    }
  });
});

describe("SquadTemplateListResponseSchema", () => {
  it("defaults an absent leader to an empty seat rather than failing", () => {
    const parsed = parseWithFallback(
      { templates: [{ key: "bug-fix", members: [{ template_key: "implementer" }] }] },
      SquadTemplateListResponseSchema,
      { templates: EMPTY_SQUAD_TEMPLATE_LIST },
      { endpoint: "test" },
    );
    const template = parsed.templates[0];
    expect(template?.key).toBe("bug-fix");
    // An empty leader key is what the UI checks before offering to staff.
    expect(template?.leader.template_key).toBe("");
    expect(template?.members[0]).toMatchObject({
      template_key: "implementer",
      role: "",
    });
  });

  it("falls back on a malformed payload", () => {
    const parsed = parseWithFallback(
      { templates: [{ members: 3 }] },
      SquadTemplateListResponseSchema,
      { templates: EMPTY_SQUAD_TEMPLATE_LIST },
      { endpoint: "test" },
    );
    expect(parsed.templates).toEqual([]);
  });
});

describe("StaffedSquadSchema", () => {
  const squad = {
    id: "squad-1",
    workspace_id: "ws-1",
    name: "Bug Fix Squad",
    leader_id: "agent-1",
    creator_id: "user-1",
    created_at: "2026-09-01T00:00:00Z",
    updated_at: "2026-09-01T00:00:00Z",
    template_key: "bug-fix",
    template_version: 1,
  };

  it("parses the created / reused split", () => {
    const parsed = parseWithFallback(
      {
        squad,
        created_agent_ids: ["agent-1"],
        reused_agent_ids: ["agent-2", "agent-3"],
      },
      StaffedSquadSchema,
      EMPTY_STAFFED_SQUAD,
      { endpoint: "test" },
    );
    expect(parsed.squad.template_key).toBe("bug-fix");
    expect(parsed.created_agent_ids).toEqual(["agent-1"]);
    expect(parsed.reused_agent_ids).toHaveLength(2);
  });

  it.each([
    { scenario: "all-new agents", created: ["agent-1"], reused: null },
    { scenario: "all-reused agents", created: null, reused: ["agent-1"] },
    { scenario: "omitted agent lists", created: undefined, reused: undefined },
    { scenario: "null agent lists", created: null, reused: null },
  ])("preserves the created squad with $scenario", ({ created, reused }) => {
    const parsed = parseWithFallback(
      { squad, created_agent_ids: created, reused_agent_ids: reused },
      StaffedSquadSchema,
      EMPTY_STAFFED_SQUAD,
      { endpoint: "POST /api/squads/from-template" },
    );

    expect(parsed.squad).toMatchObject(squad);
    expect(parsed.created_agent_ids).toEqual(created ?? []);
    expect(parsed.reused_agent_ids).toEqual(reused ?? []);
  });

  it.each([null, {}, { ...squad, id: 42 }])(
    "still rejects an invalid squad %j when the agent lists are null",
    (invalidSquad) => {
      expect(parseWithFallback(
        { squad: invalidSquad, created_agent_ids: null, reused_agent_ids: null },
        StaffedSquadSchema,
        EMPTY_STAFFED_SQUAD,
        { endpoint: "POST /api/squads/from-template" },
      )).toBe(EMPTY_STAFFED_SQUAD);
    },
  );

  it.each(["agent-1", [42]])("rejects a malformed agent list %j", (ids) => {
    expect(StaffedSquadSchema.safeParse({
      squad,
      created_agent_ids: ids,
      reused_agent_ids: [],
    }).success).toBe(false);
  });

  it("falls back when the squad is missing", () => {
    const parsed = parseWithFallback(
      { created_agent_ids: ["agent-1"] },
      StaffedSquadSchema,
      EMPTY_STAFFED_SQUAD,
      { endpoint: "test" },
    );
    // A staffing result with no squad is unusable, so the caller sees the empty
    // shape and its `squad.id === ""` guard rather than a half-real object.
    expect(parsed.squad.id).toBe("");
    expect(parsed.created_agent_ids).toEqual([]);
  });
});

describe("AgentApprovalSchema", () => {
  it("normalizes absent nullable timestamps to null", () => {
    const parsed = parseWithFallback(
      { id: "approval-1", risk_class: "production_release", status: "pending" },
      AgentApprovalSchema,
      EMPTY_AGENT_APPROVAL,
      { endpoint: "test" },
    );
    expect(parsed.decided_at).toBeNull();
    expect(parsed.decided_by).toBeNull();
    expect(parsed.executed_at).toBeNull();
    expect(parsed.plan).toBe("");
  });

  it("defaults a missing status to pending", () => {
    // Defaulting to pending is the safe direction: an approval whose status did
    // not survive the wire must not read as authorization.
    const parsed = parseWithFallback(
      { id: "approval-1" },
      AgentApprovalSchema,
      EMPTY_AGENT_APPROVAL,
      { endpoint: "test" },
    );
    expect(parsed.status).toBe("pending");
  });

  it("drops the whole list on a malformed payload", () => {
    const parsed = parseWithFallback(
      { approvals: [{ risk_class: "production_release" }] },
      AgentApprovalListResponseSchema,
      { approvals: EMPTY_AGENT_APPROVAL_LIST },
      { endpoint: "test" },
    );
    expect(parsed.approvals).toEqual([]);
  });
});
