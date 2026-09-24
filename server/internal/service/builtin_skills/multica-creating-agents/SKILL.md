---
name: multica-creating-agents
description: "Use when creating, inspecting, or debugging a Multica agent definition via the `multica agent` CLI or POST /api/agents. Not for assigning issues to agents that already exist, and not for runtime task prompts."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Creating Multica agents

This is the contract for Multica's agent-creation path: what the create entry
points accept, what the server validates and rejects, how each field is
persisted, and which fields the daemon actually reads at claim time. It is
not a parameter manual — it states source-traced facts, and every claim is
backed by `file:line` in `references/creating-agents-source-map.md`.

## Quick start (read-only inspection)

These commands read state and have no side effects:

```bash
multica agent get <agent-id> --output json      # full persisted agent record
multica agent skills list <agent-id> --output json   # current skill bindings
multica agent env get <agent-id> --output json  # plaintext env (agent owner or ws owner/admin; agents denied)
```

An agent can also be **unbound**: `runtime_id` is `NULL` (served as `""` with
`runtime_bound: false`) after its runtime was deleted, which unbinds instead of
deleting its agents (MUL-5559). An unbound agent keeps everything it owns and
stays editable, but no trigger path will run it — they all refuse with
`agent_runtime_required` — until `agent update <id> --runtime-id <runtime-id>` binds
it again. Unbound is orthogonal to archived.

`agent get` returns the persisted agent including `runtime_id`, `model`,
`thinking_level`, `service_tier`, `custom_args`, `has_custom_env`,
`custom_env_key_count`, `skills`, and — for an agent created from a built-in role
template — `template_key`, `template_version` and `autonomy_level`. It never
returns plaintext `custom_env`.

## Core model

An agent is a workspace-scoped row (table `agent`). Creation is a single
`POST /api/agents` (`multica agent create`). At task claim time the daemon
re-reads the agent row and assembles the runtime payload — so the persisted
fields, not the create-time output, are what the agent runs on.

Two distinct text fields, often confused:

- `description` is a catalog summary. It is stored and shown in listings; the
  daemon does NOT inject it into the agent's runtime prompt. Treat it as
  human-facing metadata only. Capped at 255 Unicode code points.
- `instructions` is the runtime behavior contract. The daemon reads it at
  claim time and ships it to the provider as the agent's durable instructions.
  Persona, responsibilities, boundaries, output and escalation rules go here,
  not in `description`.

## CLI / API entry points

Minimum create call (`--name` and `--runtime-id` are both required):

```bash
multica agent create --name <name> --runtime-id <runtime-id> \
  --description "<short catalog summary>" \
  --instructions "<runtime behavior contract>" \
  --output json
```

`runAgentCreate` builds a JSON body and posts it to `/api/agents`. It only
adds a key when its flag was provided — `description`/`instructions` on a
non-empty value, the rest (`runtime-config`, `custom-args`, `model`,
`thinking-level`, `service-tier`, `visibility`, …) on the flag being `Changed`
— so omitted flags fall through to server defaults rather than sending empty
strings. `--max-concurrent-tasks` is validated as 1–50 before the request is
sent.

The HTTP body (`CreateAgentRequest`) accepts: `name`, `description`,
`instructions`, `conversation_starters`, `avatar_url`, `runtime_id`, `runtime_config`, `custom_env`,
`custom_args`, `model`, `thinking_level`, `service_tier`, `visibility`,
`max_concurrent_tasks`, `mcp_config`, `skill_ids`.

The body accepts more than the CLI exposes. `conversation_starters` has no
`agent create` / `agent update` flag — the CLI can only carry it across an
`agent copy`. Setting it for a new agent means either calling `/api/agents`
directly or telling the human to use the web UI; see below for where they
will find it.

## Copying an agent

`multica agent copy <source-agent-id>` forks an existing agent's portable
configuration into a brand-new agent, leaving the source untouched. It is the
CLI/headless equivalent of the web "Duplicate" action. No dedicated server API
is involved: `runAgentCopy` reads the source with `GET /api/agents/<id>`, then
POSTs a `CreateAgentRequest` — passing the source's skill ids in `skill_ids` so
the bindings attach in the SAME create transaction (unlike `agent create`, which
binds nothing). The mutation is therefore a single atomic create.

