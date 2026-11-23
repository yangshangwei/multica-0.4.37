# Independent workspace identity review

**Preliminary static layout pass.** No blocking layout issue was found in the observed scope for `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103` at `304f74dd3ba4ddea862c94917c5bb0770355a351`. The completed installation freeze and actual build/SSR remain unverified.

The reviewer authored the read-only stdlib collector. After the reviewer's exec transport failed, the coordinator executed it once at 22:05 UTC on 2026-11-02, exit **0**, and returned bounded facts from the saved [observation](collector-observation.json). Its SHA-256 is `7f763147f0fb912406bbcddfe580043ee9198d7b1ef26547d30e9211486bf6d2` (207,459 bytes). The executed source SHA-256 is `aa6f28e3cc6c559ad15191ea0e8023b6e176de426b366dc19b3c5d0eec26289e`; the coordinator verified [the preserved revision](collect_workspace_identity.executed.py) matches that exact hash and rechecked the unchanged observation hash. The original procedure file was moved to the preserved path. The later self-reference patch was unexecuted and removed. No rerun occurred.

## Findings (fixed)

None in the candidate. Only the collector and review artifacts were written in the assigned output directory.

## Findings (not fixed)

- The owner freeze receipt was not verified. The collector searched task/coordinator/v11; it did not search the candidate's private installation directory. The receipt may exist there. Completed Electron runtime/cache freeze needs owner evidence.
- A fresh production build and actual SSR auth graph remain unverified. `apps/web/.next` is absent. The v10 registered/read module mismatch is not proven resolved by matching source files.
- The executed collector used directory lookup and inspected export targets without executing Node conditions or an explicit package self-reference branch. All returned auth probes reached the correct owned root; this remains static evidence.
- Its YAML counter reported 1,462 multiline snapshot entries and omitted inline `: {}` entries. Do not equate that count with the installer's 2,031 claim. The committed and installed locks have identical bytes.

## Verification

- **Workspace origins:** 22/22 declared dependencies and 32/32 discovered internal links resolve inside the owned candidate and match their expected package roots. No own node_modules directory is a symlink; no source symlink/overlay or unresolved donor workspace source was reported.
- **Auth:** Web, CoreProvider and shared views resolve `@multica/core/auth` inside the owned core package. Every observed `auth/index.ts` consumer shares device/inode `16777244/39590398`; every `auth/store.ts` consumer shares `16777244/39590403`. Exact source hashes are in [the JSON verdict](verdict.json).
- **Installed profile:** all three expected configuration hashes match. Both locks have SHA-256 `bb0c3d632b568a991422a25b335cf31ccbda55206cd138aed7bba63f169af4ac` and 11 importers. Profile: pnpm 10.28.2, isolated linker, layout 5, store inside the owned candidate. Shared external content caches are permitted by the identity contract.
- **Stability:** the scoped 2,885-file source/config/install inventory and own directory/dependency origins were unchanged before/after: `19f47fa81a546ce94e31adac321b08ed99bb00c37408ef1d0fa338968846a189`. HEAD and all four pinned input hashes matched.
- **Lint / TypeCheck / tests / build / SSR:** not run, as instructed. No candidate/dependency/Git mutation, service/network operation or Provider invocation occurred.

The sole owner, thread `01a19c0e-5cc4-77e0-b2d3-bd2842d39899`, must record the completed installation freeze, then build fresh and prove the actual auth registration/reference graph. Preserve guards, assertions, timeouts and v10 evidence. Required owned SSR, new wholecheck and password7 checks precede paid-provider clearance. This review grants no installation-freeze, build, wholecheck, E5/E6, paid or task-completion clearance. Final runtime cap remains 25 with 0 consumed in the dispatch; the original 14 cap is separate, and this review spent 0.
