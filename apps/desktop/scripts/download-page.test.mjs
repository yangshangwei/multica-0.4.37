// @vitest-environment node
import { readFileSync } from "node:fs";
import { JSDOM, VirtualConsole } from "jsdom";
import { afterEach, describe, expect, it, vi } from "vitest";

const pageUrl = "http://downloads.internal:18080/desktop/";
const pages = [];
const release = {
  schemaVersion: 1,
  platform: "windows",
  arch: "x64",
  version: "1.2.3",
  file: "multica-desktop-1.2.3-windows-x64.exe",
  size: 1024,
};
const config = { schemaVersion: 1, serverUrl: "http://business.internal:8080/team" };

function json(value) {
  return new Response(JSON.stringify(value), { headers: { "Content-Type": "application/json" } });
}

function openPage({ projection = release, settings = config, handle, clipboard, legacyCopy, timeout } = {}) {
  const errors = [];
  const console = new VirtualConsole();
  console.on("jsdomError", (error) => errors.push(error));
  const fetch = vi.fn(async (url, options) => {
    const response = await handle?.(url, options);
    if (response !== undefined) return response;
    if (url.endsWith("/download.json")) return json(projection);
    if (url.endsWith("/download-config.json")) return json(settings);
    if (options.method === "HEAD") return new Response(null, { headers: { "Content-Length": String(projection.size) } });
    throw new Error(`Unexpected request: ${options.method} ${url}`);
  });
  const page = new JSDOM(readFileSync(new URL("../../../deploy/desktop-updates/download.html", import.meta.url), "utf8"), {
    url: pageUrl,
    runScripts: "dangerously",
    virtualConsole: console,
    beforeParse(window) {
      window.fetch = fetch;
      window.TextDecoder = TextDecoder;
      if (clipboard) Object.defineProperty(window.navigator, "clipboard", { value: clipboard });
      if (legacyCopy) window.document.execCommand = legacyCopy;
      if (timeout) {
        const schedule = window.setTimeout.bind(window);
        window.setTimeout = (callback) => schedule(callback, timeout);
      }
    },
  });
  pages.push(page);
  const { document } = page.window;
  return { page, document, fetch, errors, find: (id) => document.getElementById(id) };
}

async function settled(view) {
  await vi.waitFor(() => {
    expect(view.find("download").dataset.state).not.toBe("loading");
    expect(view.find("server").dataset.state).not.toBe("loading");
  });
  expect(view.errors).toEqual([]);
}

afterEach(() => { while (pages.length) pages.pop().window.close(); });

