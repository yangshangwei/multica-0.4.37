# Independent AC83 review — current session

Reviewer: trellis-check (independent). Active task:
`.trellis/tasks/10-20-collaboration-gap-recovery`. Scope: AC83 only. This review
reads files and runs static checks. It launched no client, environment, provider
or DB race, and changed no product, spec, ledger, build or environment.

## Decision

**Promote AC83 to `passed`, with the limitations below.** Every clause in AC83
has current evidence that matches the current source, from either a real client
journey or the canonical service layer. Nothing blocks promotion. The limitations
are about how far the evidence reaches, and none of them is a failing or missing
clause. Installed full Main, old clients, commercial provider account/quota, other
OS and Mobile still need their own evidence. This review does not imply them.

## Evidence verified (hash-checked against current files)

| Evidence | Result | Integrity |
| --- | --- | --- |
| `continuation-2026-10-22-resume/current-lane-notification-clients.{stdout,command.json,result.json}`: paired lane+notification run | 8/8 expected, 0 unexpected/skipped/flaky, retry 0. Both AC83 cases (`collaboration-offline-result-notifications.spec.ts:15`, Web 21.4s, Electron 22.5s) passed with 0 errors | raw SHA 506299b1…66c8 matches. The two embedded attachments decode to SHA 90f2007d… (Web) and d165c59c… (Electron), which equal `current-session-{web,electron}-notifications-identities.json`. stderr is only the pnpm field warning |
| Source files at run time vs now | All 7 recorded source hashes equal current (spec 86a2efd0…, fixture ab8c09d4…, offline-handoff fixture 733fb4d4…, lifecycle, cross-client, native, provider.go). The daemon/provider binary hashes are 57f9aa8c… and 80c32250… | Spec and fixture are untracked new files with no trailing whitespace. Strict tsc and root ESLint (`lane-notification-{typecheck,lint}`) exit 0, and their recorded sources still match |
| `continuation-2026-10-22-resume/notification-current-canonical.*` at HEAD cae15dd72 | 15 top-level / 31 including subtests passed, 0 fail/skip, `-race -p 1 -parallel 1` | stdout SHA 87c23981… matches, and I re-parsed it: 15/31, no fail or skip. stderr is empty. The 5 test-file hashes equal current, and those files are committed and clean |
| `continuation-2026-10-21/offline-handoff-coordinator-rerun*` (invite/assignment/transfer/comment) | Web and Electron 2/2, retry 0 | Preflight spec c70c973f… and fixture 733fb4d4… equal current. Committed in 28681f527, unchanged since |
| Attachment credential scan | 0 matches for mcb_/mdt_/mul_/mat_, bearer, JWT, password, cookie, nonce and token fields | Proof replay evidence records only SHA256 values |

## Clause-by-clause

1. **Offline, real login, all notification kinds.** The external H account has
   no workspaces and no source membership. In both platforms H opens a client and
   closes it, and `recipient.logins` does not change while events are produced.
   H then performs one fresh password login. On Web that is a real `POST
   /auth/login` in a new context with an HttpOnly cookie. On Electron it is a
   real password login whose token goes into a new isolated profile. All six
   kinds occur exactly once:
   - information_requested
   - information_returned
   - acceptance_submitted
   - ai_completed
   - ai_failed
   - ai_stopped

   Invitation is seeded before H ever opens a client. Assignment and transfer
   (server kind `transferred`, `collaboration_workflow.go:132`), plus a new
   comment, come from the offline-handoff receipts. The final removal list shows
   three `comment_created` items reaching H in this journey too.
2. **Confirmed AI results only.** The test holds each original natural proof
   (transport returns 503) and asserts that no result notification exists and
   that `proof_state=unknown` before the proof is released. After release:
   - The group states are succeeded, failed and stopped.
   - `exited_proven` is recorded.
   - `forced_disposition=stopped` appears only for the stopped run.
   - The canonical `ResultNotificationsRequireProvenTerminalGroup` test covers
     success, explicit failure, missing, stopped and never_launched. It confirms
     zero notifications before proof, that Stop intent is not reported as a
     confirmed stop, and that the actor is null (the system does not
     impersonate a human).
