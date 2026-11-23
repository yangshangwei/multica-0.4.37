"use client";

import { useQuery } from "@tanstack/react-query";
import { AlertCircle } from "lucide-react";
import { squadListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../i18n";
import { BuiltinSquadCatalog } from "../squads/components/builtin-squad-catalog";

export function SquadTemplatesMarketPanel({ workspaceId, active }: {
  workspaceId: string;
  active: boolean;
}) {
  return <SquadTemplatesMarketPanelContent key={workspaceId} workspaceId={workspaceId} active={active} />;
}

function SquadTemplatesMarketPanelContent({ workspaceId, active }: {
  workspaceId: string;
  active: boolean;
}) {
  const { t } = useT("squads");
  const squads = useQuery({
    ...squadListOptions(workspaceId),
    enabled: active && !!workspaceId,
  });

  if (squads.isError) {
    return (
      <div role="alert" className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
        <AlertCircle className="size-4 shrink-0" aria-hidden="true" />
        <p>{t(($) => $.catalog.squads_error)}</p>
        <Button type="button" size="sm" variant="outline" onClick={() => void squads.refetch()}>
          {t(($) => $.catalog.retry)}
        </Button>
      </div>
    );
  }

  if (!squads.isSuccess) {
    if (!active) return null;
    return (
      <div role="status" className="space-y-3">
        <span className="sr-only">{t(($) => $.catalog.squads_loading)}</span>
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-4 w-full max-w-md" />
      </div>
    );
  }

  return <BuiltinSquadCatalog squads={squads.data ?? []} active={active} />;
}
