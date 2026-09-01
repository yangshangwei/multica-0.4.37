# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

(To be filled by the team)

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)

---

## Desktop Shell Entry Invariants

### `onboarded_at` is the only door into the dashboard

`apps/desktop/src/renderer/src/App.tsx` routes on two signals: `onboarded_at != null`
and workspace count. A user with a workspace but no `onboarded_at` is sent to the
onboarding overlay, not the dashboard — `CreateWorkspace` deliberately does not
stamp the flag (only `CompleteOnboarding` and `AcceptInvitation` do).

**Consequence for any new path that provisions a user server-side** (device auth,
SSO, imports, seeded demo accounts): stamp `onboarded_at` in the same transaction,
or the client will hold that user behind an onboarding flow it has nothing to
onboard. `MarkUserOnboarded` is idempotent (`COALESCE(onboarded_at, now())`), so
calling it is safe on a returning user.

Verified by: `server/internal/handler/auth_device.go` (device provisioning),
`TestDeviceLoginProvisionsIdentityAndWorkspace` asserts the response carries a
non-null `onboarded_at` for exactly this reason.

### A workspace is not usable without its status catalog

Creating a `workspace` row is not enough: `issue_status` carries the 7 built-ins per
workspace (MUL-6243) and an issue cannot be created before its status resolves.
Any code that creates a workspace outside `CreateWorkspace` must call
`issuestatus.Ensure` in the same transaction. The seed is idempotent and
concurrency-safe, so calling it for a pre-existing workspace is a cheap self-heal.
