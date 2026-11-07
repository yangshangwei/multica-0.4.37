// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { McpServerTemplate } from "@multica/core/types";
import { filterMcpTemplates, isUsableMcpTemplate, matchesMcpTemplate, mcpTemplateIdentity } from "./mcp-catalog";

const builtin: McpServerTemplate = { key: "browser", title: "Browser", description: "Search pages", config: { command: "npx" }, category: "browser" };
const deployment: McpServerTemplate = { ...builtin, source: "deployment", transport: "http", config: {}, version: "sha256:abc", category: "coding" };

describe("MCP catalog source identity and filtering", () => {
  it("accepts legacy builtins and public deployment recipes but fails closed for unknown sources or incomplete deployments", () => {
    expect(isUsableMcpTemplate(builtin)).toBe(true);
    expect(isUsableMcpTemplate(deployment)).toBe(true);
    for (const entry of [{ ...deployment, source: "future" }, { ...deployment, version: "" }, { ...deployment, transport: "future" }, { ...builtin, config: {} }]) {
      expect(isUsableMcpTemplate(entry)).toBe(false);
    }
  });
  it("matches only full source/key identity with builtin compatibility for older summaries", () => {
    expect(mcpTemplateIdentity(builtin)).not.toBe(mcpTemplateIdentity(deployment));
    expect(matchesMcpTemplate({ template_key: "browser" }, builtin)).toBe(true);
    expect(matchesMcpTemplate({ template_source: "deployment", template_key: "browser" }, builtin)).toBe(false);
    expect(matchesMcpTemplate({ template_source: null, template_key: "browser" }, builtin)).toBe(false);
    expect(matchesMcpTemplate({ template_source: "deployment", template_key: "browser" }, deployment)).toBe(true);
  });
  it("composes source, category and case-insensitive text without changing the inventory", () => {
    const templates = [builtin, deployment];
    expect(filterMcpTemplates(templates, "deployment", "coding", " SEARCH ")).toEqual([deployment]);
    expect(filterMcpTemplates(templates, "builtin", "coding", "")).toEqual([]);
    expect(templates).toHaveLength(2);
  });
});
