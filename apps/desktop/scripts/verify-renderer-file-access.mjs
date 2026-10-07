#!/usr/bin/env node
// Explicit native acceptance: node apps/desktop/scripts/verify-renderer-file-access.mjs
// Builds real preview components and the production loadRenderer function, with
// fake attachment responses, local files, and an entirely temporary profile.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const desktop = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(desktop, "../..");
const requireDesktop = createRequire(join(desktop, "package.json"));
// Use electron-vite's installed compiler, without installing a test dependency.
const requireVite = createRequire(
  createRequire(requireDesktop.resolve("electron-vite")).resolve("vite"),
);
const { build } = requireVite("esbuild");
const ts = requireDesktop("typescript");
const root = await mkdtemp(join(tmpdir(), "multica-file-boundary-"));

try {
  await mkdir(join(root, "main"));
  await mkdir(join(root, "renderer"));
  // index.ts has application lifecycle side effects. Extract this function
  // verbatim instead of starting real daemon/auth services or copying its wiring.
  const source = ts.createSourceFile("index.ts", await readFile(join(desktop, "src/main/index.ts"), "utf8"), ts.ScriptTarget.Latest, true);
  const loader = source.statements.find((node) => ts.isFunctionDeclaration(node) && node.name?.text === "loadRenderer");
  assert(loader, "Production loadRenderer must exist");
  const imports = source.statements.filter((node) => ts.isImportDeclaration(node)
    && ["./navigation-guard", "./renderer-file-access"].includes(node.moduleSpecifier.text))
    .map((node) => node.getText(source).replace(node.moduleSpecifier.getText(source), JSON.stringify(join(desktop, "src/main", node.moduleSpecifier.text))));
  await build({
    stdin: {
      contents: `${imports.join("\n")}\nimport { createRendererWebPreferences } from ${JSON.stringify(join(desktop, "src/main/renderer-web-preferences.ts"))};
        import { join } from 'node:path'; import { pathToFileURL } from 'node:url';
        const is = { dev: false }; ${loader.getText(source)}
        (${nativeAcceptance.toString()})(loadRenderer, createRendererWebPreferences);`,
      resolveDir: desktop,
      loader: "ts",
    },
    bundle: true,
    platform: "node",
    format: "cjs",
    external: ["electron"],
    outfile: join(root, "main/index.cjs"),
  });
  await build({
    stdin: { contents: rendererFixture(repo), resolveDir: desktop, loader: "tsx" },
    bundle: true,
    platform: "browser",
    format: "esm",
    jsx: "automatic",
    define: { "process.env.NODE_ENV": '"production"' },
    outfile: join(root, "renderer/preview.js"),
    logLevel: "error",
  });
  await writeFile(join(root, "renderer/index.html"), '<!doctype html><meta charset="utf-8"><title>Native file boundary</title><div id="root"></div><script src="./trusted.js"></script><script type="module" src="./preview.js"></script>');
  await writeFile(join(root, "renderer/trusted.js"), "window.trustedScript = true");
  await writeFile(join(root, "renderer/module.js"), 'export const marker = "trusted-module"');
  await writeFile(join(root, "renderer/worker.js"), 'postMessage("trusted-worker")');
  await writeFile(join(root, "preload.cjs"), 'require("electron").contextBridge.exposeInMainWorld("nativeProbe", { sandbox: process.sandboxed });');
  const env = { ...process.env };
  delete env.ELECTRON_RUN_AS_NODE;
  delete env.ELECTRON_RENDERER_URL;
  const exitCode = await new Promise((resolveExit, reject) => {
    const child = spawn(requireDesktop("electron"), [join(root, "main/index.cjs")], { env, stdio: "inherit" });
    const timer = setTimeout(() => { child.kill("SIGKILL"); reject(new Error("Native acceptance exceeded 120 seconds")); }, 120_000);
    child.once("error", (error) => { clearTimeout(timer); reject(error); });
    child.once("exit", (code) => { clearTimeout(timer); resolveExit(code); });
  });
  assert.equal(exitCode, 0, "Native file-boundary acceptance failed");
} finally {
  await rm(root, { recursive: true, force: true });
}

