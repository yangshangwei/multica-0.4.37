# Client privacy correction + increment 3b2

Completed bounded client implementation on 2026-11-14. Independent check is pending. The exact 31 source/test/locale files, before/after SHA-256 values, 24 command logs and source manifest are recorded in `client-increment-3b2-results.json`. This is unit/static evidence; no actual API, database, provider, Web or Desktop instance was started.

The normative wire contract remains `a71d71570cf1f337d0f8c299e77e307e6d37516dd516eaa6e0882548c5669520`, sections 8.8, 8.9 and 13.14–15. The previous 3a/3b1 reports and 3b1 frozen snapshot remain unchanged.

## Privacy correction, separately identified

R1 children require `content_digest:null` when `can_open:false` and a full lowercase digest when readable. Fresh nonempty A5 rejects an unreadable current child even when the caller retains its previous digest. Empty explicit A5 still works. A6 and the shared child selector also require current source-readable rows.

This correction is confined to `stage-schema.ts`, `stage-presentation.ts`, `stage-actions.ts`, `stage-child-privacy.test.ts` and the child-selection portion of `stage-code-forms.tsx`. Its initial failing log and 69-test first green remain in `s2-client-3b2-privacy-{red,green1}.log`; the final Core run includes the privacy suite.

## T4 report reads and QA consumption

- Added strict summary/detail parsing, exact issue/run/report correlation, UTF-8 body and case/log bounds, result/count derivation, explicit source-only null artifacts and SHA-256 verification of resolved redacted log bytes. Applicability stays separate from producer result.
- Account-scoped, no-store GET methods use the frozen routes. There are no browser T2 writes or path/URL fetches. Missing routes, ambiguous object denial, unavailable retained logs, unavailable storage and malformed responses remain distinct non-success states.
- Query data stays under the collaboration issue and source-workspace keys. Existing workspace source generations now fence R4 and T4, including late errors. A T4 `not_found` clears private source evidence and re-reads authoritative issue access. A surviving issue keeps its identity/history; authoritative issue denial uses normal cleanup. Cursor chains reject duplicate reports, mismatched identities, broken progression and loops.
- Shared Web/Desktop UI browses bounded run pages and explicitly opens a report. Cases and logs keep their original order; shared logs render once, as plain text, with original event/message/hash for record logs and no fabricated event for inline logs. Report/candidate/binding/source/artifact identities are visible.
- QA retains its explicit report choice. Approval needs a current full T4 detail matching the selected R4 report and candidate/source/artifact, with `reported_result:passed` and `applicability:usable`. Failed/unsettled reports remain usable for a human return; source-denied reason-only return remains intact.

## C1 typed test input

`Prepare test input` updates the existing mutable comment draft with the exact candidate reference. It preserves text and attachments, does not POST, and refuses to replace an existing subject or interfere with pending comments/uploads. An untouched mounted composer adopts that shared draft; successful original-comment retirement clears it normally.

The existing issue parser deliberately forces `submit_command:false` to keep direct runtime commands unavailable. Root confirmed that C1 must use the existing protected `mention-targets.can_submit_commands` authority instead. Preparation/send additionally require current issue read/comment rights, v6, assisted policy, managed QA, current scope/candidate/source and a valid unexpired code review. The global direct-runtime gate was not changed. Tests cover the reachable positive path and revoked mention authority.

The composer reveals/focuses the prepared input and requires the existing typed target preview/confirmation. The subject is included in local preflight identity and only in comment creation—not preview, edits or runtime endpoints. It stays frozen in the original request and GET recovery. Candidate/authority changes disable sending and retain the original subject; no new candidate is substituted. Queuing a comment is explicitly not presented as a completed or passed test.

## Verification

| Check | Final result | Evidence log suffix |
| --- | --- | --- |
| Core stage/comment/schema/draft regressions | 22 suites, 477 passed | `core-final2` |
| Shared Views stage/comment/report flows | 5 suites, 63 passed | `views-final2` |
| Core typecheck | Passed | `core-typecheck3` |
| Views typecheck | Passed | `views-typecheck3` |
| Changed Core file lint | Passed, zero findings | `core-lint1` |
| Changed Views file lint | Passed, zero findings | `views-lint2` |
| Diff/new-file whitespace, added locale keys, protected hashes | Passed | `static` |

Logs are under `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/s2-client-3b2-<suffix>.log`. The earlier broad Core run passed 500 tests in 23 suites, including the unchanged 23-test `delivery.test.ts`; the final run above focuses on the affected stage/comment suites. No results are summed across repeated runs.

Initial reds and intermediate failures remain: absent T4 methods/module; missing subject/draft handling; the forced direct-command capability discovered during UI integration; missing test dialog context; one unsupported Testing Library test option; and a literal hash label rejected by i18n lint. Each was corrected before the final green checks. Hostile script/URL/path text is exercised through the real T4 client parser and shared renderer without creating links, executing code or issuing external requests.

Protected `action.ts` and the C08 `stage-panel.tsx`, `stage-panel.test.tsx`, `stage-recovery.test.tsx` are byte-identical to the pre-3b2 baseline. No dependencies were added, no server/platform files or existing receipt envelopes changed, and no git write commands ran.

## Review handoff

Frozen source: `/Users/artisan/.multica/acceptance/pay241-20261113-01/snapshots/client-3b2-20261114T071911Z` (496 files, including the frozen wire contract). The JSON report binds its manifest hash and every changed file/log. This snapshot may be reviewed independently while later work continues.

Remaining work belongs to the main session: independent 3b2 Trellis review; M1/activity consumption after the server 2d contract is frozen; actual API/production Web/matching Desktop integration; and A01–A12. These client checks do not establish real test execution, CI, deployment, payment or complete S2 acceptance.
