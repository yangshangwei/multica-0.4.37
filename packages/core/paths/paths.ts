/**
 * Centralized URL path builder. All navigation in shared packages (packages/views)
 * MUST go through this module — no hardcoded string paths.
 *
 * Two kinds of paths:
 *  - workspace-scoped: paths.workspace(slug).xxx() — carry workspace in URL
 *  - global: paths.login(), paths.newWorkspace(), paths.invite(id) — pre-workspace routes
 *
 * Why pure functions + builder pattern:
 *  - Changing a route shape (e.g. adding workspace slug prefix) becomes a single-file edit
 *  - IDs are always URL-encoded here so callers can't forget
 *  - Zero runtime deps means this module is safe in Node (tests) and browsers
 */

const encode = (id: string) => encodeURIComponent(id);

/**
 * Encodes a docs slug, which is the one id here that legitimately contains
 * "/": the bundle nests a page as `developers/contributing`. Encoding the
 * whole thing would turn the separator into %2F and stop the route from
 * matching, so each segment is encoded on its own.
 */
const encodeDocsSlug = (slug: string) =>
  slug.split("/").filter(Boolean).map(encode).join("/");

/**
 * `?focus=` token that scrolls the agent's Instructions tab to its
 * conversation-starters editor and flashes it. Lives here because it is URL
 * vocabulary: `agentConversationStarters()` writes it and the tab reads it,
 * and a shared constant is what stops the two from drifting apart.
 */
export const AGENT_FOCUS_CONVERSATION_STARTERS = "conversation_starters";

function workspaceScoped(slug: string) {
  const ws = `/${encode(slug)}`;
  return {
    root: () => `${ws}/issues`,
    usage: () => `${ws}/usage`,
    issues: () => `${ws}/issues`,
    issueDetail: (id: string) => `${ws}/issues/${encode(id)}`,
    projects: () => `${ws}/projects`,
    projectDetail: (id: string) => `${ws}/projects/${encode(id)}`,
    autopilots: () => `${ws}/autopilots`,
    // The built-in automation templates, and the entry point behind the list's
    // "New autopilot" action — which is what keeps the templates reachable in a
    // workspace that already has autopilots. One route for both of its steps:
    // which template is being configured rides in `?template=`, the same choice-
    // inside-a-flow the agent role templates make (newAgentTemplate above).
    newAutopilotTemplate: () => `${ws}/autopilots/new/template`,
    autopilotDetail: (id: string) => `${ws}/autopilots/${encode(id)}`,
    agents: () => `${ws}/agents`,
    newAgent: () => `${ws}/agents/new`,
    // The two creation methods behind the chooser. Each is a real route so a
    // half-filled form survives a refresh and can be linked to directly.
    newAgentManual: () => `${ws}/agents/new/manual`,
    // The role-template flow. One route for both of its steps: which role is
    // being configured rides in `?template=`, because a role is a choice inside
    // the flow rather than a destination of its own.
    newAgentTemplate: () => `${ws}/agents/new/template`,
    newAgentAi: () => `${ws}/agents/new/ai`,
    // One creation conversation. It is a durable object, not a step of the
    // route above: it survives leaving the studio and is resumed later, so it
    // owns an address instead of being a query param on the "start one" screen.
    newAgentAiSession: (sessionId: string) =>
      `${ws}/agents/new/ai/${encode(sessionId)}`,
    // The human approval queue for high-risk agent actions. Nested under agents
    // rather than given a top-level segment: it is a queue about agents, and the
    // person who reviews it is the person who manages them. Static, so it resolves
    // ahead of agents/:id on both routers, the same way agents/new does.
    agentApprovals: () => `${ws}/agents/approvals`,
    agentDetail: (id: string) => `${ws}/agents/${encode(id)}`,
    // Deep link behind "customize" in a chat's empty state: the agent's
    // Instructions tab, scrolled to the conversation starters that produced
    // the buttons the viewer just looked at.
    agentConversationStarters: (id: string) =>
      `${ws}/agents/${encode(id)}?view=instructions&focus=${AGENT_FOCUS_CONVERSATION_STARTERS}`,
    memberDetail: (id: string) => `${ws}/members/${encode(id)}`,
    squads: () => `${ws}/squads`,
    squadDetail: (id: string) => `${ws}/squads/${encode(id)}`,
    inbox: () => `${ws}/inbox`,
    chat: () => `${ws}/chat`,
    chatWithAgent: (agentId: string) =>
      `${ws}/chat?agent=${encode(agentId)}`,
    chatSession: (sessionId: string) =>
      `${ws}/chat?session=${encode(sessionId)}`,
    myIssues: () => `${ws}/my-issues`,
    runtimes: () => `${ws}/runtimes`,
    runtimeDetail: (id: string) => `${ws}/runtimes/${encode(id)}`,
    runtimeSettings: (machineId: string, runtimeId: string) =>
      `${ws}/runtimes/${encode(machineId)}/runtime/${encode(runtimeId)}`,
    skills: () => `${ws}/skills`,
    skillDetail: (id: string) => `${ws}/skills/${encode(id)}`,
    settings: () => `${ws}/settings`,
    attachmentPreview: (id: string) => `${ws}/attachments/${encode(id)}/preview`,
    // In-app documentation. Workspace-scoped rather than a root /docs because
    // on web the root path is claimed by next.config.ts's beforeFiles rewrite
    // whenever DOCS_URL is set, which would intercept it ahead of the Next
    // router.
    //
    // `anchor` is a raw heading id, not a pre-encoded one: github-slugger emits
    // Unicode ("自定义运行时配置"), which is why every existing call site wraps it
    // in encodeURIComponent. Encoding it here is what lets those call sites stop.
    docs: () => `${ws}/docs`,
    docsPage: (slug: string, anchor?: string) =>
      `${ws}/docs/${encodeDocsSlug(slug)}${anchor ? `#${encode(anchor)}` : ""}`,
  };
}

export const paths = {
  workspace: workspaceScoped,

  // Global (pre-workspace) routes
  login: () => "/login",
  newWorkspace: () => "/workspaces/new",
  invite: (id: string) => `/invite/${encode(id)}`,
  invitations: () => "/invitations",
  onboarding: () => "/onboarding",
  authCallback: () => "/auth/callback",
  root: () => "/",
};

export type WorkspacePaths = ReturnType<typeof workspaceScoped>;

// Prefixes — not slug names — because we match against full URL paths.
// A path is global if it equals or begins with any of these.
// Note: `/workspaces/` (trailing slash) is the prefix — `workspaces` is reserved,
// so any path starting with `/workspaces/...` is system-owned, not user-owned.
const GLOBAL_PREFIXES = ["/login", "/workspaces/", "/invite/", "/invitations", "/onboarding", "/auth/", "/logout", "/signup"];

export function isGlobalPath(path: string): boolean {
  return GLOBAL_PREFIXES.some((p) => path === p || path.startsWith(p));
}
