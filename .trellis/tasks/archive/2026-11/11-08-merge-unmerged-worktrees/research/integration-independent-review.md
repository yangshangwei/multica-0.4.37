# Independent integration review

Reviewed at: 2026-11-08T10:04:37.506155+08:00

Integration: `/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`. Fixed baseline: `e541cfb436fed9c6733174479718b651e2cf030d`.
Scope: 45 changed/new product and test files (UI37, Go8, including3 untracked Go files). Combined path/content SHA256: `4d2bbfb7d8fb6af2fcecd184f33981ded58ffe76ee1ae7ac1d325ecb12cbc58a`. This is a review of the working delta at this instant, not approval of a later unverified commit.

## Findings (fixed)

None. This dispatch was report-only; no product files, source worktrees, indexes or refs were modified.

## Findings (not fixed)

### P2 — Continuation output can alias retained prior evidence

File: `server/cmd/server/rk1k_codex_continuation_test.go:68` (history validation) and `:107` (launch namespace validation).

`rk1kValidateContinuationHistory` only compares the new output path to original/retry output as raw strings. Its same-file checks cover ledgers/registration files, not output directories. Thus `current.Output = original.Output + "/."`, or a symlink to the original output directory, passes the history validator. `rk1kValidateContinuationLedger` then treats the same physical directory as the new namespace because `filepath.Rel` is lexical. A real continuation can write its E0b/summary artifacts into retained historical output even though the continuation is supposed to preserve that evidence.

Confirmed without a database or real provider: a private Go overlay appended `TestTakeoverContinuationOutputAliasProbe` to the existing pure fixture/control suite and replaced only TestMain setup in the compiler overlay. The original validators and reserve function ran unchanged. Both the dot-path and symlink cases successfully reserved a new ledger entry while `EvalSymlinks` proved their output resolves to the original evidence directory. `MULTICA_RUN_REAL_AGENT_SMOKE=0`, DATABASE_URL unset, repository ambient CLI guard used; no repo edits.

