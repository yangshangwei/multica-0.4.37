import { useId, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import { type CollaborationLabel, type CollaborationPropertyDefinition, type CollaborationRecipientAccount, type CollaborationScope, type CollaborationSourceOptions, type CollaborationSourceStructure } from "@multica/core/collaboration";
import { MAX_ISSUE_PROPERTY_ACTOR_VALUES, actorRefValuesFromValue, parseActorRef, type IssuePropertyValue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { CustomPropertyValueDisplay, CustomPropertyValueInput } from "../issues/components/pickers/custom-property-picker";
import { PropertyPicker, PickerItem } from "../issues/components/pickers/property-picker";
import { useT } from "../i18n";
import { CollaborationAccountPicker } from "./account-picker";

export const collaborationSelectClass = "block min-h-11 w-full min-w-0 rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

export function CollaborationLabelInput({ labels, selectedIds, onChange, disabled }: { labels: CollaborationLabel[]; selectedIds: string[]; onChange: (ids: string[]) => void; disabled: boolean }) {
  const { t } = useT("collaboration");
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const selected = selectedIds.map((id) => labels.find((label) => label.id === id)?.name ?? id);
  return <div className="min-w-0 space-y-2"><p className="text-body font-medium">{t(($) => $.fields.labels)}</p>
    <PropertyPicker open={!disabled && open} onOpenChange={setOpen} searchable onSearchChange={setSearch} searchPlaceholder={t(($) => $.fields.findLabel)} align="start"
      triggerAriaLabel={t(($) => $.fields.labels)} triggerRender={<Button type="button" variant="outline" disabled={disabled} className="h-auto min-h-11 max-w-full whitespace-normal text-left [overflow-wrap:anywhere]" />}
      trigger={selected.length ? selected.join(", ") : t(($) => $.fields.none)}>
      <PickerItem selected={selectedIds.length === 0} onClick={() => onChange([])}>{t(($) => $.fields.none)}</PickerItem>
      {labels.filter((label) => label.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase())).map((label) => <PickerItem key={label.id} selected={selectedIds.includes(label.id)}
        onClick={() => onChange(selectedIds.includes(label.id) ? selectedIds.filter((id) => id !== label.id) : [...selectedIds, label.id])}>
        <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ backgroundColor: label.color }} /><span className="min-w-0 [overflow-wrap:anywhere]">{label.name}</span>
      </PickerItem>)}
      {selectedIds.filter((id) => !labels.some((label) => label.id === id)).map((id) => <PickerItem key={id} selected onClick={() => onChange(selectedIds.filter((value) => value !== id))}>{t(($) => $.fields.unavailableSelection, { value: id })}</PickerItem>)}
    </PropertyPicker>
  </div>;
}

export function CollaborationPropertyInputs({ scope, issueId, sourceId, definitions, values, onChange, disabled }: {
  scope: CollaborationScope; issueId?: string; sourceId: string; definitions: CollaborationPropertyDefinition[];
  values: Record<string, IssuePropertyValue>; onChange: (id: string, value: IssuePropertyValue | undefined) => void; disabled: boolean;
}) {
  const { t } = useT("collaboration");
  return <fieldset disabled={disabled} className="min-w-0 space-y-4"><legend className="mb-3 text-body font-medium">{t(($) => $.fields.properties)}</legend>
    {definitions.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.fields.noProperties)}</p>}
    {definitions.map((definition) => <PropertyInput key={definition.id} scope={scope} issueId={issueId} sourceId={sourceId} definition={definition}
      value={values[definition.id]} onChange={(value) => onChange(definition.id, value)} disabled={disabled} />)}
  </fieldset>;
}

function PropertyInput({ scope, issueId, sourceId, definition, value, onChange, disabled }: {
  scope: CollaborationScope; issueId?: string; sourceId: string; definition: CollaborationPropertyDefinition;
  value: IssuePropertyValue | undefined; onChange: (value: IssuePropertyValue | undefined) => void; disabled: boolean;
}) {
  const { t } = useT("collaboration");
  const labelId = useId();
  const valueId = useId();
  const [open, setOpen] = useState(false);
  const actor = definition.type === "actor" || definition.type === "multi_actor";
  return <div className="min-w-0 space-y-2">
    <p id={labelId} className="text-body">{definition.name}{definition.archived ? ` · ${t(($) => $.fields.archived)}` : ""}</p>
    {actor ? <CollaborationActorPropertyInput scope={scope} issueId={issueId} sourceId={sourceId} labelId={labelId} pickerLabel={definition.name} multiple={definition.type === "multi_actor"}
      value={value} onChange={onChange} disabled={disabled} archived={definition.archived} />
      : <CustomPropertyValueInput property={definition} value={value} onChange={onChange} open={!disabled && open} onOpenChange={setOpen}
        trigger={<span id={valueId}><CustomPropertyValueDisplay property={definition} value={value} /></span>}
        triggerRender={<Button type="button" variant="outline" aria-labelledby={`${labelId} ${valueId}`} disabled={disabled} className="h-auto min-h-11 max-w-full whitespace-normal text-left [overflow-wrap:anywhere]" />} />}
  </div>;
}

