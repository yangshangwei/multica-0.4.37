import { describe, expect, it, vi, beforeAll } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { ApiClient } from "@multica/core/api/client";
import { setApiInstance } from "@multica/core/api";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { NavigationProvider } from "../navigation";
import type { NavigationAdapter } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { DocsContent, docsImageSrc, resolveDocsHref } from "./docs-content";

// The bundle's markdown is generated, so what matters here is that this
// renderer honours the three contracts the generator and the product rely on:
// callout bodies stay markdown, heading ids match the table of contents, and
// image/link URLs are re-pointed for an app that may run on file://.
//
// Pure URL mapping is asserted directly against the two exported helpers; the
// DOM cases below cover what only a real render can show.

const API_BASE = "https://api.example.test";

beforeAll(() => {
  setApiInstance(new ApiClient(API_BASE));
});

function adapter(overrides: Partial<NavigationAdapter> = {}): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/docs",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path: string) => `https://app.example.test${path}`,
    ...overrides,
  };
}

function renderBody(body: string, nav: Partial<NavigationAdapter> = {}) {
  return renderWithI18n(
    <WorkspaceSlugProvider slug="acme">
      <NavigationProvider value={adapter(nav)}>
        <DocsContent body={body} />
      </NavigationProvider>
    </WorkspaceSlugProvider>,
  );
}

describe("docsImageSrc", () => {
  it("rewrites a root-relative asset to the absolute API endpoint", () => {
    expect(docsImageSrc("/images/docs/agents.webp")).toBe(
      `${API_BASE}/api/docs/assets/images/docs/agents.webp`,
    );
  });

  it("leaves an absolute URL and a data URI alone", () => {
    expect(docsImageSrc("https://cdn.example.test/x.png")).toBe(
      "https://cdn.example.test/x.png",
    );
    expect(docsImageSrc("data:image/png;base64,AAA")).toBe(
      "data:image/png;base64,AAA",
    );
  });
});

describe("resolveDocsHref", () => {
  const docsPage = (slug: string, anchor?: string) =>
    anchor ? `/acme/docs/${slug}#${anchor}` : `/acme/docs/${slug}`;

  it("treats a root-relative href as a docs slug, not an app route", () => {
    expect(resolveDocsHref("/daemon-runtimes", docsPage)).toEqual({
      kind: "internal",
      href: "/acme/docs/daemon-runtimes",
    });
  });

  it("keeps a nested slug and carries an anchor through", () => {
    expect(resolveDocsHref("/developers/contributing#setup", docsPage)).toEqual({
      kind: "internal",
      href: "/acme/docs/developers/contributing#setup",
    });
  });

  it("maps the docs root to the index page", () => {
    expect(resolveDocsHref("/", docsPage)).toEqual({
      kind: "internal",
      href: "/acme/docs/index",
    });
  });

  it("leaves an in-page anchor as an anchor", () => {
    expect(resolveDocsHref("#事件过滤", docsPage)).toEqual({
      kind: "anchor",
      href: "#事件过滤",
    });
  });

  it("treats anything with a scheme as external", () => {
    expect(resolveDocsHref("https://multica.ai", docsPage).kind).toBe("external");
    expect(resolveDocsHref("mailto:hi@multica.ai", docsPage).kind).toBe("external");
  });
});

