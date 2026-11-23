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

it("parses required secret inputs without inventing values for older catalogs", () => {
  const input = { key: "database_url", label: "Database connection URL", description: "Connect to your database.", required: true, secret: true };
  expect(parse({ templates: [{ ...MCP_TEMPLATE, inputs: [input] }] }).templates[0]?.inputs).toEqual([input]);
  expect(parse({ templates: [MCP_TEMPLATE] }).templates[0]?.inputs).toEqual([]);
  expect(parse({ templates: [{ ...MCP_TEMPLATE, inputs: null }] }).templates[0]?.inputs).toEqual([]);
});

it("rejects malformed input definitions instead of dropping required fields", () => {
  const input = { key: "database_url", label: "Database connection URL", description: "", required: true, secret: true };
  for (const inputs of ["bad", [{}], [{ ...input, required: "true" }], [{ ...input, secret: "true" }], [{ ...input, key: "" }], [input, input]]) {
    expect(parse({ templates: [{ ...MCP_TEMPLATE, inputs }] }).templates).toEqual([]);
  }
});

it("normalizes legacy sources while preserving explicit unknown sources", () => {
  expect(parse({ templates: [MCP_TEMPLATE] }).templates[0]?.source).toBe("builtin");
  expect(parse({ templates: [{ ...MCP_TEMPLATE, source: "future" }] }).templates[0]?.source).toBe("future");
  expect(parse({ templates: [{ ...MCP_TEMPLATE, source: null }] }).templates).toEqual([]);
});

it("accepts deployment metadata without public configuration and strips private fields", () => {
  const deployment = {
    key: "company-search", source: "deployment", version: "sha256:opaque", transport: "http",
    config: { url: "https://private.test", headers: { Authorization: "hidden" } },
    target: { kind: "header", name: "Authorization" },
    inputs: [{ key: "token", label: "Token", required: true, secret: true,
      target: { kind: "header", name: "Authorization" } }],
  };
  const parsed = parse({ templates: [deployment] }).templates[0];
  expect(parsed).toMatchObject({ source: "deployment", version: "sha256:opaque", transport: "http", config: {} });
  expect(parsed).not.toHaveProperty("target");
  expect(parsed?.inputs?.[0]).not.toHaveProperty("target");
  expect(JSON.stringify(parsed)).not.toContain("hidden");
  expect(parse({ templates: [{ key: deployment.key, source: "deployment", version: deployment.version, transport: "stdio" }] }).templates).toHaveLength(1);
});

it.each([
  { version: undefined }, { version: "" }, { version: 2 },
  { transport: undefined }, { transport: "sse" }, { transport: false },
])("rejects an unusable deployment identity %j", (patch) => {
  const deployment = { key: "company-search", source: "deployment", version: "sha256:opaque", transport: "http" };
  expect(parse({ templates: [{ ...deployment, ...patch }] }).templates).toEqual([]);
});
