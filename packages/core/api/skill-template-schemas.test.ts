// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseWithFallback } from "./schema";
import { EMPTY_SKILL_TEMPLATE_LIST, SkillTemplateListResponseSchema } from "./schemas";

const TEMPLATE = {
  name: "multica-code-review",
  version: 1,
  description: "Review changes",
  content: "---\nname: multica-code-review\n---\n# 审查\n",
  files: [{ path: "references/checks.md", content: "Checks" }],
};

function parseCatalog(raw: unknown) {
  return parseWithFallback(
    raw,
    SkillTemplateListResponseSchema,
    { templates: EMPTY_SKILL_TEMPLATE_LIST },
    { endpoint: "GET /api/skills/templates" },
  ).templates;
}

describe("SkillTemplateListResponseSchema", () => {
  it("keeps templates distinct from persisted skills and retains future fields", () => {
    const templates = parseCatalog({
      templates: [{ ...TEMPLATE, future_field: { supported: true } }],
    });
    expect(templates).toEqual([{ ...TEMPLATE, future_field: { supported: true } }]);
    expect(templates[0]).not.toHaveProperty("id");
    expect(templates[0]).not.toHaveProperty("workspace_id");
  });

  it("defaults optional presentation and supporting-file fields", () => {
    expect(parseCatalog({ templates: [{ name: TEMPLATE.name, content: TEMPLATE.content }] }))
      .toEqual([{ name: TEMPLATE.name, content: TEMPLATE.content, version: 0, description: "", files: [] }]);
  });

  it.each([
    null,
    "not-an-envelope",
    { templates: null },
    { templates: "not-an-array" },
    { templates: [{}] },
    { templates: [{ ...TEMPLATE, name: "" }] },
    { templates: [{ ...TEMPLATE, name: "  \n" }] },
    { templates: [{ ...TEMPLATE, name: 5 }] },
    { templates: [{ ...TEMPLATE, content: undefined }] },
    { templates: [{ ...TEMPLATE, content: " \r\n" }] },
    { templates: [{ ...TEMPLATE, content: {} }] },
    { templates: [{ ...TEMPLATE, version: "one" }] },
    { templates: [{ ...TEMPLATE, files: [{ path: 4, content: "bad" }] }] },
    { templates: [{ ...TEMPLATE, files: [{ path: "checks.md" }] }] },
  ])("cannot turn malformed catalog response %# into a usable blank template", (raw) => {
    expect(parseCatalog(raw)).toEqual([]);
  });
});