3. **Opening from the account workbench.** On both clients, each title link
   opens through the account popover to the issue heading. Result links carry
   `#run-<UUID>`, which focuses the run with "Termination confirmed" and its
   records. ai_failed shows "No business result recorded" and "Partial
   recording", and the API confirms `business_outcome=null` and
   `recording.status=partial`.
4. **No notice to the acting human.** For each of the three human workflow
   actions, the actor has no notice for that `event_seq`, while the other
   participant and H have exactly one each. Offline-handoff shows the creator
   with no notices for their own invite and assignment, and H's notices stay
   unchanged after H's own transfer-back. The canonical
   `WorkflowNotificationsAndWorkbench` test asserts that invite notifications
   exclude the actor.
5. **Read/archive change no handling or authority.** All 12 actual UI POSTs
   return 200. After each one:
   - The source issue snapshot, including control, is unchanged.
   - Both of H's workbenches (participating/current) are unchanged.
   - Archiving removes the row but not the task entry.

   The canonical test confirms the workbench predicate ignores archive.
6. **Removal is content-free, and old links are denied.** H gets exactly one
   added `removed` notice (10→11). All 11 notices then have `issue_title=null`,
   `can_open=false`, `source_id=null` and no `run_id`. The live H client sees
   access loss via account WS `collaboration:access_changed`. Account frame
   payload keys contain only IDs and versions. Navigating to an old result href
   (Web page navigation, Electron deep link) shows the denial without the title.
   The API returns 404 for the issue and for the old run-records URL. H's
   participating list becomes empty, and the creator keeps the issue.
7. **Source-inbox dedup and no source records for the external participant.** The
   source creator is a real source member and participant (membership 1), and
   H's source membership is 0. At four checkpoints there are 0 `inbox_item` and 0
   `issue_subscriber` rows for the issue: after the human events, after proven
   results, after read/archive, and after removal. The canonical tests add
   workspace-wide inbox=0 and subscriber=0, and recipient-specific inbox=0 for
   system results. This matches design.md:214.
8. **Delivery failure does not affect the business operation.**
   - A required persistence failure rolls everything back atomically, in line
     with the same-transaction design. Canonical tests cover this:
     `InformationRequestRequiredWritesRollback/INSERT_INTO_collaboration_notification`,
     plus `ResultNotificationRollbackAndConcurrentReplay`, where no settled run,
     termination or event survives.
   - A failure after commit keeps the committed business data. The relevant
     tests are `RealtimeOutboxRetriesWithoutPrivatePayloads` and
     `RetainsFailedSourceInvalidation`. The outbox stays pending and the retry
     keeps its stable identity without private payload. The notification count
     stays 1, so no notification is recreated.
9. **Replay does not duplicate.**
   - A workflow POST replayed with the original key returns an identical receipt
     with identical issue and notifications.
   - The original natural proof bytes, replayed once more, return 200 with
     identical issue and run notice. The browser also shows those bytes did not
     change across 503 retries.
   - Canonically, concurrent proof replay produces exactly 2 recipients, and a
     later participant does not recompute the recipient list.
   - Offline-handoff covers replay of invite and transfer keys.

## Limitations (none blocking)

- Electron "login after offline" is a real password API login injected into a
  fresh profile. It is not login-page UX or installed full Main.
- Assignment and transfer share the server kind `transferred`. No UI assertion
  covers the creator receiving H's transfer-back. The same recipient fanout is
  shown in the browser for information_requested (bystander creator plus H) and
  canonically for invite.
- Comment self-exclusion is not asserted again in these two suites. It relies on
  current AC90/91/97 (`retain_passed`), which this review did not re-review.
- The source-isolation zero counts have no positive control in the browser case.
  The canonical tests and design.md:214 own that rule.
- The canonical post-commit publisher failure uses a creation event. The outbox
  path is generic and is not repeated for each AC83 kind.
- Browser ai_failed is the natural missing-outcome/partial-recording branch, not
  a commercial model failure. Canonical `failure` covers explicit business
  failure.
- I did not probe the listener or build again. I accepted the root preflight and
  the recorded environment (API :18996, Web :13916, renderer :6090) as given.

## Writes

Only this file and `current-session-notification-review.json`.