describe("standalone intranet download page", () => {
  it("enables only current Windows links after checking the installer, without accessing a business API", async () => {
    let finishHead;
    const view = openPage({ handle: (_url, options) => options.method === "HEAD" ? new Promise((resolve) => { finishHead = resolve; }) : undefined });
    await vi.waitFor(() => expect(finishHead).toBeTypeOf("function"));
    expect(view.find("download-primary").hasAttribute("href")).toBe(false);
    finishHead(new Response(null, { headers: { "Content-Length": "1024" } }));
    await settled(view);
    expect(view.find("download").dataset.state).toBe("available");
    for (const link of view.document.querySelectorAll("[data-download]")) {
      expect(link.href).toBe(pageUrl + release.file);
      expect(link.getAttribute("download")).toBe(release.file);
      expect(link.getAttribute("aria-disabled")).toBeNull();
    }
    expect(view.find("download-status").textContent).toContain(release.version);
    expect(view.find("server-url").textContent).toBe(config.serverUrl);
    expect(view.fetch).toHaveBeenCalledTimes(3);
    for (const [url, options] of view.fetch.mock.calls) {
      expect(new URL(url).origin).toBe(new URL(pageUrl).origin);
      expect(options.cache).toBe("no-store");
      expect(options.redirect).toBe("error");
      expect(options.signal).toBeDefined();
    }
    const pending = [...view.document.querySelectorAll("[data-pending]")];
    expect(pending).toHaveLength(4);
    for (const row of pending) {
      expect(row.textContent).toContain("待开放");
      expect(row.querySelector("a, button")).toBeNull();
    }
    expect(pending.map((row) => row.dataset.pending)).toEqual(["mac-arm64", "mac-x64", "linux-x86", "linux-arm64"]);
  });

  it.each([
    { schemaVersion: 2 }, { platform: "mac" }, { arch: "arm64" }, { size: 0 }, { size: "1024" },
    { file: "https://public.example/installer.exe" }, { file: "../installer.exe" },
    { file: "multica-desktop-9.9.9-windows-x64.exe" },
    { version: "01.2.3", file: "multica-desktop-01.2.3-windows-x64.exe" },
  ])("refuses invalid release data %j without an installer request", async (override) => {
    const view = openPage({ projection: { ...release, ...override } });
    await settled(view);
    expect(view.find("download").dataset.state).toBe("unavailable");
    expect(view.find("download-primary").hasAttribute("href")).toBe(false);
    expect(view.fetch.mock.calls.some(([, options]) => options.method === "HEAD")).toBe(false);
    expect(view.find("retry-download").hidden).toBe(false);
    expect(view.find("copy-server").disabled).toBe(false);
  });

  it("supports explicit prerelease and build metadata without describing it as stable", async () => {
    const version = "1.2.4-rc.1+test.2";
    const file = `multica-desktop-${version}-windows-x64.exe`;
    const view = openPage({ projection: { ...release, version, file } });
    await settled(view);
    expect(view.find("download-primary").href).toBe(pageUrl + file);
    expect(view.find("download-status").textContent).toContain(version);
  });

  it.each(["missing", "wrong-size", "redirect", "encoded"])("disables the download when the installer is %s", async (problem) => {
    const view = openPage({ handle: (_url, options) => {
      if (options.method !== "HEAD") return;
      if (problem === "missing") return new Response(null, { status: 404 });
      if (problem === "redirect") return new Response(null, { status: 302, headers: { Location: "https://public.example/file.exe" } });
      return new Response(null, { headers: { "Content-Length": problem === "wrong-size" ? "12" : "1024", ...(problem === "encoded" ? { "Content-Encoding": "gzip" } : {}) } });
    } });
    await settled(view);
    expect(view.find("download-primary").hasAttribute("href")).toBe(false);
    expect(view.find("download").dataset.state).toBe("unavailable");
  });

  it("recovers from missing metadata and moves to the new release on retry", async () => {
    let missing = true;
    const view = openPage({ handle: (url) => url.endsWith("/download.json") && missing ? new Response(null, { status: 404 }) : undefined });
    await settled(view);
    expect(view.find("download").dataset.state).toBe("unavailable");
    missing = false;
    view.find("retry-download").focus();
    view.find("retry-download").click();
    await vi.waitFor(() => expect(view.find("download").dataset.state).toBe("available"));
    expect(view.find("download-primary").href).toBe(pageUrl + release.file);
    expect(view.document.activeElement).toBe(view.find("download-primary"));
  });

  it.each(["malformed", "oversize-header", "oversize-stream", "redirected"])("retains the static guide with %s JSON", async (problem) => {
    const view = openPage({ handle: (url) => {
      if (!url.endsWith("/download.json")) return;
      if (problem === "malformed") return new Response("{broken");
      if (problem === "oversize-header") return new Response("{}", { headers: { "Content-Length": "1000000" } });
      if (problem === "oversize-stream") return new Response(" ".repeat(17000) + JSON.stringify(release));
      const response = json(release);
      Object.defineProperty(response, "redirected", { value: true });
      return response;
    } });
    await settled(view);
    expect(view.find("download").dataset.state).toBe("unavailable");
    expect(view.document.querySelectorAll("#install ol > li")).toHaveLength(6);
    expect(view.document.querySelectorAll("#faq details > summary")).toHaveLength(6);
    expect(view.find("server").dataset.state).toBe("available");
  });

  it("ends stalled requests and permits retry while keeping help readable", async () => {
    const view = openPage({ timeout: 10, handle: (url, options) => url.endsWith("/download.json") ? new Promise((_, reject) => {
      options.signal.addEventListener("abort", () => reject(new Error("aborted")), { once: true });
    }) : undefined });
    await settled(view);
    expect(view.find("download").dataset.state).toBe("unavailable");
    expect(view.find("retry-download").disabled).toBe(false);
    expect(view.find("install").hidden).toBe(false);
  });

  it.each(["", "javascript:alert(1)", "http://user:password@business.internal", "//business.internal", "http://business.internal\n/path"])("rejects invalid business address %j without blocking download", async (serverUrl) => {
    const view = openPage({ settings: { ...config, serverUrl } });
    await settled(view);
    expect(view.find("server").dataset.state).toBe("unavailable");
    expect(view.find("server-url").hidden).toBe(true);
    expect(view.find("copy-server").disabled).toBe(true);
    expect(view.find("retry-server").hidden).toBe(false);
    expect(view.find("download").dataset.state).toBe("available");
  });

  it("recovers missing business configuration independently", async () => {
    let missing = true;
    const view = openPage({ handle: (url) => url.endsWith("/download-config.json") && missing ? new Response(null, { status: 404 }) : undefined });
    await settled(view);
    missing = false;
    view.find("retry-server").click();
    await vi.waitFor(() => expect(view.find("server").dataset.state).toBe("available"));
    expect(view.find("server-url").textContent).toBe(config.serverUrl);
    expect(view.fetch.mock.calls.filter(([url]) => url.endsWith("/download.json"))).toHaveLength(1);
  });

  it("copies via the Clipboard API only reporting success after it resolves", async () => {
    let finishCopy;
    const writeText = vi.fn(() => new Promise((resolve) => { finishCopy = resolve; }));
    const view = openPage({ clipboard: { writeText } });
    await settled(view);
    view.find("copy-server").click();
    expect(writeText).toHaveBeenCalledWith(config.serverUrl);
    expect(view.find("copy-status").textContent).not.toContain("已复制");
    finishCopy();
    await vi.waitFor(() => expect(view.find("copy-status").textContent).toContain("已复制"));
  });

  it("uses selected visible text for HTTP-compatible copying", async () => {
    const legacyCopy = vi.fn(() => true);
    const view = openPage({ legacyCopy });
    await settled(view);
    view.find("copy-server").click();
    expect(legacyCopy).toHaveBeenCalledWith("copy");
    expect(view.page.window.getSelection().toString()).toBe(config.serverUrl);
    expect(view.find("copy-status").textContent).toContain("已复制");
  });

  it("keeps a manual selection and honest feedback when clipboard permission is denied", async () => {
    const view = openPage({ clipboard: { writeText: vi.fn().mockRejectedValue(new Error("denied")) }, legacyCopy: vi.fn(() => false) });
    await settled(view);
    view.find("copy-server").click();
    await vi.waitFor(() => expect(view.find("copy-status").textContent).toContain("Ctrl+C"));
    expect(view.find("copy-status").textContent).not.toContain("已复制");
    expect(view.page.window.getSelection().toString()).toBe(config.serverUrl);
    expect(view.find("copy-server").disabled).toBe(false);
  });

  it("ships only local resources and real in-page destinations", async () => {
    const view = openPage();
    await settled(view);
    for (const node of view.document.querySelectorAll("[src], link[href]")) {
      expect(new URL(node.getAttribute("src") ?? node.getAttribute("href"), pageUrl).origin).toBe(new URL(pageUrl).origin);
    }
    for (const link of view.document.querySelectorAll('a[href^="#"]')) expect(view.document.querySelector(link.getAttribute("href"))).not.toBeNull();
    expect(view.document.querySelector("base")).toBeNull();
    expect(view.document.querySelector("form")).toBeNull();
  });
});
