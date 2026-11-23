# Bundle fixture root-cause investigation

openBundleSource supports linux OR darwin, not Linux-only. Its source ancestor ownership/write-mode validation rejects /Volumes/artisan (uid501 mode0775). Full makecheck TMPDIR is below that ancestor. No test skip or broader product support is justified by this failure.

No source edit yet. Canonical explicit-TMPDIR reproduction next; production files remain read-only. Root must not infer that macOS bundle file validation is unsupported merely because final contained execution is Linux-only.

## Confirmed single-variable diagnosis and bounded fix

Unmodified canonical table with explicit taskTMPDIR: exit1,6pass/2fail/0skip; same source with only TMPDIR=/tmp: exit0,8pass/0fail/0skip. These are recorded separately in red-explicit-check-temp.json and baseline-safe-unix-temp.json. This confirms fixture ancestry, not Darwin unsupported behavior.

Patch is confined to provider_bundle_test.go: source fixtures use test-owned canonical standard-Unix-temp directories on Linux/Darwin; original security matrix remains, with an added shared-writable-parent case. Unsupported OSes explicitly require refusal/no artifact instead of pretending Unix snapshot support; no skip. Normal/race ProviderBundle runs use the SAME previously failing taskTMPDIR. Linuxarm64 and Windowsamd64 test binaries are being crosscompiled for root execution/compile evidence respectively; no native platform proof is inferred. Production source and shared filesystem modes remain unchanged.

## Root native Linux execution READY

Normal29pass and race29pass under previously failing taskTMPDIR, no failures/skips. Linuxarm64 binary .omx/private/provider-bundle-platform-regression/collaborationrun-linux-arm64.test built successfully, sha2567c6f941b3cba73f8e570a76acf9c173f771763093e8ff8a1ffd3239f84094859; root may run only ^TestProviderBundle in its cached isolated Linux container. Windowsamd64 compiled only. Actual Linux/Windows acceptance not inferred. Root-ready.json gives exact binary/arguments. Source helper is linux|darwin-safe fixture base, not an OS skip.

## ROOT Linux native completed / full Mac race started

root-linux-native.json: actualcachedLinux/aarch64 containerimage18cfe... runasuid65532 with deliberatelygroupwritable TMPDIR=/build-temp mode0770, networknone/read-onlyroot,29pass0fail0skip. Containerretained. This provesonlybundle-filematrix; no broadLinuxcontainment/installerclaim. RootfullDarwin package race is nowrunning withoriginalbadTMPDIR, noDBenv,guarded fakeCLIs; root-full-race.json willupdate. You maycommitstabletestcode+yourcompletedreports now(exclude root-full-race.json whileitupdates,rootowns it). Finalindependentreviewwillwaitfullpackageresult.

## ROOT reviewer ownership

/root/knowledge_web_review ownsONLYindependent-review/ inthisdirectory; do nottouch/stageitsupdateswhilewriting. Itreviewsstaticdelta now; finalverdictwaitsrootfullMacrace. Rootownsroot-full-race.json anditscompletion.

## Code commit ready

Code91113cbeb; normal/race29eachpass, negativepermissionoverlaykilled. summary.md/json and source-binding.json nowfinalized; Linuxactualroot-ownedpending. No productionchanges. Root can independentlyreview now; do not infer fullmakecheck pass.

## Root Linux proof observed

Root completed root-linux-native.json:29pass0fail0skip on nativeLinuxarm64, user65532, TMPDIR/build-temp0770, no network/read-onlybase, cachedimage/no download. Same6eca sourcehash and7c6f binaryhash. Narrowbundle matrix only; Windows remainscompileonly. Rootfullpackage-race is stillrunning and its file is excluded from child evidence commit.
