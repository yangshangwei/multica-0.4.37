# Research: Collaboration audit test coverage

- Query: What current interaction behavior is covered, and which meaningful regressions are missing?
- Scope: internal
- Date: 2026-11-05

## Findings

## Shared component test inventory

### packages/views/collaboration/account-picker.test.tsx

- distinguishes same-name accounts by full IDs and retains the chosen identity
- connects the native filter textbox to stable selected and disabled account options
- exposes navigation without selecting during IME and clears removed or empty active options
- keeps the active account through append-only paging while footer actions stay outside navigation
- browses sections, uses exact lookup, and closes with Escape without leaving the trigger
- keeps a members-only picker from selecting an undisclosed external account
- closes an already-open picker when its parent disables selection
- retains cached rows and retries the failed next page from its original cursor
- keeps cached rows visible while exposing and retrying a failed background refresh

### packages/views/collaboration/account-workbench-page.test.tsx

- opens selected detail independently of an empty current list without workspace requests
- clears denied detail and keeps the account list usable
- provides translated workbench navigation
- renders persisted activity with the original actor and before/after values
- keeps account administration on dedicated metadata reads and administration links

### packages/views/collaboration/attachment-panel.test.tsx

- closes a private preview and revokes its object URL when issue access is cleared
- offers PDFs for download only

### packages/views/collaboration/attachment-picker.test.tsx


### packages/views/collaboration/collaboration-detail.test.tsx

- shows the loaded range and keeps an older server
- restores an anchored detail on scope resume and refocuses repeated explicit opens
- opens ordinary children in their returned source workspace and downgrades to references after source access changes
- distinguishes missing child support from an explicitly empty projected list
- focuses the selected run and copies its account anchor from the shared detail
- keeps one comment list and composer across loading, polling and first-comment updates

### packages/views/collaboration/command-history.test.tsx

- keeps a cancelled original input visible after refresh while its current comment is edited and later deleted
- shows source-only readers missing legacy facts and recorded automatic cancellation without control rights
- removes open private history when loading more confirms access loss

### packages/views/collaboration/comment-composer.test.tsx

- publishes an ordinary comment with one action after the validated preflight
- shows AI impact and requires a second explicit action before publishing targeted input
- keeps changed shared input when a one-action preflight resolves after the draft changes
- invalidates grouping when another form updates the shared draft
- shows merged and later pending batches, then requires a fresh preview after suppression and editing
- describes a possible squad continuation in Chinese before any comment is sent
- groups people and AI, inserts @all without losing explicit targets, and sends the reviewed delivery
- identifies notification recipients and rejected targets instead of calling them waiting
- replies to an AI comment using its stable anchor without selecting any AI targets

### packages/views/collaboration/comment-list.test.tsx

- loads a placeholder through the human detail route before exposing editing
- does not restore a different revision into a placeholder editor
- moves focus into the delete confirmation and back to Edit when it is dismissed
- keeps the author
- keeps tombstones and nested reply links while exposing only the current author
- resolves a thread and removes only the viewer
- keeps a recovered conflict editor visible when its comment is outside the loaded page
- shows byte-limit feedback without losing pasted text or submitting an oversized edit
- confirms an older oversized refusal before unlocking its preserved input

### packages/views/collaboration/creation-form.test.tsx

- preserves the manual policy choice with the creation draft and sends it explicitly
- locks the source through delayed creation and completes the original form once
- keeps an unknown request on its source and locks original-key recovery until rejection settles
- allows another source beside idle content-free recovery but locks its active GET
- restores the form and private file after lost responses, then opens the recovered original issue
- keeps account creation entry usable with zero sources and returns keyboard focus on close
- keeps a durably rejected form through reload and sends identical input only on explicit resubmission
- opens real source-option candidates and ignores a late previous search without submitting the creation form
- keeps the parent picker and draft mounted after a failed search and retries that search
- does not reuse retained parent options after authoritative source-access denial
- keeps parent input immediate but waits 250ms and blocks Enter on previous candidates
- cancels a queued parent search when the source changes before debounce
- cancels with its draft intact and still submits natively by Enter after reopening
- reveals settled creation feedback once without moving focus or scrolling again on refresh and edits

### packages/views/collaboration/delivery-panel.test.tsx

