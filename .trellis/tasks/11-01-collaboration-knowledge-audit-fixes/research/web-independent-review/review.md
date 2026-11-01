# Independent knowledge web review

Product under review: `ed00265253a83ba04fcec73a9285275d26708dcd`, compared with `4e3b61500ed0afbc12cf4c79b1c6aea866778d0e`. Reviewer: native Codex `/root/knowledge_web_review`, separate from all implementers. Product files remained read-only; only this report directory was written. No database, migration, Provider or installed Agent CLI command ran.

Verdict: **pass-with-notes**. No blocking or newly confirmed nonblocking product defect found in the web audit repairs. Independent Core/views typecheck and lint both exited 0.

## Coverage

| Audit item | Independent assessment |
| --- | --- |
| M4 review input after refresh/erasure | The decision component remains mounted without a server candidate while its body and controls disappear. Only the canonical candidate/revision/availability/version/hash tuple resets the human decision state. No copied private body is retained. |
| M5 draft ownership and recovery | A matching in-memory proposal closes only its own form. Older recovery and decisions preserve a new draft. Unknown page membership is checked through a fresh authorized Core detail; restricted/failed detail cannot infer membership. Exact receipt matching and current scope fences remain in Core. |
| L8 submit during list refresh | Source resolution, current grant availability and local schema failures have distinct attribution. Joining a current list refresh is bounded by the existing refresh interval; drafts survive every failure. |
| L9 UTF-8 field limits | UI counts the same exported byte constants as schemas, keeps overlong input visible with accessible errors, and blocks before source resolution/write. Approval does not submit the optional decision reason. |
| L10 expired later page | Core still erases expired content and refuses the old boundary. The view resets only a later-page AbortError to the root query, without renewing old page content or unmounting a human draft. |
| L11 unsupported knowledge | Explicit CollaborationUnavailableError disables interval refetch; manual refresh/focus/reconnect retain their deliberate recheck routes. |
| L12 unresolved entries | Local removal is explicit, refuses another/obsolete viewer scope, sends no request, and does not erase the in-memory dispatch uncertainty. Own unresolved intents cannot be dismissed; captions state that removal does not cancel the server operation. |
| L13 terminology | Knowledge labels follow 工作空间, 任务 and 协作上下文; no locale keys or placeholders were lost. |

## Verification boundary

Independent targeted views/locale tests: 100 passed, zero fail/skip. Independent Core schema/actions/cache/lease tests: 41 passed, zero fail/skip. Core/views typecheck and lint exited 0; lint reports 27 existing unrelated warnings and zero errors. Exact commands/log hashes and source-tree binding are recorded in the accompanying JSON files. The implementer evidence manifest's file and log hashes were checked with no mismatch.

Full Core/Views lane suites, merged knowledge interaction review, final candidate make check/Web/Electron and real-model/release gates are separate coordinator responsibilities. Branch-local review does not close those gates.

## Integration notes

- **WV-1:** The coordinator’s full standalone Views run currently reports the known calendar-dependent admin execution filter failure. The affected test file is byte-identical to the base and was not changed by this lane; knowledge integration already has the `9c056ad68` fixed-clock regression. Preserve the failed attempt, absorb the existing fix and complete the full integrated suite. This is not a newly introduced web defect, and this review does not label the full standalone Views suite passed.
- **WV-2:** Final combined source and the mandatory full suites remain open gates. This pass-with-notes authorizes only the reviewed product scope and does not establish real-model or platform acceptance.

No product fix was applied by this reviewer.
