"use client";

import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import {
  CalendarDays,
  Check,
  ChevronRight,
  FolderKanban,
  Maximize2,
  Minimize2,
  MoreHorizontal,
  PencilLine,
  Settings2,
  X as XIcon,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { DialogTitle } from "@multica/ui/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { cn } from "@multica/ui/lib/utils";
import { api, ApiError } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentWorkspace, useWorkspacePaths } from "@multica/core/paths";
import { AppLink, resolveClickIntent } from "../navigation";
import { agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { resolveQuickCreateCreator } from "@multica/core/issues/creator-selection";
import { getProjectExecutionSquads } from "@multica/core/projects";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  useQuickCreateStore,
  captureQuickCreateScope,
  isQuickCreateScopeCurrent,
  isQuickCreateStoreReady,
  type QuickCreateActorType,
  type QuickCreateActorRef,
} from "@multica/core/issues/stores/quick-create-store";
import {
  useIssueCreateSettingsStore,
  type QuickCreateField,
} from "@multica/core/issues/stores/issue-create-settings-store";
import { useIssueDraftStore, type IssueCreateDraft } from "@multica/core/issues/stores/draft-store";
import { useCreateModeStore } from "@multica/core/issues/stores/create-mode-store";
import {
  runtimeListOptions,
  checkQuickCreateCliVersion,
  checkQuickCreateFieldsCliVersion,
  readRuntimeCliVersion,
} from "@multica/core/runtimes";
import { useShortcut } from "@multica/core/shortcuts";
import { ShortcutKeycaps } from "../common/shortcut-keycaps";
import {
  contentReferencesAttachment,
  type Agent,
  type IssuePriority,
  type SourceContextPreview,
  type Squad,
} from "@multica/core/types";
import { ClearablePillButton, PillButton } from "../common/pill-button";
import { ProjectPicker } from "../projects/components/project-picker";
import { DueDatePicker, PriorityIcon, PriorityPicker } from "../issues/components";
import { canAssignAgent } from "../issues/components/pickers/assignee-picker";
import { canAgentCreateIssues, isAgentRuntimeBound, QUICK_CREATE_MIN_AUTONOMY } from "@multica/core/agents";
import { useAuthStore } from "@multica/core/auth";
import { memberListOptions } from "@multica/core/workspace/queries";
import {
  ContentEditor,
  type ContentEditorRef,
  useFileDropZone,
  FileDropOverlay,
  useUploadGate,
  useComposerSubmit,
} from "../editor";
import { useIssueCreateUploads } from "./use-issue-create-uploads";
import { FileUploadButton } from "@multica/ui/components/common/file-upload-button";
import { useT } from "../i18n";
import { QuickCreateActorPicker } from "./quick-create-actor-picker";
import { actorKey, buildActorCatalog } from "./quick-create-actor-picker-model";
import { CreatorRecommendations } from "./creator-recommendations";
import { SourceContextPreviewCard, useSourceContextFailureMessage } from "./source-context-preview";
import { useIssueLimitUpgradePrompt } from "./use-issue-limit-upgrade-prompt";
import { IssueDescriptionAssist } from "./issue-description-assist";
import { getQuickCreateScenario } from "./quick-create-scenario";

const EMPTY_ACTORS: QuickCreateActorRef[] = [];

