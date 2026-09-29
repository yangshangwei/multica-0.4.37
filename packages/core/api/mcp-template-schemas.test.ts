// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  EMPTY_MCP_SERVER_TEMPLATE_LIST,
  McpServerTemplateListResponseSchema,
} from "./schemas";
import { parseWithFallback } from "./schema";

// Boundary defence for the built-in MCP catalog: a drifted, empty, or hostile
// payload must degrade to an empty list, never throw. The add flow reads this,
// so a bad response must not take the whole MCP settings screen down.

const MCP_TEMPLATE = {
  key: "chrome-devtools",
  title: "Chrome DevTools",
  description: "Drive a real Chrome.",
  config: { command: "npx", args: ["-y", "chrome-devtools-mcp@latest"] },
};

function parse(input: unknown) {
  return parseWithFallback(
    input,
    McpServerTemplateListResponseSchema,
    { templates: EMPTY_MCP_SERVER_TEMPLATE_LIST },
    { endpoint: "test" },
  );
}

describe("McpServerTemplateListResponseSchema", () => {
  it("parses a full payload with the config served in full", () => {
    const parsed = parse({ templates: [MCP_TEMPLATE] });
    expect(parsed.templates[0]).toMatchObject({
      key: "chrome-devtools",
      title: "Chrome DevTools",
      config: { command: "npx", args: ["-y", "chrome-devtools-mcp@latest"] },
    });
  });

  it("fills copy an older backend omits but keeps the key strict", () => {
    const parsed = parse({ templates: [{ key: "context7" }] });
    expect(parsed.templates[0]).toMatchObject({
      key: "context7",
      title: "",
      description: "",
      config: {},
    });
  });

  it("treats a null templates collection as empty (Go serializes []T as null)", () => {
    expect(parse({ templates: null }).templates).toEqual([]);
    expect(parse({}).templates).toEqual([]);
  });

  it("falls back instead of throwing on a malformed payload", () => {
    for (const malformed of [
      null,
      "nope",
      { templates: "nope" },
      { templates: [{}] }, // a template with no key is unusable
      { templates: [{ key: "", config: {} }] }, // blank key would not save
      { templates: [{ key: "ok", config: "not-an-object" }] },
    ]) {
      expect(parse(malformed).templates).toEqual([]);
    }
  });
});

it("maps market metadata and tolerates malformed optional metadata", () => {
  const parsed = parse({ templates: [{
    ...MCP_TEMPLATE, version: "1", category: "browser", requirements: ["Node.js"],
    documentation_url: "https://github.com/ChromeDevTools/chrome-devtools-mcp",
  }] });
  expect(parsed.templates[0]).toMatchObject({
    version: "1", category: "browser", requirements: ["Node.js"],
    documentationUrl: "https://github.com/ChromeDevTools/chrome-devtools-mcp",
  });
  const malformed = parse({ templates: [{ ...MCP_TEMPLATE,
    version: 2, category: false, requirements: "bad", documentation_url: ["bad"],
  }] });
  expect(malformed.templates[0]).toMatchObject({ key: MCP_TEMPLATE.key });
  expect(malformed.templates[0]?.version).toBeUndefined();
  expect(malformed.templates[0]?.category).toBeUndefined();
  expect(malformed.templates[0]?.requirements).toBeUndefined();
  expect(malformed.templates[0]?.documentationUrl).toBeUndefined();
});
