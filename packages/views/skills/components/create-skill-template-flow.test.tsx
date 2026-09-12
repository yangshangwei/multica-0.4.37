import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError, SkillCreationUnconfirmedError } from "@multica/core/api";
import type { CreateSkillRequest, Skill, SkillTemplate } from "@multica/core/types";
import type { SupportedLocale } from "@multica/core/i18n";
import { I18nProvider } from "@multica/core/i18n/react";
import { parseFrontmatter } from "@multica/core/skills/frontmatter";
import { skillDetailOptions, workspaceKeys } from "@multica/core/workspace/queries";
import enCommon from "../../locales/en/common.json";
import enSkills from "../../locales/en/skills.json";
import zhCommon from "../../locales/zh-Hans/common.json";
import zhSkills from "../../locales/zh-Hans/skills.json";

const mocks = vi.hoisted(() => ({
  workspaceId: "ws-1",
  listSkillTemplates: vi.fn(),
  listSkills: vi.fn(),
  getSkill: vi.fn(),
  createSkill: vi.fn(),
  updateSkill: vi.fn(),
  importSkillArchive: vi.fn(),
  setAgentSkills: vi.fn(),
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    listSkillTemplates: mocks.listSkillTemplates,
    listSkills: mocks.listSkills,
    getSkill: mocks.getSkill,
    createSkill: mocks.createSkill,
    updateSkill: mocks.updateSkill,
    importSkillArchive: mocks.importSkillArchive,
    setAgentSkills: mocks.setAgentSkills,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => mocks.workspaceId,
}));

vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return {
    useAuthStore: Object.assign(
      (selector?: (value: typeof state) => unknown) => selector ? selector(state) : state,
      { getState: () => state },
    ),
  };
});