function CollaborationActorPropertyInput({ scope, issueId, sourceId, labelId, pickerLabel, multiple, value, onChange, disabled, archived }: {
  scope: CollaborationScope; issueId?: string; sourceId: string; labelId: string; pickerLabel: string; multiple: boolean; value: IssuePropertyValue | undefined;
  onChange: (value: IssuePropertyValue | undefined) => void; disabled: boolean; archived: boolean;
}) {
  const { t } = useT("collaboration");
  const selected = actorRefValuesFromValue(value);
  const [accountNames, setAccountNames] = useState<Record<string, string>>({});
  const selectedAccounts: CollaborationRecipientAccount[] = selected.flatMap((reference) => {
    const parsed = parseActorRef(reference);
    return parsed ? [{ user_id: parsed.id, name: accountNames[parsed.id] ?? parsed.id, avatar_url: "", external: false }] : [];
  });
  const context = issueId ? { kind: "issue" as const, issueId } : { kind: "creation" as const, sourceWorkspaceId: sourceId };
  return <div className="min-w-0 space-y-2" role="group" aria-labelledby={labelId}>
    {selected.map((reference) => <div key={reference} className="flex min-w-0 items-center gap-2"><span className="min-w-0 text-caption [overflow-wrap:anywhere]">{accountNames[parseActorRef(reference)?.id ?? ""] ?? parseActorRef(reference)?.id ?? reference}</span>
      <Button type="button" size="sm" variant="ghost" disabled={disabled} className="pointer-coarse:min-h-11" aria-label={t(($) => $.fields.removeAccount, { name: parseActorRef(reference)?.id ?? reference })}
        onClick={() => { const next = selected.filter((item) => item !== reference); onChange(next.length ? multiple ? next : next[0] : undefined); }}>{t(($) => $.fields.clear)}</Button></div>)}
    {!archived && <CollaborationAccountPicker scope={scope} context={context} selected={multiple ? selectedAccounts : selectedAccounts[0] ?? null}
      label={pickerLabel} membersOnly multiple={multiple} maxSelected={MAX_ISSUE_PROPERTY_ACTOR_VALUES} disabled={disabled}
      onSelect={(account) => {
        if (!account || account.external !== false) return;
        setAccountNames((current) => ({ ...current, [account.user_id]: account.name }));
        const reference = `member:${account.user_id}`;
        onChange(multiple ? selected.includes(reference) ? selected : [...selected, reference] : reference);
      }} />}
  </div>;
}

export interface CollaborationSourceSelection { projectId: string | null; parentIssueId: string | null; stage: number | null }

