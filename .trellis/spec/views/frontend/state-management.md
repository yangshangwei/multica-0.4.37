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

### Core store injection through navigation plumbing

Shared Zustand stores stay in Core. A mounted `NavigationProvider` may inject a
Core-owned StoreApi through Context when a routed component must retain client
intent across a platform page-segment remount. Context supplies the dependency;
it must not become another owner of business values or API response data.
Selectors return stable state references. The current-issue history disclosure
is the bounded example: one ephemeral record per mounted navigation surface,
not a persisted preference or a registry of visited entities. Its executable
contract and tests live in [Iteration Operations](../../core/frontend/iteration-operations.md).

---

## Server State

<!-- How server data is cached and synchronized -->

(To be filled by the team)

---

## Common Mistakes

<!-- State management mistakes your team has made -->

(To be filled by the team)