// AgentCreatePanel — agent-mode body of the create-issue dialog. Renders
// only the inner content; the surrounding `<Dialog>` AND `<DialogContent>`
// (Portal + Overlay + Popup) are owned by CreateIssueDialog so mode-switching
// swaps only this body. Lifting the Portal is what eliminates the close→open
// animation flash — Base UI replays Popup enter/exit when DialogContent is
// remounted, even inside a still-open Dialog Root.
//
// `onSwitchMode` is wired by the shell — the panel calls it with an optional
// carry payload (currently `project_id`). The shared draft store carries the
// description + agent across the agent→manual flip; project_id rides through
// the same carry channel manual→agent uses, so the manual panel reads it
// from `data?.project_id` without a parallel store.
export function AgentCreatePanel({
  onClose,
  onSwitchMode,
  data,
  isExpanded,
  setIsExpanded,
  onNeedsSpace,
}: {
  onClose: () => void;
  onSwitchMode?: (carry?: Record<string, unknown> | null) => void;
  data?: Record<string, unknown> | null;
  /** Lifted to the shell so DialogContent's mode-aware className can react —
   *  same pattern as ManualCreatePanel. Shared across modes so the user's
   *  expand preference persists when switching between agent and manual. */
  isExpanded: boolean;
  setIsExpanded: (v: boolean) => void;
  onNeedsSpace?: () => void;
}) {
  const { t } = useT("modals");
  const { t: tIssues } = useT("issues");
  const { t: tProjects } = useT("projects");
  const { t: tAgents } = useT("agents");
  const promptLabelId = useId();
  const sendShortcut = useShortcut("send");
  const workspaceName = useCurrentWorkspace()?.name;
  const workspacePaths = useWorkspacePaths();
  const wsId = useWorkspaceId();
  const anchorCommentId = typeof data?.anchor_comment_id === "string" ? data.anchor_comment_id : null;
  const sourcePreview = data?.source_context_preview as SourceContextPreview | undefined;
  const sourceContextLoading = data?.source_context_loading === true;
  const sourceContextFailed = data?.source_context_failed === true;
  const sourceContextError = data?.source_context_error;
  const refetchSourceContext = data?.source_context_refetch as (() => Promise<unknown>) | undefined;
  const sourceContextExpanded = typeof data?.source_context_expanded === "boolean"
    ? data.source_context_expanded
    : undefined;
  const onSourceContextExpandedChange = data?.source_context_on_expanded_change as ((expanded: boolean) => void) | undefined;
  const sourceContextFailureMessage = useSourceContextFailureMessage();
  const showIssueLimitUpgradePrompt = useIssueLimitUpgradePrompt();
  const userId = useAuthStore((s) => s.user?.id);
  const authStatus = useAuthStore((s) => s.status);
  const membersQuery = useQuery(memberListOptions(wsId));
  const agentsQuery = useQuery(agentListOptions(wsId));
  const squadsQuery = useQuery(squadListOptions(wsId));
  const { data: members = [] } = membersQuery;
  const { data: agents = [] } = agentsQuery;
  const { data: squads = [] } = squadsQuery;
  // Pull `isSuccess` so the stale-id sweep below can distinguish "still
  // loading" from "loaded as empty". Reading length alone treats both as
  // empty and incorrectly clears a valid persisted preference on every open.
  const projectsQuery = useQuery(projectListOptions(wsId));
  const { data: projects = [], isSuccess: projectsLoaded } = projectsQuery;

  const memberRole = useMemo(
    () => members.find((m) => m.user_id === userId)?.role,
    [members, userId],
  );

  // Visible = not archived AND assignable by this user. Squads inherit
  // their leader agent's reachability: the backend always routes a squad
  // pick to the leader, so hiding squads whose leader isn't visible keeps
  // the picker honest with what the server would actually accept.
  const visibleAgents = useMemo(
    () =>
      agents.filter(
        (a) =>
          !a.archived_at &&
          isAgentRuntimeBound(a) &&
          canAssignAgent(a, userId, memberRole),
      ),
    [agents, userId, memberRole],
  );
  const visibleAgentIds = useMemo(
    () => new Set(visibleAgents.map((a) => a.id)),
    [visibleAgents],
  );
  const visibleSquads = useMemo(
    () =>
      squads.filter(
        (s) => !s.archived_at && visibleAgentIds.has(s.leader_id),
      ),
    [squads, visibleAgentIds],
  );

  // Create-with-agent runs one `multica issue create` as the chosen agent (the
  // leader, for a squad), and the server refuses it below Contributor. Those
  // actors stay listed so people recognise them, but disabled with the reason;
  // the server answers agent_autonomy_insufficient if one is sent anyway.
  const autonomyLabel = useCallback((level: string) => {
    switch (level) {
      case "observer":
      case "contributor":
      case "coordinator":
      case "operator":
        return tAgents(($) => $.role_templates.autonomy[level]);
      default:
        return level;
    }
  }, [tAgents]);
  const blockedLevels = useMemo(() => {
    const levels = new Map<string, string>();
    const agentLevels = new Map(visibleAgents.map((a) => [a.id, a.autonomy_level ?? ""]));
    for (const a of visibleAgents) {
      if (!canAgentCreateIssues(a.autonomy_level)) levels.set(actorKey({ type: "agent", id: a.id }), a.autonomy_level ?? "");
    }
    for (const s of visibleSquads) {
      const level = agentLevels.get(s.leader_id);
      if (level !== undefined && !canAgentCreateIssues(level)) levels.set(actorKey({ type: "squad", id: s.id }), level);
    }
    return levels;
  }, [visibleAgents, visibleSquads]);
  const creatorDisabledReasons = useMemo(() => new Map([...blockedLevels].map(([key, level]) => [key,
    key.startsWith("squad:")
      ? t(($) => $.create_issue.actor_picker.disabled_autonomy_squad, { level: autonomyLabel(level) })
      : t(($) => $.create_issue.actor_picker.disabled_autonomy_agent, { level: autonomyLabel(level) }),
  ])), [blockedLevels, t, autonomyLabel]);
  const canCreateAs = useCallback((ref: QuickCreateActorRef) => !blockedLevels.has(actorKey(ref)), [blockedLevels]);

  const preferencesReady = useQuickCreateStore((s) => isQuickCreateStoreReady(s, wsId, userId));
  const resetGeneration = useQuickCreateStore((s) => s.resetGeneration);
  const defaultActor = useQuickCreateStore((s) => preferencesReady ? s.defaultActor : null);
  const setDefaultActor = useQuickCreateStore((s) => s.setDefaultActor);
  const lastActorType = useQuickCreateStore((s) => preferencesReady ? s.lastActorType : null);
  const lastActorId = useQuickCreateStore((s) => preferencesReady ? s.lastActorId : null);
  const recordSuccessfulActor = useQuickCreateStore((s) => s.recordSuccessfulActor);
  const toggleFavoriteActor = useQuickCreateStore((s) => s.toggleFavoriteActor);
  const favoriteActors = useQuickCreateStore((s) => preferencesReady ? s.favoriteActors : EMPTY_ACTORS);
  const recentActors = useQuickCreateStore((s) => preferencesReady ? s.recentActors : EMPTY_ACTORS);

  useEffect(() => {
    // Same-workspace login can leave the route mirror unchanged, so it does
    // not trigger the normal workspace rehydration callback.
    if (authStatus === "authenticated" && userId && captureQuickCreateScope(wsId, userId)
      && !isQuickCreateStoreReady(useQuickCreateStore.getState(), wsId, userId)) {
      void useQuickCreateStore.persist.rehydrate();
    }
  }, [wsId, userId, authStatus, resetGeneration]);
  const visibleFields = useIssueCreateSettingsStore((s) => s.quickCreateFields);
  const keepOpen = useQuickCreateStore((s) => preferencesReady && s.keepOpen);
  const setKeepOpen = useQuickCreateStore((s) => s.setKeepOpen);
  const setLastMode = useCreateModeStore((s) => s.setLastMode);
  // The agent draft (prompt + actor) and the shared fields (project, priority,
  // due date, attachments) live in the unified issue-create draft, so a switch
  // to/from the manual form preserves them.
  const draft = useIssueDraftStore((s) => s.draft);
  const setShared = useIssueDraftStore((s) => s.setShared);
  const setManual = useIssueDraftStore((s) => s.setManual);
  const setAgent = useIssueDraftStore((s) => s.setAgent);
  const setActiveMode = useIssueDraftStore((s) => s.setActiveMode);
  const clearDraft = useIssueDraftStore((s) => s.clearDraft);

  // Resolve a candidate actor against the currently-visible agents / squads.
  // Returns null when the candidate doesn't exist in this workspace right
  // now (deleted, archived, permission revoked, etc.) so callers can fall
  // through to the next seed in the chain.
  const resolveActor = useCallback(
    (
      type: QuickCreateActorType | "agent" | "squad" | null | undefined,
      id: string | null | undefined,
    ): QuickCreateActorRef | null => {
      if (!type || !id) return null;
      if (type === "squad" && visibleSquads.some((s) => s.id === id)) {
        return { type: "squad", id };
      }
      if (type === "agent" && visibleAgentIds.has(id)) {
        return { type: "agent", id };
      }
      return null;
    },
    [visibleSquads, visibleAgentIds],
  );

  const seedActor = useCallback((): QuickCreateActorRef | null => {
    const callers: QuickCreateActorRef[] = [];
    if (typeof data?.agent_id === "string") callers.push({type:"agent",id:data.agent_id});
    if (typeof data?.squad_id === "string") callers.push({type:"squad",id:data.squad_id});
    return resolveQuickCreateCreator({
      callers,
      draftActor: draft.agent.actorType && draft.agent.actorId ? {type:draft.agent.actorType,id:draft.agent.actorId} : null,
      defaultActor: defaultActor ?? null,
      lastActor: lastActorType && lastActorId ? {type:lastActorType,id:lastActorId} : null,
      agents: visibleAgents, squads: visibleSquads, preferencesReady,
      agentsKnown: agentsQuery.data !== undefined && membersQuery.data !== undefined,
      squadsKnown: squadsQuery.data !== undefined,
      canCreate: canCreateAs,
    });
  }, [data?.agent_id, data?.squad_id, draft.agent.actorType, draft.agent.actorId, defaultActor, lastActorType, lastActorId,
    visibleAgents, visibleSquads, preferencesReady, agentsQuery.data, membersQuery.data, squadsQuery.data, canCreateAs]);

  const [actor, setActor] = useState<QuickCreateActorRef | null>(() => seedActor());

  // Re-seed once visible lists resolve (queries may be empty on first render).
  useEffect(() => {
    if (actor && resolveActor(actor.type, actor.id)) return;
    setActor(seedActor());
  }, [actor, resolveActor, seedActor]);

  const selectedAgent = useMemo<Agent | undefined>(() => {
    if (!actor) return undefined;
    if (actor.type === "agent") return visibleAgents.find((a) => a.id === actor.id);
    const squad = visibleSquads.find((s) => s.id === actor.id);
    if (!squad) return undefined;
    return visibleAgents.find((a) => a.id === squad.leader_id);
  }, [actor, visibleAgents, visibleSquads]);

  const selectedSquad = useMemo<Squad | undefined>(() => {
    if (actor?.type !== "squad") return undefined;
    return visibleSquads.find((s) => s.id === actor.id);
  }, [actor, visibleSquads]);

  // Unfinished selections live in the shared issue-create draft. The
  // last-successful actor remains a separate fallback, so closing a draft
  // never overwrites the default established by an actual create.
  //
  // Project has exactly two seeds, both carrying explicit user intent: the
  // project page (or manual panel) the modal was opened from, and the user's
  // own unfinished draft. It is deliberately NOT seeded from the last create
  // — see quick-create-store (MUL-5862).
  const [projectId, setProjectId] = useState<string | null>(() => {
    const seed = (data?.project_id as string | undefined) ?? draft.shared.projectId;
    return seed ?? null;
  });
  const projectActors = useMemo<QuickCreateActorRef[]>(() => {
    const project = projects.find((item) => item.id === projectId);
    if (!project) return [];
    const available = new Set(visibleSquads.map((item) => item.id));
    const ids = getProjectExecutionSquads(project).flatMap((config) => config.state === "configured" && config.squad_id && available.has(config.squad_id) ? [config.squad_id] : []);
    return [...new Set(ids)].map((id) => ({type:"squad",id}));
  }, [projects, projectId, visibleSquads]);
  // Recommending a creator the picker refuses would be a dead end.
  const recommendationActors = useMemo(
    () => buildActorCatalog(visibleAgents, visibleSquads).filter((entry) => canCreateAs(entry)),
    [visibleAgents, visibleSquads, canCreateAs],
  );
  // The picker refuses these, but a caller (an agent's "Assign" button) or an
  // older draft can still land on one. Say why and offer the manual path rather
  // than silently swapping the creator.
  const blockedLevel = actor ? blockedLevels.get(actorKey(actor)) : undefined;
  const autonomyBlocked = blockedLevel !== undefined;
  const [priority, setPriority] = useState<IssuePriority>(
    (data?.priority as IssuePriority | undefined) ?? draft.shared.priority,
  );
  const [dueDate, setDueDate] = useState<string | null>(
    (data?.due_date as string | undefined) ?? draft.shared.dueDate,
  );
  const [fieldPickerOpen, setFieldPickerOpen] = useState<QuickCreateField | null>(null);
  // Local state + shared draft always move together, so both the picker rows
  // and the pill's quick-clear go through here.
  const commitProject = (next: string | null) => {
    setProjectId(next);
    setShared({ projectId: next ?? undefined });
  };

  // Parent-issue context — seeded by `openCreateSubIssue` when the modal is
  // opened from the "Add sub issue" entry on an existing issue. We carry it
  // through (not as an editable form field) so a manual→agent flip preserves
  // the sub-issue intent; the agent panel never exposes this as a picker.
  // Identifier is best-effort display context only — the UUID is the
  // authoritative reference the backend/agent uses for `--parent <uuid>`.
  const parentIssueId = (data?.parent_issue_id as string | undefined) ?? undefined;
  const parentIssueIdentifier =
    (data?.parent_issue_identifier as string | undefined) ?? undefined;

  // Stale-id sweep. Once the project list query has actually resolved
  // (`isSuccess` — distinct from "data is the empty default during loading"),
  // a `projectId` that isn't in the list means the project was deleted in
  // another session. Clear local state AND the unfinished draft — the draft
  // is the only persisted copy left, and leaving it would make the next open
  // re-seed and submit the same dead value.
  useEffect(() => {
    if (!projectsLoaded || projectId === null) return;
    if (projects.some((p) => p.id === projectId)) return;
    setProjectId(null);
    if (draft.shared.projectId === projectId) {
      setShared({ projectId: undefined });
    }
  }, [projectsLoaded, projects, projectId, draft.shared.projectId, setShared]);

  // Mark the persisted draft's active mode so a later reopen and any reader of
  // the unified draft know which form is being edited.
  useEffect(() => {
    setActiveMode("agent");
  }, [setActiveMode]);

  // Daemon CLI version gate. The agent-create flow needs the runtime's
  // bundled multica CLI to be ≥ MIN_QUICK_CREATE_CLI_VERSION; older
  // daemons handle attachments and partial-failure retries incorrectly
  // (see PR #1851 / MUL-1496). Pre-check on the picker so the user gets
  // immediate feedback instead of waiting for the inbox failure; the
  // server re-validates as the trust boundary. Dev-built daemons
  // (git-describe shape) are exempted inside checkQuickCreateCliVersion
  // — frontend and server share the same signal there, so they agree by
  // construction across web/desktop/staging without comparing env flags.
  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId));
  const selectedRuntime = useMemo(
    () =>
      selectedAgent?.runtime_id
        ? runtimes.find((r) => r.id === selectedAgent.runtime_id)
        : undefined,
    [runtimes, selectedAgent?.runtime_id],
  );
  // We can only pre-check a version we can actually see. A non-admin member's
  // runtime list (ListVisibleAgentRuntimes) omits other members' private
  // machines, so a selected agent bound to such a runtime yields no row here.
  // That absence is "unknown version", NOT "daemon reported no version": the
  // two must not collapse, or the member gets the misleading "upgrade your
  // daemon" wall for a runtime that is in fact new enough (#7633). When we
  // can't pre-check, defer to the server's authoritative gate
  // (checkQuickCreateDaemonVersion, which reads the row by id regardless of
  // role) instead of failing closed in the UI.
  const canPrecheckVersion = selectedAgent?.runtime_id != null && selectedRuntime != null;
  const runtimeCliVersion = readRuntimeCliVersion(selectedRuntime?.metadata);
  const baseVersionCheck = useMemo(
    () => checkQuickCreateCliVersion(runtimeCliVersion),
    [runtimeCliVersion],
  );
  const fieldVersionCheck = useMemo(
    () => checkQuickCreateFieldsCliVersion(runtimeCliVersion),
    [runtimeCliVersion],
  );
  const usesExplicitFields = priority !== "none" || dueDate !== null;
  const versionCheck = usesExplicitFields ? fieldVersionCheck : baseVersionCheck;
  const versionBlocked =
    canPrecheckVersion &&
    (baseVersionCheck.state !== "ok" ||
      (usesExplicitFields && fieldVersionCheck.state !== "ok"));

  const initialPrompt = draft.agent.prompt || (data?.prompt as string) || "";
  // The editor is uncontrolled — we read the latest markdown via the ref at
  // submit/switch time. `hasContent` mirrors emptiness so the Create button
  // can disable correctly without a controlled-input rerender on every keystroke.
  const editorRef = useRef<ContentEditorRef>(null);
  const creatorRef = useRef<HTMLDivElement>(null);
  const [recommendationText, setRecommendationText] = useState(initialPrompt);
  const descriptionScrollRef = useRef<HTMLDivElement>(null);
  const descriptionContentRef = useRef<HTMLDivElement>(null);
  const [hasContent, setHasContent] = useState(initialPrompt.trim().length > 0);
  const [justSent, setJustSent] = useState(false);
  const [sentCount, setSentCount] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const pickActor = (next: QuickCreateActorRef) => {
    setActor(next); setAgent({ actorType: next.type, actorId: next.id }); setError(null);
  };
  const uploadGate = useUploadGate(editorRef);
  // Coordinator-owned uploads in the shared draft pool (MUL-5181, L2): a file
  // pasted into the prompt survives dialog close and mode switches, aborts on
  // logout, and is dropped after a reload. `gate` widens the editor gate with
  // the pool's placeholders.
  const {
    attachments: pendingAttachments,
    handleUpload: handleUploadFile,
    gate,
  } = useIssueCreateUploads("agent", uploadGate, editorRef);
  const { isDragOver, dropZoneProps } = useFileDropZone({
    onDrop: (files) => files.forEach((f) => editorRef.current?.uploadFile(f)),
  });

  useEffect(() => {
    // Defer focus so it lands after the dialog's focus trap has settled —
    // otherwise the trap can bounce focus back to the first focusable header
    // button on the next tick.
    const id = requestAnimationFrame(() => editorRef.current?.focus());
    return () => cancelAnimationFrame(id);
  }, []);

  // Agent create runs through the shared await-then-render composer contract
  // (single-flight ref, submit-time upload re-check, lock+spin, await→boolean,
  // clear only on acceptance). The prompt IS the editor content, so this maps
  // onto the hook directly.
  // Stale-submit guard (MUL-5181 P0): the issue draft is a SINGLETON store.
  // A late success from a dialog the user closed mid-submit must not clear a
  // newer draft typed after reopening — see ManualCreatePanel for the rule.
  const mountedRef = useRef(true);
  useLayoutEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  const submittedDraftRef = useRef<IssueCreateDraft | null>(null);
  // Set by `onAccepted` only on the branch that keeps the panel open and wipes
  // the editor; read back by `afterAccepted`. The closing branch must not
  // refocus an editor that is about to unmount with the dialog.
  const refocusAfterAcceptRef = useRef(false);

  const composer = useComposerSubmit({
    editorRef,
    uploadGate: gate,
    onSubmit: async (md): Promise<boolean> => {
      // The button already disables on !actor / versionBlocked, but the
      // ⌘+Enter path bypasses it — re-guard here and keep the draft in place.
      if (!actor || versionBlocked || autonomyBlocked || (anchorCommentId && !sourcePreview)) return false;
      // Flush the prompt editor's pending debounce before snapshotting — see
      // ManualCreatePanel.
      const pendingPrompt = editorRef.current?.flushPendingUpdate?.();
      if (pendingPrompt != null) setAgent({ prompt: pendingPrompt });
      submittedDraftRef.current = useIssueDraftStore.getState().draft;
      const submittedActor = actor;
      const submittedScope = captureQuickCreateScope(wsId, userId);
      const activeAttachmentIds = pendingAttachments
        .filter((a) => contentReferencesAttachment(md, a))
        .map((a) => a.id);
      setError(null);
      try {
        if (anchorCommentId && sourcePreview) {
          await api.createCommentSubIssue(anchorCommentId, {
            mode: "agent",
            capture_token: sourcePreview.capture_token,
            quick_create: {
              ...(actor.type === "agent"
                ? { agent_id: actor.id }
                : { squad_id: actor.id }),
              prompt: md,
              project_id: projectId ?? undefined,
              ...(priority !== "none" ? { priority } : {}),
              ...(dueDate ? { due_date: dueDate } : {}),
              ...(activeAttachmentIds.length > 0 ? { attachment_ids: activeAttachmentIds } : {}),
            },
          });
        } else {
          await api.quickCreateIssue({
            ...(actor.type === "agent"
              ? { agent_id: actor.id }
              : { squad_id: actor.id }),
            prompt: md,
            project_id: projectId ?? undefined,
            ...(priority !== "none" ? { priority } : {}),
            ...(dueDate ? { due_date: dueDate } : {}),
            parent_issue_id: parentIssueId,
            ...(activeAttachmentIds.length > 0 ? { attachment_ids: activeAttachmentIds } : {}),
          });
        }
        // Acceptance can arrive after workspace or account changes. Keep
        // the successful request, but never write into a replacement scope.
        if (submittedScope && isQuickCreateScopeCurrent(submittedScope)) {
          recordSuccessfulActor(submittedActor, submittedScope);
          setLastMode("agent");
        }
        toast.success(t(($) => $.create_issue.agent.toast_sent), {
          duration: 4000,
        });
        return true;
      } catch (e) {
        // Server returns 422 with { code, ... } for the structured rejection
        // paths the modal cares about. Surface the reason in-modal so the
        // user can switch to a live agent / upgrade their daemon without
        // leaving the flow.
        if (e instanceof ApiError && e.body && typeof e.body === "object") {
          const body = e.body as {
            code?: string;
            reason?: string;
            current_version?: string;
            min_version?: string;
            autonomy_level?: unknown;
            required_autonomy?: unknown;
          };
          if (body.code === "issue_limit_reached") {
            showIssueLimitUpgradePrompt();
            return false;
          }
          if (body.code === "agent_unavailable") {
            setError(body.reason || t(($) => $.create_issue.agent.error_agent_unavailable_fallback));
            return false;
          }
          if (body.code === "agent_autonomy_insufficient") {
            // Race fallback for a level that changed after the list loaded.
            // Refetch so the picker disables the creator and the notice offers
            // the manual path; otherwise the same submit is refused again.
            // Without a level to name, the server's own sentence still says
            // what to do.
            void agentsQuery.refetch();
            const level = typeof body.autonomy_level === "string" ? body.autonomy_level : "";
            const required = typeof body.required_autonomy === "string" ? body.required_autonomy : QUICK_CREATE_MIN_AUTONOMY;
            setError(level
              ? t(($) => $.create_issue.agent.error_autonomy, { level: autonomyLabel(level), required: autonomyLabel(required) })
              : e.message || t(($) => $.create_issue.agent.error_unknown));
            return false;
          }
          if (body.code === "daemon_version_unsupported") {
            // Race fallback: the picker pre-check should normally catch this,
            // but a runtime can silently re-register with an older CLI between
            // pre-check and submit. Same wording as the inline notice for
            // consistency.
            const cur = body.current_version || "unknown";
            setError(
              t(($) => $.create_issue.agent.error_daemon_version, {
                current: cur,
                min: body.min_version || versionCheck.min,
              }),
            );
            return false;
          }
          if (anchorCommentId && (
            body.code === "source_context_changed"
            || body.code === "anchor_comment_deleted"
            || body.code === "source_issue_deleted"
          )) {
            await refetchSourceContext?.();
            setError(sourceContextFailureMessage(e) ?? tIssues(($) => $.source_context.error_source_changed));
            return false;
          }
          if (anchorCommentId && body.code === "source_context_quick_create_unsupported") {
            setError(tIssues(($) => $.source_context.error_agent_unsupported));
            return false;
          }
          if (anchorCommentId && body.code === "source_context_server_unsupported") {
            setError(tIssues(($) => $.source_context.error_server_unsupported));
            return false;
          }
          if (anchorCommentId && body.code === "source_context_too_large") {
            setError(sourceContextFailureMessage(e) ?? tIssues(($) => $.source_context.error_too_large));
            return false;
          }
        }
        setError(
          e instanceof Error && e.message
            ? e.message
            : t(($) => $.create_issue.agent.error_unknown),
        );
        return false;
      }
    },
    // Continuous-creation mode puts the caret back so the next prompt can be
    // typed immediately. Deliberately no `containerRef`: this is a dialog whose
    // own focus trap already bounds where focus can be, and reclaiming it is the
    // point of keep-open mode.
    afterAccepted: () => (refocusAfterAcceptRef.current ? "refocus" : "none"),
    onAccepted: () => {
      refocusAfterAcceptRef.current = false;
      // A successful create ends this whole draft (shared + manual + agent);
      // last-successful actor/project preferences were saved in onSubmit.
      // Success may only consume the draft it submitted: flush the editor's
      // pending debounce first, then clear only an untouched draft — edits
      // made mid-flight or by a reopened dialog survive.
      const latePrompt = editorRef.current?.flushPendingUpdate?.();
      if (latePrompt != null) setAgent({ prompt: latePrompt });
      const untouched =
        useIssueDraftStore.getState().draft === submittedDraftRef.current;
      if (untouched) clearDraft();
      // An edit made during the request survives; the panel stays open on it.
      if (!mountedRef.current || !untouched) return;
      if (keepOpen) {
        // Stay open for continuous creation — clear the editor so the user can
        // immediately type the next prompt.
        editorRef.current?.clearContent();
        setHasContent(false);
        setRecommendationText("");
        setSentCount((c) => c + 1);
        setJustSent(true);
        setTimeout(() => setJustSent(false), 1500);
        refocusAfterAcceptRef.current = true;
      } else {
        onClose();
      }
    },
  });
  const submit = () => {
    void composer.submit();
  };
  const submitting = composer.submitting;

  // Switch to the manual form WITHOUT destroying the agent draft. The agent
  // slot (prompt + actor) is left untouched so a later manual→agent flip
  // restores it verbatim. Project / priority / due date already live in the
  // shared slot and carry across for free. Two one-time assist-inits run only
  // when the manual slot is still empty: seed the description from the prompt
  // and the assignee from the picked actor. The parent-issue context is not
  // persisted (a per-invocation intent) so it rides the carry channel.
  // `assignTo` comes from the autonomy notice, whose button promises that
  // assignee, so it replaces one left in the manual draft.
  const switchToManual = (assignTo?: QuickCreateActorRef) => {
    // The prompt is copied into the manual description on assist-init; mid-upload
    // that body has already lost the pending image (see switchToAgent).
    if (gate.isBlocked()) return;
    // Commit the shared fields to the draft so the manual panel reads them from
    // there — local state can hold a value seeded from `data` that was never
    // written through a picker.
    setShared({ projectId: projectId ?? undefined, priority, dueDate });
    if (!draft.manual.description.trim()) {
      const md = editorRef.current?.getMarkdown() ?? "";
      if (md) setManual({ description: md });
    }
    if (assignTo) {
      setManual({ assigneeType: assignTo.type, assigneeId: assignTo.id });
    } else if (!draft.manual.assigneeId && actor) {
      setManual({ assigneeType: actor.type, assigneeId: actor.id });
    }
    setLastMode("manual");
    setActiveMode("manual");
    const carry: Record<string, unknown> = {};
    if (parentIssueId) carry.parent_issue_id = parentIssueId;
    if (parentIssueIdentifier) carry.parent_issue_identifier = parentIssueIdentifier;
    onSwitchMode?.(Object.keys(carry).length > 0 ? carry : null);
  };

  // Field visibility lives in Settings → Preferences. Persist the prompt draft
  // before leaving so what the user typed survives the round-trip, then
  // close — the dialog would otherwise linger over the settings page.
  const openFieldSettings = (e: React.MouseEvent) => {
    // Persist the draft either way, but only an in-place navigation closes
    // the dialog — a modifier click opens Settings in another tab and the
    // modal stays put.
    setAgent({ prompt: editorRef.current?.getMarkdown() ?? "" });
    if (resolveClickIntent(e) !== "push") return;
    onClose();
  };

  return (
    <>
        <DialogTitle className="sr-only">{t(($) => $.create_issue.sr_agent)}</DialogTitle>

        {/* Header */}
        <div className="flex items-center justify-between px-5 pt-3 pb-2 shrink-0">
          <div className="flex min-w-0 items-center gap-1.5 text-caption">
            <span className="truncate text-muted-foreground" title={workspaceName}>{workspaceName}</span>
            <ChevronRight aria-hidden className="size-3 shrink-0 text-faint-foreground" />
            <span className="shrink-0 font-medium">{t(($) => $.create_issue.agent_breadcrumb)}</span>
          </div>
          {/* Native `title` instead of Base UI Tooltip — Tooltip opens on
              keyboard focus, and the dialog's focus trap briefly lands focus
              on the first focusable element on mount, causing the tooltip to
              auto-pop every open. Same workaround applies to expand. */}
          <div className="flex shrink-0 items-center gap-1">
            <button
              type="button"
              onClick={() => setIsExpanded(!isExpanded)}
              title={isExpanded ? t(($) => $.common.collapse_tooltip) : t(($) => $.common.expand_tooltip)}
              aria-label={isExpanded ? t(($) => $.common.collapse_tooltip) : t(($) => $.common.expand_tooltip)}
              className="flex size-8 items-center justify-center rounded-sm opacity-70 max-sm:size-11 pointer-coarse:size-11 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring hover:opacity-100 hover:bg-accent/60 transition-all cursor-pointer"
            >
              {isExpanded ? <Minimize2 className="size-4" /> : <Maximize2 className="size-4" />}
            </button>
            <button
              type="button"
              onClick={onClose}
              title={t(($) => $.common.close)}
              aria-label={t(($) => $.common.close)}
              className="flex size-8 items-center justify-center rounded-sm opacity-70 max-sm:size-11 pointer-coarse:size-11 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring hover:opacity-100 hover:bg-accent/60 transition-all cursor-pointer"
            >
              <XIcon className="size-4" />
            </button>
          </div>
        </div>

        {/* Actor picker — agents and squads in one searchable list. Squads
            route to their leader agent on the backend; the leader runs the
            quick-create flow with the squad's Operating Protocol layered
            on top, so a squad pick is "ask this squad to file the issue". */}
        <div ref={creatorRef} className="px-5 pt-1 pb-2 shrink-0">
          <CreatorRecommendations key={wsId} wsId={wsId} value={recommendationText} projectId={projectId} actor={actor}
            identityKey={`${wsId}:${userId}:${authStatus}:${resetGeneration}`} editorRef={editorRef} actors={recommendationActors} onNeedsSpace={onNeedsSpace}
            onPick={(next) => {
              creatorRef.current?.querySelector<HTMLButtonElement>("button[data-slot=popover-trigger]")?.focus();
              pickActor(next);
            }}>
            <QuickCreateActorPicker
              actor={actor}
              visibleAgents={visibleAgents}
              visibleSquads={visibleSquads}
              selectedAgent={selectedAgent}
              selectedSquad={selectedSquad}
              favoriteActors={favoriteActors}
              recentActors={recentActors}
              preferencesReady={preferencesReady}
              defaultActor={defaultActor}
              onSetDefault={(ref) => {
                const scope = captureQuickCreateScope(wsId, userId);
                if (scope) setDefaultActor(ref, scope);
              }}
              projectActors={projectActors}
              projectState={projectId ? {
                pending: projectsQuery.isPending === true,
                error: projectsQuery.isError === true,
                hasData: projectsQuery.data !== undefined,
                onRetry: () => { void projectsQuery.refetch(); },
              } : undefined}
              onToggleFavorite={(ref) => {
                const scope = captureQuickCreateScope(wsId, userId);
                if (scope) toggleFavoriteActor(ref, scope);
              }}
              agentState={{
                pending: agentsQuery.isPending === true || membersQuery.isPending === true,
                error: agentsQuery.isError === true || membersQuery.isError === true,
                hasData: agentsQuery.data !== undefined && membersQuery.data !== undefined,
                onRetry: () => { void agentsQuery.refetch(); void membersQuery.refetch(); },
              }}
              squadState={{
                pending: squadsQuery.isPending === true,
                error: squadsQuery.isError === true,
                hasData: squadsQuery.data !== undefined,
                onRetry: () => { void squadsQuery.refetch(); },
              }}
              onPick={pickActor}
              disabledReasons={creatorDisabledReasons}
            />
          </CreatorRecommendations>
          <p className="mt-1 text-caption text-muted-foreground">
            {t(($) => $.create_issue.actor_picker.creator_hint)}
          </p>
        </div>

        {actor && autonomyBlocked && (() => {
          const name = (actor.type === "squad" ? selectedSquad?.name : selectedAgent?.name) ?? "";
          const values = { name, level: autonomyLabel(blockedLevel ?? ""), required: autonomyLabel(QUICK_CREATE_MIN_AUTONOMY) };
          return (
            <div role="status" className="mx-5 mb-2 shrink-0 space-y-1.5 rounded-md border border-warning/30 bg-warning/10 px-3 py-2 text-caption text-warning-foreground">
              <p className="font-medium wrap-anywhere">{t(($) => $.create_issue.agent.autonomy_blocked_title, values)}</p>
              <p className="wrap-anywhere">{actor.type === "squad"
                ? t(($) => $.create_issue.agent.autonomy_blocked_reason_squad, values)
                : t(($) => $.create_issue.agent.autonomy_blocked_reason_agent, values)}</p>
              <p className="wrap-anywhere">{t(($) => $.create_issue.agent.autonomy_blocked_next, values)}</p>
              <Button type="button" size="sm" variant="outline" className="max-w-full max-sm:min-h-11 pointer-coarse:min-h-11"
                disabled={gate.uploading} onClick={() => switchToManual(actor)}>
                <PencilLine aria-hidden className="size-3.5" />
                <span className="truncate">{t(($) => $.create_issue.agent.autonomy_blocked_action, values)}</span>
              </Button>
            </div>
          );
        })()}

        {selectedAgent && versionBlocked && (
          <div role="status" className="mx-5 mb-2 shrink-0 rounded-md border border-warning/30 bg-warning/10 px-3 py-2 text-caption text-warning-foreground">
            {versionCheck.state === "missing"
              ? t(($) => $.create_issue.agent.version_missing, { min: versionCheck.min })
              : t(($) => $.create_issue.agent.version_below, {
                  current: versionCheck.current,
                  min: versionCheck.min,
                })}
          </div>
        )}

        {/* Prompt — same rich editor Advanced uses, so paste/drop images,
            mentions, and formatting all work. The dropZone wrapper enables
            drag-and-drop file uploads alongside paste. */}
        {/* `flex-1 min-h-0 overflow-y-auto` so the editor area absorbs the
            remaining vertical space inside the (now max-bounded) DialogContent
            and scrolls internally. Without it, pasting an image expanded the
            editor unbounded and pushed the modal past the viewport. */}
        <div
          {...dropZoneProps}
          ref={descriptionScrollRef}
          className="relative px-5 pb-3 flex flex-col flex-1 min-h-0 overflow-y-auto"
        >
          <p id={promptLabelId} className="mb-2 text-label font-medium">{t(($) => $.create_issue.agent.prompt_label)}</p>
          <div ref={descriptionContentRef} className="flex min-h-28 flex-auto shrink-0 flex-col [&>div]:min-h-0">
            <ContentEditor
              ref={editorRef}
              aria-labelledby={promptLabelId}
              defaultValue={initialPrompt}
              placeholder={anchorCommentId
                ? t(($) => $.create_issue.agent.source_context_prompt_placeholder)
                : getQuickCreateScenario(t, selectedAgent, selectedSquad)}
              onUpdate={(md) => {
                setHasContent(md.trim().length > 0);
                setRecommendationText(md);
                setAgent({ prompt: md });
              }}
              onUploadFile={handleUploadFile}
              onUploadingChange={uploadGate.onUploadingChange}
              attachments={pendingAttachments}
              onSubmit={submit}
              debounceMs={150}
            />
          </div>
          <IssueDescriptionAssist
            key={`${wsId}:${sentCount}`}
            wsId={wsId}
            mode="agent"
            editorRef={editorRef}
            onNeedsSpace={onNeedsSpace}
            onRevealEditor={(position) => {
              requestAnimationFrame(() => {
                const scroll = descriptionScrollRef.current;
                const content = descriptionContentRef.current;
                if (scroll && content) {
                  scroll.scrollTop = position === "start"
                    ? 0
                    : Math.max(0, content.offsetHeight - scroll.clientHeight);
                }
              });
            }}
            value={initialPrompt}
            attachments={pendingAttachments}
            onChange={(prompt) => {
              setHasContent(prompt.trim().length > 0);
              setRecommendationText(prompt);
              setAgent({ prompt });
            }}
            uploading={gate.uploading}
            isBlocked={gate.isBlocked}
            submitting={submitting}
          />
          {isDragOver && <FileDropOverlay />}
        </div>

        {anchorCommentId && (
          <SourceContextPreviewCard
            preview={sourcePreview}
            loading={sourceContextLoading}
            failed={sourceContextFailed}
            error={sourceContextError}
            onRetry={refetchSourceContext ? () => { void refetchSourceContext(); } : undefined}
            constrainToParent
            expanded={sourceContextExpanded}
            onExpandedChange={onSourceContextExpandedChange}
          />
        )}

        {error && (
          <div role="alert" className="px-5 pb-2 text-caption text-destructive">{error}</div>
        )}

        {/* Property toolbar — the project is visible by default; priority and
            due date live behind the overflow until exposed in settings or
            given a value. Unfinished picks remain workspace-persistent.
            When the modal was opened from "Add sub issue" on an existing
            issue, a read-only chip on the same row tells the user that the
            new issue will be filed as a sub-issue of that parent — the agent
            handles the wiring silently, but surfacing the relationship
            avoids "where did this end up?" surprise. We deliberately keep
            it non-editable: changing the parent is a `Set parent` action on
            the parent itself, not a knob in the quick-create flow. */}
        <div className="flex items-center gap-1.5 px-4 pb-2 shrink-0 flex-wrap">
          {(visibleFields.includes("project") ||
            projectId !== null ||
            fieldPickerOpen === "project") && (
            <ProjectPicker
              projectId={projectId}
              onUpdate={(u) => commitProject(u.project_id ?? null)}
              triggerRender={
                <ClearablePillButton
                  className="max-sm:min-h-11 pointer-coarse:min-h-11 [&>button]:max-sm:min-h-11 [&>button]:pointer-coarse:min-h-11"
                  onClear={projectId !== null ? () => commitProject(null) : undefined}
                  clearLabel={tProjects(($) => $.picker.clear_aria)}
                />
              }
              align="start"
              open={fieldPickerOpen === "project" ? true : undefined}
              onOpenChange={(open) => setFieldPickerOpen(open ? "project" : null)}
            />
          )}
          {(visibleFields.includes("priority") ||
            priority !== "none" ||
            fieldPickerOpen === "priority") && (
            <PriorityPicker
              priority={priority}
              onUpdate={(updates) => {
                if (updates.priority) {
                  setPriority(updates.priority);
                  setShared({ priority: updates.priority });
                }
              }}
              triggerRender={<PillButton className="max-sm:min-h-11 pointer-coarse:min-h-11" />}
              align="start"
              open={fieldPickerOpen === "priority" ? true : undefined}
              onOpenChange={(open) => setFieldPickerOpen(open ? "priority" : null)}
            />
          )}
          {(visibleFields.includes("due_date") ||
            dueDate !== null ||
            fieldPickerOpen === "due_date") && (
            <DueDatePicker
              dueDate={dueDate}
              onUpdate={(updates) => {
                const next = updates.due_date ?? null;
                setDueDate(next);
                setShared({ dueDate: next });
              }}
              triggerRender={<PillButton className="max-sm:min-h-11 pointer-coarse:min-h-11" />}
              align="start"
              open={fieldPickerOpen === "due_date" ? true : undefined}
              onOpenChange={(open) => setFieldPickerOpen(open ? "due_date" : null)}
            />
          )}
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <PillButton
                  className="max-sm:min-h-11 max-sm:min-w-11 pointer-coarse:min-h-11 pointer-coarse:min-w-11"
                  aria-label={t(($) => $.create_issue.agent.more_fields_aria)}
                  title={t(($) => $.create_issue.agent.more_fields_aria)}
                >
                  <MoreHorizontal className="size-3.5" />
                </PillButton>
              }
            />
            <DropdownMenuContent align="start" className="w-52">
              {!visibleFields.includes("project") && projectId === null && (
                <DropdownMenuItem onClick={() => setFieldPickerOpen("project")}>
                  <FolderKanban className="size-3.5 text-muted-foreground" />
                  {t(($) => $.create_issue.agent.set_project)}
                </DropdownMenuItem>
              )}
              {!visibleFields.includes("priority") && priority === "none" && (
                <DropdownMenuItem onClick={() => setFieldPickerOpen("priority")}>
                  <PriorityIcon priority="none" className="size-3.5" />
                  {t(($) => $.create_issue.agent.set_priority)}
                </DropdownMenuItem>
              )}
              {!visibleFields.includes("due_date") && dueDate === null && (
                <DropdownMenuItem onClick={() => setFieldPickerOpen("due_date")}>
                  <CalendarDays className="size-3.5 text-muted-foreground" />
                  {t(($) => $.create_issue.agent.set_due_date)}
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                render={
                  <AppLink
                    href={`${workspacePaths.settings()}?tab=preferences`}
                    onClick={openFieldSettings}
                  />
                }
              >
                <Settings2 className="size-3.5 text-muted-foreground" />
                {t(($) => $.create_issue.agent.customize_fields)}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          {parentIssueId && (
            <span
              data-testid="agent-sub-issue-chip"
              className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-caption text-muted-foreground"
              title={t(($) => $.create_issue.agent.sub_issue_of, {
                identifier: parentIssueIdentifier ?? "",
              })}
            >
              {t(($) => $.create_issue.agent.sub_issue_of, {
                identifier: parentIssueIdentifier ?? "",
              })}
            </span>
          )}
        </div>

        {/* Footer. Two layouts, one flat child list:
            - Phones get a 2x2 grid — attach / switch on the top row, keep-open
              toggle / Create on the bottom one. Laid out as a single row the
              four controls need ~383px of the 398px a 430px phone has left
              after padding, which reads as jammed and wraps outright below
              ~410px (MUL-6236).
            - From `sm` up it is the original single flex row: `mr-auto` on the
              attach group reproduces what `justify-between` did when the
              children were two wrapper divs, and `justify-self-end` goes
              inert on flex items. */}
        <div className="grid grid-cols-[auto_1fr] items-center gap-x-2 gap-y-2.5 border-t px-4 py-3 shrink-0 sm:flex sm:flex-wrap">
          <div className="flex min-h-7 items-center gap-2 sm:mr-auto">
            {/* Deliberately NOT disabled while uploading: each file is its
                own queue entry, so queueing a second one is safe and waiting
                for the first to land just to attach the next is busywork. */}
            <FileUploadButton
              size="sm"
              className="max-sm:size-11 pointer-coarse:size-11"
              multiple
              onSelect={(file) => editorRef.current?.uploadFile(file)}
            />
            {keepOpen && sentCount > 0 && (
              <span role="status" className="text-caption text-success-foreground">
                {t(($) => $.create_issue.agent.sent_count, { count: sentCount })}
              </span>
            )}
          </div>
          <button
            type="button"
            onClick={() => switchToManual()}
            disabled={gate.uploading}
            aria-disabled={gate.uploading || undefined}
            aria-busy={gate.uploading || undefined}
            title={t(($) => $.create_issue.switch_to_manual_tooltip)}
            className="pen-sheen flex shrink-0 items-center gap-1.5 justify-self-end min-h-7 text-caption px-2 py-1 rounded-sm text-muted-foreground bg-muted/80 max-sm:min-h-11 pointer-coarse:min-h-11 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring hover:text-foreground hover:bg-muted transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
          >
            <PencilLine aria-hidden className="pen-sheen-icon size-3.5" />
            <span className="pen-sheen-stroke">{t(($) => $.create_issue.switch_to_manual)}</span>
          </button>
          <label className="flex min-h-7 shrink-0 items-center gap-1.5 text-caption text-muted-foreground cursor-pointer select-none max-sm:min-h-11 pointer-coarse:min-h-11">
            <Switch
              size="sm"
              checked={keepOpen}
              onCheckedChange={setKeepOpen}
            />
            {t(($) => $.create_issue.create_another)}
          </label>
          <Button
            size="sm"
            onClick={submit}
            disabled={!hasContent || !actor || submitting || versionBlocked || autonomyBlocked || gate.uploading || (!!anchorCommentId && !sourcePreview)}
            aria-disabled={gate.uploading || undefined}
            // Sending is a busy state too, not just uploading.
            aria-busy={gate.uploading || submitting || undefined}
            title={
              autonomyBlocked
                ? t(($) => $.create_issue.agent.autonomy_blocked_tooltip)
                : versionBlocked
                  ? t(($) => $.create_issue.agent.version_blocked_tooltip, { min: versionCheck.min })
                  : undefined
            }
            className={cn(
              "justify-self-end min-w-28 max-sm:min-h-11 pointer-coarse:min-h-11",
              justSent && "bg-success/10 text-success-foreground",
            )}
          >
            {submitting ? t(($) => $.create_issue.agent.sending) : gate.uploading ? t(($) => $.create_issue.agent.uploading) : justSent ? (
              <span className="flex items-center gap-1"><Check className="size-3.5" />{t(($) => $.create_issue.agent.sent_label)}</span>
            ) : (
              <>
                {t(($) => $.create_issue.agent.submit)}
                {sendShortcut ? (
                  // Touch phones have no ⌘ key and the narrowest footer row
                  // to spare — drop the hint at the same breakpoint the
                  // footer reflows at.
                  <ShortcutKeycaps
                    shortcut={sendShortcut}
                    decorative
                    className="ml-1 max-sm:hidden"
                    keyClassName="border-background/30 bg-background/15 text-primary-foreground shadow-none"
                  />
                ) : null}
              </>
            )}
          </Button>
        </div>
    </>
  );
}
