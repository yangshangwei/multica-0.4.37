# Native comment deletion causal review — closed with evidence

Current evidence does not yet justify calling the DELETE-wait failure fixed. The full12case diagnostic replay passed, but the added before-open/confirmation screenshots and state capture introduce asynchronous delays. The new toBeEnabled assertion is largely redundant with Playwright click actionability. Neither establishes why the original confirmation produced no DELETE request.

## Facts

Original final3 PATCH committed200 and stored comment revision2 with the edited content; no DELETE reached the API. The original native failure attachment was overwritten by another participant's capture, leaving no usable author form/input-event state. The diagnostic replay records enabled Confirm deletion and a focused delete form, but describes only that passing execution.

## Source assessment

- executeCollaborationCommentChange does not optimistically patch only comment text; it retires the request and invalidates Query. The rendered comment content/revision come from the same comment object. Waiting for edited text therefore normally advances the revision as well. A later stale response could still change it, but would need evidence from the failed author state.
- CommentCard sets editor=null in mutation onSuccess, while the mutation remains pending until success dispatch. The visible edit/delete/confirm controls include send.isPending in disabled state. Its submitting.current is reset in the mutateAsync finally microtask. There is no demonstrated path from an enabled native confirmation click to a still-true submitting guard; artificially delaying the outer promise alone would not prove the production race.
- recoverFocus uses setTimeout/requestAnimationFrame, only when focus is lost, and preventScroll:true. It should not steal focus from a live button or move layout. No evidence currently ties it to the missing request.
- Base UI renders a native type=submit button and the form dispatches submit normally. Two native applications/windows plus unasserted activation make lost background input a plausible hypothesis, not an established cause.

## Required next evidence

Use the author's targeted original-timing replay with a passive recorder installed before the journey: pointerdown/up/click/submit at capture and bubble phases; target/comment identity; disabled/aria-disabled/isConnected; focused element, document focus/visibility; DOM/geometry changes around the confirmation; network/pageerror events. Avoid screenshots, waits or awaited IPC calls between opening and confirming deletion. Record native-window blur/focus events independently before the flow if feasible.

Classify where the action stops: no DOMclick means native input/readiness; click without submit means native form/event/button behavior; submit without request means component guard/authority; request without response means transport/server. A separately controlled focus-loss or stale-render sequence can then verify the suspected boundary. A passing repeated run without a diagnosed cause remains diagnostic evidence, not a repair.

No product or test source edited by this reviewer. No runtime/API/browser test executed. Keep original failure open until causal evidence supports a minimal correction and full12consumer regression.


## Causal reproduction obtained (passive5b)

The bounded original-timing passive replay completed2FAIL/3PASS. Both failed author states show the expected edited comment, visible “This comment changed”/Use latest version, enabled Use latest and disabled Confirm deletion. Passive input events prove Delete comment was clicked once and the delete form received focus, but zero Confirm deletion click events and no DELETE request occurred. Only the preceding PATCH was recorded. Playwright reported an initially unstable action, stale-alert interception, then disabled control. Independent content-free extraction is comment-delete-diagnosis/independent-causal-summary.json. The earlier missing-author-state limitation is now resolved by a matching repeated failure, not inferred from a successful screenshot.

The correct classification is a fixture missing the visible version-conflict workflow, not a lost server response or proven submitting-ref/native-focus product defect. The new branch first performs a non-writing5s trial click; only if that fails and the actual Use latest control is visible does it verify the exact edited text, require disabled confirmation, click the real version-adoption action, and require that action disappear and confirmation enable. The actual DELETE click still happens once, followed by unchanged HTTP200/tombstone/history checks. No direct revision override or retrying business-write loop is introduced.

Source review of that bounded branch passes. Final complete12consumer regression must pass and retain at least one observed adoption-branch execution (or a separate controlled stale case) before causal closure is complete. A future stale transition after trial remains a visible failure, not a silent skip. Diagnostic-only12pass remains historical and is not retroactively a fix proof.


## Final closure

Independent review verified the controlled stale-confirmation probe uses two actual HTTP PATCHes after opening deletion: revision2→3 then a new revision restoring the exact expected text. It asserts visible Use latest and disabled confirmation before recovery, then requires deletion_used_latest=true. The full original journey passed1/1 in21.927s with immutable cancellation/history/account/dispatch assertions retained. This exercises the repaired branch causally; it does not rely on screenshot delays.

Verified terminal delete-final12/results.json: **12PASS,0FAIL,0SKIP,0flaky,79.491s**, covering the complete native11case spec and Web command-history spec. Final delivery removes successful pre-action screenshots/passive collector; only failure captures remain. HelperSHA8fa9bac70a0034cf3bdbd350198e6dbcf0394715e85f283db47591c0d8c2382d; native specSHA64fef7b17066ca1ae8cebee212c56e5ff74b81126aa6361453a386255db11bdf.

The original full-run failure and natural2/5 passive failures remain unchanged. This is a proven fixture adaptation to the product's explicit stale-version confirmation contract. No product runtime code changed. Under accepted impacted-complete-suite verification, this local defect is closed; remaining whole-goal source/cleanup/report audit is root-owned.
