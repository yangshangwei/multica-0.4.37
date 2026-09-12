import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError, setApiInstance } from "@multica/core/api";
import type { ChangelogFeed } from "@multica/core/changelog";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { ScrollRestorationProvider, type ScrollRestorationAdapter, type ScrollRestorationEntry } from "../platform/scroll-restoration";
import en from "../locales/en/changelog.json";
import { ChangelogPage } from "./changelog-page";

// Parser/selection matrices and real polling/focus/reconnect behavior live in
// core/changelog/*.test.ts. This suite owns rendered states and navigation.
vi.mock("../i18n", () => ({
  useT: () => ({ t: (selector: (resource: typeof en) => string) => selector(en) }),
  useLocale: () => "en",
}));

const fixture: ChangelogFeed = {
  schemaVersion: 1,
  generatedAt: "2026-09-01T00:00:00Z",
  serverVersion: "0.4.37-server",
  feedSource: "file",
  isStale: false,
  warning: null,
  releases: [
    {
      id: "fork:example/multica:unreleased", version: "Unreleased", title: "Committed improvements",
      source: "fork", status: "unreleased", publishedAt: null, commit: null, baseCommit: null,
      sections: [{ category: "features", items: [{ text: "Read updates without leaving the desktop" }] }],
    },
    {
      id: "upstream:multica-ai/multica:v0.4.37", version: "v0.4.37", title: "Official baseline",
      source: "upstream", status: "published", publishedAt: "2026-08-31T00:00:00Z", commit: null, baseCommit: null,
      sections: [{ category: "improvements", items: [{ text: "<script>doNotExecute()</script>" }] }],
    },
  ],
};

let query: ReturnType<typeof vi.fn<() => Promise<ChangelogFeed>>>;
let client: QueryClient;
let navigation: NavigationAdapter;

beforeEach(() => {
  query = vi.fn().mockResolvedValue(fixture);
  const api = new ApiClient("https://intranet.example.test");
  vi.spyOn(api, "getChangelog").mockImplementation(query);
  setApiInstance(api);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  navigation = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/changelog",
    searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
  };
  Element.prototype.scrollIntoView = vi.fn();
  vi.spyOn(document, "hasFocus").mockReturnValue(true);
});

afterEach(() => {
  client.clear();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

function page() {
  return (
    <QueryClientProvider client={client}>
      <WorkspaceSlugProvider slug="acme">
        <NavigationProvider value={navigation}>
          <ChangelogPage desktopVersion="0.4.37-desktop" />
        </NavigationProvider>
      </WorkspaceSlugProvider>
    </QueryClientProvider>
  );
}

// Model a heading 400px down the reader, with the app shell above it. Reading
// its rectangle after scrolling must account for the reader's current offset.
function mockReaderGeometry() {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    const offset = this.tagName === "H2" ? 400 - (this.closest("main")?.scrollTop ?? 0) : 0;
    return new DOMRect(0, 80 + offset, 600, 40);
  });
}

