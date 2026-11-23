# Current exact-result and detail E2E maintenance boundary

This lane owns only `collaboration-workbench-web.spec.ts`,
`collaboration-memory-web.spec.ts`, `collaboration-boundaries-web.spec.ts`,
`collaboration-preview-web.spec.ts` and
`fixtures/collaboration-mention-journeys.ts` under `e2e/`. Original source bytes
are preserved under `before/`; frozen candidates and old failed records remain
untouched. The parallel disclosure lane owns its helper and other source files.

The behavior gaps are retained positive fixtures that complete external work
without a formal result, action checks that assume all controls are flat, and
preview tests that require Send to stay disabled after suppression changes.
Current contracts live in the shared workflow/composer and public delivery API.

Planned maintenance:

- Handler publicly submits real result/basis using current status observations;
  creator accepts the exact returned/current submission identity. Memory and
  workspace-retention tests preserve their full original post-closure checks.
- The lifecycle-conflict test delays a valid exact acceptance POST. Its former
  handler may publicly leave after submission; that independently permitted
  action advances the revision without erasing the delivered result. Preserve
  409 stale-revision, retained form/status, no automatic POST/GET, explicit rebase
  and a distinct final request key accepting the same submission.
- Reveal More/Activity through real summary controls. Compare explicit complete
  role action sets for the seeded current facts; keep all direct server 403 and
  non-queueing negatives. No capability-to-itself expected policy.
- Retain supported Preview delivery calls. After suppression invalidates it,
  enabled Send obtains fresh preflight and stops at AI confirmation with zero
  comment writes; an explicit later Send alone publishes. Retain draft, grouping,
  suppression, target/version, receipt, notification and replay assertions.

No production/API implementation, timeout, retry, skip, dependency, database or
Provider change is planned. Root owns live browser/services/DB execution; this
lane uses only static, package and test-discovery checks. Any true product
contradiction is reported upward rather than hidden by fixture changes.