describe("DocsContent directives", () => {
  // The whole reason the generator emits `:::warning` instead of a <div>: remark
  // does not parse markdown inside an HTML block, so an HTML downgrade would
  // render these 73 callouts' links and bold runs as literal text.
  it("renders markdown inside a callout body", () => {
    renderBody(":::warning\n见 [运行时](/daemon-runtimes) 和 **粗体**\n:::");

    const link = screen.getByRole("link", { name: "运行时" });
    expect(link).toHaveAttribute("href", "/acme/docs/daemon-runtimes");
    expect(screen.getByText("粗体").tagName).toBe("STRONG");
  });

  it("keeps a callout's prose when its type is unknown to the renderer", () => {
    renderBody(":::mystery\n这段文字必须保留\n:::");
    expect(screen.getByText("这段文字必须保留")).toBeInTheDocument();
  });

  it("renders a video embed as a click-to-load facade, with no third-party iframe up front", () => {
    renderBody('::video-embed{provider="bilibili" id="BV1cv7Y6gEg7" title="介绍视频"}');

    expect(screen.getByRole("button", { name: /介绍视频/ })).toBeInTheDocument();
    expect(document.querySelector("iframe")).toBeNull();
    expect(screen.getByRole("link", { name: /Bilibili/ })).toHaveAttribute(
      "href",
      "https://www.bilibili.com/video/BV1cv7Y6gEg7/",
    );
  });

  it("renders community links with each description", () => {
    renderBody(
      '::community-links{discordDescription="和团队交流" githubDescription="提交问题" xDescription="关注更新"}',
    );

    expect(screen.getByRole("link", { name: /Discord/ })).toHaveAttribute(
      "href",
      "https://discord.gg/W8gYBn226t",
    );
    expect(screen.getByText(/和团队交流/)).toBeInTheDocument();
    expect(screen.getByText(/提交问题/)).toBeInTheDocument();
    expect(screen.getByText(/关注更新/)).toBeInTheDocument();
  });
});

describe("DocsContent heading ids", () => {
  // Product code deep links into these ids (#自定义运行时配置 from runtime-docs.ts).
  // They must be unprefixed and slugged the same way extractToc does, or the
  // anchor lands at the top of the page. docs-anchor-parity.test.ts proves the
  // generator and the product agree on the values; this proves the DOM carries
  // them verbatim.
  it("gives headings the raw slugger id, with no sanitize prefix", () => {
    renderBody("## 自定义运行时配置\n\n### Custom runtime profiles");

    expect(document.querySelector("h2")).toHaveAttribute("id", "自定义运行时配置");
    expect(document.querySelector("h3")).toHaveAttribute("id", "custom-runtime-profiles");
  });

  it("de-duplicates repeated headings the way the table of contents does", () => {
    renderBody("## 概览\n\n## 概览");

    const ids = [...document.querySelectorAll("h2")].map((h) => h.id);
    expect(ids).toEqual(["概览", "概览-1"]);
  });
});

describe("DocsContent links and images", () => {
  it("navigates an internal link in-app instead of loading a document", () => {
    const push = vi.fn();
    renderBody("[运行时](/daemon-runtimes)", { push });

    fireEvent.click(screen.getByRole("link", { name: "运行时" }));

    expect(push).toHaveBeenCalledWith("/acme/docs/daemon-runtimes");
  });

  it("opens an external link in a new tab", () => {
    const { container } = renderBody("[官网](https://multica.ai)");
    const link = container.querySelector("a");

    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", expect.stringContaining("noopener"));
  });

  it("rewrites an image to the asset endpoint and keeps its alt text", () => {
    renderBody("![工作区概览](/images/docs/workspace-overview.webp)");

    const img = screen.getByAltText("工作区概览");
    expect(img).toHaveAttribute(
      "src",
      `${API_BASE}/api/docs/assets/images/docs/workspace-overview.webp`,
    );
  });
});

describe("DocsContent blocks", () => {
  it("renders a table inside an overflow wrapper so a wide one cannot widen the page", () => {
    renderBody("| a | b |\n| --- | --- |\n| 1 | 2 |");

    const table = document.querySelector("table");
    expect(table).toBeInTheDocument();
    expect(table?.parentElement?.className).toContain("overflow-x-auto");
  });

  it("renders a fenced block through the shared code block", () => {
    renderBody("```bash\nmake up\n```");
    expect(screen.getByText(/make up/)).toBeInTheDocument();
  });

  it("does not pass raw HTML through", () => {
    renderBody("<script>window.pwned = 1</script>\n\n正文");

    expect(document.querySelector("script")).toBeNull();
    expect(screen.getByText("正文")).toBeInTheDocument();
  });
});
