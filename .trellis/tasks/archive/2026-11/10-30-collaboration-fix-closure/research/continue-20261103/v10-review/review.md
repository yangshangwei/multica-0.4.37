# Independent current-byte v10 review

Verdict: **pass-with-notes** for the private executable/configuration scope, with **0 open executable blockers**. This is not a full-check acceptance or runtime-clearance record.

## Findings (fixed)

None. Product files, registered launcher/configuration, clearance, claim and old run evidence were read-only. Only this new review directory was written.

## Findings (not fixed)

- The later v10 whole command completed with exit **2** at `2026-11-02T21:12:33Z`. The original browser report contains **39 expected, 36 unexpected, 364 skipped, 0 flaky**, with one worker. Root owns diagnosis and any newly numbered full replay; these failures remain failed.
- Candidate HEAD is still `118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9`. The only current tracked diff is Next’s generated route-types import from `.next/dev/types/routes.d.ts` to `.next/types/routes.d.ts`; it was preserved under root’s instruction.
- The preparation’s `prepared-not-run` statement is historical. Existing later execution claim, hash-bound clearance, complete run and raw browser report supersede it for current state. Root owns metadata reconciliation.

## Verification

- Current launcher, source environment, both adapters and completed run/clearance bindings match exactly. The canonical environment and full log match their run hashes. All 17 clearance reference hashes match current files.
- The four inherited data parser/renderer functions are AST-identical to v9. The API pool/test boundary, publisher-key removal in only ordinary Go tests, private HOME/cache, actual recursive package permits, compile admission and original agent CLI guard are retained.
- `check-launcher-v10-preflight.py`: **32 pass, 0 fail/error/skip**, executed afresh with only private mocks and credential-free temporary Make/Bash fixtures. Original v10 runtime paths were not executed or changed.
- Four reviewed Python files parse and compile. Product lint/typecheck were not rerun in this bounded review; root owns repository verification. This review changes no product or executable file.

## Root handoff

Preserve every v10 artifact. Diagnose the actual failure set, then bind any replay to fresh numbered paths and independently reviewed code/configuration. Before fixture writes, verify actual API/database/production-Web/native-renderer provenance and exclusive ownership. Password7 and original E3–E6 remain separate gates.

Machine bindings, reference checks and actual run stats are in [review.json](review.json). The fresh fake log and syntax result are in [verification.json](verification.json) and [fake-preflight.log](fake-preflight.log).
