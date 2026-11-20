# C1/D1 code-route harness checkpoint

Parent requested a safe checkpoint after the selected provider returned official-client-only403 to the Multica SDK. No C1/D1 real model calls were made. O1 is implemented and fake-verified separately; C1/D1 is design investigation only.

Source findings:

- `daemon/local_directory.go:localDirectoryAssignmentForTask` deliberately ignores project local-directory resources for leader tasks. Leader can remain in private read-only scratch.
- `execenv/local_worktree.go` creates linked worktrees whose `.git` points into a shared original repository, and finalization may commit uncommitted changes. Mounting only a role cwd would not expose the required common Git metadata; mounting the common Git root would weaken role isolation and complicate attribution.
- `execution_mode=in_place` uses the resource's exact private directory, bypasses previous-workdir reuse, and preserves ordinary claim/Run/Report. A full standalone Git clone per role with an actual `.git` directory avoids cross-role commondir references.

Proposed next implementation, not yet proven:

1. Human fixture creates one project and local_directory resource for the synthetic repo under the experiment evidence root. Before each sequential role claim, human fixture switches local_path to that role's standalone clone; the actual claim must deliver the intended path and original task identity. Inspect resource-update side effects before using it; do not assume it cannot enqueue or invalidate work.
2. Reproduction/patch handoff transfers only actual agent commits using Git bundle/verified hashes. Operator may create clones and transfer commits but does not author test or fix code. Preserve commit IDs, trees, test hashes, author/task and source comment in platform evidence.
3. Contributor transport successor `codex_code_container.py` is separate from the consumed P1/O1 wrapper. It must permit the current role's real `.git`, forbid shared object databases/linked worktrees, keep generated AGENTS/requirements/skills immutable, and deny access to other role checkouts.
4. C1 verifier may write tests but not application patch; C1 implementer may change application code but the reproduction tests remain read-only; verifier recheck, QA and reviewers receive immutable checkout snapshots. D1 implementer needs its actual dependency/app patch and tests, with scanner output before/after; independent QA/reviews remain read-only.
5. All human G1/G2 decisions wait on a private root-approved record binding actual reviewer comments and decision hashes. No automatic model approval, no post-hoc expected-action edits.

Remaining work: actual C1/D1 orchestrator and role sequencing, synthetic project resource API fixture, frozen artifact handoff, code-wrapper positive/negative probes, scenario-specific fake full chain, independent admission and authorized provider-compatible real calls. No readiness claim follows from this checkpoint.
