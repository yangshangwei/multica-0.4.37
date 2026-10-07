import type { BrowserWindow, OnBeforeRequestListenerDetails, Session, WebContents } from "electron";

const sessions = new WeakMap<Session, Map<number, WebContents>>();

function isLiveMainFrame(
  request: OnBeforeRequestListenerDetails,
  renderers: Map<number, WebContents>,
): boolean {
  try {
    const contents = request.webContentsId === undefined
      ? undefined
      : renderers.get(request.webContentsId);
    const frame = request.frame;
    // Frame access may throw or return null during navigation/destruction.
    // Resolve the current main frame per request; never trust a captured frame,
    // the renderer-controlled URL, or resourceType (subframes can navigate).
    return !!contents && !contents.isDestroyed() && !!frame
      && !frame.detached && frame === contents.mainFrame;
  } catch {
    return false;
  }
}

/**
 * The sole onBeforeRequest owner for renderer sessions. Electron keeps only the
 * last listener for each webRequest event, so install once per session, not once
 * per window. onBeforeSendHeaders and download handlers remain independent.
 *
 * With webSecurity disabled, the HTML iframe sandbox alone does not block local
 * files. Only registered live main frames may request file resources; every
 * subframe stays denied even after leaving srcdoc, or when reusing cached files.
 * Pair this with the main-frame navigation guard before loading any renderer.
 */
export function installRendererFileAccess(window: Pick<BrowserWindow, "webContents">): void {
  const contents = window.webContents;
  const session = contents.session;
  let renderers = sessions.get(session);
  if (!renderers) {
    renderers = new Map();
    const registeredRenderers = renderers;
    session.webRequest.onBeforeRequest({ urls: ["file://*/*"] }, (request, callback) => {
      callback({ cancel: !isLiveMainFrame(request, registeredRenderers) });
    });
    sessions.set(session, renderers);
  }
  if (renderers.has(contents.id)) return;
  const id = contents.id;
  renderers.set(id, contents);
  contents.once("destroyed", () => renderers.delete(id));
}
