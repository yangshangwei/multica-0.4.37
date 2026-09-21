import { createStore } from "zustand/vanilla";
import { useStore } from "zustand";

interface ConfigState {
  cdnDomain: string;
  // True when cdnDomain serves private content via time-bounded signed URLs
  // (CloudFront signing enabled server-side). Renderers must not treat a raw
  // storage URL on that domain as a loadable media source (MUL-3254).
  cdnSigned: boolean;
  allowSignup: boolean;
  googleClientId: string;
  daemonServerUrl: string;
  daemonAppUrl: string;
  // Complete command published by a self-hosted operator for installing the
  // CLI from an internal artifact source. Empty means use the public default.
  cliInstallCommand: string;
  // Self-host gate (#3433): when true, every "Create workspace" affordance
  // must be hidden. Defaults to false so unknown / older servers behave like
  // the managed-cloud case.
  workspaceCreationDisabled: boolean;
  // Self-host-only gate for the Git provider integration (Forgejo / Gitea /
  // GitLab). When false the whole Settings → Integrations "Git providers"
  // section is hidden. Defaults to false so unknown / older servers and the
  // managed cloud (which omits the field) keep it hidden.
  vcsIntegrationAvailable: boolean;
  // Deployment policy for external messaging providers, independent of Git.
  // Hidden until config loads; a successful older-server response enables it.
  messagingIntegrationsEnabled: boolean;
  // Whether this deployment mints a session from a device id alone
  // (intranet device auth). Defaults to false and must stay false for any
  // server that does not declare it: the endpoint answers 403 both when the
  // switch is off and when the server predates it, so a client that guessed
  // "available" would replace a working login page with a doomed request.
  deviceAuthAvailable: boolean;
  featureFlags: Record<string, boolean>;
  // The running API build version, surfaced in the Help popover so
  // self-hosted operators can confirm what's deployed. Empty for dev builds
  // or servers older than this feature.
  serverVersion: string;
  // Whether the connected server validates local_directory execution_mode.
  // Defaults to false, and stays false for any server that does not declare it:
  // the dangerous ones accept worktree mode, drop the field, and run the task
  // in the user's working copy anyway (#7113). Servers that validate but
  // predate this signal are caught by the same net — indistinguishable from
  // here, and only one of the two answers is safe to guess.
  localWorktreeSupported: boolean;
  // Whether this server persists conversation_starters on agent create/update.
  // Older handlers accepted the unknown field and returned success while
  // dropping it, so absent must fail closed.
  agentConversationStartersSupported: boolean;
  setCdnConfig: (config: { cdnDomain: string; cdnSigned?: boolean }) => void;
  setAuthConfig: (config: {
    allowSignup: boolean;
    googleClientId?: string;
    workspaceCreationDisabled?: boolean;
    vcsIntegrationAvailable?: boolean;
    messagingIntegrationsEnabled?: boolean;
    deviceAuthAvailable?: boolean;
  }) => void;
  setDaemonConfig: (config: {
    daemonServerUrl?: string;
    daemonAppUrl?: string;
    cliInstallCommand?: string;
  }) => void;
  setFeatureFlags: (flags?: Record<string, boolean>) => void;
  setServerVersion: (version?: string) => void;
  setLocalWorktreeSupported: (supported?: boolean) => void;
  setAgentConversationStartersSupported: (supported?: boolean) => void;
}

export const configStore = createStore<ConfigState>((set) => ({
  cdnDomain: "",
  cdnSigned: false,
  allowSignup: true,
  googleClientId: "",
  daemonServerUrl: "",
  daemonAppUrl: "",
  cliInstallCommand: "",
  workspaceCreationDisabled: false,
  vcsIntegrationAvailable: false,
  messagingIntegrationsEnabled: false,
  deviceAuthAvailable: false,
  featureFlags: {},
  serverVersion: "",
  localWorktreeSupported: false,
  agentConversationStartersSupported: false,
  setCdnConfig: ({ cdnDomain, cdnSigned = false }) => set({ cdnDomain, cdnSigned }),
  setAuthConfig: ({
    allowSignup,
    googleClientId = "",
    workspaceCreationDisabled = false,
    vcsIntegrationAvailable = false,
    messagingIntegrationsEnabled = true,
    deviceAuthAvailable = false,
  }) =>
    set({
      allowSignup,
      googleClientId,
      workspaceCreationDisabled,
      vcsIntegrationAvailable,
      messagingIntegrationsEnabled,
      deviceAuthAvailable,
    }),
  setDaemonConfig: ({
    daemonServerUrl = "",
    daemonAppUrl = "",
    cliInstallCommand = "",
  }) => set({ daemonServerUrl, daemonAppUrl, cliInstallCommand }),
  setFeatureFlags: (flags = {}) => set({ featureFlags: { ...flags } }),
  setServerVersion: (version = "") => set({ serverVersion: version }),
  setLocalWorktreeSupported: (supported = false) =>
    set({ localWorktreeSupported: supported === true }),
  setAgentConversationStartersSupported: (supported = false) =>
    set({ agentConversationStartersSupported: supported === true }),
}));

export function useConfigStore(): ConfigState;
export function useConfigStore<T>(selector: (state: ConfigState) => T): T;
export function useConfigStore<T>(selector?: (state: ConfigState) => T) {
  return useStore(configStore, selector as (state: ConfigState) => T);
}

export function featureFlagEnabled(
  flags: Readonly<Record<string, boolean>> | undefined,
  key: string,
  defaultValue = false,
): boolean {
  return flags?.[key] ?? defaultValue;
}

export function useFeatureEnabled(key: string, defaultValue = false): boolean {
  return useConfigStore((state) =>
    featureFlagEnabled(state.featureFlags, key, defaultValue),
  );
}
