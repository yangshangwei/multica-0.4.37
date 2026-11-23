# External build TMPDIR correction independent review

Verdict: **pass-with-notes** for the task-private environment correction, with no blocking product-error masking or source/fixture weakening found. Reviewer `/root/knowledge_web_review` wrote only this report and its JSON, and ran no tests, DB, browser, service/container, Provider, installed CLI or child work. Root retains all launcher/source/git ownership.

## Diagnosis and preserved failure

Attempt2 source231a08063 actually failed makecheck(makeexit2/packageexit1) at execenv1340pass/5fail/0skip. All22 package database records and original rawlog hash are preserved. The failed checkout has no tracked modification; its original untracked.cursor/mcp.json remains122bytes/hashfb40aae9..., byte-identical to its retained privatecopy. No reset, speculative cleanup or restored-state claim was made.

The unmodified resolveGitRoot calls git rev-parse from its nominal fixture path, and cursorProjectRoot walks ancestors to the first.git. Placing global test TMPDIR inside the candidate Gitroot therefore changes the meaning of nonGit and separateCursorproject fixtures. This is a concrete test-environment boundary, not a reason to loosen either product helper.

On the same231a08063 source with no product/test delta, changing only TMPDIR to an external nonGit root yields5/5 original failing-name racepasses, then **1345/1345 complete execenv racepasses**, no failure/skip. Actual named JSONevents and rawhashes match both public records. These diagnostic runs do not relabel failedattempt2 green.

## Future private preparation

Final3 is presently only a clean provisional e6e7fe07 source tree; no make-check record exists and no freeze/start/success is inferred. Compared with the previous private launcher, changes are the new taskroot/renderer port and explicit external MULTICA_DEV_TMPDIR plus allocation checks/provenance. It still invokes the actual complete `make check`, retains the original per-package DB/CLI-guard adapters by identicalhash and adds no stage skip, timeout/retry or assertion change.

The allocated directory is a unique canonical nonsymlink /private/tmp/multica-final-check3-* root owned by the current UID with0700mode; no ancestor has.git. Its task-owner record matches the allocationhash by independent read. The launcher matches its exact allocationdirectory/pathprefix and live UID/mode, and checks git discovery failure in the already loaded actual environment before invoking makecheck. The trusted sourceenv has no Gitroot/ceiling/discovery controls or DEV_TMPDIR override; no new global Gitflags hide the fixtureerror. The ordinary checkscript then pins TMPDIR/TMP/TEMP to its task DEV_TMPDIR. Only the child environment is changed, not the operator's global TMPDIR or any sharedvolume mode. The external root is retained outside the agent ephemeral taskdir.

Private sourceenv values were neither printed nor copied; only their SHA/presencefacts are recorded. Exact launcher/allocation/temp/source/log/adapter bindings are in execenv-temp-independent-review.json.

## Limits

ET-1: This closes the environment diagnosis/preparation gap only. Failed231a08 remainsfailed, and final3 still needs final sourcefreeze after pending CCG2review/integration and an actual new complete candidate makecheck. Static15/TS13082 or package1345passes are never a fullcheckpass.

ET-2: The result binds the currently inspected private launcher/allocation/directory state. If those bytes, sourceenv or finalsource change, root must renew the launchhash/provenancebinding; this review does not create an immutable futurecandidate proof. No paidmodel or externalauthority change is implied.
