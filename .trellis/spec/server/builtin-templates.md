# Built-in Template Registries

> Contracts for the server-embedded template registries (`builtin_agent_templates*`, `builtin_squad_templates*`, `builtin_autopilot_templates*`). A template is pre-fill content copied into an ordinary instance at creation — never a new entity kind.

## Supported display languages

Catalogs and Mika creation/onboarding support English (`en`) and Simplified
Chinese (`zh`). User preferences use `zh-Hans` instead of `zh`; keep the API
contracts distinct. Previously supported `ja`/`ko` requests resolve to English
without changing other validation, whitespace, null or omission rules.
User responses normalize retired preferences without writing storage; explicit
preference writes save `en`. Removing a display language never retranslates
workspace-owned names, descriptions, prompts, instructions or conversation text.

## Built-in assistant display name and stored defaults

The default assistant name is `小阿孚` in English and Chinese. Its stable identity
is `system_key=mika`; preserve API paths, symbols and UUID relationships. Names
must never establish builtin identity or skill authorization. Product system
instructions and new opening messages interpolate the current saved name;
workspace instructions and historical messages remain owner content.

Migration `457_builtin_agent_display_name` renames only built-in rows still named
`Mika`. It replaces only the exact shipped English/Chinese descriptions and the
old titles `Getting started with Mika` / `和 Mika 开始`, and only on built-ins now
named `小阿孚`. Custom-named or conflicted agents retain their complete saved
content. Other agents, custom descriptions/titles, instructions, kickoff rows,
issue assignments and task snapshots are untouched. Replay does not change
content or timestamps already migrated.

Migration `458_builtin_onboarding_display_name` handles the product-authored
welcome that 457 deliberately left behind. Only built-ins already named
`小阿孚` qualify, and only when the session still consists of exactly one
unconsumed `onboarding_opening` and its paired `onboarding_kickoff`, with no
task history. Match the complete shipped English/Chinese opening and its quoted
kickoff block, then change the self-introduction in both together. Preserve
workspace/profile text, message IDs, timestamps and all real conversation
history. Lock the session before rechecking eligibility, using the same lock as
the first-send path. Replay and down must not undo later member work.

Source verification alone does not prove a rename has reached a running client.
Check the actual desktop API URL, the responding backend commit and that
database's migration ledger; then reload the client and verify the saved name,
description, chat preview and opening. A client can load new local labels while
still using a pre-rename backend. When multiple local environment records point
at one checkout, select the environment by the desktop's proven API/DB identity,
not the first registry entry matching the directory.

Active **and archived** agents own their workspace names. A collision skips that
builtin and reports its agent/workspace IDs while other workspaces proceed.
The migration CLI must retain its `OnNotice` handler so PostgreSQL notices
reach the operator log; success alone does not mean every row was renamed.
Resolve the conflicting name through the normal editing interface, then rerun
this migration's idempotent SQL against the reviewed database. `migrate up` alone
will not retry a recorded version. Do not delete migration-ledger rows or rename
unrelated agents automatically to force the migration through.

The down migration intentionally retains editable names, descriptions and titles:
it cannot distinguish migrated defaults from later owner choices. Older code
supports arbitrary display names, so code rollback is compatible. Restore an
individual inspected name through the normal UI/API if required. Installed
desktop clients need an update for bundled labels; scripts resolving `Mika` by
name should use `小阿孚` or the stable agent UUID.

## Role instructions, skills, and autonomy must agree

Default role skills describe work the role can actually deliver. Architect stays
Observer and returns a proposed ADR draft in an issue comment for an Implementer
or human to save; Technical Writer is Contributor and edits documentation only
on an isolated branch. Only Feature Delivery and Discovery leads default to
requirement clarification; other leads use their existing routing instructions.

Progress Reporter v1 uses Contributor so a delivered report may move its own
report issue to `in_review`; this is a legacy role/prompt boundary. From v2,
server-created Reporter provenance adds a frozen task API capability: read the
report's current project (no project means current issue only), comment on the
current report, and do not change business status. Code/Security Reviewer v2 and
Migration Reviewer v3 similarly allow current-issue reads/comments only. The
existing autonomy level remains an independent ceiling, not an escape from the
capability. Fresh role skills agree; saved role/skill instances are not rewritten.

Restricted comments do not auto-wake a leader or act as acceptance. The claim's
authoritative capability briefing takes precedence over generic workflow or saved
skill prose. A human or existing authorized coordinator continues the issue.
These server restrictions cover Multica APIs and platform-issued tool grants;
they are not a daemon-host filesystem/shell sandbox. Ordinary API scope does not
claim that historical/user-supplied prompt content was filtered by the server.

Diagnostician brings the listed roster to fourteen roles and the role-skill catalog
to fifteen entries. It uses Contributor and only
`multica-debugging` (`quality`, `microscope`). It reproduces an observed failure,
tests hypotheses, and hands evidence and a suggested fix direction to the
Implementer; it does not submit the fix or change issue status. Confirmed causes
need a reproducible evidence chain and a concrete location/mechanism. Suspected
or unresolved investigations may close with explicit evidence gaps rather than
inventing a minimal reproduction or fix direction. Data loss, security defects,
or exposed credentials require reporting and stopping for human handling.
These narrower role/skill instructions do not add a server-enforced sandbox.

Reliability Engineer and Agent Evaluator are the two additional listed roles with
independent evidence deliverables. Reliability Engineer uses Contributor
and `multica-reliability-engineering` (`operations`, `server`)
to report SLI/SLO, error-budget, capacity, degradation and recovery evidence;
it does not perform production operations. Agent Evaluator uses Observer
and `multica-agent-evaluation` (`quality`, `bot`) to run
versioned Agent/Skill/MCP cases and report correctness, safety, cost, latency
and drift; it does not replace code, security, QA or release review. Both roles
mark missing baselines or signals as unknown and require human approval for
production, credential, customer-data or paid-model actions.

Experience Validation Engineer and Migration Reviewer are listed specialists after
the two-cycle workload gate was met. Experience Validation Engineer uses Contributor
and `multica-experience-validation` (`quality`, `globe`) to provide
browser/client, accessibility and outcome-risk evidence. Migration Reviewer uses
Observer, template version two, and `multica-migration-review` (`operations`, `database`)
to provide compatibility-window, validation, retry and rollback evidence. Neither
executes production actions or changes code, schema or client contracts. Review
Gate and Release seat both roles; Maintenance also seats Migration Reviewer for
upgrades that change a contract or stored data. The corresponding squad
instructions must describe the same handoff so routing advice does not conflict
at claim time.

