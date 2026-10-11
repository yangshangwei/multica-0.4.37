import type { CliInstallCommands } from "@multica/core/config";

export type ComputerPlatform = "windows" | "macos" | "linux";

const UNIX_INSTALL = "curl -fsSL https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.sh | bash";
const WINDOWS_INSTALL = "irm https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.ps1 | iex";

export function installCommand(
  platform: ComputerPlatform,
  commands: CliInstallCommands,
  legacy: string,
): string | null {
  const explicit = commands[platform]?.trim();
  if (explicit) return explicit;
  if (platform !== "windows" && legacy.trim()) return legacy.trim();
  if (legacy.trim() || Object.values(commands).some((command) => command?.trim())) return null;
  return platform === "windows" ? WINDOWS_INSTALL : UNIX_INSTALL;
}

function remoteURL(value: string): string | null {
  // Reject characters URL() would silently strip or normalize before validation.
  const trimmed = value.trim();
  if (!/^https?:\/\/[^/?#]/i.test(trimmed)) return null;
  if (trimmed.includes(" ") || trimmed.includes("\\")) return null;
  try {
    const url = new URL(trimmed);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return null;
    const host = url.hostname.toLowerCase().replace(/\.$/, "");
    if (!host || host.includes("*") || host === "localhost" || host.endsWith(".localhost")) return null;
    // URL normalizes abbreviated, octal and hexadecimal IPv4 as well as IPv6.
    if (/^(127|0)(?:\.\d+){3}$/.test(host) || host === "[::]" || host === "[::1]") return null;
    if (/^\[::ffff:(7f[0-9a-f]{2}:|0:)/.test(host)) return null;
    return trimmed.replace(/\/+$/, "");
  } catch {
    return null;
  }
}

function quoteArgument(value: string, platform: ComputerPlatform): string {
  return `'${value.replaceAll("'", platform === "windows" ? "''" : "'\"'\"'")}'`;
}

export function connectionCommands(
  platform: ComputerPlatform,
  serverURL: string,
  appURL: string,
) {
  // Check raw inputs before trimming so invalid control-only configuration
  // cannot masquerade as missing addresses and enable managed-cloud setup.
  if (Array.from(serverURL + appURL).some((character) => {
    const code = character.charCodeAt(0);
    return code < 32 || code === 127;
  })) return null;
  const cloud = !serverURL.trim() && !appURL.trim();
  const server = remoteURL(cloud ? "https://api.multica.ai" : serverURL);
  const app = remoteURL(cloud ? "https://multica.ai" : appURL);
  if (!server || !app) return null;
  const serverArg = quoteArgument(server, platform);
  const appArg = quoteArgument(app, platform);
  return {
    setup: cloud ? "multica setup" : `multica setup self-host --server-url ${serverArg} --app-url ${appArg}`,
    configure: `multica config set server_url ${serverArg}\nmultica config set app_url ${appArg}`,
    login: "multica login --token",
    start: "multica daemon start",
  };
}
