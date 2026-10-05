"use client";
import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  createTriageRequestId,
  triageImportOptions,
  usePreviewTriageImport,
  useCommitTriageImport,
  useDownloadTriageFailures,
  type TriageImportPreview,
  type TriageImportResult,
  type TriageImportRow,
} from "@multica/core/triage";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { TriageSelect } from "./triage-fields";
import { decodeTriageCsv, TRIAGE_CONTROL } from "./triage-ui";

const MAPPING_FIELDS = [
  "ignore",
  "title",
  "description",
  "priority",
  "labels",
  "project",
  "assignee",
  "start_date",
  "due_date",
  "source_url",
  "external_id",
] as const;

export function TriageImportDialog({
  wsId,
  batchId,
  onClose,
}: {
  wsId: string;
  batchId?: string;
  onClose: () => void;
}) {
  const { t } = useT("triage");
  const paths = useWorkspacePaths();
  const previewMutation = usePreviewTriageImport(wsId);
  const commit = useCommitTriageImport(wsId);
  const download = useDownloadTriageFailures(wsId);
  const [localPreview, setLocalPreview] = useState<TriageImportPreview | null>(
    null,
  );
  const currentBatchId = batchId ?? localPreview?.batch_id;
  const existing = useQuery({
    ...triageImportOptions(wsId, currentBatchId ?? ""),
    enabled: !!currentBatchId,
  });
  const preview = localPreview ?? existing.data;
  const [file, setFile] = useState<{ name: string; csv: string } | null>(null);
  const [mapping, setMapping] = useState<Record<string, string> | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [overrides, setOverrides] = useState<Set<number>>(new Set());
  const [result, setResult] = useState<TriageImportResult | null>(null);
  const [error, setError] = useState("");
  const [reading, setReading] = useState(false);
  const intention = useRef<{ signature: string; id: string } | null>(null);
  const busy = reading || previewMutation.isPending || commit.isPending;
  const mappingDirty =
    !!preview &&
    !!mapping &&
    JSON.stringify(mapping) !== JSON.stringify(preview.mapping);
  const rows = (preview?.rows ?? []).map(
    (row) =>
      result?.results.find((r) => r.row_number === row.row_number) ?? row,
  );
  const canSelect = (row: TriageImportRow) =>
    !row.errors.length &&
    row.status !== "created" &&
    (!row.duplicate || overrides.has(row.row_number));
  const runPreview = async (
    source: { name: string; csv: string },
    map?: Record<string, string>,
  ) => {
    const signature = JSON.stringify({ source, map });
    if (intention.current?.signature !== signature)
      intention.current = { signature, id: createTriageRequestId() };
    setError("");
    try {
      const response = await previewMutation.mutateAsync({
        request_id: intention.current.id,
        filename: source.name,
        csv: source.csv,
        mapping: map,
      });
      setLocalPreview(response);
      setMapping(response.mapping);
      setSelected(
        new Set(
          response.rows
            .filter(
              (r) => !r.errors.length && !r.duplicate && r.status !== "created",
            )
            .map((r) => r.row_number),
        ),
      );
      setOverrides(new Set());
      setResult(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    }
  };
  const chooseFile = async (selectedFile: File) => {
    if (selectedFile.size > 5 * 1024 * 1024) {
      setError(t(($) => $.file_too_large));
      return;
    }
    setReading(true);
    setError("");
    try {
      let csv: string;
      try {
        csv = decodeTriageCsv(await selectedFile.arrayBuffer());
      } catch {
        throw new Error(t(($) => $.invalid_utf8));
      }
      const source = { name: selectedFile.name, csv };
      setFile(source);
      await runPreview(source);
    } catch (e) {
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    } finally {
      setReading(false);
    }
  };
  const submit = async () => {
    if (!preview || busy || mappingDirty) return;
    const eligible = rows.filter(
      (row) => selected.has(row.row_number) && canSelect(row),
    );
    if (!eligible.length) return;
    try {
      const next = await commit.mutateAsync({
        id: preview.batch_id,
        input: {
          rows: eligible.map((row) => ({
            row_number: row.row_number,
            import_duplicate: overrides.has(row.row_number),
          })),
        },
      });
      setResult((old) => ({
        ...next,
        results: [
          ...(old?.results ?? []).filter(
            (r) => !next.results.some((n) => n.row_number === r.row_number),
          ),
          ...next.results,
        ],
      }));
      setSelected(
        new Set(
          next.results
            .filter((row) => row.status === "failed")
            .map((row) => row.row_number),
        ),
      );
      const refreshed = await existing.refetch();
      if (refreshed.data) setLocalPreview(refreshed.data);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    }
  };
  const downloadFailures = async () => {
    if (!preview) return;
    try {
      const csv = await download.mutateAsync(preview.batch_id);
      const url = URL.createObjectURL(
        new Blob([csv], { type: "text/csv;charset=utf-8" }),
      );
      const link = document.createElement("a");
      link.href = url;
      link.download = `${preview.filename.replace(/\.csv$/i, "")}-failures.csv`;
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose();
      }}
    >
      <DialogContent className="flex max-h-[92dvh] flex-col overflow-hidden sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.import_csv)}</DialogTitle>
          <DialogDescription>{t(($) => $.csv_hint)}</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 space-y-5 overflow-y-auto">
          {!batchId && (
            <div className="space-y-2">
              <label
                className="text-caption font-medium"
                htmlFor="triage-csv-file"
              >
                {t(($) => $.choose_file)}
              </label>
              <Input
                id="triage-csv-file"
                type="file"
                accept=".csv,text/csv"
                disabled={busy}
                onChange={(e) => {
                  const selectedFile = e.target.files?.[0];
                  if (selectedFile) void chooseFile(selectedFile);
                }}
              />
            </div>
          )}
          {(busy || (existing.isPending && !!batchId)) && (
            <p role="status" className="text-body">
              {t(($) => $.processing)}
            </p>
          )}
          {existing.isError && <p role="alert">{existing.error.message}</p>}
          {preview && (
            <>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="break-all font-medium">{preview.filename}</p>
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.valid_rows, preview.counts)}
                </p>
              </div>
              {file && !result && (
                <details open>
                  <summary className="cursor-pointer font-medium">
                    {t(($) => $.mapping)}
                  </summary>
                  <p className="my-3 text-caption text-muted-foreground">
                    {t(($) => $.mapping_hint)}
                  </p>
                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                    {preview.headers.map((header) => (
                      <div key={header} className="space-y-1">
                        <label className="block break-words text-caption">
                          {header}
                        </label>
                        <TriageSelect
                          label={header}
                          value={mapping?.[header] ?? "ignore"}
                          onChange={(value) =>
                            setMapping((old) => ({ ...old, [header]: value }))
                          }
                          disabled={busy}
                          options={MAPPING_FIELDS.map((value) => ({
                            value,
                            label:
                              value === "title"
                                ? t(($) => $.title_field)
                                : t(($) => $[value]),
                          }))}
                        />
                      </div>
                    ))}
                  </div>
                  <Button
                    variant="outline"
                    className={`mt-3 ${TRIAGE_CONTROL}`}
                    disabled={busy}
                    onClick={() => void runPreview(file, mapping ?? undefined)}
                  >
                    {t(($) => $.apply_mapping)}
                  </Button>
                </details>
              )}
              <section className="space-y-3">
                <h3 className="font-medium">{t(($) => $.rows)}</h3>
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.csv_confirm_hint)}
                </p>
                <div className="flex flex-wrap gap-2">
                  <Button
                    variant="outline"
                    className={TRIAGE_CONTROL}
                    disabled={busy}
                    onClick={() =>
                      setSelected(
                        new Set(
                          rows
                            .filter((row) => canSelect(row) && !row.duplicate)
                            .map((row) => row.row_number),
                        ),
                      )
                    }
                  >
                    {t(($) => $.select_valid)}
                  </Button>
                  <Button
                    variant="ghost"
                    className={TRIAGE_CONTROL}
                    disabled={busy}
                    onClick={() => setSelected(new Set())}
                  >
                    {t(($) => $.clear_selection)}
                  </Button>
                  <span className="self-center text-caption" role="status">
                    {t(($) => $.selected, { count: selected.size })}
                  </span>
                </div>
                <div className="overflow-x-auto rounded-md border border-surface-border">
                  <table className="w-full text-left text-caption">
                    <thead className="bg-muted">
                      <tr>
                        <th className="p-3">{t(($) => $.rows)}</th>
                        <th className="p-3">{t(($) => $.title_field)}</th>
                        <th className="p-3">{t(($) => $.result)}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((row) => (
                        <tr
                          key={row.row_number}
                          className="border-t border-surface-border align-top"
                        >
                          <td className="p-3">
                            <label className="flex min-h-11 items-start gap-2">
                              <Checkbox
                                checked={selected.has(row.row_number)}
                                disabled={busy || !canSelect(row)}
                                aria-label={t(($) => $.row_number, {
                                  row: row.row_number,
                                })}
                                onCheckedChange={(checked) =>
                                  setSelected((old) => {
                                    const next = new Set(old);
                                    if (checked) next.add(row.row_number);
                                    else next.delete(row.row_number);
                                    return next;
                                  })
                                }
                              />
                              <span className="whitespace-nowrap">
                                {t(($) => $.row_number, {
                                  row: row.row_number,
                                })}
                              </span>
                            </label>
                          </td>
                          <td className="min-w-40 max-w-sm break-words p-3">
                            <span className="font-medium">
                              {row.values.title ?? "—"}
                            </span>
                            <details className="mt-2">
                              <summary className="cursor-pointer text-muted-foreground">
                                {t(($) => $.changes)}
                              </summary>
                              {Object.entries(row.values).map(
                                ([key, value]) => (
                                  <p className="break-words" key={key}>
                                    {key}: {value}
                                  </p>
                                ),
                              )}
                            </details>
                          </td>
                          <td className="min-w-56 max-w-lg space-y-2 p-3">
                            <p>
                              {row.status === "created"
                                ? t(($) => $.created)
                                : row.status === "failed"
                                  ? t(($) => $.failed)
                                  : row.status === "skipped"
                                    ? t(($) => $.skipped)
                                    : row.errors.length > 0 || row.error
                                      ? t(($) => $.invalid_row)
                                      : row.duplicate
                                        ? t(($) => $.duplicate_row)
                                        : row.warnings.length > 0
                                          ? t(($) => $.warning_row)
                                          : row.status === "ready"
                                            ? t(($) => $.ready_row)
                                            : t(($) => $.unknown)}
                            </p>
                            {[
                              ...row.errors,
                              ...(row.error ? [row.error] : []),
                            ].map((message, index) => (
                              <p
                                className="text-destructive"
                                key={`e-${index}`}
                              >
                                {message}
                              </p>
                            ))}
                            {row.warnings.map((message, index) => (
                              <p
                                className="text-muted-foreground"
                                key={`w-${index}`}
                              >
                                {message}
                              </p>
                            ))}
                            {row.duplicate && row.status !== "created" && (
                              <label className="flex min-h-11 items-center gap-2">
                                <Checkbox
                                  checked={overrides.has(row.row_number)}
                                  disabled={busy || !!row.errors.length}
                                  onCheckedChange={(checked) => {
                                    setOverrides((old) => {
                                      const next = new Set(old);
                                      if (checked) next.add(row.row_number);
                                      else next.delete(row.row_number);
                                      return next;
                                    });
                                    setSelected((old) => {
                                      const next = new Set(old);
                                      if (checked && !row.errors.length)
                                        next.add(row.row_number);
                                      else next.delete(row.row_number);
                                      return next;
                                    });
                                  }}
                                />
                                {t(($) => $.import_anyway)}
                              </label>
                            )}
                            {row.issue_id && (
                              <AppLink
                                href={paths.issueDetail(row.issue_id)}
                                className="underline"
                              >
                                {t(($) => $.open_task)}
                              </AppLink>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
              {result && (
                <p role="status" className="font-medium">
                  {t(($) => $.csv_results, {
                    created: rows.filter((r) => r.status === "created").length,
                    skipped: rows.filter((r) => r.status === "skipped").length,
                    failed: rows.filter((r) => r.status === "failed").length,
                  })}
                </p>
              )}
            </>
          )}
          {error && (
            <p role="alert" className="text-destructive">
              {error}
            </p>
          )}
        </div>
        <DialogFooter className="flex-wrap">
          <Button
            variant="outline"
            className={TRIAGE_CONTROL}
            disabled={busy}
            onClick={onClose}
          >
            {t(($) => $.close)}
          </Button>
          {preview && (
            <>
              <Button
                variant="outline"
                className={TRIAGE_CONTROL}
                disabled={download.isPending}
                onClick={() => void downloadFailures()}
              >
                {t(($) => $.download_failures)}
              </Button>
              <Button
                className={TRIAGE_CONTROL}
                disabled={
                  busy ||
                  mappingDirty ||
                  !rows.some(
                    (row) => selected.has(row.row_number) && canSelect(row),
                  )
                }
                onClick={() => void submit()}
              >
                {commit.isPending
                  ? t(($) => $.processing)
                  : result
                    ? t(($) => $.retry_unfinished)
                    : t(($) => $.csv_confirm)}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
