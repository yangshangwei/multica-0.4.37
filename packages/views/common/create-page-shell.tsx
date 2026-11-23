"use client";

import type { ReactNode } from "react";
import { ArrowLeft } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";

/** Shared creation-page chrome; each page owns its return destination. */
export function CreationPageShell({
  title,
  step,
  onBack,
  backLabel,
  backDisabled = false,
  chips,
  children,
}: {
  title: string;
  step: string;
  onBack: () => void;
  backLabel: string;
  backDisabled?: boolean;
  /** Right-aligned context badges (method, runtime). Hidden on small screens. */
  chips?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col bg-background">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b px-5">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={onBack}
          disabled={backDisabled}
          aria-label={backLabel}
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
        </Button>
        <div className="min-w-0">
          <h1 className="truncate text-body font-semibold">{title}</h1>
          <p className="truncate text-caption text-muted-foreground">{step}</p>
        </div>
        {chips ? (
          <div className="ml-auto hidden items-center gap-2 text-caption text-muted-foreground sm:flex">
            {chips}
          </div>
        ) : null}
      </header>
      {children}
    </div>
  );
}
