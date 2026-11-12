"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Spinner } from "@multica/ui/components/ui/spinner";
import { useT } from "../i18n";

export type CompletionHintModalState = "attention" | "matching" | "refusal" | "unknown";

export interface IssueCompletionHintModalData {
  issueId: string;
  issueRevision: number;
  targetStatus: string;
  state: CompletionHintModalState;
  message?: string;
  onConfirm?: () => void | Promise<void>;
  onRetry?: () => void | Promise<void>;
  onCancel?: () => void;
}

export function IssueCompletionHintModal({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const { t } = useT("issues");
  const [pending, setPending] = useState<"confirm" | "retry" | null>(null);
  const d = (data ?? {}) as Partial<IssueCompletionHintModalData>;
  const state = d.state ?? "unknown";
  const canConfirm = state === "attention" || state === "matching";
  const title =
    state === "matching"
      ? t(($) => $.detail.completion_hint.matching_title)
      : state === "attention"
        ? t(($) => $.detail.completion_hint.attention_title)
        : state === "refusal"
          ? t(($) => $.detail.completion_hint.refusal_title)
          : t(($) => $.detail.completion_hint.unknown_title);
  const description =
    d.message ??
    (state === "matching"
      ? t(($) => $.detail.completion_hint.matching_description)
      : state === "attention"
        ? t(($) => $.detail.completion_hint.attention_description)
        : state === "refusal"
          ? t(($) => $.detail.completion_hint.refusal_description)
          : t(($) => $.detail.completion_hint.unknown_description));

  const close = () => {
    if (pending) return;
    d.onCancel?.();
    onClose();
  };

  const confirm = async () => {
    if (!d.onConfirm || pending) return;
    setPending("confirm");
    try {
      await d.onConfirm();
      onClose();
    } finally {
      setPending(null);
    }
  };

  const retry = async () => {
    if (!d.onRetry || pending) return;
    setPending("retry");
    // A retry may open a new result dialog. Close this result before starting
    // the read so it cannot close the replacement when the read finishes.
    onClose();
    try {
      await d.onRetry();
    } finally {
      setPending(null);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) close(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" disabled={pending !== null} onClick={close}>
            {t(($) => $.detail.completion_hint.cancel)}
          </Button>
          {d.onRetry && !canConfirm ? (
            <Button type="button" disabled={pending !== null} onClick={() => void retry()}>
              {pending === "retry" ? <Spinner className="size-4" /> : t(($) => $.detail.completion_hint.retry)}
            </Button>
          ) : null}
          {canConfirm ? (
            <Button type="button" disabled={pending !== null} onClick={() => void confirm()}>
              {pending === "confirm" ? <Spinner className="size-4" /> : t(($) => $.detail.completion_hint.continue)}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
