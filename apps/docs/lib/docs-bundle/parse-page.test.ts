// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseFrontmatter, extractToc, buildNav } from "./parse-page.mjs";

describe("parseFrontmatter", () => {
  it("reads title and description", () => {
    const src = ["---", "title: 智能体", "description: 怎么用", "---", "", "正文"].join("\n");
    expect(parseFrontmatter(src)).toEqual({
      title: "智能体",
      description: "怎么用",
      body: "正文",
    });
  });

  it("treats a missing description as absent rather than empty string", () => {
    const src = ["---", "title: 欢迎", "---", "", "正文"].join("\n");
    const out = parseFrontmatter(src);
    expect(out.title).toBe("欢迎");
    expect(out.description).toBeUndefined();
  });

  it("strips surrounding quotes from a quoted value", () => {
    const src = ['---', 'title: "带: 冒号的标题"', "---", "", "正文"].join("\n");
    expect(parseFrontmatter(src).title).toBe("带: 冒号的标题");
  });

  // A page with no frontmatter would otherwise produce an untitled nav entry.
  // Failing loudly is better than shipping a blank sidebar row.
  it("throws when frontmatter is missing", () => {
    expect(() => parseFrontmatter("# 直接正文")).toThrow(/frontmatter/i);
  });

  it("throws when title is missing", () => {
    const src = ["---", "description: 只有描述", "---", "", "正文"].join("\n");
    expect(() => parseFrontmatter(src)).toThrow(/title/i);
  });
});

describe("extractToc", () => {
  it("collects h2 and h3 with slugged ids and depth", () => {
    const body = ["## 概览", "文字", "### 细节", "更多", "## 下一步"].join("\n");
    expect(extractToc(body)).toEqual([
      { depth: 2, title: "概览", id: "概览" },
      { depth: 3, title: "细节", id: "细节" },
      { depth: 2, title: "下一步", id: "下一步" },
    ]);
  });

  it("skips h1 because the page title renders it", () => {
    expect(extractToc(["# 页面标题", "## 真正的小节"].join("\n"))).toEqual([
      { depth: 2, title: "真正的小节", id: "真正的小节" },
    ]);
  });

  it("de-duplicates repeated headings the way github-slugger does", () => {
    const body = ["## Setup", "## Setup"].join("\n");
    expect(extractToc(body).map((h) => h.id)).toEqual(["setup", "setup-1"]);
  });

  // Headings inside fenced blocks are shell comments and code, not structure.
  it("ignores headings inside fenced code blocks", () => {
    const body = ["## 真", "```bash", "# 假的注释", "## 也是假的", "```", "## 也真"].join("\n");
    expect(extractToc(body).map((h) => h.title)).toEqual(["真", "也真"]);
  });

  it("does not close a longer fenced block with a shorter fence", () => {
    const body = [
      "## 真",
      "````md",
      "```",
      "## 还是假的",
      "````",
      "## 也真",
    ].join("\n");
    expect(extractToc(body).map((h) => h.title)).toEqual(["真", "也真"]);
  });

  it("strips inline markdown from the heading text but keeps the slug on the plain text", () => {
    expect(extractToc("## 用 `--flag` 运行")).toEqual([
      { depth: 2, title: "用 --flag 运行", id: "用---flag-运行" },
    ]);
  });
});

