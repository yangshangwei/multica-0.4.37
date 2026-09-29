# Independent phase-2 plan review

Date: 2026-09-30. Original checkout review; no product or global-state changes.

## Verdict: APPROVE

No blocking omissions found. The user authorized design and development and selected explicit “帮我选” activation; this review introduces no additional permission gate. The bounded plan can proceed to implementation, with the small clarifications below recorded for the implementers.

## Evidence and scope

Read `CLAUDE.md`, this task's PRD/design/implementation plan and both research notes. Reused the research findings and checked the concrete project projection, API response parsing, invocation decision/batch loader and existing LLM handler seams. No tests were run during this planning review.

| Area | Verdict | Reason |
| --- | --- | --- |
| Creator versus assignee | Pass | Explicitly distinguishes the assistant that creates an issue from its eventual assignee and project execution squad. Recommendations and adoption cannot submit, execute, set a default or update recent history. |
| Default selection | Pass | Five-level precedence is explicit; pending high-priority references do not become invalid merely because another query finishes first. Manual choice remains available and is not overwritten by later queries. Existing scoped preference lifecycle is reused. |
| Project candidates | Pass | `getProjectExecutionSquads` actually uses plural nullish precedence, preserving an explicit empty array. Configured IDs are intersected with current eligible squads; 3 project / 3 favorite / remaining recent slots enforce a total of eight and global typed-reference deduplication. Full project and full catalog paths remain reachable. |
| Recommendation ownership | Pass | Explicit button reads the live editor, uses workspace-bound request options and abort/request identity, and rejects stale completion. Adoption alone runs the existing selection path, with a fresh eligibility check. Research additionally covers mode changes, continuous-create drafts and disabling adoption during submission. |
| Permission boundary | Pass | Catalog is loaded server-side and invocation permission is distinguished from admin visibility. The existing batch target loader plus a parity-tested pure decision extraction avoids per-candidate authorization queries. Final selected candidates receive bounded revalidation. |
| Retrieval quality and bounds | Pass | All eligible descriptions are scanned before the large-catalog shortlist is limited to 40. Lexical recall limitations and non-optimality are explicit. No arbitrary first-40 filling; exact saved evidence is checked, and zero recommendations is valid. |
| LLM trust boundary | Pass | Candidate names, database IDs, instructions, runtime data and credentials are omitted from structured provider fields; temporary references map only to authoritative candidates. Untrusted text cannot grant permission. Strict ref/evidence validation, no tools/writes, payload limits, deadline, shared admission and sanitized errors make the service bounded. |
| API compatibility | Pass | New API uses explicit workspace and abort propagation, validated response shape and malformed-response errors. Old-server 404 preserves manual creation. Research explicitly requires harmless future response fields to remain compatible. |
| Verification | Pass | Canonical store/seed/projection tests, real picker tests, fake-provider handler/outbound tests, malformed-output and permission-race matrices, plus actual API Web/Electron checks cover the intended contracts without paid providers or real agent execution. |

## Small implementation clarifications

1. **P2 — Separate small- and large-catalog excerpt rules.** `design.md:39` allows all described candidates when their count is at most 40, then says to keep positive lexical evidence only. Make the latter explicitly apply to catalogs over 40. For a small-catalog candidate without a lexical hit, use a stated deterministic excerpt policy, such as the first 600 runes; hit-based windows still apply when a hit exists. This avoids accidentally removing the intended small-catalog synonym/cross-language opportunity. Test both branches.
2. **P2 — Strict field validation is not strict rejection of future fields.** At `design.md:33`, use ordinary object parsing/stripping for harmless extra server fields while rejecting invalid required fields, duplicate typed IDs and over-limit arrays. Add a harmless-extra-field acceptance case beside the malformed-response cases. This follows the research note and existing API boundary, rather than treating Zod `.strict()` as the intended policy.
3. **Optional bounded principal simplification.** A human-member-only recommendation endpoint is sufficient for the authorized Web/Desktop feature. If selected, explicitly reject agent credentials before candidate/provider access and test zero provider calls for that branch. Otherwise retain the planned effective-originator semantics and their parity tests. Neither choice permits agent credentials to borrow the credential owner's private-agent access.

## Representative implementation checks

- A stored squad default awaits squad/leader/permission data; an early agent list cannot seed a fallback and write it into the draft. Once the default is known invalid, fallback may proceed and clearing the invalid preference remains possible without disclosing its name.
- A fourth configured project squad is excluded from favorite/recent shortcuts but remains reachable through the complete project list and full-directory search. Switching project changes grouping, not the selected assistant or recommendation request payload.
- In a 550-object directory, a relevant description near the tail is scored before shortlist truncation; only its bounded evidence window and temporary ref enter the model payload. A forged ref or quote fails validation. A revoked candidate or changed description after generation is rechecked before the API returns.

## Remaining execution risks

Permission extraction is security-sensitive: preserve owner/public/private/unknown-mode behavior with tests before refactoring. Exact quotes prove that a responsibility statement was saved, not that the candidate is objectively optimal; the chosen UI wording and documented recall limitation correctly avoid that stronger claim. Default/query races, cancellation despite ignored abort, keyboard focus and Web/Electron integration still require the planned implementation evidence. Existing working-tree changes must remain outside the phase-2 increment.