export function CollaborationSourceInputs({ options, value, onChange, current, parentSearch, onParentSearch, parentsPending = false, parentsError, onRetryParents, disabled }: {
  options: CollaborationSourceOptions; value: CollaborationSourceSelection; onChange: (value: CollaborationSourceSelection) => void;
  current?: CollaborationSourceStructure; parentSearch: string; onParentSearch: (search: string) => void; disabled: boolean;
  parentsPending?: boolean; parentsError?: unknown; onRetryParents?: () => void;
}) {
  const { t } = useT("collaboration");
  const id = useId();
  const [parentOpen, setParentOpen] = useState(false);
  const [chosenParent, setChosenParent] = useState<CollaborationSourceOptions["parents"][number] | null>(null);
  const searchInput = useRef<HTMLInputElement>(null);
  const selectedParent = options.parents.find((parent) => parent.id === value.parentIssueId)
    ?? (chosenParent?.id === value.parentIssueId ? chosenParent : null)
    ?? (current?.parent_issue?.id === value.parentIssueId ? current.parent_issue : null);
  const parentLabel = value.parentIssueId ? [selectedParent?.identifier, selectedParent?.name].filter(Boolean).join(" ") || value.parentIssueId
    : t(($) => $.fields.chooseParent);
  const chooseParent = (parent: CollaborationSourceOptions["parents"][number] | null) => {
    setChosenParent(parent);
    onChange({ ...value, parentIssueId: parent?.id ?? null });
    setParentOpen(false);
  };
  const parentsReady = !parentsPending && !parentsError;
  return <fieldset disabled={disabled} className="min-w-0 space-y-4">
    <div className="space-y-2"><label htmlFor={`${id}-project`} className="text-body">{t(($) => $.fields.project)}</label>
      <select id={`${id}-project`} value={value.projectId ?? ""} className={collaborationSelectClass} onChange={(event) => onChange({ ...value, projectId: event.target.value || null })}>
        <option value="">{t(($) => $.fields.none)}</option>
        {value.projectId && !options.projects.some((item) => item.id === value.projectId) && <option value={value.projectId}>{current?.project?.id === value.projectId ? current.project.name || value.projectId : value.projectId}</option>}
        {options.projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
      </select>
    </div>
    <div className="min-w-0 space-y-2"><p className="text-body">{t(($) => $.fields.parent)}</p>
      <PropertyPicker open={!disabled && parentOpen} onOpenChange={setParentOpen} searchable onSearchChange={onParentSearch}
        searchPlaceholder={t(($) => $.fields.findParent)} searchMaxLength={200} searchInputRef={searchInput} align="start" width="w-[min(24rem,calc(100vw-2rem))]"
        navigationResetKey={`${parentSearch}:${parentsReady ? options.parents.map((parent) => parent.id).join(",") : "pending"}`}
        triggerAriaLabel={t(($) => $.fields.parent)} triggerAriaDescription={`${parentLabel}. ${t(($) => $.fields.parentHint)}`}
        triggerRender={<Button type="button" variant="outline" disabled={disabled} className="h-auto min-h-11 w-full justify-between whitespace-normal text-left [overflow-wrap:anywhere]" />}
        trigger={<><span className="min-w-0">{parentLabel}</span><ChevronDown aria-hidden className="size-4 shrink-0 text-muted-foreground" /></>}
        footer={parentsReady && options.parents_truncated ? <p className="px-2 py-2 text-caption text-muted-foreground">{t(($) => $.fields.refineParents)}</p> : undefined}>
        {parentsReady && value.parentIssueId && <PickerItem selected={false} emptyValue onClick={() => chooseParent(null)}><span className="flex min-h-8 items-center">{t(($) => $.fields.noParent)}</span></PickerItem>}
        {parentsPending ? <p role="status" className="px-2 py-3 text-body text-muted-foreground">{t(($) => $.fields.parentsLoading)}</p>
          : parentsError ? <div role="alert" className="space-y-2 px-2 py-3 text-body"><p>{t(($) => $.fields.parentsUnavailable)}</p>
            {onRetryParents && <Button type="button" variant="outline" className="min-h-11" onClick={() => { searchInput.current?.focus(); onRetryParents(); }}>{t(($) => $.fields.retryParents)}</Button>}
          </div>
          : options.parents.length === 0 ? <p role="status" className="px-2 py-3 text-body text-muted-foreground">{parentSearch.trim() ? t(($) => $.fields.noMatchingParents) : t(($) => $.fields.noParents)}</p>
          : options.parents.map((parent) => <PickerItem key={parent.id} selected={parent.id === value.parentIssueId} onClick={() => chooseParent(parent)}>
            <span className={`min-h-8 min-w-0 py-1 [overflow-wrap:anywhere] ${parent.id === value.parentIssueId ? "font-medium text-foreground" : ""}`}>
              <span className="text-caption text-muted-foreground">{parent.identifier}</span>{" "}{parent.name}
            </span>
          </PickerItem>)}
      </PropertyPicker>
      <p className="text-caption text-muted-foreground">{t(($) => $.fields.parentHint)}</p>
    </div>
    <div className="space-y-2"><label htmlFor={`${id}-stage`} className="text-body">{t(($) => $.fields.stage)}</label>
      <Input id={`${id}-stage`} type="number" min={1} max={2147483647} step={1} value={value.stage ?? ""} onChange={(event) => onChange({ ...value, stage: event.target.value === "" ? null : event.target.valueAsNumber })} />
    </div>
  </fieldset>;
}

export function CollaborationIterationInput({ options, selectedId, onChange, current, disabled, allowClearWhenDisabled = false, statusCategory }: {
  options: CollaborationSourceOptions; selectedId: string | null; onChange: (id: string | null) => void;
  current?: CollaborationSourceStructure["iteration"]; disabled: boolean; allowClearWhenDisabled?: boolean; statusCategory?: string;
}) {
  const { t } = useT("collaboration");
  const id = useId();
  return <div className="min-w-0 space-y-2"><label htmlFor={id} className="text-body">{t(($) => $.fields.iteration)}</label>
    <select id={id} value={selectedId ?? ""} disabled={disabled || !options.iterations_enabled && !allowClearWhenDisabled} onChange={(event) => onChange(event.target.value || null)} className={collaborationSelectClass}>
      <option value="">{t(($) => $.fields.none)}</option>
      {selectedId && !options.iterations.some((item) => item.id === selectedId) && <option value={selectedId}>{current?.id === selectedId ? current.name || selectedId : selectedId}</option>}
      {options.iterations.map((period) => <option key={period.id} value={period.id} disabled={!options.iterations_enabled || period.id !== current?.id && (statusCategory === "cancelled" || statusCategory === "done" && period.status !== "active")}>{period.name}</option>)}
    </select>
    {!options.iterations_enabled && <p className="text-caption text-muted-foreground">{t(($) => $.fields.iterationsDisabled)}</p>}
  </div>;
}
