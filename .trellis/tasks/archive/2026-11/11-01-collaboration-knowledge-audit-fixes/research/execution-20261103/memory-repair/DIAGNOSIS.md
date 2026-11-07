# Memory terminal-state E2E on frozen v13

State: **proposed patch only; not applied or run**. Actual failure 13 reaches the complete-issue click after all earlier concurrent memory edits, merge, restore and four-version history assertions, then times out finding `Complete issue` at line 76. The test has trace disabled; its original raw excerpt and screenshot/error-context paths remain in v13, with the relevant excerpt copied here.

`seedMemoryIssue` creates a real collaboration issue whose handler is `fixture.participant`, not the creator. The current contract requires that external handler's formal result/basis before the creator may accept it. Shared-context text and version restoration do not count as a delivery. The missing completion button is therefore the expected current presentation of a missing prerequisite, not a memory write/freeze regression.

The single-file proposal changes only the terminal-state preparation in `e2e/collaboration-memory-web.spec.ts`:

1. The participant, already present in the second real browser context, uses `Submit result`, enters result and basis, and commits the real delivery form.
2. The creator reads the exact current submission/author and success confirmation, then reloads the same `#memory` entry and uses `Accept and complete` for that precise ID.
3. The actual lifecycle POST and receipt must identify that submission, and the final readable issue must be done with the creator's accepted decision. Reopen is found through the current `More actions` disclosure.
4. All original terminal read-only, no-restore, historical purge, retained version-4 content, viewport and page-error assertions continue unchanged. The second original-key lost-save/reload test is byte-identical.

No source fixture policy, memory assertion, timeout, retry or SQL setup changed. Neither an API write nor fabricated terminal database state replaces the user-facing completion. The existing `#memory` anchor remains responsible for revealing the shared-context panel; no blanket disclosure helper masks anchor or pending-write recovery.

`proposal.json` contains the original/proposed/patch hashes. TypeScript parse-only and `git apply --check` pass. Those checks do not establish a green result. After v13 is fully retained with its final audit, independently review this exact diff and exercise both memory tests on the successor source with actual API/Web identity. Keep the original failure separate.