describe("ChangelogPage", () => {
  it("renders truthful source/status, independent versions, categorized history and plain text", async () => {
    const { container } = render(page());
    expect(await screen.findByRole("heading", { name: "Committed improvements" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Official baseline" })).toBeInTheDocument();
    expect(screen.getByText("0.4.37-desktop")).toBeInTheDocument();
    expect(screen.getByText("0.4.37-server")).toBeInTheDocument();
    expect(screen.getByText(en.versions.none)).toBeInTheDocument();
    expect(screen.getByText(en.upstream_notice)).toBeInTheDocument();
    expect(screen.getByText(en.preview_notice)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "New features" })).toBeInTheDocument();
    expect(screen.getByText("August 31, 2026")).toBeInTheDocument();
    expect(screen.getByText("<script>doNotExecute()</script>")).toBeInTheDocument();
    expect(container.querySelector("script")).toBeNull();
  });

  it("navigates exact release links through the adapter and honors the adapter hash", async () => {
    const { rerender } = render(page());
    const nav = await screen.findByRole("navigation", { name: en.navigation });
    const link = within(nav).getByRole("link", { name: /v0.4.37/ });
    const hash = `#${encodeURIComponent(fixture.releases[1]!.id)}`;
    fireEvent.click(link);
    expect(navigation.push).toHaveBeenCalledWith(`/acme/changelog${hash}`);
    navigation = { ...navigation, hash };
    rerender(page());
    await waitFor(() => expect(screen.getByRole("heading", { name: "Official baseline" })).toHaveFocus());
    expect(link).toHaveAttribute("aria-current", "location");
  });

  it("leaves history readable when an updater version is absent", async () => {
    navigation.searchParams.set("version", "999.0.0");
    render(page());
    expect(await screen.findByText(en.missing.title)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Official baseline" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: en.all_releases })).toHaveAttribute("href", "/acme/changelog");
  });

  it("labels first load, empty history and an unsupported server distinctly", async () => {
    let resolve: (value: ChangelogFeed) => void = () => {};
    query.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    render(page());
    expect(screen.getByRole("status", { name: en.loading })).toBeInTheDocument();
    await act(async () => resolve({ ...fixture, releases: [] }));
    expect(await screen.findByText(en.empty.title)).toBeInTheDocument();
  });

  it("offers retry for a server without the endpoint", async () => {
    query.mockRejectedValue(new ApiError("not found", 404, "Not Found"));
    render(page());
    expect(await screen.findByText(en.unavailable.title)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: en.error.retry })).toBeInTheDocument();
  });

  it("recovers from first-load failure via retry", async () => {
    query.mockRejectedValueOnce(new Error("offline"));
    render(page());
    expect(await screen.findByText(en.error.title)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: en.error.retry }));
    expect(await screen.findByRole("heading", { name: "Official baseline" })).toBeInTheDocument();
  });

  it("keeps content after a failed refresh and separately reports a server fallback", async () => {
    render(page());
    await screen.findByRole("heading", { name: "Official baseline" });
    query.mockRejectedValueOnce(new Error("offline"));
    fireEvent.click(screen.getByRole("button", { name: en.refresh }));
    expect(await screen.findByText(en.refresh_error)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Official baseline" })).toBeInTheDocument();
    query.mockResolvedValue({ ...fixture, isStale: true, warning: "changelog_file_invalid" });
    fireEvent.click(screen.getByRole("button", { name: en.refresh }));
    expect(await screen.findByText(en.stale_warning)).toBeInTheDocument();
    expect(screen.queryByText(en.refresh_error)).not.toBeInTheDocument();
  });

  it("refreshes when the real window regains focus, including a kept-alive route activation", async () => {
    const { rerender } = render(page());
    await screen.findByRole("heading", { name: "Official baseline" });
    vi.mocked(document.hasFocus).mockReturnValue(false);
    fireEvent.blur(window);
    vi.mocked(document.hasFocus).mockReturnValue(true);
    fireEvent.focus(window);
    await waitFor(() => expect(query).toHaveBeenCalledTimes(2));
    navigation = { ...navigation, pathname: "/acme/issues" };
    rerender(page());
    navigation = { ...navigation, pathname: "/acme/changelog", hash: `#${encodeURIComponent(fixture.releases[1]!.id)}` };
    rerender(page());
    await waitFor(() => expect(query).toHaveBeenCalledTimes(3));
  });

  it("restores desktop reading position once and does not reset it on refresh", async () => {
    render(<ScrollRestorationProvider adapter={{ get: () => ({ top: 20, height: 500 }) }}>{page()}</ScrollRestorationProvider>);
    await screen.findByRole("heading", { name: "Official baseline" });
    const main = screen.getByRole("main");
    expect(main.scrollTop).toBe(20);
    main.scrollTop = 70;
    fireEvent.click(screen.getByRole("button", { name: en.refresh }));
    await waitFor(() => expect(query).toHaveBeenCalledTimes(2));
    expect(main.scrollTop).toBe(70);
  });

  it("still refreshes a visible active reader while another window holds OS focus", async () => {
    vi.useFakeTimers();
    render(page());
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(query).toHaveBeenCalledTimes(1);
    vi.mocked(document.hasFocus).mockReturnValue(false);
    fireEvent.blur(window);
    await act(() => vi.advanceTimersByTimeAsync(60_000));
    expect(query).toHaveBeenCalledTimes(2);
  });

  it.each(["hash", "version"] as const)("resumes the saved reading position on a desktop remount with a retained %s", async (selection) => {
    const release = { ...fixture.releases[1]!, id: "fork:example/multica:v0.4.37", source: "fork" };
    query.mockResolvedValue({ ...fixture, releases: [release] });
    if (selection === "hash") navigation.hash = `#${encodeURIComponent(release.id)}`;
    else navigation.searchParams.set("version", "0.4.37");
    const saved: { scroll?: ScrollRestorationEntry; view: Map<string, string> } = { view: new Map() };
    const adapter: ScrollRestorationAdapter = {
      get: () => saved.scroll,
      getViewState: (key) => saved.view.get(key),
      setViewState: (key, value) => { if (value === undefined) saved.view.delete(key); else saved.view.set(key, value); },
    };
    const mount = () => render(<ScrollRestorationProvider adapter={adapter}>{page()}</ScrollRestorationProvider>);
    const first = mount();
    await screen.findByRole("heading", { name: "Official baseline" });
    screen.getByRole("main").scrollTop = 120;
    saved.scroll = { top: 120, height: 500 };
    first.unmount();
    vi.mocked(Element.prototype.scrollIntoView).mockClear();
    mount();
    const heading = await screen.findByRole("heading", { name: "Official baseline" });
    expect(screen.getByRole("main").scrollTop).toBe(120);
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
    expect(heading).not.toHaveFocus();
  });

  it.each(["click", "keyboard"] as const)("repeats the current version jump on explicit %s without moving shell ancestors", async (activation) => {
    mockReaderGeometry();
    navigation.hash = `#${encodeURIComponent(fixture.releases[1]!.id)}`;
    render(<div data-testid="shell">{page()}</div>);
    const heading = await screen.findByRole("heading", { name: "Official baseline" });
    const main = screen.getByRole("main");
    const shell = screen.getByTestId("shell");
    const link = within(screen.getByRole("navigation", { name: en.navigation })).getByRole("link", { name: /v0.4.37/ });
    main.scrollTop = 75;
    shell.scrollTop = 16;
    vi.mocked(Element.prototype.scrollIntoView).mockClear();
    if (activation === "keyboard") {
      link.focus();
      await userEvent.setup().keyboard("{Enter}");
    } else fireEvent.click(link);
    expect(navigation.push).toHaveBeenCalledWith(`/acme/changelog${navigation.hash}`);
    expect(main.scrollTop).toBe(400);
    expect(shell.scrollTop).toBe(16);
    expect(heading).toHaveFocus();
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });

  it("lands a fresh deep link only within the reader and does not replay it on refresh", async () => {
    mockReaderGeometry();
    navigation.hash = `#${encodeURIComponent(fixture.releases[1]!.id)}`;
    render(page());
    await screen.findByRole("heading", { name: "Official baseline" });
    const main = screen.getByRole("main");
    expect(main.scrollTop).toBe(400);
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
    main.scrollTop = 175;
    fireEvent.click(screen.getByRole("button", { name: en.refresh }));
    await waitFor(() => expect(query).toHaveBeenCalledTimes(2));
    expect(main.scrollTop).toBe(175);
  });

  it("keeps modifier clicks as new-tab actions without jumping the current reader", async () => {
    navigation.openInNewTab = vi.fn();
    navigation.hash = `#${encodeURIComponent(fixture.releases[1]!.id)}`;
    render(page());
    await screen.findByRole("heading", { name: "Official baseline" });
    const main = screen.getByRole("main");
    main.scrollTop = 75;
    vi.mocked(Element.prototype.scrollIntoView).mockClear();
    const link = within(screen.getByRole("navigation", { name: en.navigation })).getByRole("link", { name: /v0.4.37/ });
    fireEvent.click(link, { metaKey: true });
    expect(navigation.openInNewTab).toHaveBeenCalledWith(`/acme/changelog${navigation.hash}`, undefined);
    expect(navigation.push).not.toHaveBeenCalled();
    expect(main.scrollTop).toBe(75);
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });
});
