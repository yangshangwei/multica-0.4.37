# Custom Agent Categories

Users need custom categories for new and existing agents, beyond the built-in role groupings.

Acceptance criteria:
- A user who can edit an agent can assign/change/clear its category; values survive reloads.
- Creation supports selecting an existing visible category or typing a new name. Duplicates and saved drafts preserve it.
- Category filtering, search and grouping work in the directory and compose with current scopes and role/squad filters.
- Unclassified agents have a clear Uncategorized state. Role provenance and behavior remain independent.
- Web and desktop share implementation; English and Chinese text, error feedback, keyboard accessibility and overflow handling are present.
- Backend validates input and permissions; older API responses remain usable; meaningful regression tests pass.
