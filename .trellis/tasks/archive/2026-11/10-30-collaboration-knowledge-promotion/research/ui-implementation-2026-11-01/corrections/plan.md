# B2 recovery and audit correction

Main's read-only review is `../main-code-review.md`; main owns closure and platform/screenshot acceptance. The completed mechanical style pass is not rerun.

Core will distinguish a proven-unused first intent (pre-POST preservation/local validation failure or an exact known unsealed new-proposal refusal) from an operation with prior/in-flight dispatch evidence. An existing recovery identity or remembered dispatch keeps subsequent transport/4xx/not-found recovery uncertain. Only the known-unused identity can be retired to unlock the human draft. No HTTP response or receipt is fabricated and no automatic replay/new UUID is introduced.

A separate typed cleanup error will carry an already-confirmed receipt if local identity retirement fails after success/refusal. The UI will display that known result plus cleanup feedback, release the completed intent, and not relabel it as an uncertain write. The existing raw API behavior remains unchanged.

The live review will display proposer UUID when no lawful local name exists, preserve approval actor/time across later decisions, and display latest decision actor/time/reason without repeating identical approval facts. Reasons remain available-source-only; restricted/source-unavailable projections keep their existing privacy boundaries. Use incumbent dl/text controls and bilingual copy.

Tests first: first refusal/edit/new intent, earlier ambiguity/later 4xx/not-found (including remembered dispatch and retained identity), storage failure before POST versus cleanup failure after a receipt, and audit metadata visibility/deduplication. Final evidence stays in this corrections directory; original B2 results are preserved. No Go, DB, provider, router, dependencies, commits or subagents.
