// @vitest-environment node
import { describe, expect, it } from "vitest";
import { remarkMdxMermaid } from "fumadocs-core/mdx-plugins";
import { jsxToDirective } from "./jsx-to-directive.mjs";

const chart = 'flowchart TD\n  A["检查 <条件>"] --> B["继续"]';

describe("Mermaid authoring parity", () => {
  it("keeps diagram source intact in the bundle and sends it to the public renderer", () => {
    const markdown = `\`\`\`mermaid\n${chart}\n\`\`\``;
    expect(jsxToDirective(markdown)).toBe(markdown);

    const tree: Parameters<ReturnType<typeof remarkMdxMermaid>>[0] = {
      type: "root",
      children: [{ type: "code", lang: "mermaid", value: chart }],
    };
    // This synchronous plugin only consumes the AST; no VFile or completion
    // callback is used. Exercise its runtime contract without a fake file.
    Reflect.apply(remarkMdxMermaid(), undefined, [tree]);
    expect(tree.children[0]).toMatchObject({
      type: "mdxJsxFlowElement",
      name: "Mermaid",
      attributes: [{ type: "mdxJsxAttribute", name: "chart", value: chart }],
    });
  });

  it("does not upgrade ordinary and lookalike code languages", () => {
    const tree: Parameters<ReturnType<typeof remarkMdxMermaid>>[0] = {
      type: "root",
      children: [
        { type: "code", lang: "bash", value: "make up" },
        { type: "code", lang: "mermaidx", value: chart },
      ],
    };
    const before = structuredClone(tree);
    // This synchronous plugin only consumes the AST; no VFile or completion
    // callback is used. Exercise its runtime contract without a fake file.
    Reflect.apply(remarkMdxMermaid(), undefined, [tree]);
    expect(tree).toEqual(before);
  });
});