- keeps exact current and earlier results distinct while loading prior decisions and only selected evidence
- keeps the current result readable when earlier history is unavailable and retries the read
- shows unavailable legacy basis and author without inventing an acceptance

### packages/views/collaboration/execution-panel.test.tsx

- does not mark enabled host execution as unavailable
- distinguishes disabled new execution from retained host execution facts
- retains the unavailable message for a legacy execution view
- opens recorded content lazily, retains the group context and retries the next page without losing the first
- discovers retained inputs behind an empty manual execution snapshot without rendering their bodies before history opens
- omits the empty manual console only after history confirms no retained input
- keeps empty-snapshot history discovery errors visible instead of asserting there is no AI history
- opens a linked run with a legacy result while showing missing input, recording and usage explicitly
- removes the open transcript when a later page confirms access loss
- shows each partial Resume decision while preserving the live paused snapshot
- counts a lane still behind its stop barrier as paused even when its queue was never paused
- recovers a waiting cancellation using its original key without turning it into Stop

### packages/views/collaboration/field-editing.test.tsx

- restores selected labels and false property input after a lost response and recovered refusal
- submits source structure and iteration through their separate versioned actions
- does not offer source editors or request source catalogs for an external collaborator
- creates initial fields once and keeps the source choices after remounting the draft
- allows clearing obsolete initial choices when the source changes
- uses an exact confirmed source account for a members-only actor field
- keeps a property draft during option refresh and does not submit the outer form on property Enter

### packages/views/collaboration/field-inputs.test.tsx

- opens recent parent tasks without searching, selects the UUID, and returns focus without submitting
- shows search results inline, restores browsing on reopen, and can clear the optional parent
- shows an optional stage only for a parent and clears it when the parent is removed
- keeps pending or failed search rows unselectable and retries without losing other input
- distinguishes an empty workspace from an empty search and explains truncated browsing
- uses arrow/Enter selection, keeps composing Enter untouched, and dismisses only the nested picker
- explains the optional parent relationship and stage in Chinese
- keeps the restored selected name when filtered out and tolerates an older response without an identifier

### packages/views/collaboration/focus.test.tsx

- moves focus to the first live target after the focused control is removed
- treats focus inside a hidden container as lost
- leaves focus that is still on a live control alone

### packages/views/collaboration/host-execution.test.tsx

- shows limited cleanup separately from a host execution
- keeps an unknown managed execution primary even after cleanup was attempted
- preserves legacy namespace proof wording

### packages/views/collaboration/knowledge-panel.test.tsx

- requires keyboard confirmation of the displayed version and awaits the exact approval receipt
- resets review confirmation for a changed identity and removes body on source unavailability
- keeps typed decision input and review confirmation across an unchanged refresh but resets them for a new revision
- resets typed decision input when a held refresh returns a different version
- recovers a lost result through the enclosing auxiliary disclosure after remount with the original key and no second POST
- lets an external handler propose from comments to the lawful current project without a private catalog
- allows only content-free target withdrawal and shows a known refusal as a refusal
- keeps unsupported knowledge separate from the surrounding collaboration and reports truncated history
- stops interval polling an unavailable knowledge interface until an explicit refresh
- restarts at the first page when a later page loses its authorization boundary
- picks an actual recorded run as a source without memory or a raw JSON form
- pages all candidate versions with the original page authorization boundary
- keeps a selected source project pinned and requires a new selection if the relationship changes
- unlocks a definitively rejected first proposal for editing and a new explicit request
- keeps a prior lost proposal locked to its original identity after a later rejection and not-found recovery
- shows known success when receipt cleanup fails instead of calling the write uncertain
- unlocks the human draft after storage fails before POST
- checking the panel
- a review decision leaves an open proposal draft intact and only the panel
- recovering an older stored proposal keeps a new draft open while it opens the recovered candidate
- recovering a request from the same source opens it even when it is outside the loaded list page
- recovering a request whose candidate belongs to another source does not open it here
- lets a human remove a stored identity only after a check finds no result, without any request
- reports a selected source that disappears as source-changed and preserves the draft
- reports lost authority at intent preparation as proposal-unavailable and preserves the draft
- submit waits for a refreshing source list instead of reporting a changed source
- reports local validation failure as invalid input, not as an unavailable source
- fails closed with a proposal-unavailable notice when the refreshed list withdraws the grant
- attributes over-byte summary, conclusion and evidence to the field instead of a source failure
- explains an over-byte decision reason instead of the generic invalid message
- does not repeat approval actor/time as an identical latest decision
- renders supplied proposer, approval and latest decision facts without a directory lookup

