# Renderer local-file boundary

## Scope and signatures

`installRendererFileAccess(window)` in `apps/desktop/src/main/renderer-file-access.ts`
owns the session's single `webRequest.onBeforeRequest` listener for `file://*/*`.
The production `loadRenderer` installs it before either main or issue window
loads, paired with the main-frame navigation guard.

## Contract

Only a registered live WebContents' current main frame may request a local file.
Resolve frame identity for every request. Do not trust URLs, resourceType, frame
names or captured frame objects. Missing/detached/destroyed/throwing contexts deny
access. Multiple windows share one listener and destroy only their registration.
Other webRequest events, including `onBeforeSendHeaders`, remain independent.

## Validation matrix

| Request | Result |
|---|---|
| Registered live main-frame asset/worker/dynamic import | Allow |
| Sandboxed HTML subframe local fetch/XHR/script/image/style/document | Deny |
| Subframe after HTTP/file navigation or cache reuse | Deny |
| Unregistered, missing or destroyed frame/window | Deny |
| Ordinary HTTP interactive preview/image/PDF | Existing behavior |

## Good/base/bad cases

Good: native frame boundary survives navigation away from srcdoc. Base: trusted
renderer assets and HTTP preview controls load. Bad: assuming `sandbox="allow-scripts"`
blocks local reads while `webSecurity` is disabled; a CSP prefix alone can be
escaped by navigation. This boundary closes the local-file vector, not general
origin/CORS isolation. Main-process privileged fetch is outside the renderer rule.

## Required tests

Run `node apps/desktop/scripts/verify-renderer-file-access.mjs` explicitly. It uses
the production loader/preferences and real inline, attachment and full-page
preview components with fake files, local HTTP fixtures and a temporary profile.
Require attack-denial and positive script/network/asset/PDF/image controls.
Unit tests cover unknown/destroyed frame contexts and session/window lifetimes.
Neither DOM-only sandbox assertions nor absence of a message proves isolation.

## Wrong versus correct

Wrong: register a window-capturing onBeforeRequest callback for every window,
overwriting the previous listener. Correct: one session-owned listener resolves
registered WebContents and its current main frame per request.
