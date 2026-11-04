# Collaboration Detail: Delivery, Acceptance and Next Action

## Goal

Make the shared Web/Desktop detail explain what is requested, who owes the next
step, what was delivered, and what the current viewer should do. Deliver the
complete human information, submission, revision and acceptance journey across
account and source-workspace entries of the same issue.

## Background and confirmed facts

Source brief: `docs/plans/2026-11-02-collaboration-detail-optimization.md`.
Worktree: `/Volumes/artisan/code/2026/multica-0.4.37`, branch
`codex/cross-workspace-collaboration`. Existing dirty work is recorded in
`research/worktree-baseline.json`; shared workflow/locales already have source
picker changes which must survive this task.

- `server/internal/service/collaboration_handoff.go:247` already submits an
  acceptance handoff and atomically moves status/responsibility to the creator.
  Its note is optional and there is no explicit acceptance decision/rework API.
- `server/internal/service/collaboration_lifecycle.go:290` currently permits
  creator completion from any nonterminal status; generic status options include
  review. This task intentionally tightens that public business contract.
- `server/internal/service/collaboration_workflow.go:207` owns atomic handoffs;
  `:296` writes immutable events with exact handoff IDs and original request IDs.
- `server/internal/service/collaboration_attachments.go:522` already binds
  issue-intended files, validates scope and retains issue references. Submitted
  evidence must use these identities and protected downloads.
- `packages/views/collaboration/collaboration-detail.tsx:79` prioritizes generic
  properties and tools over outcomes. `comment-composer.tsx:73` requires a manual
  preview before every send. Ordinary comments do not perform a handoff.
- No structured task-level manual AI policy exists. The deployment execution
  gate, current grants, queue/run state and task policy are separate facts.
- Account and source entries resolve one collaboration issue. A collaboration
  child's completion does not complete its ordinary parent.

## Requirements

| ID | Observable requirement |
| --- | --- |
| R01 | Requirements, reference material, current responsibility and a role-specific next action are legible before auxiliary tools. |
| R02 | A formal submission requires a nonblank result and basis; text-only work needs no upload. Author, exact submission identity and time remain stable. |
| R03 | In the ordinary workflow only the initiator accepts/completes. Acceptance confirms the exact current submission. Externally handled work cannot be completed through that workflow without delivery; an initiator currently doing their own work may supply a result/basis and complete atomically without a self-transfer step. Preserve the existing scoped disabled-creator super-admin completion exception; it does not invent initiator acceptance. |
| R04 | The initiator may return the exact pending submission with a nonblank reason and explicit eligible recipient, defaulting to its actual author. Prior results and decisions remain readable through resubmission and reopening. |
| R05 | Generic status changes cannot manufacture review/completion or bypass a pending acceptance. Backlog, general blocking and cancellation retain their meanings. |
| R06 | Information requests/returns keep their exact existing chain identity and responsibility. An ordinary comment never returns information or submits a result. |
| R07 | Optional evidence uses existing issue/draft/file authorization and retention. Editing/deleting a referenced comment cannot mutate a submitted snapshot or its retained evidence. |
| R08 | Publish ordinary comments with one user action using the existing validated preflight. Explicit AI targeting requires a visible impact/target confirmation. Stale preflight cannot publish changed text, parent, files, suppression or execution observations. |
| R09 | Add an explicit initiator-owned manual policy. Existing tasks default to assisted. Manual tasks cannot create new AI input/execution or resume it; changing to manual pauses/requests stop through existing controls, without falsely claiming termination. Restoring assisted never resumes automatically. |
| R10 | Omit a large empty AI console when no AI facts or unresolved intent exists; history, meaningful targets/runs/inputs, selected run links, errors and original-key recovery remain accessible even with new execution disabled. |
| R11 | Main content orders requirements, delivered result/decision, and discussion/progress. Selected source metadata and participants are compact, labelled and discoverable. Empty properties live in editing. |
| R12 | Files, children, working summary, knowledge promotion and detailed audit are secondary entries; their permissions, editing/history, approval/reuse scope and recovery remain intact. No duplicate rendering of a comment in the main progress stream. |
| R13 | Legacy/malformed evidence is factual: unavailable basis/decision/author is not invented; a status alone does not imply delivery or acceptance. A legacy submission must be revised/resubmitted or replaced explicitly before acceptance. |
| R14 | Preserve encrypted uploaded draft/file bytes, existing scoped client intent, exact original-key recovery, revision/epoch checks, denied-access erasure, server/viewer isolation and awaited refresh. No duplicate state store or silent optimistic workflow completion. |
| R15 | Shared Web/Desktop and account/source entries show the same result/status. External participants receive no workspace membership or broader catalog/file access. Ordinary parent lifecycle stays independent. |
| R16 | Long content, narrow/intermediate/wide windows, both locales, keyboard use, anchor links and restored scroll remain usable. |
| R17 | Collect actual production-Web and actual Electron Desktop evidence of manual task → information request/return → submission → revision → resubmission → acceptance, with exact listener/build/source provenance. |