### packages/views/collaboration/memory-panel.test.tsx

- does not refocus a shared-context anchor on resume but focuses a fresh open
- leaves a dismissed version dialog closed on resume and reopens it for an explicit link
- keeps the editing base and text after a stale save until the user explicitly rebases
- requires explicit review when realtime refreshes the current version during editing
- restores an uncertain update after remount and retries its original key and payload
- confirms restore and keeps closed content read-only while permitting historical disposal
- removes private editor content after access is revoked

### packages/views/collaboration/notifications.test.tsx

- opens an AI group result on its representative run and retains issue links for older notifications
- distinguishes human handoffs and assignments from content-free removal notifications
- describes an assignment to the recipient in Chinese without using the creation form

### packages/views/collaboration/preparation-summary.test.tsx

- lazily shows exact prepared files and failed Git preparation without asserting run completion
- shows missing legacy preparation as not recorded
- renders the recorded stage reason in Chinese and leaves missing facts explicit
- removes preparation content when a later records page denies current access

### packages/views/collaboration/resource-offers.test.tsx

- lets an external owner supply a squad, an organizer adopt it, and each owner separately authorize
- keeps the selected adoption after a stale refusal and requires a fresh selection
- recovers a lost withdrawal using the original request without revoking a grant or stopping
- hides a selected inventory resource once a current refresh no longer discloses it
- drops another owner
- retains closed-issue offer history for its organizer and withdrawal for the supplying owner
- adopts current authority only after explicitly refreshing and reselecting a refused supply
- releases an unsubmitted adoption when the handler loses organizer authority without restoring it later
- releases a hidden supply form on close so the owner can still withdraw their offered resource
- preserves the original pending operation when organizer authority disappears during submission

### packages/views/collaboration/resource-panel.test.tsx

- shows the host boundary and separate resource identities before consent
- disables v6 consent when its resource response lacks all execution metadata
- confirms the real owner
- registers only a selected candidate from the explicit workspace
- commits an authorized Switch proposal only after reviewing the explicit resume requirement
- waits for coherent issue and resource observations before opening consent after another actor adopts

### packages/views/collaboration/run-handoff.test.tsx

- shows current handoff source evidence and explicit unknown reasons without claiming a delivered answer
- does not label a historical handler with another current account
- keeps unsupported handoff information unavailable
- labels a live read

### packages/views/collaboration/run-memory-records.test.tsx

- links recorded base and written versions and distinguishes empty from unrecorded bases

### packages/views/collaboration/run-navigation.test.tsx

- reopens and focuses a run from the same result notification after hiding its anchored records

### packages/views/collaboration/source-bridge.test.tsx

- reuses creation with the explicit source workspace
- keeps the same UUID and comment anchor in shared source detail
- passes a selected run through shared detail and its account link
- opens a scoped workflow after capability revalidation and closes accessibly
- does not expose ordinary assignment when the cached actor is no longer the handler
- does not show write controls without the current write context
- preserves a dirty workflow through background failure but removes it on authoritative denial

### packages/views/collaboration/workflow.test.tsx

- requires a result and basis for text-only delivery and preserves the input across dismissal
- accepts the exact observed submission and shows its author before completion
- returns the exact result with a reason and defaults to its actual author
- records a self-handled result and completes without a transfer form
- shows the current-viewer context beside Participants and opens the scoped list by keyboard in Chinese
- resolves one exact recipient, preserves an unknown transfer, and recovers its original receipt
- discards an in-flight exact lookup after the recipient text changes
- shows a real stale-revision refusal immediately and keeps input for an explicit new submit
- removes the selected participant only after confirmation with the captured revision
- shows the open information request to the person returning it
- says where the issue returns when the request

### packages/views/admin/collaboration/collaboration-memory-controls.test.tsx

