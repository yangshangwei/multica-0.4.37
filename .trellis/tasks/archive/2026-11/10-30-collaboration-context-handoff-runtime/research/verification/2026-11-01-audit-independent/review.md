# Runtime audit independent review

Verdict: **pass-with-notes** for product `7627916c8bff4fdd9c2b1287a8cfc3f033a9dc85`. No blocking product defect found in the nine audit repairs or five recall follow-ups. This is branch-local review, not final integrated acceptance. Product code and other writers’ pending changes were left untouched. No database, migration or Provider operation was run.

## Findings (fixed)

None. The parent granted report-directory ownership only.

## Findings (not fixed)

1. **RV-1 — low, Core/Go malformed-response mismatch.** `packages/core/collaboration/run-handoff-schema.ts:50` accepts a snapshot with `memory.base_version`, whereas `server/pkg/protocol/collaboration_handoff.go:370` refuses this live-only field in snapshots. A probe against the actual Core parser produced the expected red assertion; source and raw failure are retained here. Normal server handoff reads are live, and the snapshot component ignores `base_version`, so this has no demonstrated authority or normal-response effect. Recommend adding the mode/base relationship to the existing refine and a negative case to its canonical test. Not fixed because product writes are outside reviewer ownership. The evidence source is `.source.txt`, so it will not become a failing active repository test.
2. **RV-2 — integration readiness pending.** The author’s `service-full-final-summary.json` still says `running` at report time. Its existing command must finish with an actual exit code, test inventory and log hash before that required gate can close. No duplicate DB suite was started.
3. Existing frontend lint reports 27 warnings and zero errors in unrelated files; reviewed files introduce none of those warnings. They were left outside this repair scope.

## Review coverage

- REC-1/2/3/4: the sender retries the original pending chunk, fallback rebinding keeps stream/sequence/offset/messages, refused content closes with its own reason, and a healthy contiguous outcome tail is checked/deduplicated by the server. Local loss/refusal cannot be hidden by a tail. Outcome receipt and covered chunk acknowledgements share one ledger persistence. Original Stop/proof recovery remains independent and bounded.
- ENV-1/2/4: live latest memory is separate from the recorded launch base; snapshots keep their original encoding and base semantics. Admitted delegation remains whole through redaction growth. Only newly marked continuation keys claim recorded wake-cause completeness; legacy continuations remain unknown.
- RECALL-1/2: member display labels use the existing human content-account projection, without granting capability; agents remain attributable through the roster. The CLI preserves optional names. Paging/revision/budget/content/trust contracts are documented with the commands and source map.
- Five recall follow-ups: wrong source references return 404; source run IDs are canonical before HTTP; each cursor family is checked for progress; a final chunk cannot claim truncation; knowledge declaration is confined to the smoke claim route/method.
- Preview follow-up: paging controls precede comment/chunk/source text, negotiated guidance uses a 16 KiB comment budget, and it still requires reading saved full output rather than treating a visible cursor as proof of content exposure.

## Verification

All commands, exact source trees, raw logs and SHA-256 values are in `review.json`.

| Check | Result |
| --- | --- |
| Guarded protocol/CLI/recording targeted tests | 130 test/subtest passes; 0 fail/skip; 3 package passes |
| Guarded checkpoint/outcome/lifecycle `-race -count=2` | 54 test/subtest passes; 0 fail/skip |
| Go vet, affected protocol/CLI/host packages | exit 0 |
| Core + Views lint | exit 0; 0 errors, 27 unrelated warnings |
| Core + Views typecheck | exit 0 |
| Canonical Core handoff suite | 15/15 pass |
| Canonical Views handoff suite | 4/4 pass |
| New malformed snapshot probe | expected red; confirms RV-1 |
| git diff --check | exit 0 |

The author’s source-bound completed targeted/race/handler/tagged/TS summaries have matching referenced log hashes. Its build/vet summaries predate the product commit and bind a working diff rather than a source SHA; the independent vet above is current. The author’s full service run is still pending and is not counted as passed.

## Handoff

The coordinator may treat this as an independent pass-with-notes on the reviewed product SHA. Keep the full-suite completion, preserved spec/task commits and combined candidate verification as separate gates. In particular do not treat these branch results as RK1, Windows, installed-package, multi-account or final merged-source acceptance.
