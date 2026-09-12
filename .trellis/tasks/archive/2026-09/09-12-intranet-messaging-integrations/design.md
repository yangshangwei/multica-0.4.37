# Design

## Deployment policy

Add `MULTICA_MESSAGING_INTEGRATIONS_ENABLED`, parsed once at server startup, with the historical enabled behavior when unset. An explicit `false` disables all five external messaging providers. Keep `MULTICA_VCS_INTEGRATION_ENABLED` and `MULTICA_VCS_SECRET_KEY` independent. This is a provider policy, not automatic network detection or an application-wide offline mode.

Expose `messaging_integrations_enabled` as a boolean in `/api/config`. Shared core parses it, loads it into `configStore.messagingIntegrationsEnabled`, and supplies the same value to web and desktop. Older servers that omit the additive field retain historical behavior; malformed supplied values must not accidentally enable the policy.

## Server boundary

Use the existing provider wiring boundaries and unavailable responses. Disabling the policy must prevent channel factory registration, background provider refresh/consumer work, and provider-specific HTTP operations. VCS, ordinary Git checkout, MCP, task/chat functionality, and stored integration rows are outside this switch.

## UI

Reuse the existing Settings sections and agent detail composition. Gate entire messaging sections/components so child queries and binding flows never mount. Retain the Settings integrations destination for self-hosted Git. Hide the agent messaging tab and compact integration shortcuts; normalize a stale selected tab to the existing default rather than displaying an empty body.

## Scope and risk

The important risk is a hidden UI with a still-running background connector; server wiring and routes require tests. No credentials are deleted or rotated. Existing uncommitted built-in catalog work belongs to another task and must remain untouched.
