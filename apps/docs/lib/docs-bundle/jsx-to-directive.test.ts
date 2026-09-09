// @vitest-environment node
import { describe, expect, it } from "vitest";
import { jsxToDirective, UnknownJsxTagError } from "./jsx-to-directive.mjs";

describe("jsxToDirective", () => {
  it("converts a single-line Callout to a container directive", () => {
    expect(jsxToDirective(`<Callout type="info">读我</Callout>`)).toBe(
      [":::info", "读我", ":::"].join("\n"),
    );
  });

  // The whole reason this is a container directive and not a <div>: remark does
  // not parse markdown inside an HTML block, so a downgrade to HTML would
  // render the link, the bold run and the inline code as literal text in all
  // 292 callouts.
  it("keeps markdown inside the callout body parseable", () => {
    const out = jsxToDirective(
      `<Callout type="warning">见 [运行时](/daemon-runtimes)、**必读**、\`--flag\`</Callout>`,
    );
    expect(out).toBe(
      [
        ":::warning",
        "见 [运行时](/daemon-runtimes)、**必读**、`--flag`",
        ":::",
      ].join("\n"),
    );
  });

  it("collapses the warn alias onto warning", () => {
    expect(jsxToDirective(`<Callout type="warn">当心</Callout>`)).toBe(
      [":::warning", "当心", ":::"].join("\n"),
    );
  });

  // 92 of the 292 callouts carry no type attribute in the source.
  it("defaults a type-less callout to info", () => {
    expect(jsxToDirective(`<Callout>提示</Callout>`)).toBe(
      [":::info", "提示", ":::"].join("\n"),
    );
  });

  it("preserves a multi-line callout body verbatim", () => {
    const input = [
      `<Callout type="info">`,
      `第一段。`,
      ``,
      `- 一`,
      `- 二`,
      `</Callout>`,
    ].join("\n");
    expect(jsxToDirective(input)).toBe(
      [":::info", "第一段。", "", "- 一", "- 二", ":::"].join("\n"),
    );
  });

  it("converts every callout on a page, not just the first", () => {
    const input = [
      `<Callout type="info">一</Callout>`,
      ``,
      `正文`,
      ``,
      `<Callout type="warning">二</Callout>`,
    ].join("\n");
    expect(jsxToDirective(input)).toBe(
      [
        ":::info",
        "一",
        ":::",
        "",
        "正文",
        "",
        ":::warning",
        "二",
        ":::",
      ].join("\n"),
    );
  });

  it("converts VideoEmbed to a leaf directive carrying its attributes", () => {
    expect(
      jsxToDirective(`<VideoEmbed provider="bilibili" id="BV1cv7Y6gEg7" title="介绍" />`),
    ).toBe(`::video-embed{provider="bilibili" id="BV1cv7Y6gEg7" title="介绍"}`);
  });

  it("converts CommunityLinks to a leaf directive carrying its attributes", () => {
    const input = `<CommunityLinks discordDescription="甲" githubDescription="乙" xDescription="丙" />`;
    expect(jsxToDirective(input)).toBe(
      `::community-links{discordDescription="甲" githubDescription="乙" xDescription="丙"}`,
    );
  });

  it("drops the fumadocs Callout import line", () => {
    const input = [
      `import { Callout } from "fumadocs-ui/components/callout";`,
      ``,
      `## 标题`,
    ].join("\n");
    expect(jsxToDirective(input)).toBe(["## 标题"].join("\n"));
  });

  // A silent drop is the failure mode that matters here: the docs site grows a
  // fourth component, the generator ignores it, and the in-app page is quietly
  // missing a section that nobody notices until a reader asks.
  it("throws on an unregistered JSX tag instead of dropping it", () => {
    expect(() => jsxToDirective(`<Steps>一</Steps>`)).toThrow(UnknownJsxTagError);
    expect(() => jsxToDirective(`<Steps>一</Steps>`)).toThrow(/Steps/);
  });

  it("throws on an unregistered self-closing JSX tag", () => {
    expect(() => jsxToDirective(`<Mermaid chart="graph TD;" />`)).toThrow(
      UnknownJsxTagError,
    );
  });

  it("leaves plain markdown untouched", () => {
    const input = ["# 标题", "", "见 [文档](/agents)。", "", "```bash", "ls -la", "```"].join("\n");
    expect(jsxToDirective(input)).toBe(input);
  });

  // `<br />` and friends are HTML, not our components — they must pass through
  // to the markdown renderer rather than trip the unknown-tag guard.
  it("leaves lowercase HTML tags untouched", () => {
    const input = "第一行<br />第二行 <kbd>Esc</kbd>";
    expect(jsxToDirective(input)).toBe(input);
  });

  it("does not treat a tag inside a fenced code block as JSX", () => {
    const input = ["```tsx", `<Steps>一</Steps>`, "```"].join("\n");
    expect(jsxToDirective(input)).toBe(input);
  });

  it("does not treat a tag inside inline code as JSX", () => {
    const input = "写成 `<Steps>` 即可";
    expect(jsxToDirective(input)).toBe(input);
  });

  it("converts a multi-line self-closing CommunityLinks", () => {
    const input = [
      `<CommunityLinks`,
      `  discordDescription="甲"`,
      `  githubDescription="乙"`,
      `  xDescription="丙"`,
      `/>`,
    ].join("\n");
    expect(jsxToDirective(input)).toBe(
      `::community-links{discordDescription="甲" githubDescription="乙" xDescription="丙"}`,
    );
  });

  // Angle-bracket placeholders in prose (`<PROVIDER>`, `<GitHub App 的数字 ID>`)
  // are not JSX. The unknown-tag guard must not trip on them — they appear in
  // environment-variables.zh.mdx and github-integration.zh.mdx.
  it("leaves uppercase angle-bracket placeholders in prose untouched", () => {
    const input = [
      "使用 `MULTICA_<PROVIDER>_PATH` 覆盖。",
      "GITHUB_APP_ID=<GitHub App 的数字 ID>",
      "<你的命令> <Multica 的协议参数> <Agent 的自定义参数>",
    ].join("\n");
    expect(jsxToDirective(input)).toBe(input);
  });
});
