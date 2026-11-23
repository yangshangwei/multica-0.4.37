// @vitest-environment node
import { describe, expect, it } from "vitest";
import { AgentSchema, StoredAgentDraftSchema } from "./schemas";

describe("agent category response schemas", () => {
  it("preserves category, permissions, template provenance and unknown additive fields", () => {
    const source = {
      id: "agent-1", category: "研发", permission_mode: "public_to", visibility: "workspace",
      invocation_targets: [{ target_type: "workspace", target_id: "workspace-1" }],
      template_key: "implementer", template_version: 2, autonomy_level: "future-level",
      future_config: { enabled: true }, runtime_config: { provider: "test" },
      custom_args: ["--verbose"], skills: [{ id: "skill-1", name: "Review", description: "" }],
    };
    expect(AgentSchema.parse(source)).toMatchObject(source);
  });

  it.each([undefined, null, 42, {}, ["研发"]])("restores malformed stored category %j without losing the name", (category) => {
    expect(StoredAgentDraftSchema.parse({ name: "Reviewer", category })).toMatchObject({
      name: "Reviewer", category: "",
    });
  });

  it("preserves unfinished category text in a stored draft", () => {
    expect(StoredAgentDraftSchema.parse({ category: "  研发  " }).category).toBe("  研发  ");
  });
});
