"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { runtimeListOptions, isRuntimeUsableForUser } from "@multica/core/runtimes";
import type { SquadTemplate } from "@multica/core/types";
import {
  memberListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { cn } from "@multica/ui/lib/utils";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";
import { RuntimePicker } from "../agents/components/runtime-picker";
import {
  templateLanguageFor,
  useSquadTemplates,
} from "../agents/create/use-role-templates";
import { AutonomyBadge } from "../agents/create/template-create-agent-page";
import { useLocale } from "../i18n";

/**
 * Staffing a squad from a built-in template.
 *
 * One request does the whole thing on the backend, in a transaction, so this
 * modal has exactly one submit and no partial state to reconcile. What it must
 * communicate is the part a person cannot see from the result alone: an agent
 * that already exists for a role is REUSED, edits and all, rather than
 * recreated — so the success toast reports created and reused separately.
 */
export function StaffSquadTemplateModal({ onClose }: { onClose: () => void }) {
  const { t } = useT("modals");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const queryClient = useQueryClient();
  const currentUserId = useAuthStore((state) => state.user?.id) ?? null;

  const { data: templates, isLoading, isError } = useSquadTemplates();
  const { data: runtimes = [], isLoading: runtimesLoading } = useQuery(
    runtimeListOptions(wsId),
  );
  const { data: members = [] } = useQuery(memberListOptions(wsId));

  const usableRuntimes = useMemo(
    () =>
      runtimes.filter(
        (runtime) =>
          runtime.status === "online" &&
          isRuntimeUsableForUser(runtime, currentUserId),
      ),
    [currentUserId, runtimes],
  );

  const [templateKey, setTemplateKey] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [runtimeId, setRuntimeId] = useState("");
  const [workspaceAccess, setWorkspaceAccess] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const template = templates?.find((entry) => entry.key === templateKey) ?? null;
  const effectiveRuntimeId = runtimeId || usableRuntimes[0]?.id || "";

  const pick = (next: SquadTemplate) => {
    setTemplateKey(next.key);
    // Seed the name from the template but leave it editable: teams rename these.
    setName(next.title);
    setError(null);
  };

  const create = async () => {
    if (!template || !effectiveRuntimeId || creating) return;
    setCreating(true);
    setError(null);
    try {
      const staffed = await api.createSquadFromTemplate({
        template_key: template.key,
        runtime_id: effectiveRuntimeId,
        name: name.trim() || undefined,
        permission_mode: workspaceAccess ? "public_to" : "private",
        invocation_targets: workspaceAccess
          ? [{ target_type: "workspace" }]
          : [],
        language: templateLanguageFor(locale),
      });
      const squadName = staffed.squad.name || name.trim();
      // Both agents and squads changed, and a role skill was very likely
      // materialized — invalidate all three rather than guessing.
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: workspaceKeys.squads(wsId) }),
        queryClient.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) }),
        queryClient.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) }),
      ]);
      toast.success(
        staffed.reused_agent_ids.length > 0
          ? t(($) => $.squad_templates.created, {
              name: squadName,
              created: staffed.created_agent_ids.length,
              reused: staffed.reused_agent_ids.length,
            })
          : t(($) => $.squad_templates.created_no_reuse, {
              name: squadName,
              created: staffed.created_agent_ids.length,
            }),
      );
      onClose();
      if (staffed.squad.id) navigation.push(paths.squadDetail(staffed.squad.id));
    } catch (caught) {
      setError(
        caught instanceof Error && caught.message
          ? caught.message
          : t(($) => $.squad_templates.create_failed),
      );
      setCreating(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.squad_templates.title)}</DialogTitle>
          <DialogDescription>
            {t(($) => $.squad_templates.description)}
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[60vh] space-y-4 overflow-y-auto">
          {isError ? (
            <p className="text-body text-muted-foreground">
              {t(($) => $.squad_templates.load_failed)}
            </p>
          ) : isLoading ? (
            <p className="text-body text-muted-foreground">…</p>
          ) : (templates ?? []).length === 0 ? (
            <p className="text-body text-muted-foreground">
              {t(($) => $.squad_templates.empty)}
            </p>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2">
              {(templates ?? []).map((entry) => (
                <button
                  key={entry.key}
                  type="button"
                  aria-pressed={entry.key === templateKey}
                  onClick={() => pick(entry)}
                  className={cn(
                    "flex h-full flex-col items-start rounded-lg border bg-card p-3 text-left transition-colors",
                    "hover:border-primary/40 hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                    // The selected card must stay identifiable while hovered, so
                    // selection is carried by the border and text weight rather
                    // than by a background hover also uses.
                    entry.key === templateKey && "border-primary bg-accent/20",
                  )}
                >
                  <span className="flex items-center gap-2">
                    <span aria-hidden="true">{entry.avatar_emoji}</span>
                    <span
                      className={cn(
                        "text-body",
                        entry.key === templateKey
                          ? "font-semibold text-foreground"
                          : "font-medium",
                      )}
                    >
                      {entry.title}
                    </span>
                  </span>
                  <span className="mt-1.5 text-caption leading-5 text-muted-foreground">
                    {entry.description}
                  </span>
                </button>
              ))}
            </div>
          )}

          {template ? (
            <SquadTemplateRoster template={template} />
          ) : null}

          {template ? (
            <div className="space-y-3 border-t pt-4">
              <div className="space-y-1.5">
                <Label htmlFor="staff-squad-name">
                  {t(($) => $.squad_templates.name_label)}
                </Label>
                <Input
                  id="staff-squad-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                />
              </div>

              <div className="space-y-1.5">
                <Label>{t(($) => $.squad_templates.runtime_label)}</Label>
                {usableRuntimes.length === 0 && !runtimesLoading ? (
                  <p className="text-caption text-muted-foreground">
                    {t(($) => $.squad_templates.no_runtime)}
                  </p>
                ) : (
                  <>
                    <RuntimePicker
                      runtimes={runtimes}
                      runtimesLoading={runtimesLoading}
                      members={members}
                      currentUserId={currentUserId}
                      selectedRuntimeId={effectiveRuntimeId}
                      onSelect={setRuntimeId}
                    />
                    <p className="text-micro text-muted-foreground">
                      {t(($) => $.squad_templates.runtime_hint)}
                    </p>
                  </>
                )}
              </div>

              <div className="space-y-1.5">
                <Label>{t(($) => $.squad_templates.access_label)}</Label>
                <div className="flex gap-2">
                  {[false, true].map((value) => (
                    <Button
                      key={String(value)}
                      type="button"
                      size="sm"
                      variant={workspaceAccess === value ? "default" : "outline"}
                      onClick={() => setWorkspaceAccess(value)}
                    >
                      {value
                        ? t(($) => $.squad_templates.access_workspace)
                        : t(($) => $.squad_templates.access_private)}
                    </Button>
                  ))}
                </div>
                <p className="text-micro text-muted-foreground">
                  {t(($) => $.squad_templates.access_hint)}
                </p>
              </div>
            </div>
          ) : null}

          {error ? (
            <p className="text-caption text-destructive">{error}</p>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={creating}>
            {t(($) => $.common.cancel)}
          </Button>
          <Button
            onClick={() => void create()}
            disabled={!template || !effectiveRuntimeId || creating}
          >
            {creating
              ? t(($) => $.squad_templates.creating)
              : t(($) => $.squad_templates.create)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The roster a template will staff: the leader, then each seat with the role
 * note the leader routes by. Shown before creating because the roster IS the
 * template — a person choosing between two of these is choosing between rosters.
 */
function SquadTemplateRoster({ template }: { template: SquadTemplate }) {
  const { t } = useT("modals");
  return (
    <div className="space-y-2 rounded-lg border bg-muted/30 p-3">
      <div className="text-caption font-medium">
        {t(($) => $.squad_templates.roster_label)}
      </div>
      <ul className="space-y-1.5">
        <li className="flex flex-wrap items-center gap-2 text-caption">
          <span aria-hidden="true">{template.leader.avatar_emoji}</span>
          <span className="font-medium">{template.leader.title}</span>
          <span className="rounded-full border px-1.5 py-0.5 text-micro text-muted-foreground">
            {t(($) => $.squad_templates.leader_label)}
          </span>
          <AutonomyBadge level={template.leader.autonomy_level} />
        </li>
        {template.members.map((member) => (
          <li
            key={member.template_key}
            className="flex flex-wrap items-center gap-2 text-caption"
          >
            <span aria-hidden="true">{member.avatar_emoji}</span>
            <span className="font-medium">{member.title}</span>
            <AutonomyBadge level={member.autonomy_level} />
            <span className="text-muted-foreground">{member.role}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
