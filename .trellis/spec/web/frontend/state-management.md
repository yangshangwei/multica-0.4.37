# State Management

> How state is managed in this project.

---

## Overview

<!--
Document your project's state management conventions here.

Questions to answer:
- What state management solution do you use?
- How is local vs global state decided?
- How do you handle server state?
- What are the patterns for derived state?
-->

(To be filled by the team)

---

## State Categories

<!-- Local state, global state, server state, URL state -->

(To be filled by the team)

---

## When to Use Global State

<!-- Criteria for promoting state to global -->

(To be filled by the team)

---

## Server State

<!-- How server data is cached and synchronized -->

(To be filled by the team)

---

## Common Mistakes

<!-- State management mistakes your team has made -->

(To be filled by the team)

## Restricted password sessions

`AccountGate` replaces the route tree while the user must bind an old account
or change a recovery password. As soon as the auth store publishes a normal
user, the route tree resumes and its existing login/dashboard guards own
navigation. Do not navigate again in the gate form's asynchronous completion
callback: it can run after unmounting and override the resumed route's redirect.
`e2e/password-migration.spec.ts` covers legacy binding and recovery-password
change returning to the original workspace, including reload.

Background usage reporting also requires `status === "authenticated"`; a
non-null user ID can still represent a restricted session. The same UUID moving
from restricted to authenticated must trigger the normal report.
