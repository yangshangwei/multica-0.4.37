import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCreationRecipientOptions, collaborationRecipientOptions, type CollaborationLabel, type CollaborationPropertyDefinition, type CollaborationScope, type CollaborationSourceOptions, type CollaborationSourceStructure } from "@multica/core/collaboration";
import { MAX_ISSUE_PROPERTY_ACTOR_VALUES, actorRefValuesFromValue, parseActorRef, type IssuePropertyValue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { CustomPropertyValueInput } from "../issues/components/pickers/custom-property-picker";
import { PropertyPicker, PickerItem } from "../issues/components/pickers/property-picker";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";

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
  const [open, setOpen] = useState(false);
  const actor = definition.type === "actor" || definition.type === "multi_actor";
  return <div className="min-w-0 space-y-2">
    <p id={labelId} className="text-body">{definition.name}{definition.archived ? ` · ${t(($) => $.fields.archived)}` : ""}</p>
    {actor ? <CollaborationActorPropertyInput scope={scope} issueId={issueId} sourceId={sourceId} labelId={labelId} multiple={definition.type === "multi_actor"}
      value={value} onChange={onChange} disabled={disabled} archived={definition.archived} />
      : <CustomPropertyValueInput property={definition} value={value} onChange={onChange} open={!disabled && open} onOpenChange={setOpen}
        triggerRender={<Button type="button" variant="outline" aria-labelledby={labelId} disabled={disabled} className="h-auto min-h-11 max-w-full whitespace-normal text-left [overflow-wrap:anywhere]" />} />}
  </div>;
}

function CollaborationActorPropertyInput({ scope, issueId, sourceId, labelId, multiple, value, onChange, disabled, archived }: {
  scope: CollaborationScope; issueId?: string; sourceId: string; labelId: string; multiple: boolean; value: IssuePropertyValue | undefined;
  onChange: (value: IssuePropertyValue | undefined) => void; disabled: boolean; archived: boolean;
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [text, setText] = useState("");
  const [query, setQuery] = useState("");
  const options = issueId ? collaborationRecipientOptions(client, scope, issueId, query) : collaborationCreationRecipientOptions(client, scope, sourceId, query);
  const lookup = useQuery({ ...options, enabled: !disabled && !archived && query !== "" && query === text.trim() });
  const selected = actorRefValuesFromValue(value);
  const found = query === text.trim() && !lookup.isFetching ? lookup.data?.recipient : null;
  const eligible = found && found.external === false;
  return <div className="min-w-0 space-y-2" role="group" aria-labelledby={labelId}>
    {selected.map((reference) => <div key={reference} className="flex min-w-0 items-center gap-2"><span className="min-w-0 text-caption [overflow-wrap:anywhere]">{found && found.user_id === parseActorRef(reference)?.id ? found.name : parseActorRef(reference)?.id ?? reference}</span>
      <Button type="button" size="sm" variant="ghost" disabled={disabled} aria-label={t(($) => $.fields.removeAccount, { name: parseActorRef(reference)?.id ?? reference })}
        onClick={() => { const next = selected.filter((item) => item !== reference); onChange(next.length ? multiple ? next : next[0] : undefined); }}>{t(($) => $.fields.clear)}</Button></div>)}
    {!archived && <>
      <Input aria-label={t(($) => $.fields.exactAccount)} value={text} disabled={disabled} placeholder={t(($) => $.workflow.recipientHint)}
        onChange={(event) => { setText(event.target.value); setQuery(""); }} />
      <Button type="button" variant="outline" disabled={disabled || !text.trim() || lookup.isFetching} onClick={() => { setQuery(text.trim()); if (query === text.trim()) void lookup.refetch(); }}>{t(($) => $.workflow.resolve)}</Button>
      {lookup.error && <CollaborationQueryState error={lookup.error} retry={() => void lookup.refetch()} />}
      {found && <div className="space-y-2"><p role="status" className="text-body [overflow-wrap:anywhere]">{found.name}</p>
        {eligible ? <Button type="button" variant="outline" disabled={disabled || multiple && selected.length >= MAX_ISSUE_PROPERTY_ACTOR_VALUES}
          onClick={() => { if (!found || found.external) return; const reference = `member:${found.user_id}`; onChange(multiple ? selected.includes(reference) ? selected : [...selected, reference] : reference); }}>{t(($) => $.creation.useRecipient)}</Button>
          : <p className="text-caption text-muted-foreground">{t(($) => $.fields.sourceAccountOnly)}</p>}
      </div>}
    </>}
  </div>;
}

export interface CollaborationSourceSelection { projectId: string | null; parentIssueId: string | null; stage: number | null }

export function CollaborationSourceInputs({ options, value, onChange, current, parentSearch, onParentSearch, disabled }: {
  options: CollaborationSourceOptions; value: CollaborationSourceSelection; onChange: (value: CollaborationSourceSelection) => void;
  current?: CollaborationSourceStructure; parentSearch: string; onParentSearch: (search: string) => void; disabled: boolean;
}) {
  const { t } = useT("collaboration");
  const id = useId();
  return <fieldset disabled={disabled} className="min-w-0 space-y-4">
    <div className="space-y-2"><label htmlFor={`${id}-project`} className="text-body">{t(($) => $.fields.project)}</label>
      <select id={`${id}-project`} value={value.projectId ?? ""} className={collaborationSelectClass} onChange={(event) => onChange({ ...value, projectId: event.target.value || null })}>
        <option value="">{t(($) => $.fields.none)}</option>
        {value.projectId && !options.projects.some((item) => item.id === value.projectId) && <option value={value.projectId}>{current?.project?.id === value.projectId ? current.project.name || value.projectId : value.projectId}</option>}
        {options.projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
      </select>
    </div>
    <div className="space-y-2"><label htmlFor={`${id}-search`} className="text-body">{t(($) => $.fields.findParent)}</label>
      <Input id={`${id}-search`} value={parentSearch} maxLength={200} onChange={(event) => onParentSearch(event.target.value)} />
      <label htmlFor={`${id}-parent`} className="block text-body">{t(($) => $.fields.parent)}</label>
      <select id={`${id}-parent`} value={value.parentIssueId ?? ""} className={collaborationSelectClass} onChange={(event) => onChange({ ...value, parentIssueId: event.target.value || null })}>
        <option value="">{t(($) => $.fields.none)}</option>
        {value.parentIssueId && !options.parents.some((item) => item.id === value.parentIssueId) && <option value={value.parentIssueId}>{current?.parent_issue?.id === value.parentIssueId ? current.parent_issue.name || value.parentIssueId : value.parentIssueId}</option>}
        {options.parents.map((parent) => <option key={parent.id} value={parent.id}>{parent.identifier} {parent.name}</option>)}
      </select>
      {options.parents_truncated && <p className="text-caption text-muted-foreground">{t(($) => $.fields.refineParents)}</p>}
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