describe("buildNav", () => {
  it("turns the meta pages array into groups of entries", () => {
    const nav = buildNav(
      {
        title: "Multica 文档",
        pages: ["---开始使用---", "index", "concepts", "---工作区---", "workspaces"],
      },
      new Map([
        ["index", { title: "欢迎", description: "起点" }],
        ["concepts", { title: "核心概念", description: undefined }],
        ["workspaces", { title: "工作区", description: "怎么组织" }],
      ]),
    );
    expect(nav).toEqual({
      title: "Multica 文档",
      groups: [
        {
          label: "开始使用",
          items: [
            { slug: "index", title: "欢迎", description: "起点" },
            { slug: "concepts", title: "核心概念" },
          ],
        },
        {
          label: "工作区",
          items: [{ slug: "workspaces", title: "工作区", description: "怎么组织" }],
        },
      ],
    });
  });

  it("puts pages listed before any separator into an unlabelled leading group", () => {
    const nav = buildNav(
      { title: "文档", pages: ["index", "---分组---", "other"] },
      new Map([
        ["index", { title: "首页" }],
        ["other", { title: "其他" }],
      ]),
    );
    expect(nav.groups[0]).toEqual({
      label: null,
      items: [{ slug: "index", title: "首页" }],
    });
  });

  // meta.zh.json lists `developers`, a directory with its own meta file. It has
  // no page of its own, so it must not become a dangling nav row.
  it("throws when a listed slug has no generated page", () => {
    expect(() =>
      buildNav({ title: "文档", pages: ["index", "ghost"] }, new Map([["index", { title: "首页" }]])),
    ).toThrow(/ghost/);
  });

  // meta.zh.json's last entry is `developers`, a subdirectory with its own meta
  // file rather than a page. It expands into its own group.
  it("expands a subdirectory entry into a group from its own meta", () => {
    const nav = buildNav(
      { title: "文档", pages: ["index", "developers"] },
      new Map([
        ["index", { title: "首页" }],
        ["developers/contributing", { title: "参与贡献", description: "怎么提 PR" }],
        ["developers/architecture", { title: "架构" }],
      ]),
      new Map([
        ["developers", { title: "参与开发", pages: ["contributing", "architecture"] }],
      ]),
    );
    expect(nav.groups).toEqual([
      { label: null, items: [{ slug: "index", title: "首页" }] },
      {
        label: "参与开发",
        items: [
          { slug: "developers/contributing", title: "参与贡献", description: "怎么提 PR" },
          { slug: "developers/architecture", title: "架构" },
        ],
      },
    ]);
  });

  // meta.zh.json really ends with "---参与开发---" then `developers`. The
  // separator is labelling the directory, so reusing the group it opened is
  // what keeps one "参与开发" section instead of an empty one plus a duplicate.
  it("fills the group a preceding separator opened instead of adding a second one", () => {
    const nav = buildNav(
      { title: "文档", pages: ["index", "---参与开发---", "developers"] },
      new Map([
        ["index", { title: "首页" }],
        ["developers/contributing", { title: "参与贡献" }],
      ]),
      new Map([["developers", { title: "参与开发", pages: ["contributing"] }]]),
    );
    expect(nav.groups).toEqual([
      { label: null, items: [{ slug: "index", title: "首页" }] },
      { label: "参与开发", items: [{ slug: "developers/contributing", title: "参与贡献" }] },
    ]);
  });

  // The separator's label wins over the directory meta's own title: the root
  // meta is what orders and labels the sidebar.
  it("keeps the separator label when it differs from the directory meta title", () => {
    const nav = buildNav(
      { title: "文档", pages: ["---开发者---", "developers"] },
      new Map([["developers/contributing", { title: "参与贡献" }]]),
      new Map([["developers", { title: "参与开发", pages: ["contributing"] }]]),
    );
    expect(nav.groups).toEqual([
      { label: "开发者", items: [{ slug: "developers/contributing", title: "参与贡献" }] },
    ]);
  });

  // A directory following a group that already has pages is a new section.
  it("starts a new group when the open group already has items", () => {
    const nav = buildNav(
      { title: "文档", pages: ["---参考---", "cli", "developers"] },
      new Map([
        ["cli", { title: "CLI" }],
        ["developers/contributing", { title: "参与贡献" }],
      ]),
      new Map([["developers", { title: "参与开发", pages: ["contributing"] }]]),
    );
    expect(nav.groups).toEqual([
      { label: "参考", items: [{ slug: "cli", title: "CLI" }] },
      { label: "参与开发", items: [{ slug: "developers/contributing", title: "参与贡献" }] },
    ]);
  });

  it("throws when a subdirectory meta lists a page that was not generated", () => {
    expect(() =>
      buildNav(
        { title: "文档", pages: ["developers"] },
        new Map([["developers/contributing", { title: "参与贡献" }]]),
        new Map([["developers", { title: "参与开发", pages: ["contributing", "ghost"] }]]),
      ),
    ).toThrow(/developers\/ghost/);
  });

  it("throws when a generated page is missing from meta", () => {
    expect(() =>
      buildNav(
        { title: "文档", pages: ["index"] },
        new Map([
          ["index", { title: "首页" }],
          ["orphan", { title: "孤儿页" }],
        ]),
      ),
    ).toThrow(/orphan/);
  });
});