// Markdown rendering is a heavy leaf; draft creation and serialization stay real.
// Their YAML and candidate-name matrices live in core/skills/template-draft.test.ts.
vi.mock("../../rich-content", () => ({
  RichContent: ({ content }: { content: string }) => <div>{content}</div>,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { CreateSkillDialog } from "./create-skill-dialog";

const LOCALES = {
  en: { common: enCommon, skills: enSkills },
  "zh-Hans": { common: zhCommon, skills: zhSkills },
};
type TestLocale = Extract<SupportedLocale, keyof typeof LOCALES>;

const REVIEW_NAME = "multica-code-review";
const REPORT_NAME = "multica-test-report";
const REVIEW_BODY = "\n# 代码审查\n\n检查改动，报告可复现的问题。\n";
const REPORT_BODY = "\n# 测试报告\n\n记录测试范围和验证证据。\n";
const EDITED_BODY = "# Billing review\n\nReview the changed payment code.\n";
const EDITED_DESCRIPTION = "Review payment changes: include regression evidence.";
const COPIED_ID = "12345678-1234-4234-8234-123456789abc";

let catalog: SkillTemplate[];

function createCatalog(): SkillTemplate[] {
  return [
    {
      name: REVIEW_NAME,
      version: 1,
      description: enSkills.builtin_role_skills[REVIEW_NAME].description,
      content: `---\nname: ${REVIEW_NAME}\ndescription: Review a diff\nuser-invocable: true\n---\n${REVIEW_BODY}`,
      files: [{ path: "references/checklist.md", content: "# Review checklist\n" }],
    },
    {
      name: REPORT_NAME,
      version: 1,
      description: enSkills.builtin_role_skills[REPORT_NAME].description,
      content: `---\nname: ${REPORT_NAME}\ndescription: Report test evidence\n---\n${REPORT_BODY}`,
      files: [],
    },
  ];
}

function savedSkill(data: CreateSkillRequest, overrides: Partial<Skill> = {}): Skill {
  return {
    id: COPIED_ID,
    workspace_id: "ws-1",
    name: data.name,
    description: data.description ?? "",
    content: data.content ?? "",
    config: data.config ?? {},
    files: (data.files ?? []).map((file, index) => ({
      ...file,
      id: `file-${index}`,
      skill_id: COPIED_ID,
      created_at: "2026-09-12T00:00:00Z",
      updated_at: "2026-09-12T00:00:00Z",
    })),
    created_by: "user-1",
    created_at: "2026-09-12T00:00:00Z",
    updated_at: "2026-09-12T00:00:00Z",
    ...overrides,
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

function renderDialog(locale: TestLocale = "en") {
  const onClose = vi.fn();
  const onCreated = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  let language = locale;
  const dialog = () => (
    // Match production: only the active locale is mounted initially.
    <I18nProvider locale={language} resources={{ [language]: LOCALES[language] }}>
      <QueryClientProvider client={queryClient}>
        <CreateSkillDialog onClose={onClose} onCreated={onCreated} />
      </QueryClientProvider>
    </I18nProvider>
  );
  const view = render(dialog());
  return {
    onClose,
    onCreated,
    queryClient,
    changeContext(next: { locale?: TestLocale; workspaceId?: string }) {
      language = next.locale ?? language;
      mocks.workspaceId = next.workspaceId ?? mocks.workspaceId;
      view.rerender(dialog());
    },
  };
}

async function openTemplates() {
  fireEvent.click(screen.getByRole("button", { name: /Modify from template/ }));
  await screen.findByRole("button", { name: /^multica-code-review/ });
}

async function chooseReviewTemplate() {
  fireEvent.click(screen.getByRole("button", { name: /^multica-code-review/ }));
  const useTemplate = screen.getByRole("button", { name: "Use this template" });
  await waitFor(() => expect(useTemplate).toBeEnabled());
  fireEvent.click(useTemplate);
  await screen.findByRole("textbox", { name: "Name" });
}

async function openEditor() {
  await openTemplates();
  await chooseReviewTemplate();
}

function editDraft() {
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), {
    target: { value: "billing-review" },
  });
  fireEvent.change(screen.getByRole("textbox", { name: "Description" }), {
    target: { value: EDITED_DESCRIPTION },
  });
  fireEvent.change(screen.getByRole("textbox", { name: "Instructions" }), {
    target: { value: EDITED_BODY },
  });
}

function expectEditedDraft() {
  expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("billing-review");
  expect(screen.getByRole("textbox", { name: "Description" })).toHaveValue(EDITED_DESCRIPTION);
  expect(screen.getByRole("textbox", { name: "Instructions" })).toHaveValue(EDITED_BODY);
}

function expectNoWrites() {
  expect(mocks.createSkill).not.toHaveBeenCalled();
  expect(mocks.updateSkill).not.toHaveBeenCalled();
  expect(mocks.importSkillArchive).not.toHaveBeenCalled();
  expect(mocks.setAgentSkills).not.toHaveBeenCalled();
}

beforeEach(() => {
  vi.resetAllMocks();
  mocks.workspaceId = "ws-1";
  catalog = createCatalog();
  mocks.listSkillTemplates.mockResolvedValue(catalog);
  mocks.listSkills.mockResolvedValue([]);
  mocks.createSkill.mockImplementation(async (data: CreateSkillRequest) => savedSkill(data));
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
});

describe("CreateSkillDialog template creation", () => {
  it("keeps preview and editing local, then creates an independent synchronized copy", async () => {
    const originalCatalog = structuredClone(catalog);
    const { onCreated, onClose, queryClient } = renderDialog();
    expect(mocks.listSkillTemplates).not.toHaveBeenCalled();
    await openTemplates();
    expect(screen.getByText(/检查改动，报告可复现的问题。/)).toBeInTheDocument();
    expectNoWrites();

    await chooseReviewTemplate();
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue(`${REVIEW_NAME}-copy`);
    expect(screen.getByRole("textbox", { name: "Description" })).toHaveValue(
      enSkills.builtin_role_skills[REVIEW_NAME].description,
    );
    editDraft();
    expectNoWrites();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));

    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
    const [request, options] = mocks.createSkill.mock.calls[0]!;
    expect(options).toEqual(expect.objectContaining({ workspaceId: "ws-1" }));
    expect(request).toEqual(expect.objectContaining({
      name: "billing-review",
      description: EDITED_DESCRIPTION,
      files: originalCatalog[0]!.files,
      config: { template_source: { name: REVIEW_NAME, version: 1 } },
    }));
    const parsed = parseFrontmatter(request.content);
    expect(parsed.frontmatter?.name).toBe("billing-review");
    expect(parsed.frontmatter?.description).toBe(EDITED_DESCRIPTION);
    expect(request.content.endsWith(EDITED_BODY)).toBe(true);
    expect(catalog).toEqual(originalCatalog);
    expect(queryClient.getQueryData(workspaceKeys.skillTemplates("ws-1"))).toEqual(originalCatalog);
    expect(mocks.updateSkill).not.toHaveBeenCalled();
    expect(mocks.setAgentSkills).not.toHaveBeenCalled();
    expect(queryClient.getQueryData(skillDetailOptions("ws-1", COPIED_ID).queryKey)).toEqual(
      onCreated.mock.calls[0]![0],
    );
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes a pristine template draft without creating a skill or asking to discard", async () => {
    const { onClose } = renderDialog();
    await openEditor();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "Keep editing" })).not.toBeInTheDocument();
    expectNoWrites();
  });

  it("restores edited fields through both template and creation-method back navigation", async () => {
    renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Back to templates" }));
    fireEvent.click(screen.getByRole("button", { name: "Back to creation methods" }));
    expect(screen.queryByRole("button", { name: "Keep editing" })).not.toBeInTheDocument();
    await openTemplates();
    await chooseReviewTemplate();
    expectEditedDraft();
    expectNoWrites();
  });

  it("does not replace a draft merely by previewing another template", async () => {
    renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Choose another template" }));
    fireEvent.click(screen.getByRole("button", { name: /^multica-test-report/ }));
    expect(screen.getByText(/记录测试范围和验证证据。/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Keep editing" })).not.toBeInTheDocument();
    await chooseReviewTemplate();
    expectEditedDraft();
    expectNoWrites();
  });

  it("requires an explicit discard before using a different template", async () => {
    renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Choose another template" }));
    fireEvent.click(screen.getByRole("button", { name: /^multica-test-report/ }));
    fireEvent.click(screen.getByRole("button", { name: "Use this template" }));
    fireEvent.click(await screen.findByRole("button", { name: "Keep editing" }));
    await chooseReviewTemplate();
    expectEditedDraft();

    fireEvent.click(screen.getByRole("button", { name: "Choose another template" }));
    fireEvent.click(screen.getByRole("button", { name: /^multica-test-report/ }));
    fireEvent.click(screen.getByRole("button", { name: "Use this template" }));
    fireEvent.click(await screen.findByRole("button", { name: /^Discard/ }));
    expect(await screen.findByRole("textbox", { name: "Name" })).toHaveValue(`${REPORT_NAME}-copy`);
    expect(screen.getByRole("textbox", { name: "Instructions" })).toHaveValue(REPORT_BODY);
    expectNoWrites();
  });

  it.each(["close button", "Escape", "backdrop"] as const)(
    "preserves dirty fields when cancelling a discard triggered by %s",
    async (exit) => {
      const { onClose } = renderDialog();
      await openEditor();
      editDraft();
      if (exit === "close button") {
        fireEvent.click(screen.getByRole("button", { name: "Close" }));
      } else if (exit === "Escape") {
        await userEvent.keyboard("{Escape}");
      } else {
        const backdrop = document.querySelector('[data-slot="dialog-overlay"]');
        expect(backdrop).not.toBeNull();
        await userEvent.click(backdrop!);
      }
      fireEvent.click(await screen.findByRole("button", { name: "Keep editing" }));
      expect(onClose).not.toHaveBeenCalled();
      expectEditedDraft();
      fireEvent.click(screen.getByRole("button", { name: "Close" }));
      fireEvent.click(await screen.findByRole("button", { name: /^Discard/ }));
      expect(onClose).toHaveBeenCalledTimes(1);
      expectNoWrites();
    },
  );

  it("guards switching creation methods after returning to the chooser", async () => {
    renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Back to templates" }));
    fireEvent.click(screen.getByRole("button", { name: "Back to creation methods" }));
    fireEvent.click(screen.getByRole("button", { name: /Create manually/ }));
    fireEvent.click(await screen.findByRole("button", { name: "Keep editing" }));
    await openTemplates();
    await chooseReviewTemplate();
    expectEditedDraft();
    expectNoWrites();
  });

  it("retains every field on 409 and waits for a deliberate renamed submission", async () => {
    mocks.createSkill.mockRejectedValueOnce(new ApiError("Skill name already exists", 409, "Conflict"));
    const { onCreated, onClose } = renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/name|exists|different/i);
    expectEditedDraft();
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
    expect(onCreated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), {
      target: { value: "billing-review-v2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(mocks.createSkill).toHaveBeenCalledTimes(2);
    expect(mocks.createSkill.mock.calls[1]![0]).toEqual(expect.objectContaining({
      name: "billing-review-v2",
      description: EDITED_DESCRIPTION,
      content: expect.stringContaining(EDITED_BODY),
    }));
  });

  it("refuses canonical names from the loaded catalog even when the workspace is empty", async () => {
    renderDialog();
    await openEditor();
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), {
      target: { value: REPORT_NAME },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    expectNoWrites();
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue(REPORT_NAME);
  });

  it.each([400, 404])("offers retry and manual creation after a catalog %s", async (status) => {
    mocks.listSkillTemplates.mockRejectedValue(new ApiError("invalid skill ID", status, "Bad Request"));
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: /Modify from template/ }));
    expect(await screen.findByRole("alert")).not.toBeEmptyDOMElement();
    expect(screen.getByRole("button", { name: "Try again" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Back to creation methods" }));
    fireEvent.click(screen.getByRole("button", { name: /Create manually/ }));
    expect(await screen.findByRole("textbox", { name: "Name" })).toHaveValue("");
    expectNoWrites();
  });

  it("bounds an in-flight create at 30 seconds and ignores late success for navigation", async () => {
    const pending = deferred<Skill>();
    mocks.createSkill.mockReturnValueOnce(pending.promise);
    const { onCreated, onClose } = renderDialog();
    await openEditor();
    editDraft();
    vi.useFakeTimers();
    const createButton = screen.getByRole("button", { name: "Create skill" });
    fireEvent.click(createButton);
    fireEvent.click(createButton);
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Back to templates" })).toBeDisabled();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
    await act(async () => { await vi.advanceTimersByTimeAsync(30_001); });

    expect(screen.getByRole("button", { name: "Check creation result" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Close" })).toBeEnabled();
    expectEditedDraft();
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
    const request: CreateSkillRequest = mocks.createSkill.mock.calls[0]![0];
    await act(async () => { pending.resolve(savedSkill(request)); });
    expect(onCreated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
  });

  it("still warns before closing a pristine draft after resuming from a timed-out create", async () => {
    const pending = deferred<Skill>();
    mocks.createSkill.mockReturnValueOnce(pending.promise);
    const { onClose } = renderDialog();
    await openEditor();
    vi.useFakeTimers();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    await act(async () => { await vi.advanceTimersByTimeAsync(30_001); });
    vi.useRealTimers();

    fireEvent.click(screen.getByRole("button", { name: "Continue editing" }));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).not.toHaveBeenCalled();
    expect(await screen.findByText(/Closing does not cancel a request already sent to the server/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Keep editing" }));
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    fireEvent.click(await screen.findByRole("button", { name: /^Discard/ }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
  });

  it("guards newer edits before opening the original result after resuming a timed-out create", async () => {
    const pending = deferred<Skill>();
    mocks.createSkill.mockReturnValueOnce(pending.promise);
    const { onCreated, onClose } = renderDialog();
    await openEditor();
    vi.useFakeTimers();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    await act(async () => { await vi.advanceTimersByTimeAsync(30_001); });
    vi.useRealTimers();

    const submitted: CreateSkillRequest = mocks.createSkill.mock.calls[0]![0];
    const created = savedSkill(submitted);
    mocks.listSkills.mockResolvedValue([created]);
    mocks.getSkill.mockResolvedValue(created);
    fireEvent.click(screen.getByRole("button", { name: "Continue editing" }));
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Check creation result" }));
    const open = await screen.findByRole("button", { name: "Open skill" });
    expect(onCreated).not.toHaveBeenCalled();
    fireEvent.click(open);
    expect(onCreated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("button", { name: "Keep editing" }));
    expectEditedDraft();
    fireEvent.click(screen.getByRole("button", { name: "Open skill" }));
    fireEvent.click(await screen.findByRole("button", { name: /^Discard/ }));
    expect(onCreated).toHaveBeenCalledWith(created);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
  });

  it.each(["matching", "different"] as const)("does not adopt a recovered skill automatically when its content is %s", async (content) => {
    mocks.createSkill.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    const { onCreated, onClose } = renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    const check = await screen.findByRole("button", { name: "Check creation result" });
    const request: CreateSkillRequest = mocks.createSkill.mock.calls[0]![0];
    const candidate = content === "matching" ? savedSkill(request) : savedSkill({
      name: "billing-review",
      description: "A different teammate's instructions",
      content: "# Existing review\nDifferent content.\n",
    }, { id: "87654321-4321-4321-8321-cba987654321", created_by: "user-2" });
    mocks.listSkills.mockResolvedValue([candidate]);
    mocks.getSkill.mockResolvedValue(candidate);
    fireEvent.click(check);
    const open = await screen.findByRole("button", { name: "Open skill" });
    expect(onCreated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
    expect(mocks.listSkills).toHaveBeenLastCalledWith(expect.objectContaining({ workspaceId: "ws-1" }));
    expect(mocks.getSkill).toHaveBeenCalledWith(
      candidate.id,
      expect.objectContaining({ workspaceId: "ws-1" }),
    );
    expectEditedDraft();
    fireEvent.click(open);
    if (content === "different") {
      expect(onCreated).not.toHaveBeenCalled();
      fireEvent.click(await screen.findByRole("button", { name: /^Discard/ }));
    }
    expect(onCreated).toHaveBeenCalledWith(candidate);
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("keeps malformed successful creation responses in recovery without retrying", async () => {
    mocks.createSkill.mockRejectedValueOnce(new SkillCreationUnconfirmedError());
    const { onCreated, onClose } = renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    expect(await screen.findByRole("button", { name: "Check creation result" })).toBeEnabled();
    expectEditedDraft();
    expect(onCreated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
  });

  it.each([false, true])("preserves a draft through locale and catalog changes (edited: %s)", async (edited) => {
    const { changeContext, queryClient } = renderDialog();
    await openEditor();
    if (edited) editDraft();
    const name = edited ? "billing-review" : `${REVIEW_NAME}-copy`;
    const description = edited ? EDITED_DESCRIPTION : enSkills.builtin_role_skills[REVIEW_NAME].description;
    const body = edited ? EDITED_BODY : REVIEW_BODY;
    changeContext({ locale: "zh-Hans" });
    expect(await screen.findByDisplayValue(name)).toBeInTheDocument();
    expect(screen.getByDisplayValue(description)).toBeInTheDocument();
    expect(screen.getByDisplayValue(body, { normalizer: (value) => value })).toBeInTheDocument();
    act(() => {
      queryClient.setQueryData(workspaceKeys.skillTemplates("ws-1"), catalog.map((template) => ({
        ...template,
        version: 2,
        content: template.content.replace(REVIEW_BODY, "# Updated template\n"),
      })));
    });
    changeContext({ locale: "en" });
    expect(await screen.findByRole("textbox", { name: "Name" })).toHaveValue(name);
    expect(screen.getByRole("textbox", { name: "Description" })).toHaveValue(description);
    expect(screen.getByRole("textbox", { name: "Instructions" })).toHaveValue(body);
    expectNoWrites();
  });

  it("does not submit an existing draft into a newly selected workspace", async () => {
    const { changeContext } = renderDialog();
    await openEditor();
    editDraft();
    changeContext({ workspaceId: "ws-2" });
    expect(await screen.findByRole("button", { name: "Use this template" })).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Name" })).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue("billing-review")).not.toBeInTheDocument();
    expectNoWrites();
  });

  it("caches an old-workspace success only there and suppresses navigation in the new workspace", async () => {
    const pending = deferred<Skill>();
    mocks.createSkill.mockReturnValueOnce(pending.promise);
    const { changeContext, onCreated, onClose, queryClient } = renderDialog();
    await openEditor();
    editDraft();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    expect(mocks.createSkill).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ workspaceId: "ws-1" }),
    );
    changeContext({ workspaceId: "ws-2" });
    const request: CreateSkillRequest = mocks.createSkill.mock.calls[0]![0];
    const created = savedSkill(request);
    await act(async () => { pending.resolve(created); });
    await waitFor(() => expect(
      queryClient.getQueryData(skillDetailOptions("ws-1", COPIED_ID).queryKey),
    ).toEqual(created));
    expect(queryClient.getQueryData(skillDetailOptions("ws-2", COPIED_ID).queryKey)).toBeUndefined();
    expect(onCreated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
  });
});
