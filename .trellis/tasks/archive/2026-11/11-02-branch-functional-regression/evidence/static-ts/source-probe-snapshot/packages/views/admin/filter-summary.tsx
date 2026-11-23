"use client";

import { useT } from "../i18n";
import styles from "./admin-visual.module.css";

export type AdminFilterField = {
  name: string;
  label: string;
  options?: { value: string; label: string }[];
  displayValue?: string;
};

/** Describe submitted URL filters only. Draft validation and navigation stay
 * with the owning form; unlisted parameters such as cursors are not filters. */
export function AdminFilterSummary({ params, fields }: { params: URLSearchParams; fields: AdminFilterField[] }) {
  const { t } = useT("admin");
  const active = fields.flatMap(field => {
    const value = params.get(field.name);
    return value?.trim() ? [{ ...field, value: field.displayValue || (field.options?.find(option => option.value === value)?.label ?? value) }] : [];
  });
  if (!active.length) return null;
  return <div role="status" aria-label={t($ => $.filters.applied)} className={styles.filterSummary}>
    <p className="text-muted-foreground">{t($ => $.filters.applied)}</p>
    <dl>{active.map(field => <div key={field.name}>
      <dt className="text-muted-foreground">{field.label}:</dt><dd>{field.value}</dd>
    </div>)}</dl>
  </div>;
}
