"use client";

import { useId } from "react";
import { ChevronDown, Dices } from "lucide-react";
import {
  WORKSPACE_NAMES,
  WORKSPACE_NAME_SERIES,
  isWorkspaceNameSelection,
  type WorkspaceNameSelection,
} from "@multica/core/workspace/workspace-names";
import { matchLocale } from "@multica/core/i18n";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useT } from "../i18n";

export function WorkspaceNamePicker({
  selection,
  onSelectionChange,
  onRandom,
  disabled = false,
}: {
  selection: WorkspaceNameSelection;
  onSelectionChange: (selection: WorkspaceNameSelection) => void;
  onRandom: () => void;
  disabled?: boolean;
}) {
  const { t, i18n } = useT("workspace");
  const locale = matchLocale([i18n.resolvedLanguage ?? i18n.language]);
  const currentSeriesId = useId();
  const labels: Record<WorkspaceNameSelection, string> = {
    workshop: t(($) => $.name_picker.series.workshop),
    computing: t(($) => $.name_picker.series.computing),
    ai: t(($) => $.name_picker.series.ai),
    space: t(($) => $.name_picker.series.space),
    nature: t(($) => $.name_picker.series.nature),
    voyage: t(($) => $.name_picker.series.voyage),
    all: t(($) => $.name_picker.series.all),
  };

  return (
    <div className="flex shrink-0 flex-col gap-1.5">
      <div className="flex">
        <Button
          type="button"
          variant="outline"
          disabled={disabled}
          onClick={onRandom}
          aria-describedby={currentSeriesId}
          className="rounded-r-none focus-visible:z-10 max-sm:min-h-11 pointer-coarse:min-h-11"
        >
          <Dices className="size-4" aria-hidden="true" />
          {t(($) => $.name_picker.random)}
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger
            disabled={disabled}
            render={<Button type="button" variant="outline" size="icon" />}
            aria-label={t(($) => $.name_picker.choose_series)}
            aria-describedby={currentSeriesId}
            className="-ml-px rounded-l-none focus-visible:z-10 max-sm:min-h-11 max-sm:min-w-11 pointer-coarse:min-h-11 pointer-coarse:min-w-11"
          >
            <ChevronDown className="size-4" aria-hidden="true" />
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className="w-72 max-w-[calc(100vw-2rem)]"
            aria-label={t(($) => $.name_picker.menu_title)}
          >
            <DropdownMenuGroup>
              <DropdownMenuLabel>{t(($) => $.name_picker.menu_title)}</DropdownMenuLabel>
              <DropdownMenuRadioGroup
                value={selection}
                onValueChange={(value) => {
                  if (isWorkspaceNameSelection(value)) onSelectionChange(value);
                }}
              >
                {WORKSPACE_NAME_SERIES.map((series) => (
                  <DropdownMenuRadioItem
                    key={series}
                    value={series}
                    className="items-start py-2 data-checked:font-medium max-sm:min-h-11 pointer-coarse:min-h-11"
                  >
                    <span className="flex min-w-0 flex-col gap-0.5">
                      <span>{labels[series]}</span>
                      <span className="text-caption font-normal text-muted-foreground">
                        {WORKSPACE_NAMES[series]
                          .slice(0, 3)
                          .map((entry) => locale === "zh-Hans" ? entry.zh : entry.en)
                          .join(locale === "zh-Hans" ? "、" : ", ")}
                      </span>
                    </span>
                  </DropdownMenuRadioItem>
                ))}
                <DropdownMenuRadioItem
                  value="all"
                  className="py-2 data-checked:font-medium max-sm:min-h-11 pointer-coarse:min-h-11"
                >
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span>{labels.all}</span>
                    <span className="text-caption font-normal text-muted-foreground">
                      {t(($) => $.name_picker.all_description)}
                    </span>
                  </span>
                </DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <p id={currentSeriesId} className="max-w-52 text-caption text-muted-foreground">
        {t(($) => $.name_picker.current_series, { series: labels[selection] })}
      </p>
    </div>
  );
}
