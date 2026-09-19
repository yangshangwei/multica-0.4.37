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

  // AC7: an operator-mounted template comes back as an ordinary catalog entry —
  // version 0, a non-built-in name, and no special provenance — and must parse
  // just like a shipped one. The existing add flow reads exactly these fields.
  it("accepts a mounted template with version 0 and a non-built-in name", () => {
    const mounted = {
      name: "team-code-style",
      version: 0,
      description: "Our house style",
      content: "---\nname: team-code-style\n---\n# Style\n",
      files: [{ path: "references/lint.md", content: "lint rules" }],
    };
    expect(parseCatalog({ templates: [mounted] })).toEqual([mounted]);
  });

  // A malformed mounted entry must not take down the whole add flow: the
  // fallback yields an empty catalog, and parsing never throws.
  it("falls back to an empty catalog when a mounted entry is malformed", () => {
    expect(() =>
      parseCatalog({ templates: [{ name: "team-code-style", version: 0, content: "" }] }),
    ).not.toThrow();
    expect(parseCatalog({ templates: [{ name: "team-code-style", version: 0, content: "" }] })).toEqual([]);
  });
});
