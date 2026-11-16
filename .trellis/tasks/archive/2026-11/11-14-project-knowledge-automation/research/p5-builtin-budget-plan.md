# P5 builtin skill loading budget

The current full service replay exposed `TestBuiltinSkillsConformToTemplate/multica-working-on-issues`: its body has 504 lines against the existing 500-line L2 budget. Preserve the failed report and the template assertion.

The bounded change will replace the duplicated introductory knowledge detail with a short human-authority statement and an explicit link to the existing one-level `references/collaboration-workflow.md#managed-knowledge` section. That section already preserves policy, system authority, source access, adoption, reopen/reacceptance, human overrides, original-request recovery and local session contracts. Update its source-map explanation in the same change; add no supporting file, dependency, product behavior or budget exception.

Write scope: `builtin_skills/multica-working-on-issues/SKILL.md` and `references/working-on-issues-source-map.md`. Verify through the existing builtin template/source-map tests and a new fresh owned-DB full service replay. Do not add a test that duplicates the line-count implementation.
