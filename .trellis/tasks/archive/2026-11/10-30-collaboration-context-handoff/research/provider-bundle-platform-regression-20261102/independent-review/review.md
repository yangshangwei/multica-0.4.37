# Provider bundle fixture independent review

Verdict: **pass-with-notes**, no blocking fixture/security regression found. Code `91113cbebca4e08423172e86f26db5307784e215`, author-evidence tip `a71898d4b45cb746b12c687b5f38b3f943e889b1`, base `f634cf48016952164732843a66d9f53ba8b506f9`. Test SHA-256 `6eca92b5a7e88aafb0ade56ca88956318b112487c723ce58f2ca24dfe87bb045` matches the committed file, targeted tests, Linux binary provenance and completed full package result. Reviewer `/root/knowledge_web_review` wrote only review reports and ran no tests, browser, DB, service/container, installed Agent CLI or Provider operation.

## Cause and patch

The safe source opener supports **Linux and Darwin**. Its pinned-directory/openat/O_NOFOLLOW/owner/mode/nonregular policy is byte-unchanged. The unmodified valid case fails when task TMPDIR inherits non-root group-writable /Volumes/artisan; changing only TMPDIR to standard/tmp makes the original eight named cases pass. The initial Linux-only-helper hypothesis is explicitly corrected; it was not used to skip Darwin.

Only provider_bundle_test.go changed. Linux/Darwin source fixtures use a fresh private os.MkdirTemp directory under/tmp, register exact-owned-path cleanup before later failure points, canonicalize the source and retain original bytes/mode/hash setup. There is no global TMPDIR change or shared-volume chmod. The0770 parent negative changes only that newly test-owned directory. Cleanup removes only its generated root; symlink negatives remain contained there.

The original valid/symlink/parent-symlink/mutation/size/type/shared-file-mode matrix remains active on both supported platforms. The added shared-writable-parent case kills a private overlay disabling the production permission predicate: its named failure observes snapshot unexpectedly succeeding, rather than a setup error. Unsupported OSes assert ErrUnsupported and no destination artifact, not a skip or a fabricated Unix positive. Imports are standard-library only.

## Verified evidence

All raw logs, current source/helper hashes and Linux/Windows binary hashes match their manifests; the reviewer inspected evidence only.

| Recorded check | Actual result |
| --- | --- |
| Unmodified canonical test, original bad TMPDIR |6pass/2fail/0skip; valid case and parent fail |
| Same unmodified source, only TMPDIR=/tmp |8pass/0fail/0skip |
| Fixed ProviderBundle normal and race, prior failing task TMPDIR |29pass each;0fail/skip |
| Writable-parent permission overlay |named case and parent fail,exit1; mutant killed |
| Root full Darwin collaborationrun race, original final-check TMPDIR |**348pass/0fail/0skip,exit0**, complete and source unchanged |
| Actual Linux arm64 bundle matrix |29pass/0fail/0skip,exit0; uid65532,networknone,read-onlyroot,deliberately0770TMPDIR |
| Linux arm64 and Windows amd64 binaries |crosscompileexit0; hashes verified; Windows native not claimed |

Exact commands, counts, source/image/runtime/binary/log hashes are in review.json and evidence-hash-check.json. `git diff --check` exits0. Server source trees at code91113cbeb and current author-evidence tip match, so documentation changes did not drift tested code.

## Notes

PB-1: The cached-image Linux run proves only this bundle-file unit matrix. It is not full Linux containment, collaboration journey or installer evidence; Windows remains compile-only.

PB-2: The348-pass Darwin package closes this regression only. Candidate7f8ca8725 actually failed makecheck(makeexit2/packageexit1,345pass/2fail); its raw records and per-package databases remain preserved. Do not relabel it passed. Root owns committing full-race/review evidence, local integration and a new complete source-bound candidate check.

PB-3: No timeout/retry relaxation, production permission broadening, paid-model call or external authorization change was introduced. Remaining model/platform/account/release gates stay open. No reviewer self-fix was applied.