function rendererFixture(repoRoot) {
  const view = (file) => JSON.stringify(join(repoRoot, "packages/views", file));
  return `
    import { createRoot } from 'react-dom/client';
    import { flushSync } from 'react-dom';
    import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
    import { ApiClient, setApiInstance } from '@multica/core/api';
    import { I18nProvider } from '@multica/core/i18n/react';
    import { HtmlBlockPreview } from ${view("editor/html-block-preview.tsx")};
    import { HtmlAttachmentPreview } from ${view("editor/html-attachment-preview.tsx")};
    import { AttachmentPreviewPage } from ${view("attachments/attachment-preview-page.tsx")};
    import { ScrollRestorationProvider } from ${view("platform/scroll-restoration.tsx")};
    import { NavigationProvider } from ${view("navigation/context.tsx")};
    import { RESOURCES } from ${view("locales/index.ts")};
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const root = createRoot(document.querySelector('#root'));
    const navigation = { push() {}, replace() {}, back() {}, pathname: '/', searchParams: new URLSearchParams(), hash: '', getShareableUrl: p => p };
    const scroll = { get() {}, registerExternalSource() {} };
    let run = 0;
    window.probeMessages = [];
    window.addEventListener('message', e => { if (e.data?.nativeFileProbe) window.probeMessages.push(e.data); });
    window.renderPreview = (kind, html, origin) => {
      const id = String(++run);
      // Real API client, query/cache and preview; only the server is a fixture.
      setApiInstance(new ApiClient(origin));
      window.probeMessages = [];
      const element = kind === 'inline' ? <HtmlBlockPreview html={html} />
        : kind === 'attachment' ? <HtmlAttachmentPreview attachmentId={id} filename="fixture.html" onPreview={() => {}} onDownload={() => {}} />
        : <AttachmentPreviewPage attachmentId={id} filename="fixture.html" />;
      flushSync(() => root.render(<I18nProvider locale="en" resources={RESOURCES}><NavigationProvider value={navigation}><ScrollRestorationProvider adapter={scroll}><QueryClientProvider client={client}><div key={id}>{element}</div></QueryClientProvider></ScrollRestorationProvider></NavigationProvider></I18nProvider>));
    };
  `;
}

