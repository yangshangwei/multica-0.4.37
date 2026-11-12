# Recording recovery change boundary

The persisted checkpoint writer has no connection to normal or restarted host recovery. The missing behavior belongs in `recoverHost` and the recording helpers, not claim, handoff snapshot, or Provider execution.

- `host_runner.go`: only `recoverHost`, to give an existing proof priority and invoke bounded original-checkpoint replay and metadata sealing without preventing Stop/proof.
- `host_checkpoints.go`: one total telemetry deadline, exact original request replay, seal replay/monotonic ACK advancement, and persisted sender-drain completion.
- `host_checkpoint_ledger.go`: validate issued seal identity and track whether a drained tail is actually known; keep unresolved checkpoints and seals recoverable after proof ACK.
- `host_recording_recovery_test.go`: backlog deadline/proof priority, lost seal ACK and metadata advance, crash-tail uncertainty, issued seal identity.

The frozen combined handoff/checkpoint suites remain unchanged. Protocol/service/SQL, launch authorization, snapshot/hash logic, Provider execution, other worktrees, commits, and global task state are outside this change. No refactor or dependency is planned.

Baseline failures are captured in `01-red.log` and its command/source manifest. The existing blocked-recording proof regression passes and must continue to pass.
