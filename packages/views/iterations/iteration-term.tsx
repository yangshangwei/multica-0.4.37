"use client";

import { useId, useState, type ReactElement, type ReactNode } from "react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { useT } from "../i18n";
import type projects from "../locales/en/projects.json";

export type IterationTermKey = keyof typeof projects.iterations.termHelp;

export function IterationTerm({ term, children, closed = false, render }: {
  term: IterationTermKey;
  children: ReactNode;
  closed?: boolean;
  render?: ReactElement;
}) {
  const { t } = useT("projects");
  const descriptionId = useId();
  const [open, setOpen] = useState(false);
  const showFrozenNote = closed && term !== "finalScope" && term !== "finalEffective" && term !== "frozen";

  return <TooltipProvider>
    <Tooltip open={open} onOpenChange={setOpen}>
      <TooltipTrigger
        render={render}
        type="button"
        aria-describedby={open ? descriptionId : undefined}
        closeOnClick={!!render}
        onClick={render ? undefined : () => setOpen(true)}
        className={render ? undefined : "cursor-help rounded-sm text-left underline decoration-muted-foreground/50 decoration-dotted underline-offset-4 outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 pointer-coarse:min-h-11"}
      >{children}</TooltipTrigger>
      <TooltipContent id={descriptionId} role="tooltip" className="max-w-[min(22rem,calc(100vw-2rem))] px-3 py-2 leading-relaxed">
        <span>{t(($) => $.iterations.termHelp[term])}{showFrozenNote && <> {t(($) => $.iterations.termHelp.frozen)}</>}</span>
      </TooltipContent>
    </Tooltip>
  </TooltipProvider>;
}
