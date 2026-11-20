import { useId } from "react";
import { CollaborationInformationEstimateSchema, collaborationInformationTextValid,
  type CollaborationInformationBlock as InformationBlock, type CollaborationInformationRepairBasis as InformationRepairBasis, type CollaborationInformationRepairContext } from "@multica/core/collaboration";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useLocale, useT } from "../i18n";
import { useCollaborationLabels } from "./labels";

export function CollaborationInformationBlock({ block }: { block?: InformationBlock }) {
  const { t } = useT("collaboration"); const locale = useLocale();
  if (block?.state !== "complete") return <p className="text-caption text-muted-foreground">{block === undefined ? t(($) => $.information.metadata.unknown) : block.state === "legacy"
    ? t(($) => $.information.metadata.legacy) : t(($) => $.information.metadata.unavailable)}</p>;
  return <dl className="space-y-3 text-body [overflow-wrap:anywhere]">
    <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.metadata.condition)}</dt><dd className="whitespace-pre-wrap">{block.unblock_condition}</dd></div>
    <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.metadata.estimate)}</dt><dd><time dateTime={block.estimated_resolution_at}>{new Date(block.estimated_resolution_at).toLocaleString(locale)}</time></dd></div>
  </dl>;
}

export function CollaborationInformationFields({ condition, estimate, required, onCondition, onEstimate }: {
  condition: string; estimate: string; required: boolean; onCondition: (value: string) => void; onEstimate: (value: string) => void;
}) {
  const { t } = useT("collaboration"); const id = useId();
  const badCondition = condition !== "" && !collaborationInformationTextValid(condition);
  const badEstimate = estimate !== "" && !CollaborationInformationEstimateSchema.safeParse(estimate).success;
  return <>
    {!required && <p className="text-caption text-muted-foreground">{t(($) => $.information.metadata.optionalPair)}</p>}
    <div className="space-y-2"><label htmlFor={`${id}-condition`} className="text-body">{t(($) => $.information.metadata.condition)}</label>
      <Textarea id={`${id}-condition`} value={condition} required={required} aria-invalid={badCondition || undefined}
        aria-describedby={badCondition ? `${id}-condition-error` : undefined} onChange={(event) => onCondition(event.target.value)} />
      {badCondition && <p id={`${id}-condition-error`} role="alert" className="text-caption text-destructive">{t(($) => $.information.metadata.invalidText)}</p>}
    </div>
    <div className="space-y-2"><label htmlFor={`${id}-estimate`} className="text-body">{t(($) => $.information.metadata.estimate)}</label>
      <Input id={`${id}-estimate`} type="text" value={estimate} required={required} aria-invalid={badEstimate || undefined}
        aria-describedby={`${id}-estimate-hint${badEstimate ? ` ${id}-estimate-error` : ""}`} onChange={(event) => onEstimate(event.target.value)}
        placeholder={t(($) => $.information.metadata.estimateExample)} className="min-h-11" />
      <p id={`${id}-estimate-hint`} className="text-caption text-muted-foreground">{t(($) => $.information.metadata.estimateHint)}</p>
      {badEstimate && <p id={`${id}-estimate-error`} role="alert" className="text-caption text-destructive">{t(($) => $.information.metadata.invalidEstimate)}</p>}
    </div>
  </>;
}

export function CollaborationInformationRepairBasis({ context, selected, onSelect }: {
  context?: CollaborationInformationRepairContext; selected: InformationRepairBasis | null;
  onSelect: (value: InformationRepairBasis) => void;
}) {
  const { t } = useT("collaboration"); const id = useId(); const labels = useCollaborationLabels();
  if (!context) return null;
  const pending = context.pending_submission, historical = context.historical_origin;
  return <fieldset className="min-w-0 space-y-2">
    <legend className="text-body">{t(($) => $.information.repair.basis)}</legend>
    {pending ? <label className="flex min-h-11 items-start gap-3 py-2 text-body [overflow-wrap:anywhere]">
      <input type="radio" name={id} checked={selected?.kind === "pending_submission" && selected.submission_id === pending.submission_id && selected.submission_event_id === pending.submission_event_id} onChange={() => onSelect({ kind: "pending_submission", submission_id: pending.submission_id, submission_event_id: pending.submission_event_id })} className="mt-1 shrink-0 accent-primary" />
      <span>{t(($) => $.information.repair.pendingBasis)}<span className="mt-1 block select-text text-caption text-muted-foreground">{pending.submission_id}</span></span>
    </label> : <>
      {historical && <label className="flex min-h-11 items-start gap-3 py-2 text-body [overflow-wrap:anywhere]">
        <input type="radio" name={id} checked={selected?.kind === "historical_information" && selected.origin_request_id === historical.origin_request_id && selected.origin_event_id === historical.origin_event_id} onChange={() => onSelect({ kind: "historical_information", origin_request_id: historical.origin_request_id, origin_event_id: historical.origin_event_id })} className="mt-1 shrink-0 accent-primary" />
        <span>{t(($) => $.information.repair.historicalBasis, { status: labels.status(historical.status) })}<span className="mt-1 block select-text text-caption text-muted-foreground">{historical.origin_request_id}</span></span>
      </label>}
      {context.human_selection_allowed && <label className="flex min-h-11 items-start gap-3 py-2 text-body">
        <input type="radio" name={id} checked={selected?.kind === "human_selected"} onChange={() => onSelect({ kind: "human_selected" })} className="mt-1 shrink-0 accent-primary" />
        <span>{t(($) => $.information.repair.humanBasis)}</span>
      </label>}
    </>}
    <p className="text-caption text-muted-foreground">{t(($) => $.information.repair.separateRequest)}</p>
  </fieldset>;
}
