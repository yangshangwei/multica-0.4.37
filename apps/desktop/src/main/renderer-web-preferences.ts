import type { WebPreferences } from "electron";

/**
 * WebPreferences shared by every renderer window — the tabbed main window and
 * the dedicated issue windows.
 *
 * Extracted from index.ts so the security-relevant flags are pinned by a unit
 * test beside this file: a silent regression here re-opens the renderer attack
 * surface, and the entry module cannot be imported in tests (it registers app
 * lifecycle handlers on import).
 *
 * `preloadPath` is injected rather than derived from `__dirname` here to keep
 * this a pure function — the bundled main process resolves it relative to its
 * own output directory at the call sites.
 */
export function createRendererWebPreferences(
  preloadPath: string,
  systemLocale: string,
  additionalArguments: string[] = [],
): WebPreferences {
  return {
    preload: preloadPath,
    // Sandboxed preload. The preload script only uses sandbox-safe APIs: the
    // `electron` module (contextBridge, ipcRenderer — including sendSync) and
    // the polyfilled `process` (platform, argv). It therefore must remain a
    // single CJS bundle, because the sandboxed preload `require` can only load
    // `electron` plus a couple of node builtins — see electron.vite.config.ts,
    // which bundles @electron-toolkit/preload into the output instead of
    // leaving it external.
    sandbox: true,
    // Still intentionally off. Restoring webSecurity requires migrating the
    // renderer off the opaque file:// origin onto a privileged custom protocol
    // (so CORS preflight requests carry a real Origin the server can allow),
    // which needs server-side coordination first. Neither sandboxed preload
    // nor an opaque iframe origin blocks local-file reads with this flag off.
    // loadRenderer must install renderer-file-access.ts and navigation-guard.ts
    // before loading either window. That native boundary denies file requests
    // from untrusted subframes while retaining trusted renderer assets.
    webSecurity: false,
    // Required for the Chromium PDF viewer (PDFium) to activate inside
    // iframes — used by the attachment preview modal for application/pdf
    // files. Default is false in Electron; without it <iframe src=*.pdf>
    // renders blank.
    //
    // PDFium runs in its own sandboxed process. Attachment previews use
    // signed HTTP(S) URLs or downloaded blobs. The native file-request policy
    // also covers subframe PDF requests; plugins do not bypass that boundary.
    // General network/origin isolation is still limited by webSecurity above.
    //
    // When webSecurity comes back on, revisit this by hosting the PDF viewer
    // in a dedicated WebContentsView with `plugins: true` scoped to that view,
    // keeping the main renderer plugin-free.
    plugins: true,
    additionalArguments: [
      `--multica-locale=${systemLocale}`,
      ...additionalArguments,
    ],
  };
}