- uses current disabled-creator detail to permit historical purge with a validated reason
- restores an uncertain purge and retries the unchanged original even without current detail
- removes a new purge affordance when the current creator has been restored

### packages/views/admin/collaboration/collaboration-page.test.tsx

- reveals the entry only after current complete super-admin access
- self-guards the page against observers without reading any collaboration metadata
- keeps every minimal list page reachable through the injected route builder
- requires a reason and an explicit reachable successor, then records the frozen lifecycle request
- freezes a refused confirmation until the user explicitly closes it
- keeps an overlong multibyte reason editable before any write
- recovers a lost response after remount even when the creator is enabled and detail reads refuse
- keeps an empty lookup read-only and retries only the original payload on an explicit click
- keeps the opened revision and returns focus after stale details force a new confirmation
- clears an expired successor and never auto-selects them when a later refresh restores eligibility
- removes current metadata and the open form on a protected read denial
- drops open actions immediately when the current administrator becomes an observer
- does not restore another administrator

## Existing browser suites

- `e2e/admin-collaboration-switches.spec.ts`: a super administrator closes and reopens new collaboration issues from Settings and members follow
- `e2e/collaboration-account-desktop.spec.ts`: Desktop offers private resources, sends all plus AI mentions and preserves cancelled command history / Desktop creates a private collaboration issue, downloads through native IPC and restores the same issue / Desktop child links, recorded pages and result notifications survive popouts and restart / Desktop authors initial fields and preserves discussion through shared edit controls
- `e2e/collaboration-administration-desktop.spec.ts`: Desktop administers disabled-creator collaborations in the account shell and recovers the retained receipt
- `e2e/collaboration-administration-web.spec.ts`: Web administers disabled-creator collaborations with explicit succession and original receipt recovery
- `e2e/collaboration-boundaries-web.spec.ts`: Gap 5: a source workspace holding a collaboration issue cannot be deleted, open or completed, and nothing is cleaned up / Gap 7: hidden manager actions POSTed by the wrong role are refused and change nothing; a non-participant / Smaller gaps: title/date edits leave attributed records, the exact recipient resolver stays read-only, a source member resolves and reacts without notifications
- `e2e/collaboration-command-history-web.spec.ts`: cancelled waiting commands retain original content after comment changes, refresh and another account reads
- `e2e/collaboration-control-journey-desktop.spec.ts`: 
- `e2e/collaboration-control-journey-web.spec.ts`: 
- `e2e/collaboration-creation-default.spec.ts`: at the fresh-install defaults an ordinary member creates a collaboration issue while execution stays closed / with new collaboration issues closed the member sees the reason and the server refuses the create
- `e2e/collaboration-cross-client-execution.spec.ts`: AC33 Web and actual Electron collaborators execute independent A/B namespaces and one coalesced A follow-up
- `e2e/collaboration-delivery-web.spec.ts`: Web creates one private issue after lost upload/create responses and revokes an open preview through account WS / Web explicitly resubmits rejected creation input without losing files or reviving the rejected key
- `e2e/collaboration-detail-journey.spec.ts`: 
- `e2e/collaboration-editing-web.spec.ts`: Web source creation, field edits and discussion share one authorized issue / A source member outside the collaboration receives only an empty workspace refresh
- `e2e/collaboration-handoff-chain-desktop.spec.ts`: Electron information return shows the original requester and note before restoring responsibility
- `e2e/collaboration-handoff-chain-web.spec.ts`: Web directed handoff chain moves current todos with responsibility and only the creator completes
- `e2e/collaboration-host-installed.spec.ts`: Packaged full Main controls a real native host run and explicitly resumes the shared Agent
- `e2e/collaboration-host-runtime.spec.ts`: host Web and actual Electron share one ordinary Agent, preserve authors, and explicitly resume after scoped Stop
- `e2e/collaboration-host-squad.spec.ts`: a capacity-one host squad merges @A with @S, delegates to its member, joins a later instruction and queues another squad
- `e2e/collaboration-host-switch.spec.ts`: switching a squad to another native runtime waits for confirmed Stop and owner consent, then resumes there without moving local files
- `e2e/collaboration-installed-clients.spec.ts`: cached v0.6.7 CLI cannot bypass paused, unconfirmed-stop or removed-author commands
- `e2e/collaboration-installed-main.spec.ts`: installed Linux full Main and production Web hand off the same issue, preserve authority/history and reject unconfirmed Resume
- `e2e/collaboration-knowledge-deployed.spec.ts`: deployed knowledge startup flag controls one actual Host input and its authorization events
- `e2e/collaboration-knowledge.spec.ts`: Web external comment proposal, exact human review, correction and workspace derivation / Web required memory purge clears open source and managed target material / Web original-key recovery stays locked after ambiguity and recovers by GET after reload / Web definitive unapplied refusal unlocks the draft for a new request
- `e2e/collaboration-lane-close-controls.spec.ts`: 
- `e2e/collaboration-leave-web.spec.ts`: Web Leave issue refuses creator and handler; an owner
- `e2e/collaboration-lifecycle-controls.spec.ts`: 
- `e2e/collaboration-live-revoke-web.spec.ts`: Web withdrawing a grant during a permitted run stops that work, holds only its target, keeps the queued input, and needs proof plus explicit Resume
- `e2e/collaboration-machine-boundary.spec.ts`: C13 an ordinary agent
- `e2e/collaboration-memory-runtime.spec.ts`: two native runtime profiles hand off shared context through the actual run-local CLI
- `e2e/collaboration-memory-web.spec.ts`: shared context keeps concurrent edits, restores versions, and permits historical purge after closure / shared context recovers a lost save response with its original request after reload
- `e2e/collaboration-mention-journeys-desktop.spec.ts`: Desktop mixed H+A+B+S mentions retain one comment and exact queued target associations / Desktop owner revocation after preview refuses only A and original-key replay stays deduplicated / Desktop offline mentions survive a fresh profile and edits notify only newly added people
- `e2e/collaboration-mention-journeys-web.spec.ts`: Web offline mentions survive a fresh context and edits notify only newly added people
- `e2e/collaboration-offline-handoff.spec.ts`: 
- `e2e/collaboration-offline-result-notifications.spec.ts`: 
- `e2e/collaboration-preparation-records-desktop.spec.ts`: Desktop reads synthetic preparation with native 200 percent captures and unchanged proof
- `e2e/collaboration-preparation-records-web.spec.ts`: Web reads synthetic frozen preparation without inferring termination across refresh and viewers
- `e2e/collaboration-preview-web.spec.ts`: trigger preview explains shared and later batches before a suppressed target is sent
- `e2e/collaboration-recipient-picker-web.spec.ts`: Web recipient picker exposes self/recent/search results and preserves the outer dialog on Escape
- `e2e/collaboration-records-web.spec.ts`: Web shares ordinary child references and retained run records through account notification links
- `e2e/collaboration-resource-offers-web.spec.ts`: a private owner offers a resource to an external organizer and independently authorizes it
- `e2e/collaboration-resources-web.spec.ts`: resource owners register, authorize, withdraw and switch while participant management preserves authority
- `e2e/collaboration-retained-attachment.spec.ts`: Web and Electron retain exact comment file bytes and immutable AI input after display removal, deletion, refresh and cross-client restart
- `e2e/collaboration-runtime-handoff-desktop.spec.ts`: Electron deep link opens retained handoff context and honestly marks an unknown recording tail
- `e2e/collaboration-runtime-handoff-web.spec.ts`: ENV-1 live latest shared context preserves the run / Web recipient recalls retained handoff sources and omitted comments, then loses private reads on removal
- `e2e/collaboration-selection-boundary.spec.ts`: AC87 Web and actual Electron hide subtask creation for shared body/comment selections while ordinary children retain it
- `e2e/collaboration-source-surface.spec.ts`: Web and Electron source list, board, timeline and comments preserve external identity and scoped layout order
- `e2e/collaboration-terminal-reopen-web.spec.ts`: Web closed issue refuses @agent and Resume, only the creator reopens, and reopening revives nothing or the stop barrier
- `e2e/collaboration-workbench-web.spec.ts`: Web workbench keeps account scope through comments, uncertain cancellation, handoff and revocation / Web stale lifecycle conflict immediately reports refusal and preserves input for explicit retry

## Caveats / Not Found

Inventory reads test source; tests were not run during research. Recommendations and critical gaps are in assessment-b-technical.md.
