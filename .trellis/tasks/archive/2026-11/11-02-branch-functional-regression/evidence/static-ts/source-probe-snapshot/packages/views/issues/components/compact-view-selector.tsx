"use client";

import { ChevronDown } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuRadioGroup,
  DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import type { IssueView } from "@multica/core/api/schemas";
import type { ViewBarBuiltin } from "./view-bar";

/** Compact access to exactly the same scope actions and saved views as ViewBar. */
export function CompactViewSelector({
  builtins, views, activeView, onSelectView,
}: {
  builtins: ViewBarBuiltin[];
  views: IssueView[];
  activeView: IssueView | null;
  onSelectView: (view: IssueView) => void;
}) {
  const selectedBuiltin = builtins.find((item) => item.active);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={
        <Button variant="outline" size="sm" className="min-w-0 max-w-full gap-1 text-muted-foreground md:hidden">
          <span className="truncate">{activeView?.name ?? selectedBuiltin?.label}</span>
          <ChevronDown className="size-3 shrink-0" />
        </Button>
      } />
      <DropdownMenuContent align="start" className="max-w-[calc(100vw-2rem)]">
        <DropdownMenuRadioGroup
          value={activeView ? `view:${activeView.id}` : `builtin:${selectedBuiltin?.key}`}
          onValueChange={(value) => {
            const builtin = builtins.find((item) => `builtin:${item.key}` === value);
            if (builtin) builtin.onSelect();
            else {
              const view = views.find((item) => `view:${item.id}` === value);
              if (view) onSelectView(view);
            }
          }}
        >
          {builtins.map((item) => (
            <DropdownMenuRadioItem key={item.key} value={`builtin:${item.key}`}>
              <span className="truncate">{item.label}</span>
            </DropdownMenuRadioItem>
          ))}
          {views.length > 0 && <DropdownMenuSeparator />}
          {views.map((view) => (
            <DropdownMenuRadioItem key={view.id} value={`view:${view.id}`}>
              <span className="truncate">{view.name}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
