# Independent required-fix recheck

Verdict: **pass-with-notes for the RKI-1–RKI-5 code corrections** at tip `ff3532ee1` (launcher product `62bd98eb9`, Go framework product `4229e2d18`). The original changes-required review and all historical failures remain unchanged. Product files were read-only; no child agent, database command, migration, installed Agent CLI or Provider call ran.

## Disposition

| Item | Independent result |
| --- | --- |
| RKI-1 missing executed trials | Closed. Successful executor exit now requires the requested newly produced trial labels. Exit0/no-trial and exit0/partial-success become durable invocation-level failure with missing labels, without fabricated replacement trials. Normal first-incomplete stopping does not demand the intentionally unstarted later slots. A failed invocation blocks the next call before executable/account work; consumed launches still use max(public, private). |
| RKI-2 effective configuration selectors | Closed within the registered direct-host configuration boundary. Environment selector hashes include absence; alternate `CLAUDE_CONFIG_DIR` selection and tree bytes, endpoint/provider/profile selectors and explicitly selected identity/config files participate. Scratch changes to root, root contents, endpoint and profile-file contents all change the fingerprint. Only digests enter the record; no raw selector or credential content was observed in the probe output. |
| RKI-3 raw evidence persistence | Closed. Capture sync and close errors remain distinct stages; final launch publication failure changes the wrapper exit without relabeling the Provider execution result. The lifecycle checks explicit publication failures, unreadable/unfinalized capture, independently observed wrapper Wait exit and missing host summary. Thus complete-looking cached metadata cannot hide a final-write failure even if the stderr marker is absent. Known intentional H1 seed Stop accepts only its unfinalized partial state, never an explicit evidence failure or completed-result claim. |
| RKI-4 exact answer formats | Closed. H1/H2 match the raw outcome body using whole-answer anchors and explicit line separators; a single terminal LF/CRLF is allowed, extra prose/blank lines are rejected. Scenario identity, semantic values, H2b source exposure and stopped-run absence criteria remain unchanged. |
| RKI-5 registration hashes | Current dirty registration refresh matches all three computed digests: Go framework, launcher and regression script. Its `runnable=false` and empty final source ID are correct. The final clean candidate still needs its own recorded registration commit/fingerprints. |

## Checks performed here

| Command/check | Actual result |
| --- | --- |
| `python3 .../check_launcher.py` | exit0;37 OK,0 BAD; scratch fake executors only |
| Explicit Go-file contract selection (`rk1_evidence_publication_test.go`, `rk1_evidence_format_test.go`, `rk1_evidence_contract_unit_test.go`), guarded, DB/real gate unset | exit0;11 named test/subtest passes,0 fail/skip; no package TestMain |
| Same explicit file selection with `-race -count=2` | exit0;22 named passes,0 fail/skip |
| `go -C server vet -tags=agentintegration ./cmd/server` | exit0; static/compile only |
| Independent scratch configuration facts | All six checks pass: selection, contents, endpoint, selected profile bytes, frozen absence, no raw values |
| Author RKI3/RKI4 source/log manifest comparison | No mismatches. The author’s permitted fake Host/router fault proof remains18 normal/18 race passes; this reviewer only checked its source/log hashes and result boundaries, not its DB execution. |
| Current source hashes versus review-start manifest | Unchanged |

The Go contracts exercise failed sync/close and a final-record write that leaves complete-looking bytes, then independently fail the wrapper/summary comparison; Provider exit0 is retained while the sample becomes incomplete. The canonical missing-summary and unfinalized checks fail closed. Fault plans are accepted only for fake execution. No error text containing filesystem/credential material is published as a stage reason.

## Notes and open gates

1. **RKI-6 remains open:** complete same-candidate fake/race acceptance has not passed yet. The retained intermediate runtime-fixed failure `fake-proof/runtime-fixed-race.json` is still exit1 (E0a missing handoff/recording event); evidence-fault targeted proof does not replace the all-scenario proof or explain away that failure. The coordinator must finish product integration and run the correct final candidate in its own newly allocated/migrated DB after heavy tests quiesce.
2. Provider account identity/usage and native platform behavior remain outside this code delta. The configuration digest guard does not by itself prove account authority, quota source or AC43. Final execution must record the actual selected environment/account/configuration and reject unsupported runtime selectors rather than assume this finite supported selector list covers arbitrary future Provider configuration.
3. The registration refresh is not yet a final clean source binding. Preserve empty `source_id` and `runnable=false` until knowledge/delivery integration, independent final review, final framework/CLI/config/artifact fingerprints and exclusive gateway window are satisfied. No paid sample was run by this review.

Commands, exact reviewed code hashes, timestamps and log SHA-256 values are in `recheck-review.json` and `recheck-*.json`. No product patch or commit was made by this reviewer. This delta pass-with-notes permits proceeding with integration and fake acceptance; it is not authorization or proof for paid execution.