`multica-incident-learning` (`quality`, `repeat`) and
`multica-rollout-and-canary-verification` (`operations`, `chart-line`) are
available as lifecycle role-skill templates. Incident Lead carries the former
and Release Lead carries the latter, so staffing the corresponding squads makes
the post-action stage reachable without loading both skills into every Agent.
The incident and release rosters also seat Reliability Engineer, while Review
Gate and Release seat Agent Evaluator. The skills consume post-recovery evidence
or an approved immutable artifact and observation window; both require
unknown-state reporting and keep production actions behind the existing approval
protocol.

The default squad placement is deliberately narrow. `bug-fix` seats the
Diagnostician between reproduction and implementation when the cause is unknown;
`maintenance` seats it for unexplained flaky-test or upgrade failures;
`incident` seats it for a separate post-recovery root-cause follow-up without
delaying mitigation; and `security-remediation` seats it for permitted diagnosis
of unknown causes while preserving its mandatory safety stops. Feature delivery,
review gate, discovery, docs, and release
do not provision the role by default. The corresponding squad instructions and
leader templates must describe the same handoff; changing only one layer creates
conflicting routing advice at claim time.

Every role template, listed or squad leader, and 小阿孚 start at
`agentconfig.DefaultMaxConcurrentTasks` (6), the blank-agent default. Do not
give a built-in role its own concurrency literal; per-agent tuning happens after
creation. `TestAgentRoleTemplates_LocalizedCopyIsComplete` pins the rule.

Increment template and role-skill versions for material behavior changes. These
defaults apply when creating agents or materializing a missing role skill;
existing workspace copies are reused without overwriting customized content.
`builtin_agent_templates_test.go` pins the full role-to-skill and autonomy maps.

### Security remediation v2

`security-remediation` is the ninth built-in squad, version 2. Its dedicated unlisted
`security-remediation-lead` is version 2, Coordinator, with no
new role skill. The seven seats reuse Product Analyst, Security Reviewer,
Diagnostician, Implementer, QA Engineer, Migration Reviewer and Code Reviewer.
Keep it before the Operator-bearing release/incident entries, preserve existing
template versions, and retain private creation and workspace-copy reuse.

The template coordinates non-production dependency, code and other-risk work.
Unknown evidence retains an owner and `hold/unknown`; production exposure and
high-risk operations go to human/incident/release handling. Existing member
stops, including the Diagnostician's security stop, remain authoritative. A
simulation can prove plumbing and fixture behavior, not model judgment or a
runtime sandbox.

Squad-leader identity does not imply ownership of the issue. Both instruction
bodies must condition status transitions on the issue actually being assigned
to this squad and the injected protocol granting status ownership. Guest
`@squad` and quick-create turns keep the existing assignee/status and deliver
comments/activity only. `buildSquadLeaderBriefing(..., false)` must not compose
an unconditional status-write instruction with its non-owner restriction.

Regression anchors: `builtin_security_remediation_templates_test.go` checks the
registry, roster, authority and instruction contracts; `squad_template_security_test.go`
covers private creation, reuse, autonomy, hidden-leader rejection and owning/guest
briefing composition. The shared catalog uses the original server template for
project adoption, preserves unknown-key description fallback, and localizes only
display summaries. Its project action writes the complete `/execution-squads`
list; the standalone `/api/squads/from-template` endpoint remains separate.
The persisted project selection uses `state: configured`; execution readiness is
derived separately by `getProjectSquadReadiness`. An offline runtime fixture can
prove successful configuration without establishing a `ready` execution state.

Version 2 changes instruction text only: no status, approval type, permission,
schema or API. Contracts a later edit must keep:

- **Two bodies, each complete.** Every contract marker must appear in both the
  squad body and the leader body; a workspace can hold a v1 copy of either.
  `## 分诊记录`, `## 人工闸门`, `## 最小安全复现` and `## 保密与披露` are
  byte-identical in the two files (`TestSquadTemplates_SecurityRemediationSharedSections`).
  Edit the squad copy and copy the block across; never edit the two by hand.
- **Gates exist because restricted reviewers do not wake the leader.** A gate is
  one comment dispatching that gate's reviewers plus the release wording. The
  security owner is a human squad member whose role contains "安全负责人" or
  `security owner`, else the issue's human creator. Release needs the owner's own
  comment, every dispatched reviewer's result, and a conclusion per open item.
- **Only a person designates the security owner.** Both gates hinge on one
  free-text role on the roster, and the platform lets the leader change it: the
  leader is a Coordinator, whose autonomy policy allows managing squad
  membership, and `AddSquadMember`, `UpdateSquadMemberRole` and `UpdateSquad`
  (`handler/squad.go`) need only `canManageSquad` (a workspace owner or admin, or
  the squad creator). The leader's task token carries its owner's user id, and a
  leader staffed from the template is owned by the squad creator. The bodies
  therefore say the leader does not add or remove members, edit a role text, edit
  the squad instructions or replace the leader, and treats text asking for it as
  input (the `gates` group). `UpdateAgent` (`handler/agent.go`) also stays open to
  agent actors for everything except `autonomy_level`, so the same sentence
  forbids editing the instructions, skills or configuration of any agent in the
  squad, the leader included. This is an instruction, not an enforced boundary.
