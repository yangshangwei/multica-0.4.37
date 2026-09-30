"use client";

import { useRef, useState } from "react";
import { Bot, Camera, ImagePlus, Loader2, Users, X } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { useFileUpload } from "@multica/core/hooks/use-file-upload";
import { resolvePublicFileUrl } from "@multica/core/workspace/avatar-url";
import {
  AVATAR_ICON_COMPONENTS,
  AVATAR_ICON_TONE,
  formatAvatarIcon,
  resolveAvatarIcon,
  type AvatarIconName,
} from "@multica/ui/lib/avatar-icon";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import { Separator } from "@multica/ui/components/ui/separator";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AvatarCropDialog } from "./avatar-crop-dialog";

export type AvatarUploadVariant = "user" | "agent" | "squad" | "workspace";

interface AvatarUploadControlProps {
  /** Current avatar URL, raw (unresolved). `null` renders the empty state. */
  value: string | null;
  /** Drives the empty-state fallback icon/initials. */
  variant: AvatarUploadVariant;
  /** Name used for initials / first-letter fallback and the image alt. */
  name?: string;
  /** Pixel diameter of the circle. Defaults to 64. */
  size?: number;
  disabled?: boolean;
  /**
   * Fires with the uploaded file URL after a successful crop + upload. The
   * parent persists it (updateMe / updateWorkspace / updateAgent /
   * updateSquad, or stashing it for a create call). The crop dialog stays in
   * its busy state until this resolves, then closes.
   */
  onUploaded: (url: string) => void | Promise<unknown>;
  /** Persists a selected icon marker. Agent/squad pickers default to onUploaded. */
  onIconSelected?: (value: string) => void | Promise<unknown>;
  /**
   * When provided, shows a small clear affordance. Used by create flows to
   * drop a not-yet-persisted choice; edit flows omit it (removing a saved
   * avatar is out of scope).
   */
  onClear?: () => void;
  className?: string;
  ariaLabel?: string;
  /** Optional shared presentation for the resting avatar; persistence is unchanged. */
  preview?: React.ReactNode;
}

