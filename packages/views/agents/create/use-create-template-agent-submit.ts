"use client";

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { buildInvocationTargets, type AgentDraft } from "@multica/core/agents";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  cacheAgentResponse,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { classifyAgentCreateError } from "./use-create-agent-submit";
import { templateLanguageFor } from "./use-role-templates";
import { useLocale } from "../../i18n";

/**
 * Commits a role-template agent and leaves the creation flow.
 *
 * Deliberately a separate hook from useCreateAgentSubmit rather than a flag on
 * it: the request bodies have almost nothing in common. This one sends a
 * template key and the few things a person actually chose — the instructions,
 * skills, concurrency and autonomy level all come from the backend, which is
 * what makes the provenance on the created row trustworthy.
 *
 * Not optimistic, for the same reason as the manual flow: it navigates to the
 * new agent, so the agent has to exist before the destination renders.
 */
export function useCreateTemplateAgentSubmit(options: {
  templateKey: string | null;
  draft: AgentDraft;
  runtimeId: string | null;
  squadId: string | null;
}) {
  const { t } = useT("agents");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const qc = useQueryClient();

  const [creating, setCreating] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const { templateKey, draft, runtimeId, squadId } = options;

  const create = async () => {
    if (!templateKey || !runtimeId || creating) return;
    setCreating(true);
    setNameError(null);
    setFormError(null);
    try {
      const agent = await api.createAgentFromTemplate({
        template_key: templateKey,
        runtime_id: runtimeId,
        name: draft.name.trim() || undefined,
        model: draft.model.trim() || undefined,
        thinking_level: draft.thinkingLevel.trim() || undefined,
        service_tier: draft.serviceTier.trim() || undefined,
        permission_mode:
          draft.permissionScope === "private" ? "private" : "public_to",
        invocation_targets: buildInvocationTargets(draft),
        language: templateLanguageFor(locale),
      });
      if (!agent.id) throw new Error(t(($) => $.creation_studio.create_failed));

      if (squadId) {
        try {
          await api.addSquadMember(squadId, {
            member_type: "agent",
            member_id: agent.id,
          });
          await Promise.all([
            qc.invalidateQueries({
              queryKey: [...workspaceKeys.squads(wsId), squadId, "members"],
            }),
            qc.invalidateQueries({
              queryKey: [...workspaceKeys.squads(wsId), squadId],
            }),
          ]);
        } catch (error) {
          // The agent is already committed; failing the create would invite a
          // retry that produces a second one.
          toast.warning(
            t(($) => $.create_dialog.squad_join_failed_toast, {
              name: agent.name || draft.name.trim(),
              error: error instanceof Error ? error.message : "unknown error",
            }),
          );
        }
      }

      cacheAgentResponse(qc, wsId, agent);
      // Reconcile the other list projections in the background: the create
      // response is authoritative enough to open immediately, and a skill was
      // very likely just materialized into the workspace.
      void qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
      void qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
      toast.success(
        t(($) => $.creation_studio.created, {
          name: agent.name || draft.name.trim(),
        }),
      );
      navigation.push(
        squadId ? paths.squadDetail(squadId) : paths.agentDetail(agent.id),
      );
    } catch (error) {
      const nextErrors = classifyAgentCreateError(
        error,
        t(($) => $.creation_studio.create_failed),
        t(($) => $.creation_studio.name_conflict),
      );
      setNameError(nextErrors.nameError);
      setFormError(nextErrors.formError);
      setCreating(false);
    }
  };

  return {
    create,
    creating,
    nameError,
    formError,
    clearNameError: () => setNameError(null),
  };
}
