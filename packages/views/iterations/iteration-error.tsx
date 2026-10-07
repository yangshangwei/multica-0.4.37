"use client";
import { ApiError, isIterationAccessDenied } from "@multica/core/api";
import { useT } from "../i18n";
export function IterationError({ error }: { error: unknown }) {
  const { t } = useT("projects");
  const status = error instanceof ApiError ? error.status : null;
  return (
    <p role="alert">
      {t(($) =>
        isIterationAccessDenied(error)
          ? $.iterations.permission
          : status === 409
            ? $.iterations.conflict
            : status === 413
              ? $.iterations.tooLarge
              : $.iterations.error,
      )}
    </p>
  );
}
