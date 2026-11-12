# E0a actual-result review

The actual E0a behavior and lifecycle pass; full execution acceptance remains
held because operator configuration stability was not established. Independent
read-only verification passed66 evidence checks. No test or model was rerun.

`RK1-E0a-t1` completed with exactly `HOST_SMOKE_OK`, one real physical launch and
one authorized attempt. Raw stdin matches the accepted handoff binding; the
Provider made one real CLI context read and received its tool result. Two
checkpoints, termination, successful outcome, acknowledged proof and completed
seal are present. The seal reports observed/acknowledged/retained position3.
The private ledger durably acknowledges outcome, proof and seal. Actual stdout
reports Claude CLI2.1.318 and model `claude-opus-5-5`.

The original launcher exited0 and retained `finished/pass`, with no triggered
stop state. Budget accounting agrees across public and private launch records:
**1 consumed of25,24 remaining**. E0a must not be replaced or rerun.

The upfront, actual nested-build and prepared per-run Multica binaries all have
SHA-256 `1b94e13a1724f1c0091f06efefb8c961d2db09ded80dbe99f20ab6ad7bf6ead7`.
The prepared copy's observed path matches the actual launch `cwd` and run ID.
Both captures preceded Provider launch and have no observer error. Product
source, seventeen framework files, launcher/helper, tools, Provider and private
config hashes remain bound; the sole whole-checkout change is the expected
public result file.

The outer helper nevertheless exited1 with `ready_for_independent_review=false`.
Its post-run `binding()` fails at the operator-configuration equality check,
before recording its remaining reconciliation fields. Root's saved post-run
observation identifies only the `~/.claude/plugins/` tree fingerprint as changed:
registered `617d9c5d…050d8c`, observed `91deb2c6…d87660`. The source/tool/config and
CLI facts above can be reconciled independently from retained artifacts; operator
stability cannot.

Three plugin-path mtimes fall inside the trial, but mtime does not identify the
writer or reconstruct prior contents. The older61947-file inventory hashes to
`b9942cf3…13ac0`, not the registered prelaunch tree, so it cannot prove that only
harmless cache files changed. No causal attribution or exception to the original
equality rule is justified by these facts.

Preserve both original outcomes: raw launcher/public pass and helper failure.
Do not change the helper report to green. Hold H1 and subsequent scenarios until
this configuration evidence gap is explicitly dispositioned and a new model
window is granted. The released E0a window authorizes no further call.

Exact checked facts and hashes are in `e0a-independent-review.json`; the
read-only reconciliation is `verify_e0a_evidence.py`. Private stdout, ledger and
operator contents remain private. Review operations: zero Go/test/database/model
executions and no new plugin-tree scan.