// Serialized into the disposable Electron Main bundle above. Keep dependencies
// inside this function; no real profile, API, agent executable, or credentials.
async function nativeAcceptance(loadRenderer, createRendererWebPreferences) {
  /* eslint-disable @typescript-eslint/no-require-imports -- Serialized into Electron's CommonJS entry, not executed by the ESM runner. */
  const { app, BrowserWindow, session } = require("electron");
  const assert = require("node:assert/strict");
  const fs = require("node:fs");
  const http = require("node:http");
  const { dirname, join } = require("node:path");
  const { pathToFileURL } = require("node:url");
  /* eslint-enable @typescript-eslint/no-require-imports */
  const root = dirname(__dirname);
  app.setPath("userData", join(root, "profile"));
  app.setPath("crashDumps", join(root, "crashes"));
  app.on("window-all-closed", () => {});
  const windows = new Set();
  const failures = [];
  const passed = [];
  let server;
  let attachment = "";
  const marker = "FAKE_LOCAL_FILE_ONLY";
  const quote = (value) => JSON.stringify(value).replaceAll("<", "\\u003c");
  const script = (js) => `<body><script>${js}</script>`;
  const report = (label, value) => `top.postMessage({nativeFileProbe:true,label:${quote(label)},value:${value}},'*')`;
  const file = (name) => pathToFileURL(join(root, name)).href;
  const read = (label) => `fetch(${quote(file("fake.txt"))}).then(r=>r.text()).then(v=>{${report(label, "v")}}).catch(()=>{${report(label, "'denied'")}})`;
  const check = async (name, fn) => {
    try { await fn(); passed.push(name); console.log(`PASS ${name}`); }
    catch (error) { failures.push({ name, error: String(error) }); console.error(`FAIL ${name}: ${error.message}`); }
  };
  const waitFor = async (fn, label) => {
    const deadline = Date.now() + 5000;
    while (Date.now() < deadline) {
      const result = await fn();
      if (result) return result;
      await new Promise((resolveWait) => setTimeout(resolveWait, 25));
    }
    throw new Error(`Timed out: ${label}`);
  };
  try {
    fs.writeFileSync(join(root, "fake.txt"), marker);
    fs.writeFileSync(join(root, "fake.js"), report("file-script", quote(marker)));
    fs.writeFileSync(join(root, "fake.css"), "body { --file-probe: leaked; }");
    fs.writeFileSync(join(root, "fake.html"), script(report("file-navigation", quote(marker))));
    const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aT9sAAAAASUVORK5CYII=", "base64");
    fs.writeFileSync(join(root, "fake.png"), png);
    const pdf = Buffer.from("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Contents 4 0 R>>endobj\n4 0 obj<</Length 0>>stream\n\nendstream\nendobj\ntrailer<</Root 1 0 R>>\n%%EOF");
    const headers = [];
    server = http.createServer((req, res) => {
      headers.push(req.headers["x-native-control"]);
      res.setHeader("Access-Control-Allow-Origin", "*");
      if (/^\/api\/attachments\/\d+\/content$/.test(req.url)) { res.setHeader("Content-Type", "text/plain"); res.setHeader("X-Original-Content-Type", "text/html"); res.end(attachment); }
      else if (req.url === "/navigate") { res.setHeader("Content-Type", "text/html"); res.end(script(read("after-http-navigation"))); }
      else if (req.url === "/script.js") { res.setHeader("Content-Type", "text/javascript"); res.end(report("http-script", "'ok'")); }
      else if (req.url === "/image.png") { res.setHeader("Content-Type", "image/png"); res.end(png); }
      else if (req.url === "/document.pdf") { res.setHeader("Content-Type", "application/pdf"); res.end(pdf); }
      else { res.setHeader("Content-Type", "text/plain"); res.end("HTTP_CONTROL"); }
    });
    await new Promise((resolveListen) => server.listen(0, "127.0.0.1", resolveListen));
    const origin = `http://127.0.0.1:${server.address().port}`;
    await app.whenReady();
    const ses = session.defaultSession;
    ses.webRequest.onBeforeSendHeaders({ urls: ["http://127.0.0.1/*"] }, (details, callback) => {
      details.requestHeaders["X-Native-Control"] = "preserved";
      callback({ requestHeaders: details.requestHeaders });
    });
    const makeWindow = async (issue = false) => {
      const window = new BrowserWindow({ show: false, width: 800, height: 600,
        webPreferences: createRendererWebPreferences(join(root, "preload.cjs"), "en", issue ? ["--issue-window=fixture"] : []),
      });
      windows.add(window);
      window.webContents.on("console-message", (event) => {
        if (event.level >= 3) console.error("Renderer:", event.message);
      });
      const loaded = new Promise((resolveLoad, reject) => {
        window.webContents.once("did-finish-load", resolveLoad);
        window.webContents.once("did-fail-load", (_e, code, description, _url, main) => { if (main) reject(new Error(`${code}: ${description}`)); });
      });
      loadRenderer(window);
      await loaded;
      await waitFor(() => window.webContents.executeJavaScript("typeof window.renderPreview === 'function'"), "production preview bundle");
      return window;
    };
    const render = async (window, kind, html) => {
      attachment = html;
      await window.webContents.executeJavaScript(`window.renderPreview(${quote(kind)},${quote(html)},${quote(origin)})`);
      try {
        await waitFor(() => window.webContents.executeJavaScript("!!document.querySelector('iframe')"), `${kind} iframe`);
      } catch (error) {
        throw new Error(`${error.message}: ${await window.webContents.executeJavaScript("document.body.innerText")}`);
      }
    };
    const message = (window, label) => waitFor(async () => {
      const messages = await window.webContents.executeJavaScript("window.probeMessages");
      return messages.find((item) => item.label === label);
    }, label);
    const main = await makeWindow();
    const issue = await makeWindow(true);
    const trusted = async (window) => {
      const result = await window.webContents.executeJavaScript(`Promise.all([
        window.nativeProbe.sandbox && window.trustedScript,
        import('./module.js').then(m=>m.marker),
        new Promise((resolve,reject)=>{const w=new Worker('./worker.js');w.onmessage=e=>{w.terminate();resolve(e.data)};w.onerror=reject}),
        fetch(${quote(file("fake.txt"))}).then(r=>r.text())
      ])`);
      assert.deepEqual(result, [true, "trusted-module", "trusted-worker", marker]);
    };
    await check("trusted main script, dynamic import, worker, preload and file cache seed", () => trusted(main));
    await check("shared-session issue window assets", () => trusted(issue));
    assert.equal(main.webContents.session, issue.webContents.session);
    for (const kind of ["inline", "attachment", "full-page"]) {
      await check(`${kind}: local fetch/XHR, scripts, images, styles, nested frames and worker denied`, async () => {
        const worker = `fetch(${quote(file("fake.txt"))}).then(r=>r.text()).then(v=>postMessage(v)).catch(()=>postMessage('denied'))`;
        const html = script(`
          ${read("fetch")};
          const x=new XMLHttpRequest();x.open('GET',${quote(file("fake.txt"))});x.onload=()=>{${report("xhr", "x.responseText")}};x.onerror=()=>{${report("xhr", "'denied'")}};x.send();
          const s=document.createElement('script');s.src=${quote(file("fake.js"))};s.onerror=()=>{${report("file-script", "'denied'")}};document.body.append(s);
          const im=new Image();im.onload=()=>{${report("file-image", "'leaked'")}};im.onerror=()=>{${report("file-image", "'denied'")}};im.src=${quote(file("fake.png"))};
          const css=document.createElement('link');css.rel='stylesheet';css.href=${quote(file("fake.css"))};css.onload=()=>{${report("file-style", "'leaked'")}};css.onerror=()=>{${report("file-style", "'denied'")}};document.head.append(css);
          const nested=document.createElement('iframe');nested.srcdoc=${quote(script(read("nested")))};document.body.append(nested);
          const w=new Worker(URL.createObjectURL(new Blob([${quote(worker)}],{type:'text/javascript'})));w.onmessage=e=>{${report("worker", "e.data")};w.terminate()};
        `);
        await render(main, kind, html);
        const results = await Promise.all(["fetch", "xhr", "file-script", "file-image", "file-style", "nested", "worker"].map((label) => message(main, label)));
        assert.deepEqual(results.map((item) => item.value), Array(7).fill("denied"));
      });
      await check(`${kind}: interactive JavaScript and HTTP script/fetch retained`, async () => {
        await render(main, kind, `<button id="control">Control</button><script src="${origin}/script.js"></script>` + script(`fetch('${origin}/control').then(r=>r.text()).then(v=>{${report("http-fetch", "v")}});document.querySelector('#control').onclick=()=>{${report("click", "'ok'")}};document.querySelector('#control').click()`));
        assert.equal((await message(main, "click")).value, "ok");
        assert.equal((await message(main, "http-script")).value, "ok");
        assert.equal((await message(main, "http-fetch")).value, "HTTP_CONTROL");
      });
      await check(`${kind}: HTTP self-navigation retains file denial`, async () => {
        await render(main, kind, script(`location.href='${origin}/navigate'`));
        assert.equal((await message(main, "after-http-navigation")).value, "denied");
      });
      await check(`${kind}: file self-navigation rejected`, async () => {
        let blocked = false;
        const failed = (_event, code, _description, url, isMainFrame) => {
          if (url === file("fake.html") && !isMainFrame && code === -20) blocked = true;
        };
        main.webContents.on("did-fail-load", failed);
        try {
          await render(main, kind, script(`location.href=${quote(file("fake.html"))}`));
          await waitFor(async () => blocked || (await main.webContents.executeJavaScript("window.probeMessages")).some((item) => item.value === marker), "file navigation result");
          assert(blocked, "Local document became a preview instead of ERR_BLOCKED_BY_CLIENT");
          assert(!(await main.webContents.executeJavaScript("window.probeMessages")).some((item) => item.value === marker));
        } finally {
          main.webContents.off("did-fail-load", failed);
        }
      });
    }
    main.destroy(); windows.delete(main);
    const recreated = await makeWindow();
    await check("recreated main and surviving issue window retain denial", async () => {
      for (const window of [recreated, issue]) {
        await trusted(window);
        await render(window, "inline", script(read("lifetime")));
        assert.equal((await message(window, "lifetime")).value, "denied");
      }
    });
    await check("trusted HTTP image and PDF viewer retained; header handler composed", async () => {
      await recreated.webContents.executeJavaScript(`document.body.innerHTML='<img id="image" src="${origin}/image.png"><iframe src="${origin}/document.pdf"></iframe>'`);
      await waitFor(() => recreated.webContents.executeJavaScript("document.querySelector('#image').naturalWidth === 1"), "HTTP image");
      await waitFor(async () => {
        const frame = recreated.webContents.mainFrame.framesInSubtree.find((item) => item.url.startsWith("chrome-extension:"));
        if (!frame) return false;
        try { return await frame.executeJavaScript("(() => { const viewer = document.querySelector('pdf-viewer'); return viewer?.loadProgress_ === 100 && viewer.docLength_ === 1 && !!viewer.shadowRoot?.querySelector('embed'); })()"); }
        catch { return false; }
      }, "Chromium PDF viewer document loaded");
      assert(headers.length > 0 && headers.every((header) => header === "preserved"));
    });
    console.log(JSON.stringify({ electron: process.versions.electron, passed, failures }, null, 2));
  } catch (error) {
    failures.push({ name: "native harness", error: String(error) });
    console.error(error);
  } finally {
    for (const window of windows) if (!window.isDestroyed()) window.destroy();
    server?.close();
    app.exit(failures.length ? 1 : 0);
  }
}
