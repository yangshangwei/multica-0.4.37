import { Users } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { useViewStore } from "@multica/core/issues/stores/view-store-context";
import { useViewBaseline } from "../issues/surface/view-baseline-context";
import { useT } from "../i18n";

export function SourceCollaborationFilter() {
  const { t } = useT("collaboration");
  const selected = useViewStore((state) => state.externalCollaborators === true);
  const toggle = useViewStore((state) => state.toggleExternalCollaborators);
  const baseline = useViewBaseline();
  return <Button type="button" variant="outline" size="sm" aria-label={t(($) => $.source.externalFilter)} aria-pressed={selected}
    disabled={baseline?.raw.externalCollaborators === true} onClick={toggle}
    className="h-8 w-8 gap-1 px-0 text-muted-foreground aria-pressed:bg-accent dark:aria-pressed:bg-accent aria-pressed:font-semibold aria-pressed:text-foreground md:h-7 md:w-auto md:px-2.5">
    <Users className="size-3.5" /><span className="hidden md:inline">{t(($) => $.source.externalFilter)}</span>
  </Button>;
}