```bash
multica agent copy <source-agent-id> --name "My Agent (copy)"   # same runtime
multica agent copy <source-agent-id> --runtime-id <target> --model <model>  # cross-runtime fork
```

- Copied by default without a dedicated override flag: `conversation_starters`.
- Copied by default, each overridable with the matching flag: `name` (suffixed
  `" (copy)"`), `description`, `instructions`, avatar, `custom_args`,
  `max_concurrent_tasks`, invocation permission (`permission_mode` +
  allow-list), and assigned workspace skills.
- A copied `max_concurrent_tasks` is included only when the source value is
  within 1–50. Historical out-of-range values are omitted so the new agent
  receives the server default (`6`); an explicit out-of-range
  `--max-concurrent-tasks` override is rejected before any API request.
- Runtime-specific fields (`model`, `thinking_level`, `service_tier`) are copied
  ONLY when the target runtime is unchanged. `--runtime-id` selecting a
  different runtime drops them and REQUIRES `--model` (pass `--model ""` to
  accept the target runtime default), mirroring the web Duplicate clearing model
  on a runtime switch.
- Never copied: `custom_env`, `mcp_config`, `runtime_config` (secret /
  machine-local; redacted or masked on read anyway). Supply fresh values with
  the same secret-safe flags as `agent create` (`--custom-env*`, `--mcp-config*`,
  `--runtime-config`), or with `agent env set` after the copy exists.
- `--no-skills` skips copying the source's skill bindings.

## Creating from a built-in role template

`POST /api/agents/from-template` creates an ordinary agent seeded from one of the
platform's fourteen listed role templates (`GET /api/agents/templates` lists them,
with the full instructions text). There is no CLI command for this yet — it is
the web creation flow's third starting point.

The request carries only `template_key`, `runtime_id` and the few things a person
chooses: `name`, `model`, `thinking_level`, `service_tier`, `permission_mode` +
`invocation_targets`, `language`. Instructions, default skills, concurrency cap
and `autonomy_level` come from the server. `POST /api/agents` accepts NEITHER
`template_key` NOR `autonomy_level` — a client cannot claim a template's
provenance or mint an autonomy level through the ordinary create.

What the template create does that the ordinary one does not:

- copies the role's instructions onto the row (editable afterwards; a release
  never overwrites them) and records `template_key` + `template_version`;
- materializes the role's skills as workspace skills and binds them in the same
  transaction — unlike `agent create`, which binds nothing. A workspace skill
  that already carries that name is reused AS IS and never overwritten;
- seeds the default `agent.name` from the role's localized label for the
  request's `language` when `name` is omitted (English fallback): `language=zh`
  creates a 产品分析师, not "Product Analyst". Squad template staffing seats its
  agents under the same rule;
- sets `autonomy_level` from the template.

The listed `progress-reporter` role supports both daily and weekly progress
reporting. It supplies `multica-progress-report`, uses concurrency one, and has
Contributor authority so it can move its assigned report issue to `in_review`
after successfully posting the report. Its narrower instructions keep business
issues and repository files read-only. It is an ordinary role template, not a
second system agent or a new permission level.

The listed `diagnostician` role supplies only `multica-debugging`, uses
concurrency one, and has Contributor authority. It reproduces an observed
failure and tests hypotheses on an isolated branch in development/test
environments. Its report grades the cause as confirmed, suspected, or unresolved.
A confirmed cause includes evidence, a minimal reproduction for regression tests,
and a suggested fix direction for the Implementer. Incomplete investigations
state the evidence gaps and next checks. It removes temporary instrumentation
before handoff and does not submit the fix or change issue status. Those limits
are role/skill instructions, not an additional API sandbox. Updated defaults
apply to new template-created agents and missing role skills; existing workspace
copies remain unchanged.

The listed `reliability-engineer` role supplies only
`multica-reliability-engineering`, uses Contributor authority and concurrency
one. It turns SLI/SLO, error-budget, capacity, degradation and recovery signals
into evidence and follow-up tasks; it does not perform production operations.
The listed `agent-evaluator` role supplies only `multica-agent-evaluation`, uses
Observer authority and concurrency two. It evaluates versioned Agent/Skill/MCP
cases for correctness, safety, cost, latency and drift, and does not change
production configuration. Both roles mark missing baselines or signals as
unknown and retain the existing human approval boundary.

