# AC96 catalogue authority matrix boundary

This is continued S6 verification in the existing task, not a new task or plan.
The uncovered evidence is a combined current regression connecting catalogue
admission, actual participating resource owners, later participants, temporary
availability, and derived-run/current credential authority.

The behavior belongs to `CollaborationService`: `RegisterTarget`, explicit owner
consent, `AuthorizedMentionTargets`, queue admission, managed Claim/Permit,
runtime business calls, and the existing participant departure invalidator.

Write ownership is limited to:

- `server/internal/service/collaboration_catalogue_matrix_test.go`: one canonical
  matrix built with existing managed namespaces, testutil rows and service
  writers. No shared helper or production code edits.
- This directory's `catalogue-*` reports, commands, results and safe source
  hashes, to make the exact execution and its limitations reviewable.

The test will distinguish creator/current-handler organization from ordinary,
former and source-only participants; reject an uninvited owner; require each
actual leader/member/runtime owner's own current grant; prove new participants
reuse the original grants and valid paused/offline/busy targets retain waiting
input; preserve exact derived grant/runtime/owner provenance; reject leader
cross-member and stale selection/credential paths; and retain history, legitimate
Stop and ordinary-child/machine reference boundaries after withdrawal/departure.

Only task-owned `service_s5` (schema 737) may run the database race test, serially
coordinated with the root. The root owns other suites, acceptance/spec updates
and Lore commits. Production gates, real provider accounts/quota, installed
clients, kernel containment, unrelated changes and parent archival are outside
this test-only boundary. A concrete product defect is reported before widening
any write scope. Proof payloads are synthetic service evidence.
