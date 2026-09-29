"use client";

import { useRef } from "react";
import { ChevronRight, Plus, Sparkles, X } from "lucide-react";
import { useModalStore } from "@multica/core/modals";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";

export function CreateSquadChooser() {
  const { t } = useT("squads");
  const handingOff = useRef(false);
  const methods = [
    {
      modal: "staff-squad-template",
      icon: Sparkles,
      title: t(($) => $.page.template_button),
      description: t(($) => $.create_chooser.template_description),
    },
    {
      modal: "create-squad",
      icon: Plus,
      title: t(($) => $.page.custom_button),
      description: t(($) => $.create_chooser.custom_description),
    },
  ] as const;

  return (
    <Dialog onOpenChange={(open) => { if (open) handingOff.current = false; }}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus className="size-3.5" aria-hidden="true" />
        {t(($) => $.page.new_button)}
      </DialogTrigger>
      <DialogContent
        showCloseButton={false}
        // The next modal owns focus when a creation method is selected.
        finalFocus={() => !handingOff.current}
        className="flex max-h-[85dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-md"
      >
        <div className="flex shrink-0 items-start justify-between gap-3 border-b px-5 pt-4 pb-3">
          <div className="min-w-0">
            <DialogTitle>{t(($) => $.page.new_button)}</DialogTitle>
            <DialogDescription className="mt-0.5 text-caption">
              {t(($) => $.create_chooser.description)}
            </DialogDescription>
          </div>
          <DialogClose
            aria-label={t(($) => $.create_chooser.close)}
            className="shrink-0 rounded-sm p-1 text-faint-foreground transition-colors hover:bg-accent/60 hover:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            <X className="size-3.5" aria-hidden="true" />
          </DialogClose>
        </div>
        <div className="grid min-h-0 gap-2 overflow-y-auto p-5">
          {methods.map(({ modal, icon: Icon, title, description }) => (
            <DialogClose
              key={modal}
              onClick={() => {
                handingOff.current = true;
                useModalStore.getState().open(modal);
              }}
              className="group flex items-start gap-3 rounded-lg border bg-card p-4 text-left transition-colors hover:border-primary/40 hover:bg-accent/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            >
              <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground group-hover:text-foreground">
                <Icon className="size-4" aria-hidden="true" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-body font-medium">{title}</span>
                <span className="mt-0.5 block text-caption text-muted-foreground">{description}</span>
              </span>
              <ChevronRight className="size-4 shrink-0 text-faint-foreground transition-colors group-hover:text-muted-foreground" aria-hidden="true" />
            </DialogClose>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
