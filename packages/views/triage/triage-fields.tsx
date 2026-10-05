"use client";

import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import type { TriageFields } from "@multica/core/triage";
import { memberListOptions } from "@multica/core/workspace/queries";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Input } from "@multica/ui/components/ui/input";
import { PriorityPicker } from "../issues/components/pickers/priority-picker";
import { AssigneePicker } from "../issues/components/pickers/assignee-picker";
import { LabelPicker } from "../issues/components/pickers/label-picker";
import { ProjectPicker } from "../projects/components/project-picker";
import { useStatusOptions } from "../issues/utils/status-options";
import { useT } from "../i18n";
import { TRIAGE_CONTROL } from "./triage-ui";

export function TriageSelect({
  label,
  value,
  onChange,
  options,
  disabled = false,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}) {
  return (
    <Select
      items={options}
      value={value}
      onValueChange={(next) => {
        if (next !== null) onChange(next);
      }}
      disabled={disabled}
    >
      <SelectTrigger
        aria-label={label}
        className={`w-full min-w-0 ${TRIAGE_CONTROL}`}
      >
        <SelectValue>
          {options.find((o) => o.value === value)?.label ?? value}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

export function TriageField({
  label,
  children,
  id,
}: {
  id?: string;
  label: string;
  children: ReactNode;
}) {
  return (
    <div id={id} className="min-w-0 space-y-1.5">
      <div className="text-caption font-medium text-muted-foreground">
        {label}
      </div>
      {children}
    </div>
  );
}

export function ReviewerSelect({
  wsId,
  value,
  onChange,
  disabled,
}: {
  wsId: string;
  value: string | null;
  onChange: (id: string | null) => void;
  disabled?: boolean;
}) {
  const { t } = useT("triage");
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  return (
    <TriageSelect
      label={t(($) => $.reviewer)}
      value={value ?? "none"}
      onChange={(id) => onChange(id === "none" ? null : id)}
      disabled={disabled}
      options={[
        { value: "none", label: t(($) => $.unassigned) },
        ...members.map((m) => ({
          value: m.user_id,
          label: m.name ?? m.email ?? m.user_id,
        })),
      ]}
    />
  );
}

export function AcceptanceStatusSelect({
  wsId,
  value,
  onChange,
  disabled,
}: {
  wsId: string;
  value: string;
  onChange: (status: string) => void;
  disabled?: boolean;
}) {
  const { t } = useT("triage");
  const options = useStatusOptions(wsId).filter(
    (s) => s.category === "backlog" || s.category === "todo",
  );
  return (
    <TriageSelect
      label={t(($) => $.acceptance_status)}
      value={value}
      onChange={onChange}
      disabled={disabled}
      options={options.map((s) => ({ value: s.key, label: s.label }))}
    />
  );
}

export function TriageFieldsEditor({
  wsId,
  fields,
  onChange,
  showStatus = true,
}: {
  wsId: string;
  fields: TriageFields;
  onChange: (fields: TriageFields) => void;
  showStatus?: boolean;
}) {
  const { t } = useT("triage");
  const patch = (next: Partial<TriageFields>) =>
    onChange({ ...fields, ...next });
  return (
    <div className="grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2 [&_button]:min-h-11 md:[&_button]:min-h-8 [@media(pointer:coarse)]:[&_button]:min-h-11">
      {showStatus && (
        <TriageField label={t(($) => $.acceptance_status)}>
          <AcceptanceStatusSelect
            wsId={wsId}
            value={fields.status ?? "backlog"}
            onChange={(status) => patch({ status })}
          />
        </TriageField>
      )}
      <TriageField id="triage-priority-field" label={t(($) => $.priority)}>
        <PriorityPicker
          priority={fields.priority ?? "none"}
          onUpdate={(v) => patch({ priority: v.priority })}
        />
      </TriageField>
      <TriageField label={t(($) => $.project)}>
        <ProjectPicker
          projectId={fields.project_id ?? null}
          onUpdate={(v) => patch({ project_id: v.project_id })}
        />
      </TriageField>
      <TriageField label={t(($) => $.assignee)}>
        <AssigneePicker
          assigneeType={fields.assignee_type ?? null}
          assigneeId={fields.assignee_id ?? null}
          onUpdate={(v) =>
            patch({
              assignee_type: v.assignee_type,
              assignee_id: v.assignee_id,
            })
          }
        />
      </TriageField>
      <TriageField label={t(($) => $.labels)}>
        <LabelPicker
          selectedIds={fields.label_ids ?? []}
          onSelectedIdsChange={(label_ids) => patch({ label_ids })}
        />
      </TriageField>
      <TriageField label={t(($) => $.start_date)}>
        <Input
          type="date"
          aria-label={t(($) => $.start_date)}
          className={TRIAGE_CONTROL}
          value={fields.start_date ?? ""}
          onChange={(e) => patch({ start_date: e.target.value || null })}
        />
      </TriageField>
      <TriageField label={t(($) => $.due_date)}>
        <Input
          type="date"
          aria-label={t(($) => $.due_date)}
          className={TRIAGE_CONTROL}
          value={fields.due_date ?? ""}
          onChange={(e) => patch({ due_date: e.target.value || null })}
        />
      </TriageField>
    </div>
  );
}
