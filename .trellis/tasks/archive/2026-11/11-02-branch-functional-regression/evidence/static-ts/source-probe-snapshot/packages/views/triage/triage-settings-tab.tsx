"use client";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { useAuthStore } from "@multica/core/auth";
import { memberListOptions } from "@multica/core/workspace/queries";
import {
  triageSettingsOptions,
  triageCountsOptions,
  useUpdateTriageSettings,
  type TriageSettings,
} from "@multica/core/triage";
import { Switch } from "@multica/ui/components/ui/switch";
import { Button } from "@multica/ui/components/ui/button";
import {
  SettingsTab,
  SettingsSection,
  SettingsCard,
  SettingsRow,
} from "../settings/components/settings-layout";
import { useT } from "../i18n";
import { getSettingsQueryState, SettingsQueryFeedback } from "../settings/components/settings-query-feedback";
import {
  AcceptanceStatusSelect,
  ReviewerSelect,
  TriageSelect,
} from "./triage-fields";
import { TRIAGE_CONTROL } from "./triage-ui";

export function TriageSettingsTab() {
  const wsId = useWorkspaceId();
  return <WorkspaceTriageSettings key={wsId} wsId={wsId} />;
}

function WorkspaceTriageSettings({ wsId }: { wsId: string }) {
  const { t } = useT("triage");
  const { t: settingsT } = useT("settings");
  const paths = useWorkspacePaths();
  const userId = useAuthStore((state) => state.user?.id);
  const members = useQuery(memberListOptions(wsId));
  const memberState = getSettingsQueryState(members);
  const currentMember = memberState.hasData
    ? members.data?.find((member) => member.user_id === userId) ?? null
    : null;
  const role = currentMember?.role ?? null;
  const settings = useQuery(triageSettingsOptions(wsId));
  const settingsState = getSettingsQueryState(settings);
  const hasSettings = currentMember !== null && settingsState.hasData;
  const counts = useQuery({
    ...triageCountsOptions(wsId),
    enabled: hasSettings && settings.data?.supported === true,
  });
  const mutation = useUpdateTriageSettings(wsId);
  const [draft, setDraft] = useState<TriageSettings | null>(null);
  const value = draft ?? settings.data;
  const editable = hasSettings &&
    (role === "owner" || role === "admin") && !mutation.isPending;
  useEffect(() => {
    if (!hasSettings) setDraft(null);
  }, [hasSettings]);
  const patch = (next: Partial<TriageSettings>) =>
    editable && value && setDraft({ ...value, ...next });
  return (
    <SettingsTab
      title={t(($) => $.settings)}
      description={t(($) => $.settings_description)}
    >
      <SettingsQueryFeedback
        query={members}
        loadingMessage={settingsT(($) => $.query_feedback.permissions_loading)}
        errorMessage={settingsT(($) => $.query_feedback.permissions_load_failed)}
        refreshMessage={settingsT(($) => $.query_feedback.permissions_refresh_failed)}
      />
      {memberState.hasData && !currentMember && (
        <p role="alert">{settingsT(($) => $.query_feedback.access_denied)}</p>
      )}
      <SettingsQueryFeedback
        query={settings}
        loadingMessage={t(($) => $.loading)}
        errorMessage={t(($) => $.settings_load_failed)}
        refreshMessage={t(($) => $.settings_refresh_failed)}
      />
      {hasSettings && (value?.supported !== true ? (
        <p>{t(($) => $.unsupported)}</p>
      ) : (
        <>
          {role === "member" && <p>{t(($) => $.admin_only)}</p>}
          <SettingsSection description={t(($) => $.sources)}>
            <SettingsCard>
              <SettingsRow label={t(($) => $.enable)}>
                <Switch
                  className="after:-inset-y-3.5"
                  aria-label={t(($) => $.enable)}
                  checked={value.enabled === true}
                  disabled={
                    !editable ||
                    (value.enabled && (counts.data?.pending ?? 0) > 0)
                  }
                  onCheckedChange={(enabled) => patch({ enabled })}
                />
              </SettingsRow>
              {(counts.data?.pending ?? 0) > 0 && (
                <p className="px-4 py-3 text-caption text-muted-foreground">
                  {t(($) => $.pending_block, {
                    count: counts.data?.pending ?? 0,
                  })}
                  <AppLink
                    className="ml-2 underline"
                    href={`${paths.triage()}?view=all`}
                  >
                    {t(($) => $.review_link)}
                  </AppLink>
                </p>
              )}
              <SettingsRow label={t(($) => $.acceptance_status)} size="select">
                <AcceptanceStatusSelect
                  wsId={wsId}
                  value={value.acceptance_status}
                  onChange={(acceptance_status) => patch({ acceptance_status })}
                  disabled={!editable}
                />
              </SettingsRow>
              <SettingsRow label={t(($) => $.require_priority)}>
                <Switch
                  className="after:-inset-y-3.5"
                  aria-label={t(($) => $.require_priority)}
                  checked={value.require_priority === true}
                  disabled={!editable}
                  onCheckedChange={(require_priority) =>
                    patch({ require_priority })
                  }
                />
              </SettingsRow>
            </SettingsCard>
          </SettingsSection>
          <SettingsSection
            title={t(($) => $.responsibility)}
            description={t(($) => $.responsibility_hint)}
          >
            <SettingsCard>
              <SettingsRow
                label={t(($) => $.responsibility)}
                size="select-wide"
              >
                <TriageSelect
                  label={t(($) => $.responsibility)}
                  value={value.responsibility_mode}
                  disabled={!editable}
                  onChange={(mode) => {
                    if (
                      mode === "none" ||
                      mode === "notify" ||
                      mode === "assign"
                    )
                      patch({
                        responsibility_mode: mode,
                        responsibility_member_id:
                          mode === "none"
                            ? null
                            : value.responsibility_member_id,
                      });
                  }}
                  options={(["none", "notify", "assign"] as const).map((v) => ({
                    value: v,
                    label: t(($) => $[v]),
                  }))}
                />
              </SettingsRow>
              {value.responsibility_mode !== "none" && (
                <SettingsRow label={t(($) => $.reviewer)} size="select-wide">
                  <ReviewerSelect
                    wsId={wsId}
                    value={value.responsibility_member_id}
                    onChange={(responsibility_member_id) =>
                      patch({ responsibility_member_id })
                    }
                    disabled={!editable}
                  />
                </SettingsRow>
              )}
            </SettingsCard>
          </SettingsSection>
          <div className="flex flex-wrap items-center gap-3">
            <Button
              className={TRIAGE_CONTROL}
              disabled={
                !editable ||
                !draft ||
                (value.responsibility_mode !== "none" &&
                  !value.responsibility_member_id)
              }
              onClick={async () => {
                if (!editable || !draft) return;
                try {
                  await mutation.mutateAsync({
                    enabled: value.enabled,
                    acceptance_status: value.acceptance_status,
                    require_priority: value.require_priority,
                    responsibility_mode: value.responsibility_mode,
                    responsibility_member_id: value.responsibility_member_id,
                    expected_revision: value.revision,
                  });
                  setDraft(null);
                } catch {
                  /* Keep the draft for correction or retry. */
                }
              }}
            >
              {mutation.isPending ? t(($) => $.saving) : t(($) => $.save)}
            </Button>
            {mutation.isSuccess && !draft && (
              <span role="status">{t(($) => $.saved)}</span>
            )}
          </div>
          {mutation.isError && (
            <p role="alert" className="text-destructive">
              {mutation.error.message}
            </p>
          )}
        </>
      ))}
    </SettingsTab>
  );
}
