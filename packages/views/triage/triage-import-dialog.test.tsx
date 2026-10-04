import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import type { TriageImportPreview } from "@multica/core/triage";
import en from "../locales/en/triage.json";
import { TriageImportDialog } from "./triage-import-dialog";
const mocks = vi.hoisted(() => ({
  preview: vi.fn(),
  commit: vi.fn(),
  refetch: vi.fn(),
}));
vi.mock("../i18n", () => ({
  useT: () => ({ t: (selector: (value: typeof en) => string) => selector(en) }),
}));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));
vi.mock("../navigation", () => ({
  AppLink: ({
    href,
    children,
  }: {
    href: string;
    children: React.ReactNode;
  }) => <a href={href}>{children}</a>,
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  usePreviewTriageImport: () => ({
    mutateAsync: mocks.preview,
    isPending: false,
  }),
  useCommitTriageImport: () => ({
    mutateAsync: mocks.commit,
    isPending: false,
  }),
  useDownloadTriageFailures: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: () => ({ data: undefined, refetch: mocks.refetch }),
}));
const row = (n: number) => ({
  row_number: n,
  values: { title: `Task ${n}` },
  warnings: [] as string[],
  errors: [] as string[],
  duplicate: false,
  duplicate_issue_id: null,
  similar_issue_ids: [],
  status: "ready" as const,
  issue_id: null,
  error: null,
});
const preview: TriageImportPreview = {
  batch_id: "batch",
  filename: "tasks.csv",
  headers: ["title"],
  mapping: { title: "title" },
  rows: [
    { ...row(1), warnings: ["Project not found; left empty"] },
    { ...row(2), duplicate: true },
    { ...row(3), errors: ["Date is invalid"] },
  ],
  counts: { valid: 0, warning: 1, error: 1, duplicate: 1 },
  limits: { max_rows: 1000, max_bytes: 5242880 },
};
function file(bytes: Uint8Array) {
  const value = new File([new Uint8Array(bytes).buffer], "tasks.csv", {
    type: "text/csv",
  });
  Object.defineProperty(value, "arrayBuffer", {
    value: () => Promise.resolve(bytes.buffer),
  });
  return value;
}
beforeEach(() => {
  vi.clearAllMocks();
  mocks.preview.mockResolvedValue(preview);
  mocks.refetch.mockResolvedValue({ data: preview });
});
it("keeps duplicate/error rows out of initial commit, then accepts an explicit per-row duplicate override", async () => {
  mocks.commit
    .mockResolvedValueOnce({
      batch_id: "batch",
      results: [{ ...row(1), status: "created", issue_id: "issue-1" }],
      created: 1,
      skipped: 0,
      failed: 0,
    })
    .mockResolvedValueOnce({
      batch_id: "batch",
      results: [{ ...row(2), status: "created", issue_id: "issue-2" }],
      created: 1,
      skipped: 0,
      failed: 0,
    });
  render(<TriageImportDialog wsId="ws" onClose={vi.fn()} />);
  fireEvent.change(screen.getByLabelText(en.choose_file), {
    target: { files: [file(new TextEncoder().encode("title\nTask 1"))] },
  });
  await screen.findByText("Date is invalid");
  expect(screen.getByRole("row", { name: /Task 3/ })).toHaveTextContent(
    "Invalid row",
  );
  expect(screen.getByRole("row", { name: /Task 2/ })).toHaveTextContent(
    "Duplicate external ID",
  );
  expect(screen.getByRole("row", { name: /Task 1/ })).toHaveTextContent(
    "Review warnings",
  );
  fireEvent.click(screen.getByRole("button", { name: en.csv_confirm }));
  await waitFor(() => expect(mocks.commit).toHaveBeenCalledOnce());
  expect(mocks.commit.mock.calls[0]![0]).toEqual({
    id: "batch",
    input: { rows: [{ row_number: 1, import_duplicate: false }] },
  });
  await waitFor(() => expect(mocks.refetch).toHaveBeenCalledOnce());
  fireEvent.click(screen.getByRole("checkbox", { name: en.import_anyway }));
  fireEvent.click(screen.getByRole("button", { name: en.retry_unfinished }));
  await waitFor(() => expect(mocks.commit).toHaveBeenCalledTimes(2));
  expect(mocks.commit.mock.calls[1]![0]).toEqual({
    id: "batch",
    input: { rows: [{ row_number: 2, import_duplicate: true }] },
  });
});
it("rejects invalid UTF-8 before uploading a preview", async () => {
  render(<TriageImportDialog wsId="ws" onClose={vi.fn()} />);
  fireEvent.change(screen.getByLabelText(en.choose_file), {
    target: { files: [file(new Uint8Array([0xff]))] },
  });
  expect(await screen.findByRole("alert")).toHaveTextContent(en.invalid_utf8);
  expect(mocks.preview).not.toHaveBeenCalled();
});
