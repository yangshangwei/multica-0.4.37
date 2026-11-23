# Assessment A — independent design review

Date: 2026-11-20. Target: `packages/views/modals/quick-create-issue.tsx`. Mode: Operate. Scope: the approved blocked-feedback task, including its named helper, recipient-row, and vocabulary changes.

This assessment was completed independently of detector findings and Assessment B. I read the current source, approved `prd.md` / `design.md`, repository rules, and Impeccable critique guidance. I did not read prior critique reports or other assessments. `.impeccable/critique/ignore.md` is absent. The incumbent implementation is the visual authority; no PRODUCT.md / DESIGN.md was supplied by project context.

## Verdict

The no-recipient dead end is removed. The warning explains the missing prerequisite, offers two visible exits, and explicitly reassures the user that their draft is kept. Both exits worked in the isolated browser check: manual mode received the description, and agent setup navigation closed the modal while reopening restored the description. A blocked primary button remains keyboard reachable, exposes its reason independently of the tooltip, and moves focus to a useful next action.

The independent pass did not flag a remaining in-scope P0, P1, or P2 defect before its findings were frozen. The parent subsequently supplied a measured RCP-2 spacing failure, recorded separately as an integration delta below; final readiness depends on that correction. Existing polish observations are also listed separately. This is a design assessment, not a replacement for the parent task's automated acceptance results.

## Design specificity

The flow is recognizably an agent delegation composer. The recipient control, responsibility-based recommendation, description refinement, handoff verb, and manual fallback express Multica's task model. The restrained neutral modal, compact property pills, and semantic warning treatment fit the surrounding operational interface. This task improves the existing visual language coherently; it does not need a new visual concept.

The strongest product-specific detail is the distinction between writing an issue manually and asking an agent to file it. The warning and action copy make that distinction visible at the exact failure point. The two assist actions stay subordinate to the chosen recipient and task description rather than becoming a second primary workflow.

## Heuristics

| # | Heuristic | Score | Evidence and remaining limit |
|---|---|---:|---|
| 1 | Visibility of system status | 3/4 | The blocked button exposes a concise reason on keyboard focus; the persistent no-recipient warning explains the durable blocker. Loading, stale, success, and error semantics are present in source. Some nonpersistent blockers still require focus or hover, as approved. |
| 2 | Match between system and real world | 3/4 | “Send to agent” / “交给智能体” accurately describes delegation. The no-recipient copy explains why creation is unavailable. “Runtime” and autonomy remain domain concepts that a first-time user may need to learn. |
| 3 | User control and freedom | 4/4 | Close, expand, mode switch, and setup recovery are available. Both recovery routes preserve work in the live check. Suggestions require explicit adoption, and refinement retains its undo path in source. |
| 4 | Consistency and standards | 3/4 | The recipient control now shares the outlined property-control vocabulary; both AI helpers share tone and cancel/retry conventions. The inherited visible-title / accessible-dialog-title mismatch remains explicitly outside scope. |
| 5 | Error prevention | 3/4 | A single state resolver aligns explanation and submission gating; zero-candidate recommendations are disabled. Native disabling remains reserved for busy states. The existing broader composer and picker edge cases belong to automated verification. |
| 6 | Recognition rather than recall | 3/4 | The recipient, task label, helper prerequisites, keyboard hint, and recovery actions are visible. The picker keeps remembered/default choices in context. Some transient blocker reasons and header explanations remain on demand. |
| 7 | Flexibility and efficiency | 4/4 | The send chord works from a non-input control, avoiding the previous dead key after selection. The live check from Close produced exactly one intercepted request. Keep-open, default selection, direct picking, and optional AI assistance support different working styles. |
| 8 | Aesthetic and minimalist design | 3/4 | The normal composer has a clear primary action and quiet assistance. The warning is visually distinct without introducing a competing treatment. In the narrow English blocked state it consumes much of the editor's vertical room. |
| 9 | Error recognition, diagnosis, and recovery | 4/4 | The no-recipient notice names the problem, explains the prerequisite, offers setup or manual creation, and preserves the draft. Empty-description activation focuses the editor; no-recipient activation focuses setup. Recommendation failures have an alert plus retry in source. |
| 10 | Help and documentation | 3/4 | Contextual instructions answer what to write, who receives it, and how to recover. The modal provides enough task-level guidance to proceed; agent setup terminology is explained at its destination rather than exhaustively in this composer. |
| **Total** | | **33/40** | **Good** |

