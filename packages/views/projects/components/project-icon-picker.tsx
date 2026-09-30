"use client";

import { AVATAR_ICON_COMPONENTS, AVATAR_ICON_TONE, formatAvatarIcon, type AvatarIconName } from "@multica/ui/lib/avatar-icon";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { resolveProjectIcon } from "./project-icon";

export function ProjectIconPicker({ value, onSelect }: { value?: string | null; onSelect: (value: string) => void }) {
  const { t } = useT("common");
  const selected = resolveProjectIcon(value);
  return (
    <div role="group" aria-label={t(($) => $.avatar_upload.icon_label)} className="grid max-h-64 grid-cols-8 gap-1 overflow-y-auto p-2">
      {(Object.keys(AVATAR_ICON_COMPONENTS) as AvatarIconName[]).map((name) => {
        const Icon = AVATAR_ICON_COMPONENTS[name];
        const label = t(($) => $.avatar_upload.icon_names[name]);
        return (
          <button
            key={name}
            type="button"
            aria-label={label}
            title={label}
            aria-pressed={selected === name}
            onClick={() => onSelect(formatAvatarIcon(name))}
            className={cn(
              "flex size-8 items-center justify-center rounded-md outline-hidden transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring",
              AVATAR_ICON_TONE[name].text,
              selected === name && "bg-accent ring-1 ring-ring",
            )}
          >
            <Icon aria-hidden="true" className="size-4" />
          </button>
        );
      })}
    </div>
  );
}
