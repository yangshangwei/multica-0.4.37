# Type Safety

> Type safety patterns in this project.

---

## Overview

<!--
Document your project's type safety conventions here.

Questions to answer:
- What type system do you use?
- How are types organized?
- What validation library do you use?
- How do you handle type inference?
-->

(To be filled by the team)

---

## Type Organization

<!-- Where types are defined, shared types vs local types -->

(To be filled by the team)

---

## Validation

<!-- Runtime validation patterns (Zod, Yup, io-ts, etc.) -->

Successful API responses must retain valid resource identity when an optional
collection is empty. Go nil slices serialize as `null`; Zod `.default([])` only
handles missing values. Initialize successful server response collections to
empty slices and normalize documented nullable collections at the client
boundary with `.nullish().transform((items) => items ?? [])`. Keep essential
resource fields and invalid collection types strict. See `StaffedSquadSchema`
and `agent-template-schemas.test.ts` for the template-creation regression.

---

## Common Patterns

<!-- Type utilities, generics, type guards -->

(To be filled by the team)

---

## Forbidden Patterns

<!-- any, type assertions, etc. -->

(To be filled by the team)