Probe evidence: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/merge-continuation-alias-review-hegqeusb/probe3.log`, with `overlay.json` and temporary source inputs in that directory. Earlier probe attempts are also retained: attempt1 failed to compile because removing TestMain made4 imports unused; attempt2's assertion neglected macOS /var→/private/var canonicalization. Attempt3 resolves both paths and confirms the defect.

Recommendation: bind original, retry and current output to canonical effective namespaces (resolve symlinked existing ancestors when the leaf is not created yet), reject aliases to either historical output, and retain the existing ledger/file identity and budget checks. Prefer distinct non-overlapping output namespaces or an explicitly freshly allocated owned directory. Add direct dot/symlink regression controls; do not weaken other historical predicates. Main coordinator has been notified and owns the fix.

## Behavior and standards checked

- Workflow success notification is a separate optional `onApplied`, invoked only for the existing successful receipt set and a successful status. Cancellation still goes through `onDismiss`; unknown/rejected results preserve original-intent recovery. Source bridge retains embedded modal, request busy state and focus behavior.
- Restricted rich-content placeholders remain text/SVG only: no `href`, `src`, resolver lookup or downloadable resource is introduced. User-provided labels remain React text. Empty alt/filename cases are covered.
- Mixed collaboration batch selections disable status/priority/assignee/delete up front and retain handler guards; iteration assignment stays hidden for these selections. Clear selection remains usable and associated with the explanation.
- Existing reaction accessible names/descriptions are unchanged. No blanket replacement of older workflow/test files occurred.
- Input border/fill split is coordinated across affected controls/read-only surfaces. Remaining mobile input tokens are independent; chart `--color-input` occurrences are locally defined by ChartContainer series config and are not missed global fill consumers.
- Project headings now use existing role tokens. Shared UI/core/views package boundaries and locale parity remain intact; no dependency manifests or runtime/backend production files changed.
- Real Knowledge/context entry points retain build tags and early explicit opt-in gates before provider work. Continuation caps15 new/17 total preserve two historical consumed attempts. E5 persists the finished canary-failure record before stopping remaining trials; the real entry requires exactly one selected scenario.
- Source-specific dedupe/disposition records and full preservation evidence were consulted. This review does not re-certify every source archive or settle unfinished remaining-source decisions owned by other agents.

## Verification

- Independently ran `pnpm exec turbo run lint typecheck --filter=@multica/ui --filter=@multica/views --concurrency=1`: PASS,6/6 tasks;4 cached and2 executed. Log `/tmp/merge-independent-static-review.log`. Existing lint warnings remain; no lint errors.
- Independently ran `git diff --check`: PASS.
- Pure continuation alias probe: reproduced both admitted aliases; it is a bug demonstration, not a product acceptance test.
- Reviewed implementation evidence: Views54 tests, stylesheet/text37, translations66 pass; Go context new4 normal/race pass; Knowledge8 deterministic top-level suites/98 assertions normal/race pass. These product suites were not unnecessarily rerun in this report-only pass.
- Full make check, fixed-commit context cleanliness replay, browser visual review (especially input/ring contrast, narrow batch toolbar and resource placeholders), and restored root-worktree validation are still coordinator gates. No full-gate or visual approval is claimed.

## Spec synchronization

After fixing the output alias guard, update the Knowledge experiment contract to record canonical output namespace separation. Frontend documentation should distinguish input boundary/fill tokens and success-only `onApplied` from dismissal; keep its visual/accessibility claims limited to measured states.

## Decision

REQUEST CHANGES for the confirmed output-evidence alias issue. No other confirmed code regression was found in this45-file delta. Re-review the narrow fix and rerun its deterministic controls, then proceed to the coordinator's complete verification before landing.


## Follow-up: output namespace repair approved — 2026-11-08T10:12:03.979182+08:00

The original P2 above is retained as historical evidence and is now addressed by the narrow repair. No product files were edited by this reviewer.

Reviewed final files:

- `rk1k_codex_continuation_test.go`: SHA256 `4cfa83adbe754ca12352ea9635601d534a4f91ef642dbedb19f47d7dad3a4080`.
- `rk1k_codex_continuation_controls_test.go`: SHA256 `3cf5c0cc133b8ddb411f6e0c13d4c71168b3b8d7841d0b3d9ca28aecc6ee24db`.

The output helper rejects ambiguous parent traversal, resolves existing symlinked ancestors before appending not-yet-created suffixes, and rejects dangling links/non-directory ancestors. Bidirectional overlap checks reject equal/ancestor/descendant roots. Existing-ancestor `os.SameFile` checks cover macOS case-insensitive aliases even when EvalSymlinks retains the input spelling. The launch validator now resolves the actual launch path as well as the registered output root, so an inner `provider` symlink cannot escape into prior evidence. Raw registration and launch strings remain unchanged for original-request identity and evidence binding.

Tests are independent of incidental failure: root aliases must fail in the history validator itself; nested-launch tests first prove the distinct output roots pass history validation, then require reservation refusal. Each refusal preserves the original current-ledger hash. Positive tests retain uncreated disjoint directories, similarly prefixed names, and legitimate symlinked fresh ancestors.

Independent validation used a fresh private compiler overlay with database-free TestMain setup, the exact final validator/helper bytes, and the actual final control suite. All14 original/retry root alias/overlap rejections,3 positive fresh namespace cases,2 nested-launch escapes and4 additional pure helper probes passed. The macOS case-alias probes executed with inode identity proof and did not skip. No database or real provider was invoked; real-agent opt-in remained0 and the ambient CLI guard was used. Evidence: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/merge-continuation-alias-followup-m4rug_e_/final-probe.log`, exact inputs in `final-input-hashes.json`. `gofmt -l` and `git diff --check` also passed.

**Updated code review decision: APPROVE for the reviewed code delta and these exact repair hashes.** No remaining confirmed code finding. This does not replace the final clean-SHA full make check, context source-cleanliness replay, final source disposition completeness, or root restoration checks. The coordinator reports UI visual checks on actual shared controls passed in both themes at375/720 with no overflow and preserved fill/geometry; this reviewer did not independently repeat that browser session.
