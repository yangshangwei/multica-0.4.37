"use client";
import { useState } from "react";
import { CheckIcon, CopyIcon } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { copyText } from "@multica/ui/lib/clipboard";
import { useT } from "../../i18n";
import styles from "../admin-visual.module.css";

export function InstallationIdentifier({ id }: { id: string }) {
  const { t } = useT("admin");
  const [copyResult, setCopyResult] = useState<{ id: string; success: boolean } | null>(null);
  const copied = copyResult?.id === id && copyResult.success;
  return <details className="min-w-0 text-caption">
    <summary className="w-fit cursor-pointer text-muted-foreground">{t($ => $.installations.id)}</summary>
    <div className="min-w-0 space-y-2 pt-2">
      <Button type="button" variant="ghost" size="sm" onClick={async () => setCopyResult({ id, success: await copyText(id) })}>
        {copied ? <CheckIcon aria-hidden="true" /> : <CopyIcon aria-hidden="true" />}
        <span aria-live="polite">{copied ? t($ => $.installations.copiedId) : t($ => $.installations.copyId)}</span>
      </Button>
      {copyResult?.id === id && !copyResult.success && <p role="alert" className="text-caption text-muted-foreground">{t($ => $.installations.copyFailed)}</p>}
      <code className={`${styles.codeValue} block break-all select-all`}>{id}</code>
    </div>
  </details>;
}
