"use client";
import { useQuery } from "@tanstack/react-query";
import {
  memberListOptions,
  agentListOptions,
} from "@multica/core/workspace/queries";
import { projectListOptions } from "@multica/core/projects/queries";
import { labelListOptions } from "@multica/core/labels/queries";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../i18n";
import { TriageSelect, TriageField } from "./triage-fields";
import { TRIAGE_CONTROL } from "./triage-ui";

export function TriageFilters({
  wsId,
  params,
  history,
  onChange,
}: {
  wsId: string;
  params: URLSearchParams;
  history: boolean;
  onChange: (key: string, value: string) => void;
}) {
  const { t } = useT("triage");
  const { t: issueT } = useT("issues");
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: agents = [] } = useQuery({
    ...agentListOptions(wsId),
    enabled: !history,
  });
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const { data: labels = [] } = useQuery(labelListOptions(wsId));
  const any = { value: "any", label: t(($) => $.any) };
  const memberOptions = [
    any,
    ...members.map((m) => ({
      value: m.user_id,
      label: m.name ?? m.email ?? m.user_id,
    })),
  ];
  const creatorOptions = [
    any,
    ...members.map((member) => ({
      value: member.user_id,
      label: t(($) => $.creator_member, {
        name: member.name ?? member.email ?? member.user_id,
      }),
    })),
    ...agents.map((agent) => ({
      value: agent.id,
      label: t(($) => $.creator_agent, { name: agent.name }),
    })),
  ];
  const select = (
    key: string,
    label: string,
    options: { value: string; label: string }[],
  ) => (
    <TriageField key={key} label={label}>
      <TriageSelect
        label={label}
        value={params.get(key) || "any"}
        onChange={(value) => onChange(key, value === "any" ? "" : value)}
        options={options}
      />
    </TriageField>
  );
  return (
    <div className="grid grid-cols-1 gap-3 px-4 pb-4 sm:grid-cols-2 lg:grid-cols-4">
      {select(
        "source",
        t(($) => $.source),
        [
          any,
          { value: "manual", label: t(($) => $.manual) },
          { value: "csv", label: t(($) => $.csv) },
        ],
      )}
      {history ? (
        <>
          {select(
            "result",
            t(($) => $.result),
            [
              any,
              ...(
                [
                  "accept",
                  "accept_and_execute",
                  "reject",
                  "duplicate",
                  "snooze",
                  "unsnooze",
                  "reopen",
                  "assign_reviewer",
                ] as const
              ).map((value) => ({ value, label: t(($) => $[value]) })),
            ],
          )}
          {select(
            "processed_by",
            t(($) => $.processed_by),
            memberOptions,
          )}
        </>
      ) : (
        <>
          {select(
            "priority",
            t(($) => $.priority),
            [
              any,
              ...(["urgent", "high", "medium", "low", "none"] as const).map(
                (value) => ({ value, label: issueT(($) => $.priority[value]) }),
              ),
            ],
          )}
          {select(
            "project_id",
            t(($) => $.project),
            [any, ...projects.map((p) => ({ value: p.id, label: p.title }))],
          )}
          {select(
            "label_id",
            t(($) => $.labels),
            [any, ...labels.map((l) => ({ value: l.id, label: l.name }))],
          )}
          {select(
            "reviewer_id",
            t(($) => $.reviewer),
            memberOptions,
          )}
          {select(
            "creator_id",
            t(($) => $.creator),
            creatorOptions,
          )}
        </>
      )}
      {(history
        ? (["processed_after", "processed_before"] as const)
        : (["entered_after", "entered_before"] as const)
      ).map((key) => (
        <TriageField key={key} label={t(($) => $[key])}>
          <Input
            type="date"
            aria-label={t(($) => $[key])}
            className={TRIAGE_CONTROL}
            value={params.get(key) ?? ""}
            onChange={(e) => onChange(key, e.target.value)}
          />
        </TriageField>
      ))}
    </div>
  );
}
