# Windows managed daemon startup failure tree

## Facts versus interpretation

Run45690925142 still has valid installed identity/server/user, profile credential and config server match, but foreground daemon never becomes ready. management_deployment_id absent is not proof of missing Desktop setup: initializeManagedTransport persists it at managed_lifecycle.go:138, after installation/user lookup, APIconfig/me, handoff verification and credential-store checks. A failure before that commit leaves it absent intentionally.

ReserveManagedProcess passes startChild's child.Process.Pid to writeManagementJSON(daemon.pid); foreground CheckManagedProcessAbsent permits os.Getpid. No code evidence that the launcher PID is used. Background PID+errlog alone cannot prove the foreground completed all prelogger gates.

## Ordered failure tree and projection

| Stage | Actual source/branches | Safe classification families |
|---|---|---|
| Root command | main.go Execute/FormatError, Cobra flags/args, requireHumanLocalCommand | command_flag/command_unknown/command_scope/go_panic/go_fatal |
| Background prerequisite | cmd_daemon.go558–610, live daemon identity, managedProcessProfile, requireDaemonAuth, managed stdin | profile_config_read/profile_config_parse/profile_path/home_unavailable/control_io/auth_missing; handoff categories below |
| Background spawn/reservation | cmd_daemon.go611–690; resolve executable, directory/errlog, Windows breakaway fallback, ReserveManagedProcess | executable_resolution/daemon_directory/crash_log_open/descriptor_prepare/child_spawn; shared lock/PID tree below |
| Foreground gate1 | cmd_daemon.go962 managedProcessProfile → cli.LoadCLIConfigForProfile, os.UserHomeDir, controlLstat | profile_config_read/profile_config_parse/profile_path/home_unavailable/control_io |
| Foreground gate2 | cmd_daemon.go967 CheckManagedProcessAbsent → managedProfileDirectory → EnsureDirectory → managementProcessIdentity → ticket lock → readPID → controlread | profile_invalid/directory_invalid/identity_owner/identity_permissions/os_boot_identity/lock_record/lock_json/lock_owner/lock_ticket/lock_exhausted/lock_busy/pid_invalid/pid_live/control_invalid/control_live; filesystem target+operation where error is raw os.PathError |
| Foreground gate3 | cmd_daemon.go971 readManagedHandoffForCommand → ReadManagedStartupHandoff | handoff_profile/handoff_read/handoff_size/handoff_json/management_json/noncanonical_json/handoff_scope/handoff_account/handoff_server/handoff_workspace |
| Logger/config/identity | cmd_daemon.go975+ → logger, workspaces/options, LoadManagedInstallation, initializeManagedTransport | identity_file/identity_changed/identity_size/identity_persisted/installation_id/installation_key/installation_seed/installation_public_key/management_deployment/profile_unenrolled/profile_user; downstream managed_transport category set |

Raw errors are formatted directly for untyped local failures (cli/errors.go:userMessage). The classifier consumes only existing bounded private log tails, matches anchored literals/prefixes, and emits fixed identifiers. Raw syscall errors additionally yield a fixed file target/config_pid_control_lock and operation; arbitrary path/value never leaves the process. Panic/flag/fatal signatures are classified before their payload. Nonempty unknown output remains explicitly unclassified rather than false absence.

The mapping covers every explicit literal from the three prelogger gates and transitive local helper errors; OS syscall text may only narrow to operation/target, not uniquely identify a caller reused across gates. No classifier can guarantee exact call stack if the binary emits no structured location. Preserve that limit and use next native categories as evidence before a product fix.

## Implementation and verification

Added one pure fixture helper `collaboration-daemon-startup-diagnostics.mjs`. Existing controller boundedlog reading invokes it for daemon.log/daemon.err.log; safe public projection revalidates category and operation/target allowlists. Each result has output_present, categories, io. No log text, error interpolation, path, PID, command argument, credentials or signedhandoff bytes are returned. No new CLI invocation occurs. Classification describes the bounded existingtail, not necessarily only the most recent start attempt if multiple automatic attempts occurred.

Prefix table includes all errors.New literals from managedPID, managedlock, processidentity, stdinhandOff and called profile/control helper slices, plus raw CLIconfig prefixes, OS home errors, spawn wrappers, Cobra flags/commands and panic/fatal signals. A source-reading regression checks these real stable literals remain covered; functional fixtures check classifications and unknown nonempty output, private-path/token removal, and forged projection rejection. Raw os.PathError only identifies fixedIOoperation/target and may not uniquely distinguish reused callers.

Related diagnostic/workflow/controller Node suites35PASS/0FAIL/0SKIP; node syntax/diff checks passed. Existing errlog from the failedrunner is unavailable; this change is diagnostic only and needs another native run. It does not declare a product root cause or make management_deployment_id absence invalid. No product code, timeout, retry, identity, workflow or startup policy changes. No commit/push.