All ten heuristics apply to this operational form. Scores describe the reviewed surface, including acknowledged existing polish, rather than assigning perfect scores solely because the requested patch is complete.

## Cognitive load and emotional journey

The default composer has low cognitive load: the task description is the main work, the selected recipient is explicit, optional metadata stays behind the overflow menu, and AI assistance is opt-in. The no-recipient state introduces one actionable decision with two alternatives: set up an agent or switch to manual. No memory bridge is required because the draft follows both routes.

| Checklist item | Assessment |
|---|---|
| Single focus | Pass: description entry when ready; recovery choice when no recipient exists. |
| Chunking | Pass in the composer. The existing expanded picker exposes five sibling category filters, a local exception. |
| Grouping | Pass: recipient and its helper text remain together; warning actions are grouped within the warning; metadata and footer actions have separate rows. |
| Visual hierarchy | Pass: selected recipient, description, and primary action are distinct from assistance. The warning takes precedence only when the main path is blocked. |
| One thing at a time | Pass: setup/manual recovery resolves the durable blocker; refinement and recommendation are optional. |
| Minimal choices | Pass in the composer. The existing picker categories—All, Mika, Agents, Squads, Coordination—exceed four visible options. |
| Working memory | Pass: relevant recipient/context stays visible, and the draft survives both recovery paths. |
| Progressive disclosure | Pass: optional fields, picker details, and AI results are revealed on demand. |

The composer has zero checklist failures. Opening the existing picker introduces two related local exceptions (chunking and number of choices), giving that expanded panel moderate load. Its category structure was not changed by this task and is not an archive blocker.

The earlier emotional valley was an unexplained disabled primary action. The new warning converts that into a comprehensible setup choice and makes preservation of work explicit. Keyboard activation gives a useful destination rather than a silent refusal. The ending is safer: the user reaches a workable manual form or the setup page without losing their description. The narrow English warning is text-heavy, but its two exits remain immediately visible.

## Strengths

1. **Recovery is concrete and trustworthy.** The notice combines cause, next step, and draft reassurance; live navigation and manual-mode checks confirmed that reassurance.
2. **The primary action explains itself across input methods.** It is native-enabled but `aria-disabled` when blocked, has a separate accessible description, displays a focus tooltip, and routes activation to the relevant field or recovery action.
3. **The changes strengthen the existing hierarchy.** The outlined recipient becomes a clear control, both helpers share a restrained visual role, and “Send to agent” consistently describes the result. The 375-pixel footer keeps the full action label and 44-pixel primary target visible.

## Scoped findings

The independent pass did not identify a P0–P2 finding within the approved task boundary. I am not adding speculative findings to meet a numerical quota. The original no-recipient blocking condition is now recoverable through two tested paths. The measured RCP-2 integration finding received after this pass is separated below rather than retroactively presented as independent evidence.

Implementation evidence relevant to this conclusion:

- `quick-create-issue.tsx:673`: single submission-state mapping and blocker reasons.
- `quick-create-issue.tsx:707`: activation routes focus to editor, picker, autonomy recovery, or setup.
- `quick-create-issue.tsx:748`: modal send chord excludes editable targets, repeats, composition, and already handled events.
- `quick-create-issue.tsx:905`: persistent no-recipient warning and the two recovery actions.
- `quick-create-issue.tsx:1191`: accessible blocker description outside the button; the footer preserves a direct layout child.
- `creator-recommendations.tsx:101`: shared helper tone, no-candidate disabling, inline explanation, cancel control, stale status, and retry.

## Personas

**Jordan — first-time user.** The blocked state now answers “why can I not send this?” and offers two specific actions. The current workspace's absence of a usable agent is explicit. Jordan can continue in manual mode without understanding runtime setup first. Remaining friction: “runtime” still requires domain understanding, and in the English narrow layout the explanatory notice leaves a small editor window. Neither prevents reaching a usable path.

**Sam — keyboard / accessibility dependent user.** Live keyboard Tab reached the `aria-disabled` submit button and displayed “Write a task description first”; its `aria-describedby` referenced the same text. Enter focused the editor. In the no-recipient state, activation focused the setup link. The status/alert roles and labels are present in source. The known “Quick create issue” versus “Create with agent” dialog-name difference remains outside scope. An actual VoiceOver/NVDA session and 200% browser zoom were not run, so this review does not certify those experiences.

