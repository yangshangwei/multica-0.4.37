# Desktop isolation research

Existing desktop-probes/results.json, extended-results.json, alternatives-results.json and lifecycle-results.json record Electron 39.8.7 experiments with fake local files.

Baseline permits file fetch/XHR and file scripts from sandboxed previews. A CSP prefix blocks direct reads but is escaped by file/HTTP navigation. Native-frame-deny rejects subframe file requests including after HTTP navigation while preserving HTTP scripts/fetch and click controls. Shared/recreated windows retain rejection and load trusted assets; HTTP PDF/image and header controls succeed.

These are planning experiments, not production verification. Implementation must use production preferences/policy and actual HTML preview paths. Missing/unverifiable/destroyed frame contexts fail closed. Main-process session fetch is privileged and outside the untrusted renderer contract.

This closes the reported local-file vector; webSecurity remains disabled and general origin isolation is not claimed. A protocol/CORS migration is out of scope. API references are captured in desktop-probes/electron-security.md and electron-web-request.md. onBeforeRequest has one owner per session, separate from onBeforeSendHeaders.