The listed `migration-reviewer` role (template version 2) and `architect` role
remain Observers. When migration evidence or old-client compatibility is missing,
they report the evidence gaps, proposed follow-up issue, owner and acceptance
criteria in the current issue comment. A squad lead with issue-creation authority
or a person creates that follow-up; the Observer does not. The ADR role skill is
version 6. These revisions apply to new agents and missing role skills only;
existing instructions and skill copies, including customized files and bindings,
remain unchanged and need an explicit workspace maintenance action to update.

`autonomy_level` (`observer` / `contributor` / `coordinator` / `operator`, or
empty for no declared policy) is enforced on the agent's OWN API requests, not
just displayed. An `observer` agent's issue status/assignee change and issue
create are rejected with 403 — on every route that writes those fields, including
`POST /api/issues/batch-update`, and including an explicitly null assignee;
automation and squad writes need `coordinator`;
recording a high-risk action needs `operator` plus an approved request
(`multica approval --help`). An empty level is unrestricted, which is what every
agent created before role templates carries. Only a person can change the value:
`PUT /api/agents/{id}` rejects `autonomy_level` from a task token, even though
that token carries the owner's user id.

Staffing through a role template requires at least `coordinator` when the caller
is an agent with a declared level. The selected role must be at or below that
caller's level: a Coordinator cannot create an Operator. Human callers and
agents without a declared level keep their existing access checks and behavior.

These are checks on the covered HTTP endpoints, not process or credential
isolation. The ordinary create route still produces an undeclared agent, and
webhook credentials and shell commands have separate access boundaries. An agent
must not use those paths, another agent, or another credential to bypass its
declared policy. The most direct such path is closed server-side: a task token
(`mat_`) or cloud PAT (`mcn_`) gets 403 from `POST /api/cli-token` and from
every `/api/tokens` route — create, list, renew and revoke — so an agent cannot
mint a JWT or personal access token in its owner's name and re-enter as a human
caller. A recorded approval does not sandbox the daemon host.

## Field contracts

For an HTTP instruction update that must preserve concurrent edits, first read
`GET /api/agents/{id}` and require `X-Multica-Instructions-Precondition: 1`.
Send `instructions`, `expected_instructions` (the exact original body), and
`expected_updated_at` (the complete returned timestamp) together to PUT. A stale
body or timestamp returns 409 without saving or publishing an update; reload and
review the new content before trying again. These optional fields have no CLI
flags and require a fully upgraded API deployment, since older servers ignore
unknown fields. Omitting both preserves ordinary update behavior.

| Field | Persisted as | Validated? | Consumed by |
|---|---|---|---|
| `name` | `agent.name` | required, 400 if empty | listings, runtime payload |
| `description` | `agent.description` | 400 if > 255 code points | catalog/listing only — NOT the runtime prompt |
| `instructions` | `agent.instructions` | none | daemon → provider at claim time |
| `conversation_starters` | `agent.conversation_starters` (JSON array) | at most 3 items; each requires a label (≤80 code points) and prompt (≤4000 code points) | human-facing Chat empty state only; selecting one prefills the composer and does not start a run |
| `avatar_url` | `agent.avatar_url` | none; an explicit non-empty value is preserved, while omitted/empty creates a random `emoji:<glyph>` avatar | catalog/listing UI only — NOT the runtime prompt |
| `runtime_id` | `agent.runtime_id` (nullable) | required at create (400) + must resolve to a runtime in this workspace | selects runtime/provider; `NULL` means unbound — see below |
| `model` | `agent.model` (nullable) | none beyond runtime support | daemon reads; empty = runtime default |
| `thinking_level` | `agent.thinking_level` (nullable) | provider-level enum/safe-token gate; unknown literal → 400. Pi accepts only `off|minimal|low|medium|high|xhigh|max`, then the daemon checks the selected model's RPC-discovered subset. ACP runtimes that advertise an effort selector in `session/new` (currently `reasonix` and `hermes`) take the safe-token path and are checked against the discovered catalog by the daemon; that catalog covers only the model the discovery session was on, so other models show no picker until per-model probing exists. `hermes` covers two binaries — jcode advertises and applies an effort, Hermes Agent advertises none and gets no picker — so the answer there comes from the runtime's discovered catalog, not the provider name. Because that catalog is only written once a client requests a model list, a `hermes` runtime that has never been discovered is refused with a distinct "has not reported a model catalog yet" 400 rather than being assumed capable; `reasonix`, whose provider name does determine the binary, is allowed in that state. A runtime with no reasoning control at all (e.g. `copilot`, which executes outside ACP) rejects EVERY non-empty value and says so — that 400 is a capability answer, not a bad token | daemon; empty = runtime default |
| `service_tier` | `agent.service_tier` (nullable) | Codex-only safe token; other providers reject; exact model/tier pair checked by daemon | daemon → Codex app-server; empty = local Codex config |
| `custom_args` | `agent.custom_args` (JSON array) | JSON shape checked CLI-side; server stores as-is | daemon (extra CLI switches); defaults to `[]` |
| `runtime_config` | `agent.runtime_config` (JSON) | JSON shape checked CLI-side; server stores as-is | runtime-specific config; defaults to `{}` |
| `custom_env` | `agent.custom_env` (JSON object) | — | daemon (process env); see Env & secrets |
| `mcp_config` | `agent.mcp_config` (raw JSON) | CLI checks it is a JSON object or `null`; server stores as-is. At create, literal `null` is dropped (no-op); at update, `null` clears the column | daemon → provider (provider-specific MCP handling); redacted on read |
| `visibility` | `agent.visibility` | — | access control; defaults to `private`; gates who can read/route a private agent (e.g. a private squad leader) — NOT the runtime prompt |
| `max_concurrent_tasks` | `agent.max_concurrent_tasks` | integer from 1 through 50; out-of-range values return 400 | scheduler task cap; defaults to `6` |

