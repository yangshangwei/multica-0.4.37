# E0a execution helper preparation

Prepared, not runnable. The private permit has `allow_build`, `allow_check` and
`allow_run` all false. This preparation ran only Python syntax checks and seven
credential-free observer/environment controls; no Go, database, installed
Provider/version probe or real model call was performed. Root currently reports
the shared core window unavailable. User-facing terminal selection is also
being clarified separately.

The helper calls the existing unchanged `run_rk1.py RK1-E0a 1`, never a direct
real-test command. It retains the original operator HOME, genuine Provider
settings and credential resolution. Task/server/session variables are removed
by the documented exact boundary, then pinned Go/cache/temp/database settings
are supplied. Public records store hashes and non-sensitive paths, not the
environment or credentials. The operator-facts comparison runs under the actual
subprocess environment; a mismatch refuses before execution.

The execution copy is
`/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104/.omx/private/runtime-real-20261105/e0a_entry.py`.
It matches [e0a_entry.py](e0a_entry.py) byte-for-byte. [verification.json](verification.json)
records its current hash and [observer controls](observer-controls.txt).

Root's remaining sequence, after independent review and an explicitly agreed
window:

1. Commit/apply the final reviewed registration in the execution checkout, keep
   the accepted product and seventeen framework files unchanged, and verify
   whole-checkout Git status is clean. Upfront and nested builds must use this
   same execution HEAD and default VCS stamping.
2. Allocate/migrate/verify a fresh owned database and an owner-only external
   `/private/tmp/` directory. Write a private0600 JSON config with keys
   `DATABASE_URL`, `database` and `temp_root`. The helper never allocates or
   migrates databases; root's allocation receipt proves database identity and
   migration764.
3. Invoke `describe --config <private JSON> --permit <private permit>`. This
   creates the private `binding-proposal.json` using read-only source/operator
   facts. It does not run Go, the database or the Provider. Root reviews and
   copies that exact object into the permit's `binding`, adds the agreed
   `window_id` and timezone-aware `window_expires_at`, and sets only
   `allow_build=true`.
4. Invoke `build` with those same arguments. This performs the one upfront
   `go build -o <private>/upfront-multica ./cmd/multica` using the final
   structured environment. It records process ID, actual exit, before/after
   binding, CLI hash/size and `go version -m` metadata. Root reviews the result,
   binds its SHA-256 in `build_result_sha256`, and opens `allow_check`.
5. Invoke `check` with the same arguments. It calls only the original launcher's
   `--check-only`, under exactly the build environment. Root reviews exit0 and
   the unchanged binding, binds `check_result_sha256`, and opens `allow_run`
   only for the agreed live model window.
6. Invoke `run` once. Before the original launcher starts it creates an exclusive
   fsynced `run-attempt.json`; an interrupted/failed attempt is never retried by
   this helper. The launcher's original public/private accounting and stop rules
   remain authoritative. No follow-up scenario is started automatically.

The passive observer starts before the launcher. It examines only the two
documented Multica paths under the new task temp directory, using two cheap
stable metadata observations before reading bytes once. Regular-file, ownership,
non-symlink and before/after inode/size/mtime checks protect each capture. Once
captured, an unchanged file is never reread or rehashed. The control test checks
one full read per binary across repeated polling. Captures are private copies,
never executed and never injected into the Provider's environment or PATH.

After execution, the helper binds the prepared CLI path to the actual launch
record's `cwd` and run ID, compares the upfront/nested/prepared byte hashes and
reads each captured file's Go build metadata. It records launcher PID/exit,
source before/after, public measured starts, private launch accounting and trial
classification. Public record creation is the sole allowed post-run whole-tree
status change. Missing/mismatched captures remain an evidence gap, without
replacing or rerunning the trial. Capture is an observation after files appear,
not a gate guaranteed to intercept before Provider launch.

`ready_for_independent_review` is a recorder-integrity result, not a claim that
the model passed the scenario. Root must inspect the actual E0a public verdict,
lifecycle, one measured launch, exact stdin/context exposure and stop state,
then release the model window before agent reasoning resumes. Request/version
metadata remains separate from observed model/version evidence.

The seven small controls cover regular files, file/ancestor symlinks, foreign
owner, unstable reads, exact two-pattern capture with bounded full reads, and
preservation of genuine operator settings while stripping server/session state.
They do not prove live capture timing, actual CLI byte equality, DB availability
or any real model behavior. None of the paid-series prerequisites is inferred
from these controls.
