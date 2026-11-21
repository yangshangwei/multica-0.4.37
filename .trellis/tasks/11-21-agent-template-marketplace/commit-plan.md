# Commit ownership and execution

The user approved implementation and instructed autonomous continuation. Execute scoped, reversible commits after final verification; do not ask again or include unrelated work.

## Work commit

Intent: `feat(agents): make role discovery available alongside team capabilities`

- Include the 18 task-owned Core/Views, locale, E2E and spec paths listed in the verification record, plus this task's planning and evidence files.
- Preserve the original working files in place. The checkout already had uncommitted market `CollectionPageHeader`, navigation-provider test wiring, and `learn_more` copy before this task. Those changes landed separately in `a1d791dcb` during final verification. Refresh the isolated index from that new parent and commit only this task's remaining diff; do not replay or revert the earlier work.
- The updated market tagline is part of this task because it adds agent templates to the capability description. The help link and header implementation belong to the separately committed parent.
- Review the isolated staged diff and run its market page test before committing. Use a Lore intent/body/trailer message and do not push.
- Preserve all unrelated staged entries. After commit, refresh only the task-owned entries in the ordinary index so other work retains its previous staged/unstaged state.

## Bookkeeping commits

1. Once work is committed and all acceptance evidence is recorded, archive only `11-21-agent-template-marketplace`. Use `--no-commit` and make a scoped Lore bookkeeping commit to avoid absorbing another task's work.
2. Record this session with the work commit hash, using `--no-commit`, then commit only the newly generated journal delta. Existing journal edits from another session remain untouched.

## Unrelated work

The checkout has extensive pre-existing changes outside this task. None belongs to this commit plan. The four shared files are now compared to their separately committed header/provider/help-link baseline. Do not bulk stage, amend, reset, clean, or push.
