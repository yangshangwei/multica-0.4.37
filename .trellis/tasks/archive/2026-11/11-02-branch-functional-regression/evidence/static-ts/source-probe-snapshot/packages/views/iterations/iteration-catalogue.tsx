"use client";
import { useQuery } from "@tanstack/react-query";
import { iterationCatalogueOptions } from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { useT } from "../i18n";

export function useIterationCatalogue(wsId: string, enabled = true) {
  return useQuery({ ...iterationCatalogueOptions(wsId), enabled });
}

export function IterationReference({ id, catalogue }: { id: string | null | undefined; catalogue: { id: string; name: string }[] }) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  if (id === undefined) return <span>{t(($) => $.iterations.unknownHistory)}</span>;
  if (id === null) return <span>{t(($) => $.iterations.unassignedState)}</span>;
  return <AppLink href={paths.iterationDetail(id)} className="inline-flex min-h-8 max-w-full items-center rounded [overflow-wrap:anywhere] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11 pointer-coarse:min-w-11">{catalogue.find((item) => item.id === id)?.name ?? t(($) => $.iterations.unavailableReference)}</AppLink>;
}
