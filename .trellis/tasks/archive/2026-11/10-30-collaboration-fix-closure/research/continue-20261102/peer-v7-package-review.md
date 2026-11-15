# Peer v7 package/isolation patch independent review

Verdict: **changes-required**, one blocking package-resolution mismatch. The supplied patch is not approved for integration yet; this report is not final adapter/launcher clearance.

## Verified behavior

The proposed delta preserves selected v6 API-pool removal and adds canonical package/ledger checks, explicit mode1 Go/SQL/evidence context, and correct consumption of Go test value flags plus aliases. Canonical/default/relative selections are resolved under both -C forms; wrong directories, multiple/wildcard selectors and missing/malformed ledger packages refuse.

Independent fake executions under deny-network: package-binding **30/30**, historical isolation **14/14**, pnpm reporting **5/5** pass. Real cached Go probes on a temporary stdlib-only module confirmed seven list scenarios and five compile-only cases for -bench/-test.bench/-test.run/-benchtime/-test.benchtime. Every actual test command included an initial -c and the fixture had no TestMain; no test body, DB, account or Provider ran.

## PV7-P1 — list and test can resolve different packages (blocking)

The recursive resolver invokes `go list` with the selected package and -C context, but drops -modfile while preserving it in the eventual test command. Actual Go in a temporary module resolves `example/first` normally and `alternate/first` with alternate.mod. The exact peer wrapper accepted a ledger for example/first, queried the normal module, then forwarded test with alternate.mod. Test execution was intercepted and did not run.

Fail closed for package-resolution modifiers not supported by the private caller, including -modfile/-overlay and their equals forms, or forward their exact relevant list/build configuration. Add a fake refusal contract and retain the real Go-list oracle. This is a bounded helper correction; no production module/database change is needed.

## Scope limits

- scripts/test-go.sh currently uses standard timeout/race/p/parallel flags; it does not require -modfile/-overlay. Their unsupported status must be enforced rather than accepted with a mismatching resolver.
- The historical outer-mode value splitter is unchanged; general mode1 benchmark/test-alias value forwarding and arbitrary module/overlay options are not certified.
- Current selected adapter-v6 has evolved to `908740910811956d4586c05e19ee6fcb40cfd56acf7bfbd0933bfed46e446a5d`. Apply an independently accepted bounded patch onto the current owned source; preserve newer environment/HOME isolation rather than replacing its whole file.
- Make/source-environment and HOME launcher fixes are separate. No clearance was written.

## Binding

| Artifact | SHA-256 |
|---|---|
| peer Go | `eafed32e38ea69de0f73272df5c9cc817ec0741726b54fc19d27466742f57076` |
| peer pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |
| bounded patch | `843268bd093429fe7a1488b2e62b2a0ba63204faf084241f1c552122bf5db465` |
| peer package tests | `ffafa89ada008bd1829ec74e3dbe58dd1058fc0fbdef95d99be61156af753d1a` |

The isolated implementation directory is `/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v7-knowledge-advance-20261102`; its Go source matches the retained final-source evidence. Raw fake output, network-denied Go commands, artifact hashes and the captured -modfile mismatch are retained in peer-v7-package-review.json.

