# Independent initialization kernel review

Date: 2026-11-27. Scope: server/internal/materialrepo only, with task design/research and owning specs. No product code was changed by this reviewer while the implementation owner was active.

## Findings and disposition

1. P2 — Generated joined paths could exceed the snapshot path limit even when root_path itself was valid (catalog.go BuildCatalog). Reported to root; implementation owner added joined-path validation and exact-boundary/one-over regression. Fixed by implementation owner; exact boundary/one-over regression and final race suite passed in independent recheck.
2. P2 — A valid before snapshot near MaxEntries/MaxSnapshotBytes could produce a Candidate whose completed tree necessarily exceeded the same validation limits (inspect.go candidate construction). Reported to root/owner; must validate projected post-state before returning a candidate, not discover this after remote publication. Fixed by implementation owner: Inspect validates projectSnapshot before returning the candidate; publication/merge verification shares the projection. Exact-budget output passes and one-entry/one-byte overflow refuses. Independent race test recheck passed.
3. P2 — Shipped JSON Schema material/source reference patterns accept .GIT/config and leading/trailing whitespace segments while kernel validPath rejects them. Reproduced with Node RegExp against the actual schema asset. These assets are format documentation, not current authorization enforcement, but this inconsistency would mark unusable paths structurally valid. Reported for aligned regex and concrete pattern probes. Fixed by implementation owner: patterns now refuse case-folded .git, segment-edge Unicode whitespace and C0/C1 controls; Go and explicit Node regex tests share a path corpus. All three real ECMAScript regex tests passed in independent recheck.

## Verified design properties

- Exactly20 embedded base files: three entries, six schemas, eleven templates; no business versions, fake requirements, .gitkeep or fabricated acceptance results.
- Existing README and unmanaged ordinary data are excluded from writes and checked for byte/mode preservation. Custom AGENTS/templates/schema conflict; known manifest semantic formatting is accepted without altering observed bytes.
- Candidate validation regenerates the expected missing-file plan and compares its digest/files; mutating and rehashing only the supplied files cannot bypass the catalog.
- Snapshot trust remains explicitly external. A hash is not a signature; repository identity, task/runtime provenance, human approval, branch identity, actual merge and native gates remain caller obligations.
- Strict single-document YAML rejects unknown fields/versions, duplicates, aliases/anchors, custom tags, non-string identities, malformed/oversized/deep input. Nil or incomplete trees/read bodies do not become absence or readiness.
- Publication diff requires complete exact added-file set; merged proof retains the prior tree and required bytes. No ordinary PRD allowlist was widened.

## Known compatibility constraints (not changed)

The package rejects executable/symlink/submodule modes anywhere in the complete repository, including outside a nested material root. It also rejects unrelated concurrent target changes via exact-tree preservation. Both are documented conservative refusals, not data loss. Full product integration must explicitly retain these limits or design opaque unchanged Git-entry preservation and preflight reinspection before promising arbitrary existing-code-repo support.

No runtime/DB/API/UI or maintenance-upgrade implementation is claimed by this kernel. Full task remains incomplete.

## Final independent verification

- Baseline go test ./internal/materialrepo -count=1 -v: pass.
- Final go test ./internal/materialrepo -race -count=1 after all three fixes: pass (3.180s).
- node --test server/internal/materialrepo/schema-path.test.mjs: pass, 3 tests, 0 skips.
- Final go vet ./internal/materialrepo: pass.
- Go type checking: package compiled successfully through final go test; no TypeScript files changed, so TS typecheck is not applicable to this slice.
- gofmt -l internal/materialrepo/*.go: empty.
- Initial Node regex probe reproduced finding3; final schema-path tests verify the corrected refusal corpus.
- No network, database, real account or frontend tests were used by this review.

All three actionable findings are resolved. No remaining blocking code finding in the independent kernel slice. The documented compatibility limits and missing product integration remain; this report does not certify full initialization workflow completion.