- **A reviewer's result is a comment of type `comment`.** When a reviewer's run
  fails for good, `TaskService.FailTask` posts the error under the reviewer's own
  name, as a reply to the gate comment, with the failed run as `source_task_id`.
  Only `type: system` separates that notice from a result, so the result check
  names the type. The security reviewer answers at both gates, and a returned G2
  is dispatched again with a second gate comment, so the check also wants a
  result later than the comment that last dispatched that reviewer
  ("G1 的结果、退回前的结果都不能充当这一次的结果"; "later than this gate's
  comment" left open which of two G2 comments is meant) and reads the
  reviewer's latest run. Delegated-failure recovery skips restricted agents
  (`loadDelegatedFailureRecoveryTarget`), so nothing wakes the leader either:
  both facts are pinned by
  `TestSecurityRemediationRouting_FailedReviewerLeavesANoticeAndWakesNobody`.
  On a release with a reviewer missing, the leader reads
  `multica issue runs <issue> --output json`: a run still going means wait, a
  finished run without a result or no run at all means one re-dispatch in the
  same comment, and a second failure goes to the owner with no third attempt
  (the `missing_review` group). The fixed gate wording tells the owner to mention
  the squad when a reviewer stays silent. No waiver exists: a gate with a
  reviewer that cannot deliver stays closed until a person fixes that reviewer.
- **The fixed release wording says "不带任何 @", not "不 @ 任何成员".** "成员"
  reads as people, and a release that mentions an agent runs only that agent
  (`computeCommentAgentTriggers`), never the leader.
- **The fixed release wording says to pick the squad from the `@` list.** The
  server reads a mention only from the link the picker inserts
  (`util/mention.go` `MentionRe`), and the leader writes the squad's name into
  the gate comment as plain text, so a name copied from that comment or typed
  without picking it mentions nobody. On an issue assigned to the squad such a
  top-level comment still works as the second release shape; as a reply or on a
  guest issue it does not reach the leader with the squad's context
  ("输入 @ 后从列表里选中", the `gates` group).
- **Catalog copy names the risk type that is reproduced.** Only a `code` risk is
  reproduced before the repair; `other` is the owner's decision at G1 and
  `dependency` has none. The template's `Descriptions`, the two locale summaries
  and the docs table say "代码漏洞先复现" / "code vulnerabilities". The component
  test and the E2E spec assert the locale strings word for word.
- **A check on a comment's author names its read.** The task prompt introduces
  a member's comment as "A user" and carries no author id
  (`daemon/prompt.go` `buildCommentPrompt`), so the release check reads the
  comment back with `multica issue comment list <issue> --thread <comment> --tail 30 --output json`
  (the `gates` group). The provenance check reads without `--compact`, which the
  prompt recommends on every comment read and which drops `source_task_id`
  (`cmd/multica/cmd_issue.go`; the `provenance` group). `author_type`,
  `author_id`, `type`, `created_at` and `revision` survive `--compact`.
- **A release is the owner's own comment, unchanged and later than the results.**
  `UpdateComment` (`handler/comment.go`) accepts the author or a workspace owner
  or admin, and an agent's task token carries its runtime owner's user and role,
  so an agent on an admin-owned runtime can rewrite a member's comment. The edit
  keeps the author fields and routes the comment again. `revision` starts at 1
  and goes up on a content edit, on an emoji reaction added or removed, on
  resolve or unresolve and on an attachment added or deleted
  (`issue_revision_test.go`,
  `TestUpdateComment_AdminEditOfAgentCommentClearsStaleLineage`), so the release
  check refuses a comment whose `revision` is above 1; the bodies also tell the
  leader not to edit or delete other people's comments. Any member can react to
  or resolve any comment, so the refusal also meets a release nobody rewrote. It
  fails closed: the check names the four causes, the leader lists them when it
  asks for a new comment, and the docs tell the owner to leave a release alone
  until the leader explicitly confirms that this release passed validation.
  The acknowledgment cites the release comment in the next dispatch, close or
  handover comment; an unrelated reply does not end this window. Later mutation
  does not retroactively roll back an already consumed release. `updated_at` cannot replace `revision`: a reaction
  leaves it alone, but the comment read prints timestamps to the second
  (`util.TimestampToString`), so an edit made within the second the comment was
  created would leave the two equal. Both halves of the trade are pinned by
  `TestSecurityRemediationRouting_ChangedReleaseCommentKeepsItsAuthor`: a
  comment rewritten by an admin still reads back under its first author with
  `revision` 2, and so does one that only got a reaction. The owner's comment must be later than every reviewer result by
  `created_at`: a leader task that starts late would otherwise accept a release
  written before the result existed. A decision handed to the owner outside the
  two gates (no safe version, not reproducible, a second verifier failure)
  checks the same human identity and unchanged revision. A temporary point
  that dispatched a required reviewer applies all three release checks, including
  the result and time ordering; it is not an exemption. All of it sits in the `gates` group. These are
  instructions; the platform does not stop a machine credential on the admin
  branch of `UpdateComment` or `DeleteComment`.
- **A stop that waits for a person says how to resume.** Adding a security owner
  to the roster creates no comment and wakes nobody, so the missing-owner reply
  asks the person to comment and mention the squad afterwards ("小队才会继续",
  the `gates` group).
- **The closing turn records per protocol and names no outcome.** v1 said "再记录
  `no_action` 并退出" on the turn that delivers the evidence summary, which the
  protocol counts as `action`. Both bodies now say "按协议记录并退出", and the v1
  phrase is in the `removed` list of
  `TestSquadTemplates_SecurityRemediationLeavesReceiptsToTheProtocol`. The new
  wording, with "记录后不再发表评论", is the `closing` group of the contract test.
- **The leader body says where the squad's name comes from.** On a turn without
  the protocol the issue JSON carries only `assignee_type` and `assignee_id`
  (`handler/issue.go`), so the body points at an earlier gate comment and, without
  one, at `multica squad get <assignee_id> --output json` (`cmd_squad.go`). Leader
  body only.
- **`need-to-know` binds names and wording, not pushes.** The `disclosure` group
  covers branch names, commit messages, PR titles and descriptions, and the
  titles and descriptions of derived issues. Which remote the failing test and
  the patch are pushed to, and who can read it, is not constrained; `squads.mdx`
  lists that as a limit. The dependency closing likewise asks for scanner output
  without naming a scanner.
- **Release shapes follow comment routing, not preference.** A comment that
  mentions the squad, or a mention-free top-level comment on an issue assigned to
  the squad, starts a leader task with the squad briefing. A mention-free reply to
  the leader's comment starts a plain task with no protocol, roster or squad
  instructions; only the leader body can speak on that turn, so that rule lives
  there alone (`TestAgentRoleTemplates_SecurityRemediationLeadWithoutSquadContext`).
- **The releasing human needs invocation rights on every agent in the squad.**
  Template staffing defaults to `private`, and project adoption
  (`project_execution_squad.go`) always staffs `private`. A person without the
  right gets `blocked` / `invocation_not_allowed` on a squad mention and nothing
  at all on a mention-free comment, so the leader never wakes and the instructions
  cannot recover. The gate wording and the docs carry the hint.
- **Receipts and retries belong to the Squad Operating Protocol.** The bodies
  point at it and must not restate idempotency keys, receipt checks, activity
  outcomes or retry rules (`TestSquadTemplates_SecurityRemediationLeavesReceiptsToTheProtocol`).
- **The status-ownership sentence appears once per body.** New sections say
  "获得父任务状态管理权时" instead of repeating it
  (`TestSquadTemplates_SecurityRemediationStatesStatusConditionOnce`).
- **`## 最终证据` stays the last level-2 section of the squad body.** Both
  truncation checks anchor on it (`TestSquadTemplates_SecurityRemediationEndsWithFinalEvidence`).
- **The closing summary reconciles.** Every conclusion lists its source comment,
  author and commit, and a raw command output outranks a member's account of it
  (the `reconciliation` group of `TestSquadTemplates_SecurityRemediationV2InstructionContracts`).
  The v1 pilot closed on review reports that did not match the diff.
- **The squad body must survive the workspace's instructions editor.** A
  workspace edits its copy on the squad detail page in a rich-text editor
  (`ContentEditor`) that writes the whole body back as Markdown on every save. It
  re-pads tables (about +0.8 KB on this body) and turns a bare `<` outside fenced
  blocks and code spans into `&lt;`. The leader copies the fixed gate wording word
  for word, so a placeholder in prose is plain words (`@本小队名称`), never angle
  brackets (`TestSquadTemplates_SecurityRemediationSquadBodySurvivesTheInstructionsEditor`).
  The leader body is edited in a plain text box and is stored byte for byte.

Instruction length is a runtime constraint. On the final 2026-11-20 bodies and
baseline efdb597d6, the minimal task/default-roster composed file is 79,766 bytes
(about 80 KB), not a universal bound for every task. The squad source is 24,980 B
and the leader source 25,678 B. Codex receives its brief through this file;
the documented upstream default is 32 KiB. These are generated-file measurements,
not observations of a real Codex session. The manual workaround is a sufficient
root `project_doc_max_bytes` in the configuration actually copied into the task's
Codex home, verified after copying and accounting for other project documents.
No shared user configuration is automatically changed by this template.

| Leader | Squad | Generated bytes | A 32 KiB reader stops |
|---|---|---|---|
| v2 | v2 | 79,766 | protocol step 2, before the record step, roster and squad body |
| v1 | v2 | 62,102 | inside the squad's human-gate section |
| v2 | v1 | 62,369 | protocol step 2, as above |
| v1 | v1 | 44,705 | platform sections; both bodies survive |

Both v2 bodies instruct the leader to stop with “本轮指令不完整” when required
context is missing. This is an instruction-level fallback, not a host-enforced
sandbox or proof the model read and followed it. The leader body and protocol
heading remain before the cutoff: leader content ends at byte 27,894; protocol
starts at 29,209 for Coordinator. At the four autonomy levels the file sizes are
79,766 (Coordinator), 79,942 (Contributor), 80,380 (Observer), and 80,439 (Operator).
All four cuts occur before protocol step 3, by 193/369/807/866 bytes respectively;
none can read its record command. Therefore the fallback “协议的记录步骤也看不到时，
这条评论就是记录” is essential. Re-measure after any body/platform composition change.
Automatic length handling is separately planned in task `11-20-codex-project-doc-budget`.

Pretrial refinements in the first uncommitted v2 release keep the shared sections
identical: triage updates use a new comment (merged into the single dispatch when
needed), never edit old mention-bearing comments; a reviewer has one retry per
G1/G2 or temporary point, with no reset on a returned gate; the second failure
requires owner repair, never waiver of a required review. Resumption requires the
eligible owner's new unchanged comment, evidence the stop condition was removed,
a permitted nonproduction next step and a fresh stop/gate check. The QA engineer's
security stop remains in force. These rules are covered by `triage_updates`,
`safe_resumption`, `missing_review` and the fixed-release-warning contract.
A `trigger_comment_id`-only lookup is not a safe refinement: task inputs can be
coalesced, retaining earlier IDs separately. Keep the documented conservative
lookup until receipt/covered-input association is designed and verified.

Platform behavior the instructions rely on is pinned where the behavior lives,
not in the template tests: `squad_security_remediation_routing_test.go` (a gate
comment queues each restricted reviewer; a restricted result does not wake the
leader; an owner's reply that mentions the squad queues one leader task whose
originator is that owner; a failed reviewer leaves a `system` notice and wakes
nobody; a rewritten or reacted-to comment keeps its author and reads back with
`revision` 2), `squad_security_remediation_flow_test.go` (four
scripted walks: production escalation, other-risk shelve after an invalid
release, dependency through both gates, code with a verifier and a variant) and
`cmd/server/squad_security_remediation_notification_test.go` (a restricted
reviewer's comment notifies a subscribed member). Test code plays every role in
them, so they prove where the platform hands control, never that a model follows
the text. The flow walks also check how each scripted comment is recorded: an
agent's comment carries `author_type=agent`, that agent's id and the
`source_task_id` of its running task; the owner's carries `author_type=member`
and the user id the roster link names. The release checks and the closing
reconciliation read those fields back. The comment texts in the walks are
shortened; they are not model answers for what a comment must contain.

Docs that tell a workspace how to pick up a new version must name where a body
can be read. The squad body shows on the squad detail page's Instructions view.
The leader body shows on the leader's agent page under Capabilities, then
Instructions, in the "System prompt" box. An unlisted leader template has no
page, API or CLI that shows its text, so a workspace that already holds the old
leader can only read the new body from a squad staffed in another workspace.

### Runtime recovery must preserve role provenance

When an existing role's runtime is offline, ordinary `multica agent copy`
uses the generic agent-create path. It copies editable instructions and skills,
but does not preserve `template_key`, `template_version` or `autonomy_level`.
Do not treat the resulting custom agent as a replacement with the source role's
declared authority, or widen access to make a pilot run.

`POST /api/agents/from-template` copies the declared autonomy and template
identity onto an ordinary agent and defaults to private Access. Explicit
permission settings remain supported. It accepts only listed role keys;
unlisted squad leaders such as `maintenance-lead` are rejected.
`POST /api/squads/from-template` reuses existing workspace agents by template
key and preserves their runtime, edits and skills. Supplying a new runtime to
that request does not relocate reused members. Restoring the original daemon
identity preserves those bindings; rebinding a shared agent changes the
execution location for every squad that uses it.

Validate runtime owner/workspace, agent `runtime_id`, `template_key`,
`template_version`, `autonomy_level`, Access and skills separately from
connectivity. A runtime heartbeat proves registration; it does not prove model
authentication, a successful role turn, or independent QA. Read pending and
active tasks before restarting a shared daemon. With a user PAT/JWT, workspace
discovery covers all account memberships even when the CLI profile has a
selected `workspace_id`; a workspace-scoped daemon token remains limited to
its bound workspace.

Regression anchors: `TestCreateAgentFromTemplate_CopiesTheRoleOntoAnOrdinaryAgent`,
`TestCreateAgentFromTemplate_RejectsUnknownAndUnlistedKeys`,
`TestCreateSquadFromTemplate_ReusesExistingRoleAgents`, and
`TestCreateSquadFromTemplate_ReusedAgentKeepsItsEdits`. Their assertions concern
template identity, declared authority and retained workspace edits; use a real
task result to establish model execution. A copied prompt plus an online badge
is insufficient evidence of a functioning role.

For squad handoffs, `util.ParseMentions` reads Markdown links such as
`[@Reviewer](mention://agent/<UUID>)`; plain `@Reviewer` or a missing bracket
does not enqueue that member. The comment CLI sends content without converting
names. A human reply to a leader's comment can create an ordinary agent turn
through `EnqueueTaskForThreadParent`, without squad-leader briefing. Explicitly
mention the squad when the next turn needs its leader protocol. A leader's
member handoff must produce a distinct member run with `source=delegation`,
the originating leader task and trigger comment; role headings authored by
the leader do not establish independent review. Capture the current runtime
briefing when investigating delivery, and distinguish its presence from model
compliance. `TestCreateComment_SquadMentionTriggersLeader` and
`TestClaim_LeaderTaskFromCommentMention_InjectsBriefing` cover the product
paths; real task and comment provenance is still needed for pilot acceptance.

### Scenario: scripted squad integration checks

#### 1. Scope / Trigger

Use a deterministic provider double when validating squad plumbing without a
real model. It can exercise actual queue/auth/comment/return/status paths, but
its canned decisions and patch replay do not establish model judgment or
instruction compliance. Label every simulated report and retain earlier failures.

#### 2. Signatures

- `MULTICA_CODEX_PATH` can select a test-created app-server executable; the
  `codex` provider key then selects an adapter, not proof of a real Codex process.
- `daemon.taskMulticaEnvironment` supplies `MULTICA_TOKEN`, `MULTICA_AGENT_ID`,
  `MULTICA_TASK_ID`, `MULTICA_WORKSPACE_ID` and the task CLI configuration.
- `multica issue comment add <issue> --parent <current-trigger> --content-file ./report.md`
  and `multica squad activity <issue> action|no_action --reason <text>` use the
  current task's authority and provenance.

#### 3. Contracts

Require the supplied `mat_` credential for role actions; an operator's human
credential is setup authority, not a substitute for a member's identity. Match
the product comment's `author_id` and `source_task_id` to the actual task. Member
dispatch and leader return require precise `delegation`, the correct
`delegated_from_task_id`, and the corresponding trigger comment. Headings and
self-reported task IDs are insufficient.

Write agent comment files inside the current workdir. The CLI rejects an external
`/tmp` file unless explicitly overridden; a simulation should follow the same
boundary rather than use `--allow-external-file`. Its process deadline must leave
the CLI's transport timeout and context grace time to report the result.

An unlisted leader cannot be recreated with the listed-role endpoint. If an
authorized pilot needs a custom coordinator, set its declared autonomy through
the normal human-only update, verify it before use, and retain `template_key` as
empty. Record source template/version/content hash separately as `derived_from`.
Do not fabricate built-in provenance or rebind unrelated shared roles.

#### 4. Validation & Error Matrix

| Observation | Permitted conclusion / handling |
| --- | --- |
| Malformed mention creates no member task | Syntax rejection verified; no successful dispatch |
| Actual member comment wakes one leader | Return wiring verified for those task identities |
| Scripted QA command succeeds | Fixture check passed; no independent AI judgment claim |
| Comment write times out | Outcome uncertain; read back before a fresh trigger |
| Standalone CLI returns `READY` | That client worked; SDK/squad execution remains unproven |
| Handler package exits 0 without test events | Verification failed; do not accept a silent DB skip |

#### 5. Good / Base / Bad Cases

Good: a labelled script drives actual task-token operations and an independent
checker verifies persisted actors, source tasks, return edges and activity.
Base: the same product paths remain available to a real provider.
Bad: count a canned report, human-authored member result, or online runtime badge
as a completed model-driven security workflow.

#### 6. Tests Required

Retain the private-owner return, no-action and current-leader attribution checks
in `squad_worker_comment_wakes_leader_test.go`, `squad_no_action_test.go` and
`squad_evaluation_provenance_test.go`. Run handler tests against an explicitly
selected disposable, migrated `DATABASE_URL`: current `handler/TestMain` refuses
missing/invalid/unreachable configuration. Older candidates may use a local
fallback or silently skip; never run them without the explicit isolated boundary.
Assert the exact expected `run`/`pass` event sets, no skips/failures, and use
`scripts/go-test-with-agent-cli-guard.sh` to reject ambient agent CLI execution.

#### 7. Wrong vs Correct

Wrong: `task.status == completed` implies real-model acceptance.
Correct: separately record execution mode, verify the persisted task/comment
graph and actual commands, and leave model behavior unverified for a scripted run.

The existing architecture, security, release, reliability and progress skills also
carry the five governance checks identified by the lifecycle review: old-client/plugin
contract matrices, trust-boundary and supply-chain/SBOM review, recovery/RPO/RTO
evidence, and product-outcome baseline/window/result evidence. Each of these five
now carries an explicit `失败样例` rule that returns `hold` or `unknown` when the
governing evidence (fallback/compatibility window, source and trust-boundary review,
backup and recovery drill, product baseline/window) is missing, rather than accepting
a passing build or a deferred-upgrade assumption as proof. These remain composable
responsibilities rather than five new listed roles. The release squad (template
version 4) now strings these five checks into a release-gate drill and reports which
it skipped: security, supply-chain and old-client/plugin compatibility evidence is
sourced upstream from the review gate, and the product-outcome check hands off to the
progress-report owner and stays `unknown` until a baseline and observation window
exist.

Progress-report version 4 keeps evidence-gap follow-up inside the report's
read-only business boundary: list missing evidence and a suggested owner in the
current report issue, then hand task creation to that owner. Failure examples
must not grant writes that the role and skill permission sections prohibit.

Migration Reviewer template version 2 and ADR role-skill version 6 keep missing
evidence follow-up inside Observer authority: write gaps, a proposed issue,
owner and acceptance criteria in the current issue comment, then hand creation
to a squad lead with the required authority or a person. The reviewer/architect
does not create the issue. Architect's own role instructions remain version 2;
Migration Review's skill remains version 1. Tests check each responsible text
separately, alongside the existing Observer issue-create 403 regression.
Version changes identify new defaults; they never re-materialize or overwrite
existing workspace copies, even when a new agent binds an older skill by name.
`TestCreateAgentFromTemplate_ObserverRevisionsPreserveWorkspaceCopies` verifies
new versions and unchanged old instructions, content, config, files and bindings.

## Role skill body language and explicit workspace updates

Role, squad-leader, squad, and Mika instruction bodies are authored in
Simplified Chinese. Picker language still selects labels and descriptions only;
the instructions editor shows the canonical persisted body. Preserve CLI syntax,
protocol values, the Mika name placeholder, and each role's behavior when
translating. Existing copies are never retranslated when a viewer changes locale.

Use the explicit maintenance workflow in `scripts/localize-agent-instructions.md`
for existing instructions. Template key/version alone cannot identify an
unmodified copy: compare the full historical body, and translate that version's
rules. Agent/squad PUT supports paired `expected_instructions` and
`expected_updated_at` fields with an atomic SQL comparison. Echo GET's complete
RFC3339 timestamp, discover `X-Multica-Instructions-Precondition: 1`, and finish
upgrading every API replica before relying on this optional-field contract.
Stale writes return 409 without updates or events; successful writes keep the
normal notification path. Conditional rollback also compares the recorded
post-write timestamp and validates the original backup hash.

The fifteen `builtin_role_skills/*/SKILL.md` bodies and discovery descriptions use
Simplified Chinese. Descriptions state when the skill applies and its concrete
result, preserving important delivery and authority boundaries without repeating
the workflow. Keep canonical names, other frontmatter, CLI syntax and
protocol/status values unchanged. Translate the working method, not the role's
permissions or business rules. These translations do not
set a global response language or change the role instructions' output contract.

Language-only edits keep the existing behavior version; bundle hashes already
track content changes. Materialization must continue to reuse existing workspace
copies unchanged. Updating an existing copy is an explicit content update scoped
to the intended workspace, with unrelated frontmatter and customizations
preserved. Historical bodies need their own translation when their rules differ
from today's template; do not silently upgrade behavior while localizing.

Grounding changes to the ADR, requirement-clarification and documentation skills
are material behavior changes, not translation-only edits. Requirement-clarification
and documentation sit at versions 3 and 2; the ADR skill is now at version 6.
After grounding, it gained the old-client/plugin compatibility matrix, the explicit
failure-example/`hold` rule, and an Observer-compatible evidence-gap handoff.
ADR facts need supporting task/code/document evidence;
requirement clarification separates known requirements from unverified technical
advice; documentation summaries must follow searches of the final files and
disclose coverage limits. Requirement-clarification version 3 retains the generic
evidence rules and removes CSV-specific routing from the default role skill.
CSV regression review material lives in
`scripts/skill-eval/references/csv-export-safety.md`, outside distributed skill
bundles and model fixtures. Preserve the original CSV case inputs so the reviewer
reference does not become a supplied answer.
Supporting references must ship in `RoleSkillTemplate.Files` and in any native
evaluation snapshot or explicit workspace update. The reference-delivery test
`TestRoleSkillTemplates_FilesMatchSource` in `builtin_agent_templates_test.go`
checks complete bundles against source files without requiring a domain-specific
attachment on a general-purpose role skill.

`skill update --content-file` replaces the entire stored content but does not
synchronize frontmatter into database metadata. When updating a description,
send both `description` and the content with matching frontmatter through the
normal update API. Omit `files` and unrelated fields
to preserve supporting files, bindings and labels. The endpoint has no atomic
compare-and-swap contract: compare before writing and verify afterwards, without
claiming those checks eliminate concurrent-write races. New tasks receive the
updated body through the existing content-hash cache.

## Skill template catalog

`GET /api/skills/templates` returns the fifteen role-skill templates under the
existing authenticated workspace route group. It reads the embedded registry,
does not materialize workspace rows, and returns no database identity. The UI
edits a snapshot and creates a new ordinary skill via `POST /api/skills`.
`config.template_source` is editable information, not authorization or official
provenance. Existing workspace instances, assignments and autonomy levels remain
independent from this read-only catalog.

### Scenario: operator-mounted skill-template directory

**1. Scope / Trigger** — Infra/env wiring: air-gapped deployments cannot pull
skills from clawhub/skills.sh/github, so the catalog gains a second source read
from a read-only mounted directory. Handler source switched from
`service.RoleSkillTemplates()` to `TaskService.SkillTemplates()`.

**2. Signatures**
- `func (s *TaskService) SkillTemplates() ([]RoleSkillTemplate, error)` — nil-safe;
  `RoleSkillTemplates()` (embed, stable order) first, then mounted entries.
- `scanSkillTemplateDir(dir string, embedNames map[string]struct{}) []RoleSkillTemplate`.

**3. Contracts**
- Env: `MULTICA_SKILL_TEMPLATE_DIR` (optional). Read once at handler assembly,
  `TrimSpace`d into `TaskService.SkillTemplateDir`. Empty/unset = embed-only.
- Compose: `${SKILL_TEMPLATE_DIRECTORY:-./skill-templates}:/app/data/skill-templates:ro`;
  default env points at that mount.
- On-disk: `<name>/SKILL.md` (+ optional supporting files), byte-identical shape
  to `builtin_role_skills/<name>/`. Directory name is the authoritative template
  `Name`; frontmatter still parsed for `description`.
- Response: unchanged `SkillTemplateResponse` (`name/version/description/content/files`);
  mounted entries carry `version: 0`. No frontend or `SkillTemplateListResponseSchema`
  change — `version` already `.default(0)`.

**4. Validation & Error Matrix** (each is skip-one-entry + `slog.Warn`, never a list-level failure)
- name fails `^[A-Za-z0-9_-]+$` (also blocks `..`, `/`, `\`, dot-prefix) -> skip
- top-level entry is a symlink (`entry.Type()&os.ModeSymlink`) -> skip, SKILL.md never read
- not a directory / missing `SKILL.md` / frontmatter has no `name` -> skip
- name clashes with an embed role skill -> skip (embed wins, D4)
- supporting file is a symlink, or per-file (1 MiB) / total (8 MiB) / count (256) cap exceeded -> skip file
- dir unset / missing / unreadable -> embed-only, no error (missing dir is not `slog.Warn`ed)

**5. Good/Base/Bad Cases**
- Good: `team-style/SKILL.md` + `references/lint.md` -> lists after embed with files.
- Base: env unset -> byte-identical to the prior embed-only catalog.
- Bad: `sneaky` symlinked to an external dir -> not listed, body not leaked.

**6. Tests Required** — `skill_template_dir_test.go`: `EmbedOnlyWhenDirUnset`
(nil/`{}`/empty), `MissingDirIsEmbedOnly`, `MountedEntryWithFiles`,
`MalformedEntriesSkipped`, `OversizedSkillMdSkipped`, `PathEscapeSkipped`
(supporting-file symlink), `TopLevelSymlinkSkipped`, `EmbedWinsOnNameClash`.
`skill_template_test.go`: `IncludesMountedDirectory` + the retained verbatim
embed-only test (nil `TaskService`). Frontend: `skill-template-schemas.test.ts`
(version-0/non-builtin parse + malformed-mount fallback) and
`skill-presentation.test.ts` (mounted name -> null -> raw-description panel fallback).

**7. Wrong vs Correct**
- Wrong: `os.Stat(<name>)` to test IsDir — follows a top-level symlink and reads
  its `SKILL.md`, leaking an out-of-tree body even though supporting files are refused.
- Correct: check `entry.Type()&os.ModeSymlink != 0` first (link's own mode, no
  follow) and skip; then `entry.IsDir()`. Symlink refusal is uniform across the
  top-level entry and supporting files.

## MCP deployment catalog

`McpCatalog` is the shared boundary for listing and trusted creation. It merges
the nine built-in recipes with `<key>/mcp.json` from `MULTICA_MCP_TEMPLATE_DIR`;
zero/unset directory configuration retains the built-ins. Inject configuration
in the handler rather than reading environment variables inside pure parsers.
The directory belongs to the API server, while stdio commands and remote
connections belong to the agent runtime. Listing never executes or probes them.

- Manifest schema 1 declares localized metadata, stdio/HTTP config and restricted
  inputs. Static env/headers and arbitrary command/URL interpolation are forbidden.
  Inputs map to fixed env/arg/header targets; credentials never go into argv.
  HTTP requires `MULTICA_MCP_TEMPLATE_ALLOW_HTTP=true`; HTTPS is the default.
  Public deployment responses contain no config, targets, filesystem paths or
  input values. Built-ins retain their existing public config for compatibility.
- Identity is `(source, key)` (`builtin` or `deployment`); same-key sources coexist.
  Persist nullable `template_source` alongside key/version, backfill existing
  template rows as builtin, and leave custom rows null. Rename retains identity;
  full config replacement clears all three fields. No extra index or foreign key.
- Deployment versions hash the validated normalized content with a stable
  versioned hash domain. Include config, input mappings/constraints and all
  localized metadata; ignore whitespace/object order but preserve array order.
  Versions must survive process restarts, locale changes and replica changes.
  Built-in numeric revisions keep their semantics.
- Creation selects source/key/version and resolves inputs from the same snapshot
  that passed the version check. Missing source means builtin only; unknown source,
  incomplete identity and simultaneous custom config are rejected. Changing or
  deleting files never mutates saved instances or bindings.
- Share the `mcp_source_version=1` serializer boundary across workspace list,
  create/update and agent-assignment summaries. Without it, deployment source/key/
  version are null while ordinary management fields remain. Built-ins are unchanged.
  This prevents an old client from associating deployment/playwright with builtin/playwright.
- Scan on every request with bounded enumeration/read budgets: 64 KiB per file,
  256 first-level entries, 4 MiB total reads including invalid files. Reject unknown
  fields/schema, duplicate JSON keys, trailing JSON, excessive depth, symlinks,
  escapes and special files; constrain file opens to the root during races.
- Empty/unset/missing directories successfully return built-ins. Skip individual
  invalid manifests with key/error-category logs. Directory IO/permission and
  aggregate limits return `503 mcp_catalog_unavailable`, not a partial success.
  Do not include manifests or input values in logs/errors. Saved management and
  builtin creation remain usable while the deployment directory is unavailable.
- Return `409 mcp_template_changed` for a stale version and
  `409 mcp_template_unavailable` for a removed/invalid selected entry. Ordinary
  input/source validation remains 400. Clients use codes, not English text.

Compose mounts `${MCP_TEMPLATE_DIRECTORY:-./mcp-templates}` read-only at
`/app/data/mcp-templates`. Offline bundles provide an empty directory and the
manifest guide, not build-host recipes. First rollout needs a backend/database
and client update; later atomic file replacements require no restart. Multiple
API replicas must mount synchronized contents. Format, publication and rollback
semantics live in `docs/mcp-catalog-publishing.md`.

Required coverage: stable hashes and source collisions; live file add/update/remove;
invalid-entry isolation versus directory failure; strict parsing and read/path
limits; restricted input resolution; stale/unavailable errors; write-only summaries;
legacy source negotiation; rename/replacement and retained assignments. Handler
tests need an isolated migrated database and must actually execute, not pass via
the no-database `TestMain` exit.

## Onboarding skills require task provenance

`BuiltinSkills()` returns general platform skills. Use `TaskBuiltinSkills` for
dispatch: `multica-onboarding` requires the task's own built-in Mika and a
persisted `onboarding_kickoff` in its chat session. The session marker preserves
availability for follow-up and retry turns. Do not use display names, prompt
text, or only the current input batch to decide eligibility.

Both inline and slim claims, and subsequent bundle resolution, must use the same
scope. A failed scope read must fail the request rather than silently omit the
skill. General builtin downloads do not query onboarding scope. Regression
coverage lives in `daemon_builtin_skills_scope_test.go` and
`builtin_skill_scope_test.go`.

## Autopilot prompt language and business boundaries

The ten `builtin_autopilot_templates/*/PROMPT.md` bodies use canonical
Simplified Chinese, following the role-instruction and role-skill convention.
Locale selects catalog labels and the title stored on a newly created autopilot;
it does not select another execution body. The preview, `autopilot.description`,
and the dispatched brief must preserve the same content. Template releases and
locale changes never rewrite existing workspace copies.

Keep the five `run_only` patrols distinct from the five `create_issue` summaries.
Patrols search the workspace for their existing open issues before creating work,
append evidence to an existing issue when it covers the finding, and create no
issue or comment when there is no substantive finding. Summaries comment on the
issue dispatch already created, including for an empty reporting period. A
missing check result is not a verified failure or a zero count.

The bug-triage template is version 2 because it explicitly maps severity to the
supported `priority` values: `urgent`, `high`, `medium`, and `low`. `none` remains
unprioritized when evidence is insufficient. Contributor authority is required
by its instructions to change priority; observers recommend in comments. State
whether each write was applied or only recommended, according to the actual
response. Do not assume that every priority write has a dedicated server-side
observer rejection: the task's autonomy policy must also be respected. The other
templates keep their existing versions unless their behavior changes.

Daily Progress Report v1 is immediately before Weekly Progress Report v2 in
the roster. Daily uses `0 18 * * *` and the preceding 24 hours; weekly keeps
`0 17 * * 1` and the preceding seven days. Both are `create_issue` summaries
with dated titles. Template adoption selects timezone and execution context;
source queries must separately apply the requested workspace/project filter.

Both reporting prompts require paginated evidence and status history for
period completions, disclose truncation/missing data, and exclude reporting
issues from business counts. A successful report comment permits only the
current report issue to enter `in_review`. `TaskService.CompleteTask` does not
change issue status; `SyncRunFromIssue` completes a create-issue run on
`in_review` or `done`. An Observer can post a report but cannot make that
transition. Weekly v2 repairs the previous blanket status prohibition;
existing v1 workspace copies are deliberately preserved.

`TestAutopilotTemplateCreate_ChineseTemplatesDispatchVerbatim` covers all ten
templates through real database creation, schedule dispatch and daemon claim;
`e2e/autopilot-template-zh.spec.ts` covers the Chinese browser flow with real APIs
and an isolated runtime fixture. Neither check launches a real model. Content
review must still compare the prompts' evidence, thresholds and authority rules;
string assertions alone do not establish semantic equivalence.

`autopilot_template_progress_test.go` covers daily/weekly comment delivery and
review closeout, Observer refusal and unchanged source tasks with real DB
fixtures. Its handler harness invokes the sync callback explicitly;
`e2e/progress-reporting.spec.ts` verifies the real server listener wiring and
the role/catalog/adoption UI. E2E report comments are explicit fixtures, not
evidence that an LLM independently gathered or summarized the data.

## Convention: create_issue autopilot templates must carry a `{{date}}` issue title template

**What**: Every built-in autopilot template with `ExecutionMode: "create_issue"` must set `IssueTitleTemplate` to `"<Chinese title> — {{date}}"` (em dash), using its canonical Chinese catalog title. Every `run_only` template must leave it empty so the column lands NULL.

**Why**: `dispatchCreateIssue` creates the issue BEFORE the agent runs, and `interpolateTemplate` (`server/internal/service/autopilot.go`) falls back to `ap.Title` when `issue_title_template` is empty. A `create_issue` template without `{{date}}` produces an identically-titled issue every period — 30 indistinguishable "每日变更回顾" issues in a month. `run_only` opens no issue, so a title template there would be dead weight.

**Example** (`server/internal/service/builtin_autopilot_templates_roster.go`):

```go
// Good — create_issue: dated title, one distinguishable issue per run
{
    Key:              "daily-change-review",
    ExecutionMode:    "create_issue",
    IssueTitleTemplate: "每日变更回顾 — {{date}}",
    ...
}

// Good — run_only: zero value, column lands NULL
{
    Key:           "hourly-queue-check",
    ExecutionMode: "run_only",
    ...
}

// Bad — create_issue without {{date}}: every run opens an identically-titled issue
{
    ExecutionMode:      "create_issue",
    IssueTitleTemplate: "每日变更回顾",  // rejected by the roster test
    ...
}
```

**Enforcement**: `TestAutopilotTemplates_IssueTitleTemplates` (`server/internal/service/builtin_autopilot_templates_test.go`) asserts every create_issue template uses its Chinese title, contains `{{date}}`, and passes `ValidateIssueTitleTemplate`; every run_only template is the empty string. `{{date}}` is the only placeholder `ValidateIssueTitleTemplate` accepts — do not add others without updating that validator and its docs. A schedule dispatch uses its trigger's timezone; a manual run without a trigger ID keeps the existing UTC fallback.

## Gotcha: template-decided fields come from the server, never the request

> **Warning**: `POST /api/autopilots/from-template` must not accept `title` / `description` / `cron_expression` / `execution_mode` / `issue_title_template` from the client.
>
> Provenance honesty: a client that could supply its own prompt while claiming `template_key` would stamp a template's identity on an autopilot that runs something else entirely. The handler takes all content fields from the registry entry; the request carries only `template_key`, `assignee_id`, and optional `assignee_type` / `project_id` / `timezone` / `language` / `subscribers`. The e2e spec (`e2e/autopilot-template.spec.ts`) asserts the create body carries none of the template-decided fields.


## Administration-managed resource publication

Managed publication is opt-in and preserves the existing deployment consumer protocols.
See [Resource publishing](./resource-publishing.md) for upload limits, locking,
CAS/replay, audit recovery, catalog snapshot and compatibility contracts.

## Delivered work and accepted stages

1. **Scope:** ordinary issue turns, child stage barriers and Review Gate routing; this is separate from account collaboration state machines.
2. **Signatures:** `writeWorkflowIssue`, `notifyParentOfChildDone`, `isTerminalChildStatus`, `CompleteTask`; review-gate-lead template version 3, review-gate squad version 3.
3. **Contract:** an agent with status-write authority delivers an assigned issue/sub-issue to `in_review`; restricted reporter/reviewer task capabilities deliver a comment and leave status to an authorized actor. Human acceptance or an existing authorized integration produces `done`; `done/cancelled` alone close the ordinary child stage barrier. Completing an execution does not change the issue. Unrestricted same-issue member result comments use the existing leader wake route; restricted comments retain durable non-dispatch eligibility. Neither is child-stage acceptance.
4. **Matrix:** child→in_review waits; child→done/cancelled checks its parent stage; dispatch leaves the parent in progress; unknown required specialist evidence cannot pass review. Guest/Agent Identity status prohibitions remain authoritative.
5. **Cases:** good is a delivered child visibly awaiting acceptance; base is existing same-issue worker return; bad is telling a child that in_review automatically advances the next stage or bypassing acceptance with a new parent trigger.
6. **Tests:** generated assignment/comment/leader briefs state the same distinction. Review Gate leader and squad must independently cover all six actual review seats, with conditions and missing-evidence behavior. Keep skill line budgets, source maps, bilingual summaries and generated Chinese docs synchronized.
7. **Wrong/correct:** wrong: runtime completed ⇒ issue accepted. Correct: delivery, execution completion, human acceptance and parent wake are distinct facts. Role template upgrades never overwrite existing workspace copies; new defaults require explicit instance upgrade for old agents.
