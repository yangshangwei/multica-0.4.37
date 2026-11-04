import type { CollaborationIssue } from "@multica/core/collaboration";
import { actorRefValuesFromValue, parseActorRef } from "@multica/core/types";
import { CustomPropertyValueDisplay } from "../issues/components/pickers/custom-property-picker";
import { useT } from "../i18n";

/** Read only the issue's selected values; this component never opens catalogs. */
export function CollaborationFieldSummary({ issue }: { issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const structure = issue.source_structure;
  const priorities = new Map<string, string>((["none", "low", "medium", "high", "urgent"] as const).map((value) => [value, t(($) => $.priorities[value])]));
  const definitions = (issue.property_definitions ?? []).filter((definition) => {
    const value = issue.properties?.[definition.id];
    return value !== undefined && value !== null && value !== "" && (!Array.isArray(value) || value.length > 0);
  });
  if (issue.priority === "none" && !issue.start_date && !issue.due_date && !issue.labels?.length && !definitions.length &&
    !structure?.project && !structure?.parent_issue && structure?.stage == null && !structure?.iteration) return null;
  return <section aria-label={t(($) => $.fields.title)} className="min-w-0 space-y-4">
    <h3 className="text-body font-semibold">{t(($) => $.fields.title)}</h3>
    <dl className="grid min-w-0 gap-x-6 gap-y-3 text-body [&_dt]:mb-1 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:[overflow-wrap:anywhere]">
      {issue.priority !== "none" && <div><dt>{t(($) => $.workflow.priority)}</dt><dd>{priorities.get(issue.priority) ?? issue.priority}</dd></div>}
      {issue.start_date && <div><dt>{t(($) => $.workflow.startDate)}</dt><dd>{issue.start_date}</dd></div>}
      {issue.due_date && <div><dt>{t(($) => $.workflow.dueDate)}</dt><dd>{issue.due_date}</dd></div>}
      {!!issue.labels?.length && <div><dt>{t(($) => $.fields.labels)}</dt><dd>{issue.labels.map((label) => label.name).join(", ")}</dd></div>}
      {definitions.map((definition) => <div key={definition.id}><dt>{definition.name}{definition.archived ? ` · ${t(($) => $.fields.archived)}` : ""}</dt><dd>
        {definition.type === "actor" || definition.type === "multi_actor"
          ? actorRefValuesFromValue(issue.properties?.[definition.id]).map((value) => parseActorRef(value)?.id ?? value).join(", ") || t(($) => $.fields.none)
          : <CustomPropertyValueDisplay property={definition} value={issue.properties?.[definition.id]} />}
      </dd></div>)}
      {structure && <>
        {structure.project && <div><dt>{t(($) => $.fields.project)}</dt><dd>{structure.project.name || structure.project.id}</dd></div>}
        {structure.parent_issue && <div><dt>{t(($) => $.fields.parent)}</dt><dd>{`${structure.parent_issue.identifier ?? ""} ${structure.parent_issue.name || structure.parent_issue.id}`.trim()}</dd></div>}
        {structure.stage != null && <div><dt>{t(($) => $.fields.stage)}</dt><dd>{structure.stage}</dd></div>}
        {structure.iteration && <div><dt>{t(($) => $.fields.iteration)}</dt><dd>{structure.iteration.name || structure.iteration.id}</dd></div>}
      </>}
    </dl>
  </section>;
}