**Alex — power user.** The ready form remains compact. A modifier send chord from Close dispatched exactly one intercepted quick-create request in the isolated live browser. The revised source also excludes picker portals, text inputs, plain Enter, and composition. No additional confirmation or mandatory assistance was introduced. Alex's existing direct path is retained while blocked attempts now provide a useful correction target.

## Out-of-scope polish observations

- **P3 — narrow English no-recipient state has little editor room.** At 375×812, English copy wraps the recipient/helper section and warning into more lines than Chinese. The task description's scrolling region becomes short, while both warning actions and the full footer remain visible. A later layout pass could shorten the warning copy or grow the blocked layout; that would be an optional improvement beyond this task's approved composition.
- **P3 — accessible dialog name differs from the visible mode title.** The live surface exposes “Quick create issue” while the visible header says “Create with agent”; the PRD explicitly defers this coordinated selector change. Address it in the existing polish scope, not as a new requirement here.
- **P3 — picker category density.** Five existing category filters require more scanning than the two-choice composer recovery. This review did not identify a new picker navigation defect, and changing its information architecture would exceed the bounded patch.

No changes to `pen-sheen`, `border-beam`, shared Switch contrast, header tooltip mechanics, property focus rings, or unrelated alignment are recommended as part of finishing this task.

## Evidence and limits

The parent-provided production web environment was `http://localhost:60677` with a synthetic review workspace and account. I created independent fresh headless Chromium contexts and tabs using the installed Playwright runtime. Empty-recipient states used local response interception for agent/squad lists. Quick-create submission was intercepted with a local failure response so no agent was executed. Chinese screenshots locally overrode only the synthetic `/api/me` language response. No user's browser or Desktop session was attached.

Native CUA was attempted first. `cua.getState()` returned `browsers: []`, and `cua.createBrowserTab("iab", "about:blank", { visible: false })` returned `Browser is not available: iab`. The CUA completion summary was sent. The browser-use fallback guidance was read before using isolated Chromium. Fresh browser instances were closed in `finally`; the process check found no remaining headless Chromium process after this review.

Independent live checks completed:

- Desktop empty-description, ready, and no-recipient layouts.
- Keyboard focus tooltip and accessible description; native `disabled=false` with `aria-disabled=true` for a blocked action.
- Empty-description Enter activation focusing the content editor.
- No-recipient activation focusing setup, including coordinate activation at 375×812.
- Manual recovery preserving the description.
- Setup navigation to `/agents`, modal closure, and draft restoration on reopen.
- Modifier shortcut from a non-input control dispatching exactly one intercepted request.
- 375×812 English dark and Chinese light warning/footer visibility. Both primary buttons measured 112×44 pixels and remained within the dialog.

Fresh screenshots were inspected using `view_image`, under:

`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-blocked-20261120-bnee5aep/assessment-a/`

Relevant captures: `1280-en-light-no-recipients.png`, `1280-en-light-empty-description-focused.png`, `desktop-en-light-ready.png`, `375-en-dark-no-recipients.png`, and `375-zh-Hans-light-no-recipients.png`.

Visual comparison found both helper controls appropriately subordinate; this assessment did not measure contrast ratios. Loading/failure/autonomy/version/source-context variants and AI cancellation/stale/retry behavior were assessed from source, not exhaustively exercised here. The parent task's unit/E2E verification remains authoritative for those branches. The review used the running production build; the parent identified pending nonvisual boundary fixes and owns verification of its final rebuild.

## Post-assessment integration delta

After the independent assessment and 33/40 score were completed, the parent reported a measured **P2 RCP-2 spacing failure** in the English empty-description layout: the permanent recipient hint is 24px from the recipient trigger but only 12px from the description label because the recommendation prerequisite wraps between the trigger and permanent hint. This does not satisfy the approved rule that the hint belong more closely to the recipient control than to the next field. The independent pass saw this capture but did not measure or flag the spacing reversal, so this is explicitly credited to the parent's integration check.

The parent plans a minimal grouping correction within the approved scope. Its final screenshot and measurements must confirm RCP-2 before archive. This delta does not change the finding that the no-recipient dead end is removed, and the independent heuristic score remains frozen rather than being retrofitted to a later input. No detector findings or Assessment B output were consumed.

Questions skipped: user already approved implementation scope and asked to finish/archive.
