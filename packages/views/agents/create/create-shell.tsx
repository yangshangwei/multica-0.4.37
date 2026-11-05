"use client";

import type { ComponentProps, ReactNode } from "react";
import { CreationPageShell } from "../../common/create-page-shell";
import { useT } from "../../i18n";

export function AgentCreateShell(
  props: Omit<ComponentProps<typeof CreationPageShell>, "backLabel">,
) {
  const { t } = useT("agents");
  return (
    <CreationPageShell
      {...props}
      backLabel={t(($) => $.creation_studio.back)}
    />
  );
}

export function AgentCreateChip({ children }: { children: ReactNode }) {
  return <span className="rounded-full bg-muted px-2 py-1">{children}</span>;
}
