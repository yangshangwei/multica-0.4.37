# Security squad autonomous repair

## Goal
Let a squad composed entirely of agents process agent-created issues through nonproduction security assessment, reproduction, repair, independent verification and review, delivering to `in_review` without mandatory human creator/owner or G1/G2 release comments.

## Background and authorization
The user approved the six-part change proposal in this conversation, then explicitly requested implementation and creation of this tracking task. This is implementation approval of the existing plan; no new product decisions remain.
Current v2 squad and leader instructions require human gates; restricted reviewers cannot dispatch execution; shared diagnostician and QA instructions stop on any security defect. Existing instances retain copied instructions.

## Requirements
- R1: Remove human-creator/owner prerequisite and mandatory G1/G2 releases. Keep assessment and independent review as evidence checks. Missing/failed required results never count as success.
- R2: Return actual delegated reviewer results to their original authorized security squad coordinator automatically, without giving reviewers general dispatch, status or acceptance rights. Preserve inert comments and other restricted-role guarantees.
- R3: Permit scoped nonproduction diagnostics and tests using local fixtures/synthetic data; preserve stop conditions for production operations, credentials/customer data, unauthorized access, external disclosure and human-only risk acceptance/wont-fix.
- R4: Arrange reproduction before repair, independent verification and bounded repair/review retries. Keep evidence tied to actual source tasks and commits; changed code invalidates previous results. Deliver only to `in_review`, not `done`.
- R5: Ship updated default templates, skills, bilingual product descriptions and docs. Preserve custom saved instances; make the existing-instance upgrade procedure explicit and report deployment/configuration limits honestly.

## Acceptance criteria
- AC1 (R1,R4): An all-agent fixture with an agent-created issue can traverse assessment, reproduction, repair, verification and review without human comments and finish in_review.
- AC2 (R2): Genuine current delegated reviewer completion returns only to the authorized originating squad/leader. Forged/unrelated/stale results, mentions, progress and duplicate events cannot dispatch arbitrary work or mark acceptance.
- AC3 (R2,R4): Failures/no-result outcomes permit bounded recovery, never pass review; revoked authority and changed assignment prevent inappropriate continuation.
- AC4 (R3): Local authorized security diagnosis/testing is permitted; production, secrets, disclosure, risk acceptance and unapproved scope still require human handling.
- AC5 (R5): Defaults and documentation agree; saved instance behavior and upgrade limits are stated. Existing ordinary role restrictions continue to pass regression tests.

## Out of scope
Production deployment, external messages, accepting business risk, broad role permission relaxation, replacing existing workflow architecture, overwriting unrelated custom instances or accepting issues as done.
