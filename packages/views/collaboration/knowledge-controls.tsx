import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { useT } from "../i18n";
import type { CollaborationKnowledgeCandidateSummary, KnowledgeSourceSelection } from "@multica/core/collaboration";

export function KnowledgeSelect({ label, value, items, onChange, disabled = false }: {
  label: string; value: string; items: { value: string; label: string }[]; onChange: (value: string) => void; disabled?: boolean;
}) {
  return <Select items={items} value={value} onValueChange={(next) => { if (next !== null) onChange(next); }} disabled={disabled}>
    <SelectTrigger aria-label={label} className="w-full min-w-0 pointer-coarse:min-h-11"><SelectValue /></SelectTrigger>
    <SelectContent>{items.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
  </Select>;
}

export function KnowledgeState({ candidate }: { candidate: CollaborationKnowledgeCandidateSummary }) {
  const { t } = useT("collaboration");
  return <span className="text-caption text-muted-foreground">{t(($) => $.knowledge.states[candidate.state])}
    {candidate.availability !== "restricted" && candidate.expires_at && Date.parse(candidate.expires_at) <= Date.now() && <> · {t(($) => $.knowledge.expired)}</>}
  </span>;
}

export function KnowledgeSourceLabel({ source }: { source: KnowledgeSourceSelection }) {
  const { t } = useT("collaboration");
  return <span className="[overflow-wrap:anywhere]">{t(($) => $.knowledge[source.kind])} · {source.kind === "run_record" ? source.record_id
    : `${source.kind === "comment" ? `${source.comment_id.slice(0, 8)} · ` : ""}${t(($) => $.knowledge.version, { version: source.version })}`}</span>;
}
