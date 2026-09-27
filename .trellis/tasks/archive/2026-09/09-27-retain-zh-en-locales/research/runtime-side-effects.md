# Runtime side effects

During the isolated API restart, the repository's `make up` command invoked its documented automatic TTL garbage collection before starting the requested component. It removed expired agent environments `agent-audit-0926` and `agent-head-recheck-0926`, including their allocated test databases/profiles. The original checkout environment `multica_0_4_37-492` was not removed or changed by that cleanup. This was disclosed in the user-facing progress update.

For subsequent restarts, source `scripts/dev-env.sh` as a library and call the existing named component helpers with the correct manifest/environment, avoiding `cmd_up` and global garbage collection. Never destroy the original environment or its data.
