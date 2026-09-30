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

## Template provenance and directory classification

`resolveAgentRole` resolves immutable template provenance from `template_key`
and the built-in catalogs. Never infer capability or authority from the editable
name, avatar or category. Template provenance remains unchanged and searchable even when someone assigns
a different directory category; the directory does not repeat it as a row tag.

`resolveAgentDirectoryCategory` is the single directory classification rule:
1. A nonblank manually saved category takes precedence.
2. Without one, fall back to the template-derived role kind.
3. Agents without a recognized template use General-purpose agents.

Preset names in English and Chinese normalize to `preset:other`,
`preset:specialist`, or `preset:coordinator`. Other names become `custom:<name>`.
These keys identify view filters/groups; the API still stores the original name.
The resolved category drives grouping, top chips, dropdown filtering/counts and
search. No display category changes agent permissions or instructions.

An unavailable template catalog leaves only template-dependent fallback rows
unknown. Explicit categories and template-free agents remain usable. Unknown
rows get a pending/unavailable group; preset counts are lower bounds (`n+`)
until complete, and filtered empty results are not asserted when unknown rows
could match. Custom-category results do not depend on template catalogs. Retry
restores fallback classification without changing saved agents or preferences.

Mika-specific helper text still requires an active agent with `system_key ===
"mika"` and interpolates its saved name; never identify it by editable name or
attribute its capabilities to every general-purpose agent.

## Template picker instances — navigate, never mutate
Match active instances by `template_key` + `!archived_at`; show all renamed
instances, omit archived. Cards are non-interactive containers; existing members
and "create another" are real `<AppLink>`s. Opening a member NEVER creates an
agent or adds a squad member. Keep `squad` query params on creation links.

The gallery's custom scroll view-state takes precedence over ordinary platform
scroll capture because configuration shares its pathname. Update that view-state
on gallery scrolling as well as card navigation, including scrolling to zero;
otherwise sidebar/tab navigation can restore an older card-click position. Do
not write clamped offsets from loading/error layouts before content restoration.

## Custom categories

- `Agent.category` is a user-editable single category name, independent of role
  provenance and autonomy. The backend persists it on the agent row. Empty means
  automatic fallback classification; missing or malformed legacy response fields
  normalize safely.
- Create, template-create and update accept `category`. Writes trim surrounding
  whitespace, reject interior control characters and cap names at 50 Unicode
  code points. An omitted update preserves the category; `""` clears it.
- Suggest category names only from the caller's visible workspace agent list.
  Categories do not grant access or alter instructions. Do not reuse or rewrite
  `template_key` to classify a user-authored agent.
- Use the shared Base UI Combobox with a visible arrow, rather than a native
  datalist whose popup is browser-dependent. Opening the arrow shows existing
  names and Default category even with a selected value. Typing filters suggestions
  and exposes a "Use new category" option for valid unmatched names. Empty and
  failed suggestions explain free entry. The field's `saveMode` distinguishes
  creation drafts from automatically saved settings in its helper text.
- The first three suggestions are the localized names for General-purpose
  agents, Specialists and Coordinators, followed by deduplicated visible saved
  category names. Defaults remain available without saved agents or while their
  query fails. A preset saves its displayed name as an ordinary category string;
  it sets directory classification without modifying role-template provenance,
  authority or instructions.
- Shared `AgentDraft`, stored/manual/builder drafts and duplicates carry category.
  The settings field saves category independently of name/description using the
  shared autosave hook, including failure feedback, validation and IME deferral.
  Clean fields follow Query refreshes; an in-flight local draft survives until
  its matching save succeeds. Never resend a cached category in a profile edit.
- `filters.categories` is the only classification filter, using resolved keys;
  it composes with ownership scope, search, access and squads. `groupBy` supports
  `none` and `category`, defaulting to category. Groups order the three presets
  first, then custom names, then unresolved fallback rows.

## Saved display preferences

The agents view store uses persisted version 1. Version-0 `role`/`category`
grouping migrates to `category`; explicit `none` remains. Named raw category
filters convert to resolved keys and take precedence over legacy role filters.
If no category filter was selected, legacy role choices become preset keys.
A blank-only old category filter clears because blank now means automatic
fallback. Canonical version-1 keys must not be converted again. Preserve scope,
sort, hidden columns (including explicit `[]`), squads and all unrelated filters.

Fresh hidden columns remain concise (owner/access/runtime/runs/model/created).
`setScope` sets only scope; toggling a category never changes Mine/All. Label the
activity column with its window ("Runs (30 days)", "No activity (30d)").

## Wrong vs Correct
- Wrong: `agent_member_ids ?? []` at the boundary — turns "roster unknown" into
  "empty squad" and hides members on older servers. Correct: keep `undefined`
  and fall back to the members endpoint.
- Wrong: keep an explicitly categorized OpenCode under the template-derived
  General-purpose group. Correct: use the effective category for all directory
  navigation, retaining template provenance as separate searchable metadata.
