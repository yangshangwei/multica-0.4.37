# Agent Discovery

Directory identification, role grouping, squad membership, and template/member
separation for the agents page. Saved agents are the default directory; the
role-template catalog lives only in the New agent → Use template flow. Do not
reintroduce a template inventory into the directory.

## Squad complete-membership contract (cross-layer)

### Scope / Trigger
The directory narrows by squad and labels squad membership, which needs a
squad's full agent roster — not the truncated three-person preview.

### Signatures & Contracts
- `GET /api/squads` — `SquadResponse` gains optional `agent_member_ids: string[]`.
  Current servers emit the COMPLETE agent-only member IDs (leader included),
  collected BEFORE the 3-person `member_preview` truncation. Empty squads encode
  `[]`, never `null`; `member_preview` stays length 3.
- Core schema: `agent_member_ids: z.array(z.string()).optional().catch(undefined)`
  — absence / null / malformed all normalize to `undefined` (roster unknown),
  never an asserted empty list.
- Older servers omit the field. When a squad is SELECTED and its full array is
  absent, load `GET /api/squads/:id/members` via `squadMembersOptions`, keyed
  `["workspaces", wsId, "squads", squadId, "members"]` (wsId-scoped, under the
  squad subtree so existing realtime invalidations cover it). Parse the payload
  with `parseWithFallback`; malformed throws, never silently empty.

### Validation & behavior matrix
- `agent_member_ids` present → use it; no per-squad fetch.
- absent + squad selected → fetch members; GATE results while pending; show a
  retry alert on error — never render a false "no matching agents" empty state.
- absent + no squad selected → only the explicit `leader_id` is a certain
  membership; do not guess the rest.

### Tests
- Go `TestListSquadsCompleteAgentMembership`: count=6 / preview=3 /
  agent_member_ids=5, shared specialist across squads, empty→`[]`, archived +
  foreign-workspace excluded.
- Core `squad-members-query.test.ts` / `squad-members.test.ts`: schema
  normalization + fallback query key.
- `e2e/agents-discovery.spec.ts`: legacy 503 → retry; no eager per-squad fetch.

## Role provenance — don't infer capability
Role KIND (coordinator | specialist | other) derives from `agent.template_key`
matched against `useRoleTemplates()` / `useSquadTemplates().leader` ONLY. Never
infer role or capability from the editable name, avatar emoji, or autonomy level
— those are user-owned and prove nothing. Unknown/custom agents stay in
Other/All with no invented capability. The row label states template provenance
("角色模板：X"), not enforced current behavior.

## Template picker instances — navigate, never mutate
Match active instances by `template_key` + `!archived_at`; show all renamed
instances, omit archived. Cards are non-interactive containers; existing members
and "create another" are real `<AppLink>`s. Opening a member NEVER creates an
agent or adds a squad member. Keep `squad` query params on creation links.

## Preferences & scope
Fresh `AGENT_DEFAULT_HIDDEN_COLUMNS` is concise (owner/access/runtime/runs/model/
created hidden). The merge spreads persisted prefs AFTER defaults so an existing
`hiddenColumns` (including an intentional `[]`) wins exactly — never overwrite it.
`setScope` sets only scope; `toggleFilter` never mutates scope — scope (Mine/All)
and role/squad filters compose. Label the activity column with its window
("Runs (30 days)", "No activity (30d)").

## Wrong vs Correct
- Wrong: `agent_member_ids ?? []` at the boundary — turns "roster unknown" into
  "empty squad" and hides members on older servers. Correct: keep `undefined`
  and fall back to the members endpoint.
- Wrong: group or label a renamed agent by its current name. Correct: resolve by
  `template_key` — a renamed "支付实现" is still the implementer role.
