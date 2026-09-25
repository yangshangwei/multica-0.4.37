# Squad discovery and management

User approved implementation of the 2026-09-25 design audit in this thread.

## Acceptance criteria
- Saved workspace squads and built-in templates have distinct accessible views, with workspace squads as the default.
- Template browsing retains project configuration and existing-instance navigation, without the short nested scrolling catalog.
- One primary creation control retains template and custom creation.
- Squad names are real keyboard-accessible links; nested actions do not trigger row navigation.
- Scope selection exposes selected semantics; filter clearing is a separate keyboard-accessible action; icon controls have names.
- Wide rows keep identity and metadata together; creator and creation time are opt-in defaults without overwriting saved preferences.
- Template copy describes use cases concisely; custom squad descriptions are never overwritten.
- All four locales remain structurally consistent.
- Focused tests, lint, type checks, static checks, and visual review support completion.

No new dependencies, server API changes, runtime status claims, or usage metrics.
