Method: dual-agent (A: blocked_design_review · B: blocked_evidence_review)

# Agent-create final scoped design review — 2026-11-20

The original blocked-state P1 is resolved. Users can read why submission is blocked, reach the correction target with keyboard or pointer, and choose setup or manual creation without losing their draft. The independent design assessment scores the surface **33/40 (Good)**. After that assessment, integration measurements caught a remaining recipient-hint spacing reversal; one local grouping correction was made and the final independent browser matrix confirmed it. The design score remains the original independent score, not a retrofitted score for the later correction.

## Design health

| Heuristic | Score | Main evidence |
| --- | ---: | --- |
| Status visibility | 3 | Persistent durable blocker, focus/hover reason and accessible description |
| Real-world language | 3 | Consistent handoff verb and concrete recovery choices |
| User control | 4 | Both exits preserve the draft; assistance stays optional |
| Consistency | 3 | Same helper conventions and outlined recipient control |
| Error prevention | 3 | Shared submit-state mapping and single-flight composer guards |
| Recognition | 3 | Contextual instructions and visible recipient/default choices |
| Efficiency | 4 | Modifier send from non-input controls, without portal/input hijacking |
| Minimalism | 3 | Compact composer; narrow English warning is text-heavy |
| Error recovery | 4 | Specific cause, next steps and protected draft |
| Contextual help | 3 | Reasons and prerequisites at the blocked action |
| **Total** | **33/40** | **Good** |

## Product fit and findings

The neutral, compact Operate surface retains Multica's existing vocabulary: recipient, responsibilities, description, optional AI help and explicit handoff. The no-recipient warning now presents one decision with two concrete paths. The strongest improvement is meaningful recovery at the exact point where a new user was previously stuck.

The P2 grouping defect measured in the first browser batch is fixed. The hint now lives with its picker, so recommendation prerequisites cannot wrap between them. The final picker-to-hint gap is **4px**, versus **12–80px** to the description in available states. This was verified in English and Chinese at 1280px and 375px widths.

No in-scope P0/P1 remains. Explicitly deferred P3 observations remain: narrow English warning density, the accessible dialog name differing from the visible mode title, and existing picker category density. They do not prevent the approved task from completing and were not expanded into this change.

## Independent evidence

Assessment A reviewed the source and fresh real screenshots without detector or Assessment B findings. It checked both draft-preserving exits, keyboard explanation/focus and the non-input send path. Native CUA had no available browser; both reviews used fresh isolated Chromium contexts and never attached to the user's browser or Desktop.

Assessment B checked **16 contexts / 32 empty-and-filled samples** on final production build `yPiCLeEob9_90sBlHK-1y`. Footer and submit controls were fully reachable, pointer targets were valid, and there was no horizontal document overflow. Matching-state helper colors and opacity are identical. Actual composited contrast is **5.913:1 / 6.340:1 enabled** and **3.737:1 / 4.952:1 disabled** for light/dark themes.

Final CLI detector scan: **0 findings** across the four changed components. The successfully injected headless overlay made whole-page Inter/type-hierarchy/background-heading observations; these were reviewed against the incumbent product conventions, not misrepresented as new component defects. No human-visible overlay is claimed. The temporary detector live server and all review browsers were stopped.

## Personas and limits

- **Jordan:** the durable blocker now gives setup/manual choices and preserves work; runtime vocabulary still belongs to the existing product model.
- **Sam:** Tab and hover expose the reason, accessible description is always present, and activation routes focus correctly. Actual VoiceOver/NVDA speech and physical touch were not tested.
- **Alex:** the send chord works after selection and on non-input controls while retaining editor/portal/plain-Enter behavior.

See `design-assessment-a.md`, `design-assessment-b.md`, `visual-verdict.json` and `acceptance.md` for receipts, source boundaries and exact testing limits. The original score was 25/40; this review is 33/40. The comparison is scoped to the same surface and ten heuristics.

Questions skipped: the user already approved the implementation scope and requested completion and archival.
