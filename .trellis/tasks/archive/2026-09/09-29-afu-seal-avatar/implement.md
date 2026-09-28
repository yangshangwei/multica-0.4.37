# Implementation plan

1. Export the approved glyph outline into a fixed SVG and 512 px PNG under
   `server/internal/handler/builtin_agent_avatars/` (no runtime font dependency).
2. Backend lane: embed the PNG, serve the fixed versioned public route
   `/api/avatars/builtin/afu-seal-v1.png`, use it when creating Mika, resolve
   it through the existing public URL helper even without storage, and migrate
   only built-in agents with the old `emoji:🦄` default. Test migration preservation,
   creation and asset delivery using isolated test data.
3. Frontend lane: replace the two placeholder spans with the existing ActorAvatar
   primitive and one shared default-avatar path. Resolve the path against the API
   base for desktop. Reuse mobile's existing relative URL resolver before image
   validation. Regression-test onboarding URL wiring and mobile URL semantics.
4. Run focused Vitest suites, Go avatar/migration tests, affected package typechecks,
   lint/static checks and visual comparison to the approved concept. Review the
   full scoped diff; do not stage unrelated work or publish.

The stored URL remains an ordinary image URL; no new avatar protocol or identity
heuristic is introduced. A PNG is served because native RN Image does not reliably
render SVG, while the SVG remains the editable source of the artwork.
