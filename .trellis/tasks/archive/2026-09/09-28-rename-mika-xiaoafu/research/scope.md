# Scope evidence

- Shared frontend: four locale files / 28 strings, two templates / six string values, one bootstrap error; seven files total.
- `server/internal/service/builtin_agents.go:16-25` separates identity, default name and prompt placeholder.
- `server/internal/handler/mika_agent.go:120-126` returns existing rows without overwriting content.
- `server/internal/handler/mika_onboarding_opening.go:58-67` interpolates the current saved name.
- `server/internal/handler/mika_agent_test.go:123-139` protects customized name/description/instructions.
- `server/migrations/046_agent_unique_name.up.sql:20` covers archived name conflicts too.
- `server/internal/util/mention.go:16` uses UUID targets; `server/cmd/multica/cmd_autopilot.go:943` can resolve CLI arguments by current name.
- `apps/web/features/landing/i18n/{en,zh}.ts` contains only historical release-note mentions.
- Mobile has no Mika-specific literal and renders the service-provided name.

Unrelated skill-market edits exist in the original checkout. Implementation is isolated and integration must preserve them.
