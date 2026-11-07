import { CollaborationRecipientNotFoundError, CollaborationUnavailableError, isCollaborationDenied } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";

export function CollaborationQueryState({ error, retry, denied = false }: { error: unknown; retry?: () => void; denied?: boolean }) {
  const { t } = useT("collaboration");
  return <div role="alert" className="min-w-0 space-y-3 py-6 text-body [overflow-wrap:anywhere]">
    <p>{denied || isCollaborationDenied(error) ? t(($) => $.page.denied) : error instanceof CollaborationRecipientNotFoundError ? t(($) => $.workflow.recipientNotFound) : error instanceof CollaborationUnavailableError ? t(($) => $.page.unavailable) : t(($) => $.page.loadError)}</p>
    {retry && <Button variant="outline" className="pointer-coarse:min-h-11" onClick={retry}>{t(($) => $.page.refresh)}</Button>}
  </div>;
}
