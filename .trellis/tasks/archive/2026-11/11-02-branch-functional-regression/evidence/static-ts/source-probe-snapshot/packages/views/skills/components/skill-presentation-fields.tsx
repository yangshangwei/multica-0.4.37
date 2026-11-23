"use client";

import { useId, useState, type ReactNode } from "react";
import {
  SKILL_CATEGORIES,
  SKILL_CATEGORY_DEFAULT_ICON,
  SKILL_ICON_NAMES,
  isSkillCategory,
  type SkillCategory,
  type SkillIconName,
  type SkillPresentationMeta,
} from "@multica/core/skills";
import { Label } from "@multica/ui/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { SKILL_CATEGORY_TONE, SKILL_ICON_COMPONENTS } from "../lib/skill-presentation-icon";
import { SkillPresentationIcon } from "./skill-presentation-icon";

/**
 * Category / icon editor shared by the create dialog and the detail page.
 * Controlled: the owner keeps the draft and decides when it is saved. The
 * category picks the colour and the fallback icon; an explicit icon only
 * overrides the shape, so changing category with `icon: null` stays null.
 * Labels are workspace labels and live in `ResourceLabelPicker`, not here.
 */
export function SkillPresentationFields({
  value,
  onChange,
  disabled = false,
  layout = "stacked",
}: {
  value: SkillPresentationMeta;
  onChange: (next: SkillPresentationMeta) => void;
  disabled?: boolean;
  /** `stacked` renders its own labels; `rows` yields one field per row for a property grid. */
  layout?: "stacked" | "rows";
}) {
  const { t } = useT("skills");
  const baseId = useId();

  const categoryLabels = Object.fromEntries(
    SKILL_CATEGORIES.map((key) => [key, t(($) => $.categories[key])]),
  ) as Record<SkillCategory, string>;

  const categoryField = (
    <Select
      items={categoryLabels}
      value={value.category}
      disabled={disabled}
      onValueChange={(next) => {
        if (!isSkillCategory(next) || next === value.category) return;
        onChange({ ...value, category: next });
      }}
    >
      <SelectTrigger
        id={`${baseId}-category`}
        className="w-full"
        aria-label={t(($) => $.presentation.category_label)}
      >
        <SelectValue>
          <SkillPresentationIcon meta={{ ...value, icon: null }} size="sm" />
          <span className="truncate">{categoryLabels[value.category]}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent align="start">
        {SKILL_CATEGORIES.map((key) => (
          <SelectItem key={key} value={key}>
            <span className="flex items-center gap-2">
              <SkillPresentationIcon
                meta={{ category: key, icon: null }}
                size="sm"
              />
              {categoryLabels[key]}
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );

  const iconField = (
    <IconPicker
      value={value}
      disabled={disabled}
      onPick={(icon) => onChange({ ...value, icon })}
    />
  );

  if (layout === "rows") {
    return (
      <>
        <PresentationRow
          label={t(($) => $.presentation.category_label)}
          htmlFor={`${baseId}-category`}
        >
          {categoryField}
        </PresentationRow>
        <PresentationRow label={t(($) => $.presentation.icon_label)}>
          {iconField}
        </PresentationRow>
      </>
    );
  }

  return (
    <div className="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-3">
      <div className="space-y-1.5">
        <Label
          htmlFor={`${baseId}-category`}
          className="text-caption text-muted-foreground"
        >
          {t(($) => $.presentation.category_label)}
        </Label>
        {categoryField}
      </div>
      <div className="space-y-1.5">
        <span className="block text-caption text-muted-foreground">
          {t(($) => $.presentation.icon_label)}
        </span>
        {iconField}
      </div>
    </div>
  );
}

// Mirrors the detail page's PropertyRow grid so the presentation rows line
// up with Name / Description without importing a page-private component.
function PresentationRow({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor?: string;
  children: ReactNode;
}) {
  return (
    <div className="grid gap-1.5 py-2.5 sm:grid-cols-[128px_minmax(0,1fr)] sm:gap-4">
      <label
        htmlFor={htmlFor}
        className="pt-1.5 text-caption text-muted-foreground sm:text-body"
      >
        {label}
      </label>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Icon picker
// ---------------------------------------------------------------------------

function IconPicker({
  value,
  disabled,
  onPick,
}: {
  value: SkillPresentationMeta;
  disabled: boolean;
  onPick: (icon: SkillIconName | null) => void;
}) {
  const { t } = useT("skills");
  const [open, setOpen] = useState(false);
  const tone = SKILL_CATEGORY_TONE[value.category];
  const defaultIcon = SKILL_CATEGORY_DEFAULT_ICON[value.category];
  const DefaultIcon = SKILL_ICON_COMPONENTS[defaultIcon];

  const pick = (icon: SkillIconName | null) => {
    onPick(icon);
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        disabled={disabled}
        aria-label={t(($) => $.presentation.icon_pick)}
        className="flex h-8 items-center gap-2 rounded-lg border border-input px-1.5 pr-2.5 text-body transition-colors hover:bg-accent/50 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
      >
        <SkillPresentationIcon meta={value} size="sm" />
        <span className="text-caption text-muted-foreground">
          {value.icon ?? t(($) => $.presentation.icon_follow_category)}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80">
        <button
          type="button"
          aria-pressed={value.icon === null}
          onClick={() => pick(null)}
          className={cn(
            "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-body transition-colors hover:bg-accent",
            value.icon === null && "bg-accent font-medium",
          )}
        >
          <span
            aria-hidden
            className={cn(
              "inline-flex size-6 items-center justify-center rounded-md",
              tone.bg,
              tone.text,
            )}
          >
            <DefaultIcon className="size-3.5" />
          </span>
          {t(($) => $.presentation.icon_follow_category)}
        </button>
        <div
          role="group"
          aria-label={t(($) => $.presentation.icon_pick)}
          className="grid max-h-56 grid-cols-8 gap-1 overflow-y-auto"
        >
          {SKILL_ICON_NAMES.map((name) => {
            const Icon = SKILL_ICON_COMPONENTS[name];
            const selected = value.icon === name;
            return (
              <button
                key={name}
                type="button"
                aria-label={name}
                aria-pressed={selected}
                title={name}
                onClick={() => pick(name)}
                className={cn(
                  "flex size-8 items-center justify-center rounded-md transition-colors hover:bg-accent",
                  selected ? cn(tone.bg, tone.text) : "text-muted-foreground",
                )}
              >
                <Icon className="size-4" />
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