function initialsOf(name: string): string {
  return name
    .split(" ")
    .map((word) => word[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

function AvatarFallback({
  variant,
  name,
  size,
}: {
  variant: AvatarUploadVariant;
  name: string;
  size: number;
}) {
  if (variant === "agent") {
    return <Bot style={{ width: size * 0.5, height: size * 0.5 }} />;
  }
  if (variant === "squad") {
    return <Users style={{ width: size * 0.5, height: size * 0.5 }} />;
  }
  const text =
    variant === "workspace"
      ? name.charAt(0).toUpperCase()
      : initialsOf(name);
  return (
    <span className="font-semibold" style={{ fontSize: size * 0.4 }}>
      {text}
    </span>
  );
}

/**
 * Runs the caller's persistence callback and reports whether it succeeded.
 *
 * A rejection is deliberately swallowed rather than toasted: the caller that
 * performed the write already owns that feedback. `AgentDetailPage.handleUpdate`
 * toasts and then rethrows so autosave can render a failed state, and the
 * settings tabs toast their own; the create flows only stash the value in
 * local state and cannot fail at all. Reporting it here too would show the
 * same failure twice. The upload this control runs itself has no other owner,
 * so that one is still announced.
 */
async function persistedByCaller(
  save: () => void | Promise<unknown>,
): Promise<boolean> {
  try {
    await save();
    return true;
  } catch {
    return false;
  }
}

/**
 * Shared click-to-upload avatar control for web/desktop. Renders the current
 * avatar with a hover "change" affordance; on pick it opens {@link
 * AvatarCropDialog} for reposition/zoom, then uploads the cropped image
 * through the existing `/api/upload-file` chain and hands the URL back via
 * `onUploaded`. For agents and squads the click opens a picker that offers
 * Lucide icons instead — both paths produce an `avatar_url` value. Business
 * persistence stays with the caller.
 */
export function AvatarUploadControl({
  value,
  variant,
  name = "",
  size = 64,
  disabled = false,
  onUploaded,
  onIconSelected,
  onClear,
  className,
  ariaLabel,
  preview,
}: AvatarUploadControlProps) {
  const { t } = useT("common");
  const { upload } = useFileUpload(api);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [pickedFile, setPickedFile] = useState<File | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [previewError, setPreviewError] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);

  const storedIcon = resolveAvatarIcon(value);
  const iconName = storedIcon ?? ((!value || previewError)
    ? variant === "agent" ? "bot" : variant === "squad" ? "users" : null
    : null);
  const Icon = iconName ? AVATAR_ICON_COMPONENTS[iconName] : null;
  const iconTone = iconName ? AVATAR_ICON_TONE[iconName] : null;
  const resolved = value && !iconName ? resolvePublicFileUrl(value) : null;
  const hasImage = !!resolved && !previewError;
  const hasAvatar = !!storedIcon || hasImage;
  const iconEnabled = variant === "agent" || variant === "squad" || !!onIconSelected;

  const openFileDialog = () => fileInputRef.current?.click();

  const closePicker = () => {
    setPickerOpen(false);
  };

  const handlePickerOpenChange = (next: boolean) => {
    setPickerOpen(next);
  };

  const handlePick = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = ""; // allow re-selecting the same file
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      toast.error(t(($) => $.avatar_upload.select_image));
      return;
    }
    setPreviewError(false);
    setPickedFile(file);
    setDialogOpen(true);
  };

  const handleCropped = async (cropped: File) => {
    setBusy(true);
    try {
      const result = await upload(cropped);
      if (!result) return;
      if (!(await persistedByCaller(() => onUploaded(result.link)))) return;
      setDialogOpen(false);
      setPickedFile(null);
      toast.success(t(($) => $.avatar_upload.updated));
    } catch (err) {
      // Only the upload above can land here — see persistedByCaller.
      toast.error(
        err instanceof Error ? err.message : t(($) => $.avatar_upload.failed),
      );
    } finally {
      setBusy(false);
    }
  };

  // Busy for the whole save, same as an upload: an edit caller PATCHes on every
  // pick, so leaving the trigger live would let a second pick race the first.
  // Both writes are last-one-to-arrive-wins on the server, and the loser can be
  // the one the user chose last — a permanently wrong avatar, since the
  // invalidate that follows only converges on whatever the server kept.
  //
  // No success toast on purpose: the avatar swaps to the chosen icon in place,
  // so the change is already visible.
  const handleIconSelected = async (picked: AvatarIconName) => {
    closePicker();
    setPreviewError(false);
    setBusy(true);
    try {
      await persistedByCaller(() =>
        (onIconSelected ?? onUploaded)(formatAvatarIcon(picked)),
      );
    } finally {
      setBusy(false);
    }
  };

  const avatarButton = (
    <button
      type="button"
      // With icons enabled this button is the popover trigger and Base UI
      // supplies the click handler; without it the click goes straight to the
      // file dialog, which is this control's original single-purpose shape.
      onClick={iconEnabled ? undefined : openFileDialog}
      disabled={disabled || busy}
      aria-label={ariaLabel ?? t(($) => $.avatar_upload.change)}
      className={cn(
        "group relative h-full w-full overflow-hidden bg-muted text-muted-foreground outline-none",
        "flex items-center justify-center",
        "focus-visible:ring-2 focus-visible:ring-ring",
        "disabled:cursor-not-allowed disabled:opacity-60",
        "rounded-full",
        iconTone?.bg,
        iconTone?.text,
        className,
      )}
      style={{ width: size, height: size }}
    >
      {preview ?? (Icon ? (
        <Icon aria-hidden="true" style={{ width: size * 0.5, height: size * 0.5 }} />
      ) : hasImage ? (
        <img
          src={resolved ?? undefined}
          alt={name}
          className="h-full w-full object-cover"
          onError={() => setPreviewError(true)}
        />
      ) : (
        <AvatarFallback variant={variant} name={name} size={size} />
      ))}

      {!disabled && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
          {busy ? (
            <Loader2 className="h-5 w-5 animate-spin text-white" />
          ) : (
            <Camera className="h-5 w-5 text-white" />
          )}
        </div>
      )}
    </button>
  );

  return (
    <div className="relative shrink-0" style={{ width: size, height: size }}>
      {iconEnabled ? (
        <Popover open={pickerOpen} onOpenChange={handlePickerOpenChange}>
          <PopoverTrigger render={avatarButton} />
          <PopoverContent
            align="start"
            className="w-80 gap-0 p-0"
          >
            <div className="p-1">
              <button
                type="button"
                onClick={() => { closePicker(); openFileDialog(); }}
                className="flex w-full items-center gap-1.5 rounded-md px-1.5 py-1.5 text-body outline-hidden transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground"
              >
                <ImagePlus className="size-4 shrink-0 text-muted-foreground" />
                {t(($) => $.avatar_upload.upload_image)}
              </button>
            </div>
            <Separator />
            <div className="p-2">
              <p className="mb-2 text-caption font-medium text-muted-foreground">
                {t(($) => $.avatar_upload.icon_label)}
              </p>
              <div role="group" aria-label={t(($) => $.avatar_upload.icon_label)} className="grid max-h-64 grid-cols-8 gap-1 overflow-y-auto">
                {(Object.keys(AVATAR_ICON_COMPONENTS) as AvatarIconName[]).map((name) => {
                  const ChoiceIcon = AVATAR_ICON_COMPONENTS[name];
                  const label = t(($) => $.avatar_upload.icon_names[name]);
                  return (
                    <button
                      key={name}
                      type="button"
                      aria-label={label}
                      title={label}
                      aria-pressed={iconName === name}
                      onClick={() => handleIconSelected(name)}
                      className={cn(
                        "flex size-8 items-center justify-center rounded-full outline-hidden transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring",
                        AVATAR_ICON_TONE[name].text,
                        iconName === name && "bg-accent ring-1 ring-ring",
                      )}
                    >
                      <ChoiceIcon aria-hidden="true" className="size-4" />
                    </button>
                  );
                })}
              </div>
            </div>
          </PopoverContent>
        </Popover>
      ) : (
        avatarButton
      )}

      {onClear && hasAvatar && !busy && !disabled && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            setPreviewError(false);
            onClear();
          }}
          className="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full border bg-background text-muted-foreground shadow-sm transition-colors hover:bg-muted hover:text-foreground"
          aria-label={t(($) => $.avatar_upload.remove)}
        >
          <X className="h-3 w-3" />
        </button>
      )}

      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handlePick}
      />

      <AvatarCropDialog
        file={pickedFile}
        open={dialogOpen}
        busy={busy}
        onOpenChange={(next) => {
          if (busy) return;
          setDialogOpen(next);
          if (!next) setPickedFile(null);
        }}
        onCropped={handleCropped}
      />
    </div>
  );
}
