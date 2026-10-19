import type { CollaborationIssue } from "@multica/core/collaboration";
import { actorRefValuesFromValue, parseActorRef } from "@multica/core/types";
import { CustomPropertyValueDisplay } from "../issues/components/pickers/custom-property-picker";
import { useT } from "../i18n";

/** Read only the issue's selected values; this component never opens catalogs. */
export function CollaborationFieldSummary({ issue }: { issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const structure = issue.source_structure;
  const priorities = new Map<string, string>((["none", "low", "medium", "high", "urgent"] as const).map((value) => [value, t(($) => $.priorities[value])]));
  return <section aria-label={t(($) => $.fields.title)} className="min-w-0 space-y-4">
    <dl className="grid min-w-0 gap-x-8 gap-y-4 text-body sm:grid-cols-2 [&_dt]:mb-1 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:[overflow-wrap:anywhere]">
      <div><dt>{t(($) => $.workflow.priority)}</dt><dd>{priorities.get(issue.priority) ?? issue.priority}</dd></div>
      {issue.start_date !== undefined && <div><dt>{t(($) => $.workflow.startDate)}</dt><dd>{issue.start_date || t(($) => $.fields.none)}</dd></div>}
      {issue.due_date !== undefined && <div><dt>{t(($) => $.workflow.dueDate)}</dt><dd>{issue.due_date || t(($) => $.fields.none)}</dd></div>}
      {issue.labels !== undefined && <div><dt>{t(($) => $.fields.labels)}</dt><dd>{issue.labels.map((label) => label.name).join(", ") || t(($) => $.fields.none)}</dd></div>}
      {(issue.property_definitions ?? []).map((definition) => <div key={definition.id}><dt>{definition.name}{definition.archived ? ` · ${t(($) => $.fields.archived)}` : ""}</dt><dd>
        {definition.type === "actor" || definition.type === "multi_actor"
          ? actorRefValuesFromValue(issue.properties?.[definition.id]).map((value) => parseActorRef(value)?.id ?? value).join(", ")
          : <CustomPropertyValueDisplay property={definition} value={issue.properties?.[definition.id]} />}
      </dd></div>)}
      {structure && <>
        <div><dt>{t(($) => $.fields.project)}</dt><dd>{structure.project?.name || structure.project?.id || t(($) => $.fields.none)}</dd></div>
        <div><dt>{t(($) => $.fields.parent)}</dt><dd>{structure.parent_issue ? `${structure.parent_issue.identifier ?? ""} ${structure.parent_issue.name || structure.parent_issue.id}`.trim() : t(($) => $.fields.none)}</dd></div>
        <div><dt>{t(($) => $.fields.stage)}</dt><dd>{structure.stage ?? t(($) => $.fields.none)}</dd></div>
        <div><dt>{t(($) => $.fields.iteration)}</dt><dd>{structure.iteration?.name || structure.iteration?.id || t(($) => $.fields.none)}</dd></div>
      </>}
    </dl>
  </section>;
}
