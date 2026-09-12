# Messaging integrations in isolated deployments

## User decision

The deployment has no public internet access. The user approved a deployment setting that disables external messaging integrations on the server and hides the matching Settings and agent-detail entries, while retaining self-hosted Git integrations.

## Requirements

- Operators can disable Lark, Slack, DingTalk, WeCom, and Telegram together through deployment configuration.
- Disabled providers cannot open background connections, accept new bindings, or perform provider API operations, even if old credentials remain configured.
- Web and desktop Settings hide all five provider sections when disabled. The self-hosted Git section and its configuration remain available independently.
- Agent detail hides messaging tabs, binding affordances, and status shortcuts when disabled, and does not issue provider queries.
- Existing installations and encrypted credentials are retained for a later explicit re-enable.
- The shipped configuration and documentation explain the isolated-network setting and the separate Git prerequisites.

## Acceptance criteria

- Public runtime config accurately exposes the operator's messaging policy.
- Disabled behavior is tested with provider credentials/services present; enabled behavior remains covered.
- Malformed runtime responses are parsed safely, without dropping an explicit disabled policy because of unrelated optional fields.
- Settings tests prove Git remains visible while the five messaging integrations are absent.
- Agent tests cover hidden entries and recovery from a previously selected integrations tab.
- No new dependencies, migrations, or unrelated catalog changes.
