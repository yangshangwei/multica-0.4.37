"use client";

import { useId, useRef, useState } from "react";
import { Combobox } from "@base-ui/react/combobox";
import { useQuery } from "@tanstack/react-query";
import { Check, ChevronDown, Plus } from "lucide-react";
import {
  AGENT_CATEGORY_MAX_LENGTH,
  getAgentCategoryNames,
  isAgentCategoryValid,
} from "@multica/core/agents";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";

export function AgentCategoryInput({
  workspaceId,
  value,
  onChange,
  onBlur,
  onCompositionChange,
  disabled = false,
  saveMode = "create",
  id,
}: {
  workspaceId: string;
  value: string;
  onChange: (value: string) => void;
  onBlur?: () => void;
  onCompositionChange?: (composing: boolean) => void;
  disabled?: boolean;
  saveMode?: "create" | "auto";
  id?: string;
}) {
  const { t } = useT("agents");
  const generatedId = useId();
  const inputId = id ?? generatedId;
  const helpId = `${inputId}-help`;
  const anchorRef = useRef<HTMLDivElement>(null);
  const [filtering, setFiltering] = useState(false);
  const { data: agents = [], isPending, isError } = useQuery({
    ...agentListOptions(workspaceId),
    enabled: !disabled && !!workspaceId,
  });
  const customCategories = getAgentCategoryNames(agents);
  const suggestions = [...new Set([
    t(($) => $.discovery.roles.other),
    t(($) => $.discovery.roles.specialist),
    t(($) => $.discovery.roles.coordinator),
    ...customCategories,
  ])];
  const invalid = !isAgentCategoryValid(value);
  const trimmedValue = value.trim();
  const canUseNew = !!trimmedValue && !invalid && !suggestions.includes(trimmedValue);
  const items = [...suggestions, ...(canUseNew ? [trimmedValue] : []), ""];
  const filteredItems = filtering && trimmedValue
    ? items.filter((item) => item.toLocaleLowerCase().includes(trimmedValue.toLocaleLowerCase()))
    : items;

  return (
    <div className="space-y-1.5">
      <Combobox.Root
        items={items}
        filteredItems={filteredItems}
        value={value}
        inputValue={value}
        disabled={disabled}
        onInputValueChange={(next, details) => {
          if (details.reason === "input-change" || details.reason === "input-clear") {
            setFiltering(true);
            onChange(next);
          }
        }}
        onValueChange={(next) => {
          if (next !== null) onChange(next);
          setFiltering(false);
        }}
        onOpenChange={(_open, details) => {
          if (details.reason !== "input-change") setFiltering(false);
        }}
      >
        <Combobox.InputGroup ref={anchorRef} className="relative">
          <Combobox.Input
            render={<Input className="pr-9" />}
            id={inputId}
            name="agent-category"
            autoComplete="off"
            aria-label={t(($) => $.category.label)}
            aria-describedby={helpId}
            aria-invalid={invalid || undefined}
            onBlur={onBlur}
            onCompositionStart={() => onCompositionChange?.(true)}
            onCompositionEnd={() => onCompositionChange?.(false)}
            placeholder={t(($) => $.category.placeholder)}
          />
          <Combobox.Trigger
            aria-label={t(($) => $.category.show_options)}
            className="absolute inset-y-0 right-0 flex w-9 items-center justify-center rounded-r-lg text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
          >
            <ChevronDown className="size-4" aria-hidden="true" />
          </Combobox.Trigger>
        </Combobox.InputGroup>
        <Combobox.Portal>
          <Combobox.Positioner anchor={anchorRef} align="start" sideOffset={4} className="z-50">
            <Combobox.Popup className="w-[var(--anchor-width)] max-w-[calc(100vw-2rem)] rounded-lg bg-surface-raised p-1 text-body text-popover-foreground shadow-[var(--menu-shadow)] ring-1 ring-surface-border">
              {(isPending || isError || customCategories.length === 0) && (
                <p role="status" className="px-2.5 py-2 text-caption text-muted-foreground">
                  {t(($) => isPending ? $.category.loading : isError ? $.category.load_failed : $.category.empty)}
                </p>
              )}
              <Combobox.Empty className="px-2.5 py-2 text-caption text-muted-foreground">
                {t(($) => $.category.no_matches)}
              </Combobox.Empty>
              <Combobox.List className="max-h-60 overflow-y-auto outline-none" aria-label={t(($) => $.category.label)}>
                {(category: string) => (
                  <Combobox.Item
                    key={category}
                    value={category}
                    className="flex min-h-9 cursor-default items-center gap-2 rounded-md px-2.5 py-2 outline-none data-highlighted:bg-accent data-highlighted:text-accent-foreground data-selected:font-medium"
                  >
                    {canUseNew && category === trimmedValue ? <Plus className="size-4 shrink-0" aria-hidden="true" /> : (
                      <span className="size-4 shrink-0"><Combobox.ItemIndicator><Check className="size-4" aria-hidden="true" /></Combobox.ItemIndicator></span>
                    )}
                    <span className="min-w-0 break-words">
                      {category === "" ? t(($) => $.category.default)
                        : canUseNew && category === trimmedValue ? t(($) => $.category.use_new, { name: category }) : category}
                    </span>
                  </Combobox.Item>
                )}
              </Combobox.List>
            </Combobox.Popup>
          </Combobox.Positioner>
        </Combobox.Portal>
      </Combobox.Root>
      <p id={helpId} className={`text-caption ${invalid ? "text-destructive" : "text-muted-foreground"}`}>
        {invalid
          ? t(($) => $.category.invalid, { max: AGENT_CATEGORY_MAX_LENGTH })
          : t(($) => saveMode === "auto" ? $.category.hint_auto : $.category.hint)}
      </p>
    </div>
  );
}
