# V11 artifact, source and evidence reuse review

The artifact byte checks passed. The current verdict is **blocked on corrected RK1 identity labels/binding JSON**. The canonical product SourceID is `4622182a7e129f1712f883eaea36a4756b183f539b5733eb818b7f8df9195fcb`; `3f97…` is a separate HEAD/overlay normalization identity. No candidate corruption or product code defect was found by this artifact review. This verdict gives no paid, wholecheck or task-completion clearance.

## Findings (fixed)

- File: `collector.md`.
- Issue: Initial collector inputs used the original checkout's private binding path, an abbreviated E2E commit, and a source inventory missing root build/workspace files. The all-tracked snapshot needed its separate algorithm.
- Fix: Before the one execution, changed the private binding to the candidate checkout, used exact commit `118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9`, added all standard product ROOTS, and independently computed the Git-ordered path/hash/lstat-mode snapshot. The owner executed the frozen standard-library collector once; the executed code and output remain untouched. No product file changed.

## Findings (not fixed)

- File: Original preparation identity labels and candidate `owned-bindings-v11.json`.
- Issue: Preparation presented checkout HEAD/overlay identity `3f9788baf2ad3d922df86c4817b7fe0f3183e1a5c6c0e40d43fc76cd4f2c1d88` as the standard product SourceID. Both the collector and the owner's independent standard helper produced `4622…` for the same 7,962 files, with zero file-hash differences. The owner traced `3f97…` to the statically read private `checkout-source-id-v11.cjs`, SHA `817348f249009f05e0e05a4a08d0f76581cda60fac9f9ce88b18255fd4e85bc8`; its distinct method explains the value.
- Recommendation: Label canonical product `4622…`, checkout HEAD/overlay `3f97…` and tracked fingerprint `26e3…` separately, then narrowly verify corrected JSON/hash against the preserved snapshot and artifact pins. The owner is correcting metadata; these files are outside this reviewer's write scope. Original binding SHA `ccc11e4d7e79a19c964b75d4b2c3354268a4bf28fa7ad153c23aa0a95b30b6ba` passed its byte pin and belongs to the original snapshot, not the pending corrected binding.

- File: Prior RKI6 and knowledge/CCG acceptance evidence.
- Issue: Historical fake normal/race `62 + 62` belongs to recorded sources `639…` / `9f72…`, framework baseline `5eaea44f…`, Go 1.27 and CLI `b1d54…`. Current canonical source `4622…`, HEAD `304f74dd3ba4ddea862c94917c5bb0770355a351`, Go 1.26.6 and CLI `c008…` differ. Old knowledge/CCG proofs on `9f72…` also do not automatically establish the new identity.
- Recommendation: Retain historical attribution. Reuse only explicitly proven unchanged code/protocol/guard contracts or exact retained helper/log bytes within their scope. Current-source fake, CLI/runtime, real knowledge/CCG and paid acceptance remain pending. No test was relabeled or rerun.

- File: Workspace/export and installed artifact evidence.
- Issue: Static resolution and disk equality establish their own facts. They do not establish a production Next/SSR build, current process ownership, full installed/runtime acceptance or real-provider comprehension.
- Recommendation: Keep those registered gates separate. No builds, services, database/API, Provider or network calls were allowed in this review.

- File: `collector-result.json`, raw `file_hash_declarations`.
- Issue: The generic 51,710-row association heuristic includes expected cross-associations where dictionaries contain several file/hash pairs; an SDK LICENSE could resolve to candidate/LICENSE. These are not authoritative product mismatches.
- Handling: Used the owner's targeted actual-file observations for artifact conclusions and preserved the immutable executed collector/output. This diagnostic limitation is documented rather than rewriting or rerunning recorded evidence.

## Verification

The candidate HEAD and before/after HEAD/status were stable. Independent standard product manifests agreed on `4622…` and all 7,962 file hashes. The separate all-tracked fingerprint independently matched `26e38d9e62f8a7cfa8cba243f2387934ee067677b6fa58fde2e12f0026b66920`.

The root owner's targeted read-only observations verified these disk facts:

| Artifact | Actual observation |
| --- | --- |
| CLI | SHA `c008c02377c4c52665000a0620a5a7b3f90f4d10d54d53e1d74bce97dccc6197`, 29,314,610 bytes; pin matched |
| Renderer | Installed and staged each contain 493 identical file records; every SHA, size, mode and path set match. Declared 493-file manifest SHA `bc46754412b91fac5f6649b7b37ad537e067bb9ab6fa6f0c4012cb0fced91d18` |
| Preload | One file on each side; SHA `68a9036e201a462ffd10a1a4890e052a0b6b342c0b74d6d8ac489d9f5d8e78e5`; file records match |
| Main | One file on each side; SHA `99129d6206e52ac9fdbe7454e1391d7e3b97dd80091f7461fc335a34d61b5678`; file records match |

The static graph's 66 export realpaths stay under the candidate. The owner reported the expected 12 production paths and six adjacent test paths. Original/current E2E patch IDs both match `3a28b3fc5a05aa7c39e9c6e310d1cc5427441f8a`; delta SHA is `fef30433f9117b94d4ed03a2d48d134b225dffbd4e58a6a7ab850627744db58f`. These are source/patch observations, not executed E2E results. Semantic code and preserved-guard review belongs to the separate source/code lane, which reported zero code blockers.

The independent launcher/code lane verified the pinned Node 22.23.2, pnpm 10.28.2 and Go 1.26.6 hashes, all 1,069 pnpm SDK file hashes, and retained `42 + 5` preflight logs without replay. Its report SHA is `4c4e45192733c052dd259ae421fc8c5c6008fbf297a2a890bd5557e1421f37b8`. This review attributes those observations to that lane and treats retained logs as historical evidence.

The reviewer command transport failed once, so the owner executed the frozen collector through its working surface. It exited 0 with no collector errors. Markdown collector SHA: `ab51ab619af21c2007a6c592d70a47b10ecd10c031ab0e591dc92d4ab310a8c4`; extracted code SHA: `15d8ace69b04209353422d2d09750524dc4ec908f776d1949ffea82324ac0a85`; preserved 60,526,174-byte result SHA: `8ff7626840b14c29206f9796898fc30c4cfc767a031aa9857e13d7b3d026f184`.

- Lint: not run; read-only scope.
- TypeCheck: not run; read-only scope.
- Tests: not run; owner prohibited repository/Provider execution.
- Production Next/SSR, current-source fake/CLI/full/real gates: pending.
- Paid/source gate: inactive. Runtime budget 25, consumed 0; original budget 14 stays separate.

Reviewer-authored files are confined to this evidence directory: `collector.md`, `source-binding-discrepancy.json`, `source-binding-discrepancy.md`, `verdict.json` and `verdict.md`. The owner-created executed collector and observation files were preserved. A corrected binding JSON narrow recheck can follow; this verdict stops at the current recorded snapshot.
