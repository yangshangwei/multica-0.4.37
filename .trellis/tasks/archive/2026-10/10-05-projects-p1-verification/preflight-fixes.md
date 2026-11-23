# Verification prerequisites

The P1 branch inherited a reproducible full-suite blocker: TestBuiltinSkillsConformToTemplate reports multica-creating-agents/SKILL.md at 534 body lines, above the 500-line limit. The failure predates P1 (also recorded in the T1 merge evidence).

Cleanup plan before editing: move the existing Workspace MCP servers subsection unchanged into the one-level references/workspace-mcp.md support file; leave an explicit pointer in SKILL.md and update its source-map reference. Preserve all instructions and examples. Do not copy or overwrite the original main worktree's unrelated uncommitted work. Re-run the existing built-in skill contract test; no new behavior-mirroring test is needed for this documentation-only extraction.

Results: the 77-line Workspace MCP section was preserved verbatim (apart from promoting its heading) in the supporting reference. TestBuiltinSkillsConformToTemplate passed after extraction and the new P1 documentation. The generated in-app docs bundle was regenerated, including the previously stale self-host quickstart payload.

The full Views run also exposed an existing asynchronous Select test race: it awaited the click but immediately queried portal options synchronously. The test now waits for those same accessible options with findByRole; labels and selected actor UUID assertions are unchanged. An isolated original run passed, while the full-suite failure is retained in .omx/p1-ts-all.log; final complete rerun remains required.
