// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parse as parseYaml } from "yaml";
import type { SkillTemplate } from "../types";
import {
  buildSkillTemplateCreateRequest,
  createSkillTemplateDraft,
  parseFrontmatter,
} from "./index";

const NAME = "multica-code-review";
const BODY = "\n# 代码审查\n\n保留原文。  \n\n---\n\n正文中的分隔线\n";

function makeTemplate(overrides: Partial<SkillTemplate> = {}): SkillTemplate {
  return {
    name: NAME,
    version: 3,
    description: "Review changes",
    content: `---\nname: ${NAME}\ndescription: Review changes\nuser-invocable: false\n---\n${BODY}`,
    files: [{ path: "references/checks.md", content: "# 检查\n" }],
    ...overrides,
  };
}

function yamlFields(content: string) {
  const start = content.indexOf("\n") + 1;
  const end = content.indexOf("\n---", start);
  expect(end).toBeGreaterThan(start);
  return parseYaml(content.slice(start, end + 1));
}

describe("createSkillTemplateDraft", () => {
  it.each([
    { existing: [], expected: `${NAME}-copy` },
    { existing: [NAME, "multica-security-review"], expected: `${NAME}-copy` },
    { existing: [`${NAME}-copy`], expected: `${NAME}-copy-2` },
    { existing: [`${NAME}-copy`, `${NAME}-copy-2`], expected: `${NAME}-copy-3` },
    { existing: [`${NAME}-copy-2`], expected: `${NAME}-copy` },
  ])("uses an available independent name: $expected", ({ existing, expected }) => {
    const names = Object.freeze(existing);
    expect(createSkillTemplateDraft(makeTemplate(), names).name).toBe(expected);
  });

  it("captures source content and the chosen description without changing body bytes", () => {
    const template = makeTemplate();
    expect(createSkillTemplateDraft(template, [], "审查修改并报告问题")).toEqual({
      templateName: NAME,
      templateVersion: 3,
      templateCategory: null,
      templateIcon: null,
      sourceContent: template.content,
      name: `${NAME}-copy`,
      description: "审查修改并报告问题",
      body: BODY,
      files: [{ path: "references/checks.md", content: "# 检查\n" }],
    });
  });

  it("uses the catalog description unless an explicit description is provided", () => {
    expect(createSkillTemplateDraft(makeTemplate(), []).description).toBe("Review changes");
    expect(createSkillTemplateDraft(makeTemplate(), [], "").description).toBe("");
  });

  it("copies each supporting file so editing the draft cannot mutate the catalog", () => {
    const template = makeTemplate();
    const original = structuredClone(template);
    Object.freeze(template.files[0]);
    Object.freeze(template.files);
    Object.freeze(template);

    const draft = createSkillTemplateDraft(template, []);
    draft.files[0]!.path = "references/changed.md";
    draft.files[0]!.content = "Changed";
    draft.files.push({ path: "notes.md", content: "New" });
    draft.name = "my-review";
    draft.body = "Changed body";

    expect(template).toEqual(original);
  });

  it.each([
    "# Missing frontmatter",
    "---\nname: unterminated\n",
    "---\nname: [broken\n---\nbody",
    "---\nname: first\nname: second\n---\nbody",
    "---\n- not\n- a-map\n---\nbody",
    "---\nnot-a-map\n---\nbody",
    "---\n---\nbody",
    "---\nname: valid\n---not-a-boundary\nbody",
    "---\nname: valid\nmetadata: *missing-anchor\n---\nbody",
  ])("rejects invalid frontmatter %# with an actionable error", (content) => {
    expect(() => createSkillTemplateDraft(makeTemplate({ content }), []))
      .toThrow(/YAML|frontmatter/i);
  });

  it.each(["", "  \n"])("rejects an empty catalog name %#", (name) => {
    expect(() => createSkillTemplateDraft(makeTemplate({ name }), [])).toThrow(/name/i);
  });

  it.each(["", " \r\n"])("rejects an empty template body %#", (body) => {
    expect(() => createSkillTemplateDraft(makeTemplate({
      content: `---\nname: ${NAME}\n---\n${body}`,
    }), [])).toThrow(/body/i);
  });
});

