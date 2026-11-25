// Stable errors emitted along daemon start's config, process reservation and
// inherited-handoff gates. Match only fixed prefixes; never return their text.
const prefixes = [
  ["read CLI config:", "profile_config_read"], ["parse CLI config:", "profile_config_parse"], ["resolve profile dir:", "profile_path"],
  ["$USERPROFILE is not defined", "home_unavailable"], ["$HOME is not defined", "home_unavailable"],
  ["managed mode requires a named local profile", "profile_invalid"], ["managed handoff requires an explicit daemon profile", "handoff_profile"],
  ["management identity path must be a real directory", "directory_invalid"], ["management identity belongs to another OS user", "identity_owner"],
  ["management identity permissions must be private", "identity_permissions"], ["OS boot identity unavailable", "os_boot_identity"],
  ["managed locking is unavailable on this OS", "lock_platform"], ["invalid management lock record", "lock_record"],
  ["invalid management lock JSON", "lock_json"], ["invalid management lock owner", "lock_owner"], ["invalid management lock ticket", "lock_ticket"],
  ["management lock ticket exhausted", "lock_exhausted"], ["installation identity has a live lock owner", "lock_busy"],
  ["invalid managed daemon PID evidence", "pid_invalid"], ["managed daemon process exit is not confirmed", "pid_live"],
  ["managed control process exit is not confirmed", "control_live"], ["invalid managed process reservation", "pid_reservation"],
  ["invalid management control credential file", "control_invalid"], ["invalid management control credential", "control_invalid"],
  ["read managed startup handoff failed", "handoff_read"], ["managed startup handoff size is invalid", "handoff_size"],
  ["invalid managed startup handoff", "handoff_json"], ["invalid management JSON envelope", "management_json"],
  ["noncanonical management JSON envelope", "noncanonical_json"], ["invalid managed startup scope", "handoff_scope"],
  ["invalid managed startup account version", "handoff_account"], ["invalid management server URL", "handoff_server"],
  ["invalid or duplicated managed workspace", "handoff_workspace"], ["you are not logged in.", "auth_missing"],
  ["resolve executable path:", "executable_resolution"], ["create daemon directory:", "daemon_directory"], ["open log file ", "crash_log_open"],
  ["prepare inherited descriptors for daemon", "descriptor_prepare"], ["start daemon:", "child_spawn"], ["start daemon (no breakaway):", "child_spawn"],
  ["management identity must be a regular file", "identity_file"], ["management identity file changed while reading", "identity_changed"],
  ["oversized management identity", "identity_size"], ["invalid persisted installation identity", "identity_persisted"],
  ["invalid installation ID", "installation_id"], ["invalid installation key version", "installation_key"],
  ["invalid installation private key", "installation_seed"], ["installation public key does not match private key", "installation_public_key"],
  ["invalid management deployment ID", "management_deployment"], ["managed daemon requires an explicit profile", "profile_invalid"],
  ["managed profile installation is not enrolled", "profile_unenrolled"], ["managed profile has no user identity", "profile_user"],
  ["managed profile user is invalid", "profile_user"], ["managed profile user identity is invalid", "profile_user"],
  ["managed startup identity does not match this process", "handoff_identity"], ["managed handoff signature or original bytes changed", "handoff_signature"],
  ["managed profile identity mismatch", "profile_identity"], ["managed daemon deployment changed", "deployment_changed"],
  ["daemon credential principal differs from managed profile", "credential_principal"], ["desktop and daemon principals differ", "handoff_principal"],
  ["managed credentials are unavailable;", "credentials_missing"], ["managed credentials belong to another installation", "credentials_installation"],
  ["invalid management base64url value", "management_encoding"], ["noncanonical management base64url value", "management_encoding"],
  ["unknown flag:", "command_flag"], ["unknown shorthand flag:", "command_flag"], ["flag needs an argument:", "command_flag"],
  ["unknown command ", "command_unknown"], ["daemon start is unavailable inside a collaboration run", "command_scope"],
  ["daemon start is not available inside a daemon-managed task", "command_scope"], ["panic:", "go_panic"], ["fatal error:", "go_fatal"],
];
const ioTargets = new Set(["config", "pid", "control", "lock", "owner", "other"]);
const ioOperations = new Set(["open", "read", "write", "lstat", "stat", "mkdir", "rename", "remove", "readlink", "readdir"]);
const categories = new Set(prefixes.map(([, category]) => category));

export function daemonStartupDiagnostic(text) {
  const bounded = typeof text === "string" ? text.slice(-65536) : "";
  const found = new Set();
  const io = [];
  for (const raw of bounded.split(/\r?\n/).slice(-256)) {
    const line = raw.replace(/^\s*(?:Error:\s*)?/, "");
    for (const [prefix, category] of prefixes) if (line.startsWith(prefix)) found.add(category);
    const match = /^(?:read CLI config: |load CLI config: )?(open|read|write|lstat|stat|mkdir|rename|remove|readlink|readdir) (.*)/.exec(line);
    if (match) {
      const operation = match[1];
      const separator = match[2].indexOf(": ");
      // Drive separators are :/ or :\, not the PathError's colon-space.
      // Rename carries two paths, so do not infer one target from that string.
      const segments = separator > 0 && operation !== "rename" ? match[2].slice(0, separator).replaceAll("\\", "/").split("/") : [];
      const basename = segments.at(-1);
      const target = segments.includes(".installation-locks") ? "lock" : basename === "management-control.json" ? "control"
        : basename === "daemon.pid" ? "pid" : basename === "config.json" ? "config" : basename === ".desktop-user-id" ? "owner" : "other";
      if (!io.some(item => item.operation === operation && item.target === target)) io.push({ operation, target });
    }
  }
  return { output_present: bounded.trim().length > 0, categories: [...found], io };
}

/** Revalidate a private diagnostic before crossing into the public report. */
export function projectDaemonStartupDiagnostic(value) {
  if (!value || typeof value !== "object") return null;
  return { output_present: value.output_present === true,
    categories: Array.isArray(value.categories) ? [...new Set(value.categories.filter(item => categories.has(item)))].slice(0, categories.size) : [],
    io: Array.isArray(value.io) ? value.io.filter(item => ioOperations.has(item?.operation) && ioTargets.has(item?.target))
      .slice(0, ioTargets.size * ioOperations.size).map(({ operation, target }) => ({ operation, target })) : [] };
}
