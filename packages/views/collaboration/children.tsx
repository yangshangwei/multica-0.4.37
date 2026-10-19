import type { CollaborationIssue } from "@multica/core/collaboration";
import { paths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { useT } from "../i18n";

/** This projection is the only authority for child references and open actions. */
export function CollaborationChildren({ issue }: { issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const children = issue.ordinary_children;
  return <section aria-label={t(($) => $.children.title)} className="min-w-0 space-y-3">
    <h3 className="text-body font-semibold">{t(($) => $.children.title)}</h3>
    {!children ? <p className="text-body text-muted-foreground">{t(($) => $.children.unavailable)}</p>
      : children.items.length === 0 ? <p className="text-body text-muted-foreground">{t(($) => $.children.empty)}</p>
      : <>
        <p className="text-caption text-muted-foreground">{t(($) => $.children.independent)}</p>
        <ul className="space-y-2">{children.items.map((child) => {
          const reference = <><span className="text-caption text-muted-foreground">{child.identifier}{" "}</span><span className="text-body">{child.title}</span></>;
          return <li key={child.id} className="min-w-0">
            {child.can_open === true && children.source_workspace_slug
              ? <AppLink href={paths.workspace(children.source_workspace_slug).issueDetail(child.id)} aria-label={`${child.identifier} ${child.title}`} className="flex min-h-11 min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 rounded-md px-2 py-2 [overflow-wrap:anywhere] hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">{reference}</AppLink>
              : <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 px-2 py-2 [overflow-wrap:anywhere]">{reference}</div>}
          </li>;
        })}</ul>
      </>}
  </section>;
}
