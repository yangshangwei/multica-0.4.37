# Retained failures

- router-iter01: 15namedpass/3namedfail. Three custom-status fixture keys used accepted_+fullUUID (41characters), violating established 32character issue_status_key_check before hint requests. Shorter25character synthetic keys correct fixture only. Not product behavioral red proof.
- router-iter02:22namedpass/1namedfail. Legacy actual create expected201 got500 duplicate uq_issue_workspace_number. dbfx.Issue derives max(number)+1 but does not advance workspace.issue_counter, unlike public CreateInTx. Mixing test fixture with real create requires task-owned counter synchronization before request; product unchanged. Exact201 and persisted done assertion retained.
- handler-nul-red01:0namedpass/2namedfail (test plus subtest). New public target_status containing JSON NUL incorrectly returned503, expected400; nonblank-only decoding sent invalid PostgreSQL text through Resolve. Reuse existing status key syntax validator before query; keep Resolve for catalog authority and active status. This is actual red evidence; following frozen runs must show green. Raw logs and record hashes retained.

Counts include subtests and are never added as independent coverage. Failed databases are retained, never cleaned or reused for acceptance.

- backend-format02: exit2, transient missing brace in the new handler while adding target-status validation. Parser error retained in private log; corrected and backend-format03 exited0. No DB was accessed by these formatting checks. Not behavioral red proof.
