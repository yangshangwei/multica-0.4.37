# Design

Keep `SkillLibraryCatalog` as owner of the two views and template query. Replace the workspace shelf with a compact, keyboard-accessible "From template" action opening the existing catalog. Retain the mounted-but-hidden workspace panel so virtualizer and list state survive navigation.

Keep existing catalog/source/category/search derivations and copy session. Internal `market` identifiers stay to preserve preferences; user-facing names become templates. Remove only shelf collapse state/setter/persistence/copy. Drop the shared page heading count; tab counts retain distinct scopes.

Preserve visual tokens, typography and existing cards. Place workspace content directly below the tab/action area. Old persisted shelf fields must have no UI effect and must not be written by new persistence. Keep empty-workspace initialization, drafts, copy navigation and focus behavior unchanged.

Use component tests for separation, entry navigation, counts, loading/errors and focus; state tests for preference preservation; browser tests for real copy creation, wide/narrow layouts and dark mode. Reverting the scoped UI diff needs no data migration.
