"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { useCurrentMember } from "@multica/core/permissions";
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
  const paths = useWorkspacePaths();
  const settings = useQuery(triageSettingsOptions(wsId));
  const counts = useQuery({
    ...triageCountsOptions(wsId),
    enabled: settings.data?.supported === true,
  });
  const { role } = useCurrentMember(wsId);
  const mutation = useUpdateTriageSettings(wsId);
  const [draft, setDraft] = useState<TriageSettings | null>(null);
  const value = draft ?? settings.data;
  const editable =
    (role === "owner" || role === "admin") && !mutation.isPending;
  const patch = (next: Partial<TriageSettings>) =>
    value && setDraft({ ...value, ...next });
  return (
    <SettingsTab
      title={t(($) => $.settings)}
      description={t(($) => $.settings_description)}
    >
      {settings.isPending ? (
        <p role="status">{t(($) => $.loading)}</p>
      ) : settings.isError ? (
        <div role="alert">
          {settings.error.message}
          <Button onClick={() => void settings.refetch()}>
            {t(($) => $.retry)}
          </Button>
        </div>
      ) : value?.supported !== true ? (
        <p>{t(($) => $.unsupported)}</p>
      ) : (
        <>
          {!editable && !mutation.isPending && <p>{t(($) => $.admin_only)}</p>}
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
      )}
    </SettingsTab>
  );
}
