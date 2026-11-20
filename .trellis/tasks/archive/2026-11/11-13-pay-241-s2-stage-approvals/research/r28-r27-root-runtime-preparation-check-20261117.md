# R27 ROOT integration and R28 preparation check

Verdict: **ACCEPTED_R27_ROOT_THREE_PATHS_R28_PREPARATION_ONLY**.
No concrete blocker was found in this scope. Runtime binding remains
`WAIT_R28_READ_INCREMENT_ACCEPTANCE`; this is not startup permission.

This reviewer loaded the current eight-entry task check context and performed
only filesystem reads, hashes and descriptor checks. Only this new report and
its JSON were written. No shell/process, SQL/DB/API, test/build, browser, signal,
Git or subagent execution occurred; no source/config/history/staging was changed.

## Findings (fixed)

None. The already-integrated R27 changes were verified without further edits.

## Findings (not fixed)

- The final R28 API binary and accepted GREEN source directory are absent,
  as are the new socket/ready outputs. These are expected WAIT states while
  the separate R28 read increment is unfinished. No startup or case is admitted.
- Complete R28 regression/product acceptance, whole-ROOT/source equivalence,
  final runtime identity/binary binding, original four scenarios and A01–A12/S2
  remain outside this review. R24/R25 holders, S3–S5 and parent closure stay open.

## Verification

- **Selected ROOT integration:** current bytes and modes of exactly these three
  files match accepted `R27/source-green-r02` and the integration proof:

| `server/internal/service/` file | Accepted SHA256 |
| --- | --- |
| `collaboration_completion_preview.go` | `5fc937a4…` |
| `collaboration_completion_preview_managed_submission_test.go` | `f1aa77c0…` |
| `collaboration_completion_preview_admin_submission_test.go` | `658bb12c…` |

All three are mode 0644. The recorded before/after ROOT hashes agree; no selected
source drift. This does not establish equivalence of other ROOT files.

- **Lint/static analysis:** retained actual ROOT `gofmt -l` and service `go vet`
  exited 0 with empty output; all log hashes and sizes match. No rerun.
- **TypeCheck:** retained actual ROOT service `go test -c` exited 0. Its binary
  SHA256 `343e1c12…`, logs and explicit green2 profile match the record.
  No inherited ambient environment or TestMain/test-body execution is recorded.
- **Tests:** no new execution. ROOT compilation is not a ROOT runtime test;
  the separately accepted R27 private normal/race evidence remains distinct.
  Main reported Node config/runner syntax pass; this checker did not execute it.

R28's preparation record names a copy with 5,050 files from the R27 R02 baseline.
The three selected baseline records match; the current whole R28 source was
not independently audited while its test owner writes. Three separate service
allocation/profile/clone references reconcile with their hashes and database
names; their retained allocation snapshots record 0 test commands/connections.

The draft's 249 prior staging/output descriptors are identical before/after;
current path/inode/uid/mode/type/size metadata matches. Adding only the eight
recorded slots yields 257 current paths, with no missing/extra metadata paths.
Every new slot is unique, absent from the old inventory, empty and 0700; staging
root is restored 0500. Retained descriptor SHA fields were compared between
records; archive contents were not read or freshly hashed. No tar-byte acceptance.

Bridge/browser hashes, slot order, staging/output paths, socket and API-port
references agree. The runner points to the new configuration and retains
workers 1/retries 0/maxFailures 1; these are preparation checks, not a case run.

The hash-bound read-only before-API snapshot matches the draft environment.
It records execution=false, creation=true, host-execution=false, epoch 48;
43 host runs settled, 28 artifacts complete, 0 leases/bindings, unsettled runs or
unreleased reservations. Its 29 historical task rows include 5 running statuses
without leases, and one unresolved stage intent remains preserved. No status,
gate, unknown intent or history was normalized; no live DB query was performed.

The parent CSV still has 24 rows: **0 PASS / 9 PARTIAL / 15 NOT_RUN**.
This check accepts only the selected integration and preparation evidence.
