import { useEffect, useRef, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { useT } from "@multica/views/i18n";
import type { CloseBehavior } from "../../../shared/close-behavior";

/**
 * close-behavior-prompt.tsx
 *
 * Renderer-side modal that listens for the main process's
 * `close-behavior:prompt` IPC and asks the user how to handle a main-window
 * close. Mounted once near App root; responds over
 * `closeBehaviorAPI.respond(requestId, { action, remember })`.
 *
 * This component is desktop-only. Web never sees the prompt — the IPC
 * surface simply doesn't exist there, so the prompt is silently inert.
 */
export function CloseBehaviorPrompt() {
  const { t } = useT("desktop");
  const [openRequestId, setOpenRequestId] = useState<string | null>(null);
  const [remember, setRemember] = useState(false);
  const [traySupported, setTraySupported] = useState<boolean | null>(null);
  // Tracks the last requestId we've already responded to, so a single
  // IPC response goes out even when the AlertDialog open-change cycle
  // (button onClick → setOpenRequestId(null) → onOpenChange(false))
  // would otherwise call respond() twice.
  const respondedRef = useRef<string | null>(null);

  // Listen once for prompt IPC from main. Clean up on unmount.
  useEffect(() => {
    const off = window.closeBehaviorAPI.onPrompt(({ requestId }) => {
      respondedRef.current = null;
      setOpenRequestId(requestId);
      setRemember(false);
    });
    return off;
  }, []);

  // Cache the tray environment detection the first time the prompt opens.
  // Main has its own guard, so this is only used to decide whether to
  // disable the minimize button + explain why.
  useEffect(() => {
    if (openRequestId === null || traySupported !== null) return;
    let cancelled = false;
    void window.closeBehaviorAPI.isTraySupported().then((supported) => {
      if (!cancelled) setTraySupported(supported);
    });
    return () => {
      cancelled = true;
    };
  }, [openRequestId, traySupported]);

  const respond = (action: CloseBehavior) => {
    const requestId = openRequestId;
    if (requestId === null) return;
    if (respondedRef.current === requestId) return;
    respondedRef.current = requestId;
    window.closeBehaviorAPI.respond(requestId, { action, remember });
    setOpenRequestId(null);
  };

  return (
    <AlertDialog
      open={openRequestId !== null}
      onOpenChange={(next) => {
        // Treat any uncontrolled close (ESC, scrim click) as Cancel so
        // main doesn't stay wedged waiting for a respond. respond() reads
        // the latest openRequestId via the functional setState form, so
        // calling it from a button onClick AND from this onOpenChange in
        // the same event loop dedupes to a single IPC.
        if (!next) respond("ask");
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t(($) => $.close_behavior.prompt.title)}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t(($) => $.close_behavior.prompt.description)}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <label className="flex items-center gap-2 text-body">
          <Checkbox
            checked={remember}
            onCheckedChange={(v) => setRemember(v === true)}
          />
          <span>{t(($) => $.close_behavior.prompt.remember)}</span>
        </label>

        <AlertDialogFooter>
          <AlertDialogCancel onClick={() => respond("ask")}>
            {t(($) => $.close_behavior.prompt.cancel)}
          </AlertDialogCancel>
          {traySupported !== false && (
            <AlertDialogAction onClick={() => respond("minimize")}>
              {t(($) => $.close_behavior.prompt.minimize)}
            </AlertDialogAction>
          )}
          <AlertDialogAction
            onClick={() => respond("quit")}
            className="bg-destructive text-white hover:bg-destructive/90"
          >
            {t(($) => $.close_behavior.prompt.quit)}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
