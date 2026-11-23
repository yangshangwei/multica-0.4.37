# v13 complete-check result

**Failed: make exit 2.** The actual final filesystem identity audit passed; it does not turn the failed command into acceptance.

- Candidate: `85ef5d20578f63c7c4f879f035acbd414e80f306`. UTC 2026-11-03T10:01:06.135061+00:00 — 2026-11-03T11:23:27.231689+00:00.
- Static 15/15; TypeScript 13,244 passed.
- Go: 82 isolated race packages, 73 tested / 9 no-test packages; 20,963 named pass, 0 fail, 22 named skip; vet reached completion.
- Playwright: 441 identities = 337 pass + 13 fail + 91 explicit skip; zero unrun/blocked, retry or flaky identities. Every skip has expectedStatus=skipped and a skip annotation.
- Knowledge UI: all 8 recorded Web/native cases passed. Deployed OFF/ON probes retain their explicit skip status.
- Root live API/DB/Web proofs and their hashes are linked in result-summary.json. No source was edited, no full run was repeated, and no Provider was called.

## Failed browser identities

- `e2e/collaboration-boundaries-web.spec.ts:163` — Gap 5: a source workspace holding a collaboration issue cannot be deleted, open or completed, and nothing is cleaned up
- `e2e/collaboration-boundaries-web.spec.ts:273` — Gap 7: hidden manager actions POSTed by the wrong role are refused and change nothing; a non-participant's @agent never queues
- `e2e/collaboration-boundaries-web.spec.ts:425` — Smaller gaps: title/date edits leave attributed records, the exact recipient resolver stays read-only, a source member resolves and reacts without notifications
- `e2e/collaboration-memory-web.spec.ts:7` — shared context keeps concurrent edits, restores versions, and permits historical purge after closure
- `e2e/collaboration-mention-journeys-desktop.spec.ts:102` — Desktop owner revocation after preview refuses only A and original-key replay stays deduplicated
- `e2e/collaboration-mention-journeys-desktop.spec.ts:107` — Desktop offline mentions survive a fresh profile and edits notify only newly added people
- `e2e/collaboration-mention-journeys-web.spec.ts:46` — Web owner revocation after preview refuses only A and original-key replay stays deduplicated
- `e2e/collaboration-mention-journeys-web.spec.ts:65` — Web offline mentions survive a fresh context and edits notify only newly added people
- `e2e/collaboration-preparation-records-web.spec.ts:7` — Web reads synthetic frozen preparation without inferring termination across refresh and viewers
- `e2e/collaboration-preview-web.spec.ts:8` — trigger preview explains shared and later batches before a suppressed target is sent
- `e2e/collaboration-workbench-web.spec.ts:24` — Web workbench keeps account scope through comments, uncertain cancellation, handoff and revocation
- `e2e/collaboration-workbench-web.spec.ts:165` — Web stale lifecycle conflict immediately reports refusal and preserves input for explicit retry
- `e2e/settings-preferences.spec.ts:107` — hidden manual issue fields persist and remain usable through the overflow menu

Raw reasons, trace/screenshot hashes and all skip annotations are in browser-failures.json and playwright-inventory.json. The original run/log remain unchanged. #7 is still open; root owns repair disposition and future candidate acceptance.