describe("buildSkillTemplateCreateRequest", () => {
  it("keeps unknown metadata values that alias the original name or description", () => {
    const draft = createSkillTemplateDraft(makeTemplate({
      content: [
        "---",
        `name: &original-name ${NAME}`,
        "description: &original-description Original description",
        "metadata:",
        "  source-name: *original-name",
        "  source-description: *original-description",
        "---",
        BODY,
      ].join("\n"),
    }), [], "Edited description");

    const request = buildSkillTemplateCreateRequest(draft);

    expect(yamlFields(request.content!)).toEqual({
      name: `${NAME}-copy`,
      description: "Edited description",
      metadata: { "source-name": NAME, "source-description": "Original description" },
    });
  });

  it("synchronizes metadata while preserving unknown YAML values and types", () => {
    const template = makeTemplate({
      content: [
        "---",
        `name: ${NAME}`,
        "description: >",
        "  Old folded",
        "  description",
        "user-invocable: false",
        "disable-model-invocation: true",
        "allowed-tools:",
        "  - Read",
        "  - Bash",
        "metadata:",
        "  enabled: false",
        "  retries: 3",
        "  labels: [alpha, beta]",
        "  nested:",
        "    budget: 1.5",
        "    optional: null",
        '    literal: "false"',
        "future-null: null",
        'future-string: "00123"',
        "---",
        BODY,
      ].join("\n"),
    });
    const draft = createSkillTemplateDraft(template, []);
    draft.name = "  my-custom-review  ";
    draft.description = '首行: "检查"\n第二行：\n  缩进与冒号: value\n';
    draft.body = "\n# 我的审查\n\n---\n\n保留末尾空格。  ";

    const request = buildSkillTemplateCreateRequest(draft);
    const { name: _oldName, description: _oldDescription, ...originalFields } = yamlFields(template.content);

    expect(request.name).toBe("my-custom-review");
    expect(request.description).toBe(draft.description);
    expect(yamlFields(request.content!)).toEqual({
      ...originalFields,
      name: request.name,
      description: request.description,
    });
    expect(parseFrontmatter(request.content!).body).toBe(draft.body);
  });

  it.each([
    '引用 "双引号" 与单引号\'，冒号: value',
    "第一行\n第二行\n",
    "第一行\r\n第二行",
    "true",
    " leading and trailing spaces ",
    "",
  ])("round-trips an edited description exactly: %#", (description) => {
    const draft = createSkillTemplateDraft(makeTemplate(), [], description);
    draft.name = "true";
    const request = buildSkillTemplateCreateRequest(draft);

    expect(yamlFields(request.content!)).toMatchObject({ name: "true", description });
    expect(request.description).toBe(description);
  });

  it("keeps CRLF body bytes and the source frontmatter line endings", () => {
    const body = "\r\n# 原文\r\n\r\n---\r\n尾部空格  \r\n";
    const template = makeTemplate({
      content: `---\r\nname: ${NAME}\r\ndescription: Original\r\nuser-invocable: false\r\n---\r\n${body}`,
    });
    const draft = createSkillTemplateDraft(template, []);
    expect(draft.body).toBe(body);
    draft.body = "\r\n# 已编辑\r\n\n混合换行\r\n---\r\n结尾  ";

    const request = buildSkillTemplateCreateRequest(draft);
    expect(request.content!.startsWith("---\r\n")).toBe(true);
    expect(request.content!.slice(0, request.content!.indexOf("\r\n---\r\n", 5)))
      .not.toMatch(/(?<!\r)\n/);
    expect(parseFrontmatter(request.content!).body).toBe(draft.body);
    expect(yamlFields(request.content!)["user-invocable"]).toBe(false);
  });

  it("copies files again and records informational provenance without official origin", () => {
    const template = {
      ...makeTemplate(),
      config: { origin: { type: "builtin_role_skill", name: NAME } },
    };
    const draft = createSkillTemplateDraft(template, []);
    const before = structuredClone(draft);
    Object.freeze(draft.files[0]);
    Object.freeze(draft.files);
    Object.freeze(draft);

    const request = buildSkillTemplateCreateRequest(draft);
    expect(request.config).toEqual({ template_source: { name: NAME, version: 3 } });
    expect(request.files).toEqual(template.files);
    expect(request.files).not.toBe(draft.files);
    expect(request.files![0]).not.toBe(draft.files[0]);
    request.files![0]!.content = "Changed request";
    request.files!.push({ path: "notes.md", content: "New" });

    expect(draft).toEqual(before);
    expect(template.files[0]!.content).toBe("# 检查\n");
  });

  it.each(["", " \n"])("rejects an edited empty name %#", (name) => {
    const draft = createSkillTemplateDraft(makeTemplate(), []);
    draft.name = name;
    expect(() => buildSkillTemplateCreateRequest(draft)).toThrow(/name/i);
  });

  it.each(["", " \r\n"])("rejects an edited empty body %#", (body) => {
    const draft = createSkillTemplateDraft(makeTemplate(), []);
    draft.body = body;
    expect(() => buildSkillTemplateCreateRequest(draft)).toThrow(/body/i);
  });

  it("revalidates frontmatter when building a request", () => {
    const draft = createSkillTemplateDraft(makeTemplate(), []);
    draft.sourceContent = "---\nname: [broken\n---\nbody";
    expect(() => buildSkillTemplateCreateRequest(draft)).toThrow(/YAML|frontmatter/i);
  });
});

describe("template presentation defaults", () => {
  it("carries valid category/icon into config.presentation and drops unknown values", () => {
    const template = { ...makeTemplate(), category: "engineering", icon: "git-pull-request" };
    const draft = createSkillTemplateDraft(template, []);
    expect(draft.templateCategory).toBe("engineering");
    expect(draft.templateIcon).toBe("git-pull-request");
    expect(buildSkillTemplateCreateRequest(draft).config).toEqual({
      template_source: { name: template.name, version: template.version },
      presentation: { category: "engineering", icon: "git-pull-request" },
    });

    const bogus = createSkillTemplateDraft({ ...makeTemplate(), category: "nope", icon: "x" }, []);
    expect(bogus.templateCategory).toBeNull();
    expect(buildSkillTemplateCreateRequest(bogus).config).toEqual({
      template_source: { name: template.name, version: template.version },
    });
  });
});
