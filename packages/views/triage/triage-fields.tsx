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
import { Button } from "@multica/ui/components/ui/button";
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
  controlId,
  error,
}: {
  id?: string;
  label: string;
  children: ReactNode;
  controlId?: string;
  error?: string;
}) {
  const labelId = id ? `${id}-label` : undefined;
  const errorId = id ? `${id}-error` : undefined;
  const Label = controlId ? "label" : "div";
  return (
    <div id={id} className="min-w-0 space-y-1.5">
      <Label
        id={labelId}
        {...(controlId ? { htmlFor: controlId } : {})}
        className="block text-caption font-medium text-muted-foreground"
      >
        {label}
      </Label>
      {children}
      {error && (
        <p id={errorId} role="alert" className="text-caption text-destructive">
          {error}
        </p>
      )}
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
  candidateMode = false,
  errors,
  pickerAffordance = false,
  idPrefix = "triage",
}: {
  wsId: string;
  fields: TriageFields;
  onChange: (fields: TriageFields) => void;
  showStatus?: boolean;
  /** Use candidate wording when editing the pending intake form. */
  candidateMode?: boolean;
  errors?: {
    start_date?: string;
    due_date?: string;
  };
  /** Add a scoped boundary/focus treatment to picker controls in the intake form. */
  pickerAffordance?: boolean;
  /** Keep control labels and errors separate from other mounted triage forms. */
  idPrefix?: string;
}) {
  const { t } = useT("triage");
  const patch = (next: Partial<TriageFields>) =>
    onChange({ ...fields, ...next });
  const projectLabel = candidateMode
    ? t(($) => $.candidate_project)
    : t(($) => $.project);
  const assigneeLabel = candidateMode
    ? t(($) => $.candidate_assignee)
    : t(($) => $.assignee);
  const pickerTrigger = pickerAffordance
    ? <Button variant="outline" className={`h-auto w-full min-w-0 flex-wrap justify-start py-1 ${TRIAGE_CONTROL}`} />
    : undefined;
  return (
    <div className={`grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2 [&_button]:min-h-11 md:[&_button]:min-h-8 [@media(pointer:coarse)]:[&_button]:min-h-11 ${pickerAffordance ? "[&>div>div]:w-full" : ""}`}>
      {showStatus && (
        <TriageField
          id={`${idPrefix}-acceptance-status-field`}
          label={t(($) => $.acceptance_status)}
        >
          <AcceptanceStatusSelect
            wsId={wsId}
            value={fields.status ?? "backlog"}
            onChange={(status) => patch({ status })}
          />
        </TriageField>
      )}
      <TriageField
        id={`${idPrefix}-priority-field`}
        label={t(($) => $.priority)}
      >
        <PriorityPicker
          priority={fields.priority ?? "none"}
          triggerRender={pickerTrigger}
          triggerAriaLabel={t(($) => $.priority)}
          onUpdate={(v) => patch({ priority: v.priority })}
        />
      </TriageField>
      <TriageField
        id={`${idPrefix}-project-field`}
        label={projectLabel}
      >
        <ProjectPicker
          projectId={fields.project_id ?? null}
          triggerRender={pickerTrigger}
          triggerAriaLabel={projectLabel}
          onUpdate={(v) => patch({ project_id: v.project_id })}
        />
      </TriageField>
      <TriageField
        id={`${idPrefix}-assignee-field`}
        label={assigneeLabel}
      >
        <AssigneePicker
          assigneeType={fields.assignee_type ?? null}
          assigneeId={fields.assignee_id ?? null}
          triggerRender={pickerTrigger}
          triggerAriaLabel={assigneeLabel}
          onUpdate={(v) =>
            patch({
              assignee_type: v.assignee_type,
              assignee_id: v.assignee_id,
            })
          }
        />
      </TriageField>
      <TriageField
        id={`${idPrefix}-labels-field`}
        label={t(($) => $.labels)}
      >
        <LabelPicker
          selectedIds={fields.label_ids ?? []}
          triggerRender={pickerTrigger}
          triggerAriaLabel={t(($) => $.labels)}
          onSelectedIdsChange={(label_ids) => patch({ label_ids })}
        />
      </TriageField>
      <TriageField
        id={`${idPrefix}-start-date-field`}
        controlId={`${idPrefix}-start-date-field-control`}
        label={t(($) => $.start_date)}
        error={errors?.start_date}
      >
        <Input
          id={`${idPrefix}-start-date-field-control`}
          type="date"
          aria-labelledby={`${idPrefix}-start-date-field-label`}
          aria-describedby={errors?.start_date ? `${idPrefix}-start-date-field-error` : undefined}
          aria-invalid={errors?.start_date ? true : undefined}
          className={TRIAGE_CONTROL}
          value={fields.start_date ?? ""}
          onChange={(e) => patch({ start_date: e.target.value || null })}
        />
      </TriageField>
      <TriageField
        id={`${idPrefix}-due-date-field`}
        controlId={`${idPrefix}-due-date-field-control`}
        label={t(($) => $.due_date)}
        error={errors?.due_date}
      >
        <Input
          id={`${idPrefix}-due-date-field-control`}
          type="date"
          aria-labelledby={`${idPrefix}-due-date-field-label`}
          aria-describedby={errors?.due_date ? `${idPrefix}-due-date-field-error` : undefined}
          aria-invalid={errors?.due_date ? true : undefined}
          className={TRIAGE_CONTROL}
          value={fields.due_date ?? ""}
          onChange={(e) => patch({ due_date: e.target.value || null })}
        />
      </TriageField>
    </div>
  );
}