Defaults when omitted or explicitly `null`: `max_concurrent_tasks` → `6`.
Other defaults when omitted: `runtime_config` → `{}`, `custom_env` → `{}`,
`custom_args` → `[]`, `avatar_url` → a random `emoji:<glyph>`, `visibility` →
`private`
(all materialized server-side before the insert). `custom_args`/`runtime_config`
are typed `[]string`/`any` and marshaled as-is — the JSON-shape rejection
happens in the CLI, not the create handler.

The 1–50 concurrency range applies consistently to create and update. On
create, an omitted field defaults to 6 while an explicitly supplied 0 is
rejected; on update, omission preserves the current value. The CLI performs the
same range check before sending create or update requests.

`thinking_level` is validated only at the provider level: fixed-vocabulary
providers reject an unrecognized literal, while dynamic-vocabulary providers
such as Codex/OpenCode accept a syntactically safe token. Pi's provider-level
vocabulary is fixed (`off|minimal|low|medium|high|xhigh|max`), but its exact
supported subset is model-specific and discovered from the local Pi RPC model
catalog. A value unsupported for the chosen model is NOT rejected here — the
daemon checks its local model catalog at execution time, logs a warning, and
omits the incompatible override.

Set it from the CLI with `--thinking-level` on `agent create` and `agent
update`, mirroring `--model`: the flag is a thin pass-through to the top-level
`thinking_level` field, and on update an empty string (`--thinking-level ""`)
clears it back to the runtime default. The CLI deliberately does not enumerate
the valid levels — they are runtime/model-specific (Claude currently uses
`low|medium|high|xhigh|max`; Pi uses
`off|minimal|low|medium|high|xhigh|max`; Codex values are discovered from the
runtime's model catalog). It forwards the token, the server applies the
provider's fixed-enum or safe-token gate, and the daemon performs the exact
model/level check. A runtime whose provider has no thinking concept rejects any
non-empty value with a 400.

`service_tier` is the matching first-class Codex speed control. Set it with
`--service-tier <catalog-id>` on create/update; use `--service-tier ""` on
update to clear it. The runtime model catalog owns both availability and
display copy (currently `priority`, shown as Fast). The server accepts safe
future Codex catalog IDs, while the daemon verifies the exact model/tier pair
before execution and omits a stale incompatible override. Agents without an
explicit model fail closed because the effective config.toml model is unknown.

### conversation_starters

The product calls this feature **Conversation starters** (中文：对话开场建议).
Use that name when talking to a human — the wire field `conversation_starters`
is an implementation detail they never see. A human configures them on the
agent's **Instructions** tab; the deep link is
`/<workspace>/agents/<id>?view=instructions&focus=conversation_starters`.

They are up to three label + prompt pairs shown above the composer when
someone opens a new Chat with this agent. Selecting one **only fills the
composer** — it never starts a run, so they are suggestions, not actions.
Omitting the field on create defaults to `[]`; omitting it on update preserves
the stored value, and an explicit `[]` clears it. An agent with none
configured still shows three built-in generic defaults in that empty state, so
"the Chat shows suggestions" does not mean this agent has any of its own.

### model vs custom_args

`model` is a first-class persisted column the daemon reads directly.
`custom_args` are normally raw provider CLI args. The CLI help notes that some
providers (codex app-server, openclaw) reject `--model` inside `custom_args` —
but that is documented CLI guidance, not a server-enforced invariant; nothing
in the create handler inspects `custom_args` for a model flag. Provider
backends may consume protocol selectors before launch:

- Pi filters `--thinking` because the first-class `thinking_level` field owns
  that flag and must be its only source.
- ZeroClaw consumes `--agent <alias>` / `--agent-alias <alias>` (including
  `=value` forms) and sends the value as the ACP `session/new.agentAlias`
  parameter. `zeroclaw acp` has no such CLI flag. Set one of these custom args
  when ZeroClaw has multiple agents and no `[acp].default_agent`; omit it for a
  sole-agent config so ZeroClaw can auto-select that agent.

Never put credentials or other secrets in `custom_args`. Daemon command logs
redact argument values, but values that a backend does not consume still live
in the provider process's argv and may be visible to other local processes
through `ps` or `/proc`. Put provider credentials in `custom_env` instead,
using its stdin or 0600 file input where possible.

## Env & secrets

`custom_env` is secret material. The CLI offers three input channels; two keep
secrets out of shell history and the process list:

```bash
multica agent create --name <name> --runtime-id <runtime-id> --custom-env-stdin --output json
multica agent create --name <name> --runtime-id <runtime-id> --custom-env-file <0600-json> --output json
```

`--custom-env-stdin` reads the JSON object from stdin; `--custom-env-file`
reads it from a file (suggested mode 0600). The third channel,
`--custom-env <json>`, puts the value on the command line where shell history
and `ps` can see it — avoid it for real secrets.

Read-side facts (these are the wrong assumptions to avoid):

- Agent resources never expose plaintext `custom_env`. `agent
  list/get/create/update` and WS events return only `has_custom_env` (bool) and
  `custom_env_key_count` (int).
- Reading plaintext values requires the dedicated `GET /api/agents/{id}/env`
  endpoint (`multica agent env get`). It is gated to the **agent's own human
  owner** or a workspace **owner/admin**, and **agent actors are denied**
  regardless of the backing member's role — a running agent cannot read another
  agent's secrets, not even one its own human owns.
- Writing values after creation does NOT go through `agent update`. The generic
  update handler rejects any `custom_env` field with a 400 ("use PUT
  /api/agents/{id}/env"). Plaintext env writes are handled by
  `PUT /api/agents/{id}/env` (`multica agent env set`), which carries the same
  gate and writes an audit row.

### mcp_config

`mcp_config` is the agent's MCP server configuration (a JSON object such as
`{"mcpServers": {…}}`). It is also secret material — MCP entries routinely embed
API tokens — and offers the same three input channels as `custom_env`, on BOTH
`agent create` and `agent update`:

```bash
multica agent create --name <name> --runtime-id <runtime-id> --mcp-config-file <0600-json> --output json
multica agent update <agent-id> --mcp-config-stdin --output json
multica agent update <agent-id> --mcp-config 'null'   # clears the config
```

`--mcp-config-stdin` / `--mcp-config-file` keep the value out of shell history
and `ps`; the inline `--mcp-config <json>` does not. The CLI requires a JSON
**object** or the literal `null`; a top-level array or primitive is rejected
client-side, and empty stdin/file input errors rather than silently clearing.

Two ways `mcp_config` differs from `custom_env`:

- **It IS settable through `agent update`.** Unlike `custom_env`, `mcp_config`
  has no dedicated audited endpoint — the generic `PUT /api/agents/{id}` accepts
  it. Tri-state per the raw request body: field omitted → no change; `null` →
  clear; object → replace.
- **It is serialized on read, but redacted.** `agent get`/`list` return
  `mcp_config` only to callers allowed to view agent secrets; otherwise the
  field is `null` and `mcp_config_redacted` is `true`. Agent actors never see
  it, and a workspace may force redaction for everyone.

Provider support is not uniform: Qwen Code accepts a managed `mcp_config` through a daemon-owned 0600 temporary JSON file passed with `--mcp-config`; it is removed when the run exits. Leave the field unset (`null`) to inherit Qwen Code native settings.

#### Workspace MCP servers

A workspace keeps a LIBRARY of MCP servers (workspace Settings → MCP, or
`multica workspace mcp list|add|update|remove`). Adding one there gives it to
NO agent — same shape as a workspace skill. It reaches an agent only when
someone assigns it:

```bash
multica workspace mcp list --output table        # find the server id
multica agent mcp add <agent-id> <server-id>     # give it to one agent
multica agent mcp disable <agent-id> <server-id> # stop sending it, keep the assignment
multica agent mcp remove <agent-id> <server-id>  # take it away
```

At claim time the effective set is:

| Layer | Reaches the agent when |
| --- | --- |
| runtime-local servers | always (the daemon merges the runtime's own file) |
| workspace servers | assigned to THIS agent and left enabled |
| the agent's own `mcp_config` | always; it WINS on a name collision |

Two consequences worth knowing before writing an agent's config: assigning a
shared server does not require re-listing it in `mcp_config` (they merge), and
`mcp_config` is now only about servers private to that agent — a
managed-but-empty `{}` no longer means anything about the workspace layer,
because nothing is inherited in the first place.

The stored entry is **write-only** — reads return the server's name and
transport, never urls, commands, headers, or env, for any role.

## Skill binding

Creating an agent does NOT bind any workspace skill — binding is a separate
call after the agent exists. Two distinct verbs:

- `add` is additive — it merges the given ids with existing bindings
  (`POST /api/agents/{id}/skills/add`).
- `set` is replace-all — it overwrites the entire binding list with exactly
  the given ids (`PUT /api/agents/{id}/skills`); `--skill-ids ''` clears all.

```bash
multica agent skills add <agent-id> --skill-ids <skill-id> --output json
multica agent skills list <agent-id> --output json
```

At claim time the daemon assembles the agent's skills as workspace-bound skills
FIRST, then appends the platform built-in skills. `LoadAgentSkills` loads each
bound skill's content plus its supporting files; built-in skills are embedded
at compile time and loaded from `SKILL.md` + sibling files. Both reach the
provider as skill content — which is why capability belongs in a bound skill,
not pasted into `instructions`.

## Side effects needing approval

Read-only (safe): `agent get`, `agent skills list`, `agent env get`.

State-changing (require an explicit instruction — do not run speculatively):

- `multica agent create` — inserts a new agent row.
- `multica agent copy` — inserts a new agent row (a fork of an existing agent);
  the source is left untouched.
- `multica agent skills add` / `set` — mutate bindings (`set` is destructive:
  it drops bindings not in the new list).
- `multica agent env set` — overwrites the full `custom_env` map and writes an
  audit row.

## Common wrong assumptions

- "`description` is the prompt." It is not — only `instructions` reaches the
  runtime. A rich description with empty instructions yields a named shell with
  no operating contract.
- "Create binds the agent's skills." It does not; bind explicitly afterward.
- "`agent update` can rotate env." It cannot — it 400s on `custom_env`; use the
  env endpoint.
- "`mcp_config` behaves like `custom_env` on update." It does not — `mcp_config`
  IS settable via `agent update` (`--mcp-config`), with `--mcp-config null` to
  clear; only `custom_env` is gated behind the dedicated env endpoint.
- "`agent get` shows env values." It shows only `has_custom_env` and
  `custom_env_key_count`.
- "Every accepted body field has a CLI flag." `conversation_starters` does not
  — `agent create`/`agent update` cannot set it, and `agent copy` only carries
  an existing value forward.
- "An invalid `thinking_level`/`model` combo is caught at create." Only an
  unknown provider-level literal is — model-specific gaps fail at run time.
- "`set` and `add` are interchangeable for skills." `set` replaces all
  bindings; using it when you meant `add` silently removes capabilities.
- "A template agent is a special kind of agent." It is not — same table, same
  Access, same task lifecycle. `template_key` is provenance, not status.
- "Upgrading Multica updates template agents' instructions." It does not. The
  text was copied at creation and belongs to the workspace.

## References

`references/creating-agents-source-map.md` maps every contract above to its
`file:line` on the current tree, the runtime effect, and a safe read-only
verification command.
