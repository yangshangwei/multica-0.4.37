# Design

The gaps belong to shared views and browser selectors. Preserve auth APIs, state ownership, explicit recommendation adoption and inbox semantics.

## Change boundary

- `packages/views/auth/password-form.tsx` and tests: repair the blur/submit interaction at the form, retaining keyboard validation, error association and request deduplication.
- `packages/views/modals/quick-create-issue.tsx`, `creator-recommendations.tsx` and relevant tests if needed: bound recommendations within the creation dialog while retaining editor space and reachable suggestions. Reuse current flex/overflow patterns and tokens.
- `e2e/core-business-journeys.spec.ts` and `e2e/upstream-selected-fixes.spec.ts`: replace obsolete explicit-role selectors with accessible semantics scoped to inbox rows. Retain all business assertions.
- `e2e/password-registration.spec.ts` and `e2e/quick-create-actor-picker-phase2.spec.ts`: preserve current assertions, strengthening first-click or reachability only where useful.

No new abstractions or dependencies are expected. Browser geometry and pointer events are authoritative; avoid DOM tests that merely assert CSS class strings.

## Environment

Use an isolated synthetic-account PostgreSQL database and production Web and a task-owned development API. Save provenance for baseline and repaired builds. Run inbox/recommendations in legacy auth with the deterministic local provider and registration in password mode. Verify `/api/config` and provider `/health` after mode changes. Serialize builds per checkout; never touch existing human services.

## Risks and rollback

Verify blur validation, keyboard focus, error announcement, password visibility, desktop geometry, long text and suggestion scrolling. Roll back only task-owned patches, never reset unrelated dirty files.