## State, role and action contract

| Authoritative facts | Responsible actor | Primary action | Secondary actions |
| --- | --- | --- | --- |
| Todo/backlog; no pending submission | Current handler | Start work | Information request, transfer |
| In progress/general block; no pending submission | Current handler | Submit result | Information request, transfer |
| Open information request addressed to handler | Recorded recipient | Return information | Comment, nested request |
| Pending exact submission; review; creator is handler | Initiator | Accept and complete | Return for revision, request information |
| Creator is handler; no pending submission/request | Initiator/handler | Record result and complete | Start work, transfer |
| Completed | Viewers | Read result and recorded decision | Initiator reopen; optional knowledge promotion |
| Cancelled | Viewers | Read cancellation and retained history | Initiator reopen |
| Other participant/source-only reader | Participant/reader | Discuss/provide material when authorized | Read files/history within current access |
| Review without a corresponding submission | Actual current handler | Factual status/repair actions | No inferred acceptance affordance |

## Decisions and boundaries

- Follow the source brief's recommended completion policy and revision behavior;
  no smaller UI-only substitution. Acceptance notes may be empty; actor/time and
  exact submission binding may not.
- Introduce task policy as `manual` / `assisted`; absence on old clients/servers
  is unknown, never evidence of manual enforcement. Existing stored tasks remain
  assisted. Do not infer policy from free text.
- Retain existing issue/handoff/event/operation/file mechanisms rather than
  adding a separate task/result state or result storage system.
- No real-model account/quota usage, deployment, broad cleanup, mobile UI, new
  dependencies, AI scheduler redesign or knowledge-governance redesign.
- Existing Web/Desktop form/operation JSON uses the synchronous local storage
  adapter; it is not encrypted client storage. The source brief requires reusing
  that scoped recovery mechanism, not introducing a new encryption/key lifecycle.
  Encrypted uploaded file drafts and bytes remain a separate preserved contract.
- On 2026-11-03 the user explicitly chose to retain the management forced-complete
  exception. Existing R38 authorization, disabled-creator scope, reason/audit,
  transaction and original-key recovery remain. A completed status without a
  linked initiator decision is factual completion, not an accepted submission.
- Do not reset/revert/commit/archive other sessions' work. Only owned changes may
  be committed after verification; shared-file baseline hunks must be protected.
- The user's active goal explicitly authorizes artifact review, task activation,
  implementation and verification in this session. Generic skill instructions
  asking for a second process approval do not supersede that authorization.

## Acceptance matrix

| Case | Requirements | Proof required |
| --- | --- | --- |
| A01 External manual handler | R01,R09,R10,R15 | Correct next actor/action, enforced policy, no empty console or expanded source access. |
| A02 Information chain | R06,R14 | Recorded requester/recipient and exact returns; comment has no transition effect. |
| A03 Text-only submission | R02,R07 | Result/basis required; exact stable author/time; transfer to creator; no forced file. |
| A04 Revision/resubmission | R04 | Exact result ID, reason, eligible recipient and retained earlier record/decision. |
| A05 Acceptance/source entry | R03,R15 | Initiator confirms exact submission; issue completes; same outcome at source entry. |
| A06 Self-handled completion | R03 | Atomic stored result/basis and initiator decision without self-transfer interaction. |
| A07 Unauthorized decision | R03,R04,R15 | Server rejects other participant and machine/agent principals; truthful UI capabilities. |
| A08 Generic status bypass | R05,R13 | Review/done cannot be fabricated; pending submission cannot be silently skipped. |
| A09 Ordinary comment | R06,R08 | Single Publish, validated preflight, zero AI commands and no handoff. |
| A10 Explicit AI target | R08,R09 | Target/impact preview and task policy/grant/deployment gate enforced. |
| A11 Disabled execution/history | R09,R10,R14 | Retained runs/inputs, Stop, unknown original-request recovery and truthful shutdown. |
| A12 Source comment/file changes | R02,R07 | Frozen result/evidence survives comment editing/deletion and protected reads. |
| A13 Legacy evidence | R13 | No invented result/decision or stage; clear unavailable/revision path. |
| A14 Stale/unknown/reconnect | R08,R14 | Inputs retained; exact request recovery; no duplicate delivery or false success. |
| A15 Access/server/viewer change | R07,R14,R15 | Scope and epoch cleanup denies protected reads/files and stale writes. |
| A16 Parent and auxiliary flows | R12,R15 | Parent independent; summary/knowledge/children permissions and recovery preserved. |
| A17 Layout and real journeys | R01,R11,R16,R17 | Production Web and actual Desktop chain, both locales/narrow content, keyboard/anchors/scroll and before/after captures. |

Canonical product matrices belong in Server/Core tests. Views own happy-path
wiring, accessible presentation and named UI regressions. Native/browser tests
prove the actual entry points instead of repeating every pure matrix in DOM.
