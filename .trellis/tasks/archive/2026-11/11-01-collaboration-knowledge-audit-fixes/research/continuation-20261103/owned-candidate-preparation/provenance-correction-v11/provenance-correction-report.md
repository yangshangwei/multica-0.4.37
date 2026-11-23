# v11 source provenance correction

The canonical RK1/product SourceID is `4622182a7e129f1712f883eaea36a4756b183f539b5733eb818b7f8df9195fcb` over7,962 tracked regular files under the unchanged `source_manifest.py` ROOTS. The standard script was executed directly without API/DB activity and matches the independent runtime collector's file set and hashes.

Three identities now have separate, explicit purposes:

- Canonical product/RK1 identity: standard source_manifest.py sorted relative-path NUL file-SHA256 LF digest, `4622182a7e129f1712f883eaea36a4756b183f539b5733eb818b7f8df9195fcb`.
- Development-environment HEAD/overlay identity: scripts/dev-env.sh checkout_source_id, `3f9788baf2ad3d922df86c4817b7fe0f3183e1a5c6c0e40d43fc76cd4f2c1d88`. It includes HEAD/diff and the narrowly normalized next-env import; it is not a canonical product SourceID. Its unchanged method file hash is `817348f249009f05e0e05a4a08d0f76581cda60fac9f9ce88b18255fd4e85bc8`.
- Full repository tracked snapshot: ordered path/SHA256/lstat-mode compact JSON digest, `26e38d9e62f8a7cfa8cba243f2387934ee067677b6fa58fde2e12f0026b66920` over17,182 files. It is distinct from the product-root manifest.

The first preparation conflated the meaning of the source ID. Its reports, original launcher/bindings and raw47-check evidence are preserved as historical. New bindings are `owned-bindings-v11-provenance.json` (`6db6435e476b8f448a412d4b097e16e6131ddbf7458450ced1cf69a8f281d80b`); new launcher is `run-make-check-v11-provenance.py` (`dd0f5903b8b604f409fd10505645b9cc3920028ea4af56ad0adab8ee5a96063e`). The source env remains byte-identical (`999decf083adf9cea0271cef37834fe6528104d4ca63f8e69c90afacd01ecd8f`). New launcher checks the canonical ROOTS, exact manifest file set/hash map and computed digest, and records all three identities separately.

New offline fake preflight45/45 passed, including unchanged parser/guard cases plus product-root/digest refusals and distinct run-record labels. The actual corrected binding validation passed current canonical product/source/tool/workspace/CLI/desktop checks and reached the expected missing root ownership-proof refusal. No actual clearance/safe-env/run state or repository/API/DB/service/Provider execution occurred.

All CLI, renderer, preload, dependency layout and non-source binding fields remain unchanged; the candidate stays clean at304f74dd3. Root/runtime must review the new binding/launcher hashes before source-binding acceptance; live19846/Redis proof, minimal SSR, whole check and paid/runtime acceptance remain with their existing owners.

Machine summary: [provenance-correction-summary.json](./provenance-correction-summary.json), SHA-256 `749aeae4e41ff9bb3c0329e4883a57352a402ca9de70518e2111903c3f434a3d`.
