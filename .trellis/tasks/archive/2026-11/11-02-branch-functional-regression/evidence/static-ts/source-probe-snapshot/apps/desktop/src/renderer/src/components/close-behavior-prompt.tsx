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
  // Cleared synchronously so button, dialog-close, and unmount cannot reply twice.
  const pendingRequestRef = useRef<string | null>(null);

  // Listen once for prompt IPC from main. Clean up on unmount.
  useEffect(() => {
    const api = window.closeBehaviorAPI;
    const off = api.onPrompt(({ requestId }) => {
      pendingRequestRef.current = requestId;
      setOpenRequestId(requestId);
      setRemember(false);
      setTraySupported(null);
    });
    return () => {
      off();
      const requestId = pendingRequestRef.current;
      pendingRequestRef.current = null;
      if (requestId !== null) api.respond(requestId, { action: "ask", remember: false });
    };
  }, []);

  // Acknowledge delivery after rendering; user decision time has no deadline.
  // Recheck every opening because the Linux tray host can come and go.
  useEffect(() => {
    if (openRequestId === null) return;
    window.closeBehaviorAPI.acknowledge(openRequestId);
    let cancelled = false;
    void window.closeBehaviorAPI.isTraySupported().then((supported) => {
      if (!cancelled) setTraySupported(supported);
    }).catch(() => {
      if (!cancelled) setTraySupported(false);
    });
    return () => {
      cancelled = true;
    };
  }, [openRequestId]);

  const respond = (action: CloseBehavior) => {
    const requestId = pendingRequestRef.current;
    if (requestId === null) return;
    pendingRequestRef.current = null;
    window.closeBehaviorAPI.respond(requestId, { action, remember });
    setOpenRequestId(null);
  };

  return (
    <AlertDialog
      open={openRequestId !== null}
      onOpenChange={(next) => {
        // The ref prevents button clicks and dialog close events from
        // responding twice to the same request.
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
          {traySupported === true && (
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
