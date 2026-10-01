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

### Administrative requests with uncertain outcomes

An unresolved control request is a client command draft, not cached server
state. Persist only its original idempotency key, non-secret payload and stable
server/actor/organization/target scope. Use an independent StorageAdapter key per
request; a shared JSON-array read/modify/write can lose another tab's command
even when both tabs successfully read back their own write. Adapters without key
enumeration fail closed before sending a command they cannot later recover.

Do not register these drafts as `workspaceScoped: false` in the workspace draft
cleanup registry. That registry removes global keys during *any* workspace
leave/delete. Administrative intents use explicit session-only cleanup instead.

On reload, query the original actor/key before enabling replacement. An empty
lookup alone never proves a delayed request cannot commit. Retire an old draft
only after a durable receipt, or an endpoint-specific rejection whose locked,
monotonic version contract makes that exact mutation impossible. Keep generic
conflicts/network failures on the original key. Server data and operation
receipts continue to use scoped React Query caches.
