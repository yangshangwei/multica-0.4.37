# S3 native interception diagnosis — 2026-11-18

**R09 is ready for a fresh main-owned four-top field run; business acceptance remains open.** The native `/health` transport probe ran on the current matching Electron fixture and renderer under the main's isolated unauthenticated profile. The author only wrote/syntax-checked the probe, reviewed raw results, corrected its own recovery transport and ran static checks. It executed no API/DB/browser body or process and changed no production/shared/S2 file.

## Actual retained evidence

All field R01–R05 sources/databases/raw reports remain intact. Each had four planned tops, 2 PASS /1 FAIL /1 NOT_RUN. R01–R03 had eight orderings, 4 PASS /1 FAIL /3 NOT_RUN; R04–R05 had 5 PASS /1 FAIL /2 NOT_RUN. They are failed complete candidates, not retroactive NOT_RUNs. R05 retains actual successful PATCH200 revision6 upstream, browser status0 and the unknown fresh creator edit intent. Native exit0 occurs during owned teardown, so this is not native crash evidence. Current GET200/API receipts did not retire the UI intent by themselves.

The main ran `runtime/native-transport-probe-r01/result.json` on Electron39.8.7 / Chromium142.0.7444.265 / Node22.22.1, with production `webSecurity:false`, `sandbox:true`, `contextIsolation:true`, `nodeIntegration:false`. The actual six cases are:

| Method | Transport | Upstream status | Browser status |
| --- | --- | --- | --- |
| GET | natural | browser-owned | 200 |
| GET | route.fetch + route.fulfill | 200 | 0 |
| GET | route.continue | browser-owned | 200 |
| PATCH | natural | browser-owned | 405 |
| PATCH | route.fetch + route.fulfill | 405 | 0 |
| PATCH | route.continue | browser-owned | 405 |

Body bytes/hash are identical in each method's three controls; fulfilled browser headers are empty. Actual/provisional Origin are both absent. This proves a native interception interaction in this installed toolchain; it does not prove omitted CORS headers, a product failure, or any business case. The main reports identical before/after business row counts, DB/migration ledger and gate. The probe uses only owned loopback `/health` GET/PATCH with no credentials and retains its native profile/raw failure/success evidence.

## Small correction and static proof

The before-edit plan and original fixture snapshot live in `verification/client-native-forward-r09`. Nonlost `HeldRequest` continues its unchanged original browser request, installs an exact Request-identity response wait before continuation, joins both outcomes using `Promise.allSettled`, and saves real browser status/JSON/allHeaders. The15-second bound is unchanged. Deliberately lost winning ACKs still use the real `route.fetch` result followed by `route.abort`, retaining exact original API identity. No synthetic business headers/bodies, fresh auto-Recover, forced click, storage mutation, production preference change or timeout extension was introduced.

Targeted strict TypeScript and ESLint both exit0, and the two owner-file hashes are stable before/after static checks. Exact commands/raw logs/result hashes are in `verification/client-native-forward-r09`; full machine record accompanies this report. Current source seals:

- Spec: `10cac566342deb24c68e90fd15cdcfd1c8a6aec50c9572116dfbb683c77b67ca`.
- Fixture R09: `e11aa601afe3bb82d245a6af04c67a99587fbc5f0c4964f284392cd4852d74d5`.

## Remaining proof

The main must freeze R09 and run fresh R06 with all four field tops/eight orderings/retries0. The full55-comment batch/current WebSocket reauthentication is separately open. Extras-A must preserve a real native delayed-response status; `route.fulfill` status0 cannot be reported as a known authority denial or a passing late-hydration fence. All notifications/uploads/revoked access, Extras-B actual runtime, v2/r05 blocking and final independent S3 acceptance remain open. Parent24/TC-16 and S3 archive status are unchanged by this diagnostic.
