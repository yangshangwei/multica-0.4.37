# Account detail restoration root cause

The platform correctly restores620. The account-detail root still uses browser `overflow-anchor:auto`, so a later successful Query refetch removes its retained error banner and the browser changes620 to476 independently. The banner is112px high plus32px layout gap: exactly144px. Setter tracing records only620 writes; final maximum remains above620. See `scroll-root-cause.json` and retained layout trace.

A passive first diagnostic passed both clients before recovery settled; a second diagnostic waited for the real top error to disappear and reproduced the unchanged620 expectation failing at476 on Electron. This explains why early polling occasionally passed without proving stable restoration.

The fix sets `[overflow-anchor:none]` only on the account workbench detail scroll root. This root already has explicit memento capture/restoration on both platforms; browser anchoring must not become a second offset owner after that restoration. Explicit link/focus navigation and user scrolling continue normally. Generic restoration hooks and source-workspace scrolling are unchanged.

The existing global-scope E2E gains a controlled actual-read failure/recovery: cached issue content survives503, scope resume restores620 while its next real GET is held, release allows the successful response, error removal and rendered frames settle, then620 is asserted again. Real response headers are retained, no automatic retries/timeouts changed, and held requests finish before route cleanup. Existing original global assertions remain.

Final verification in progress. Private failed first build (unit test edited during build), first diagnostic early-pass and an interrupted incorrect test URL matcher are preserved as separate operational attempts; neither is a product fix proof.
