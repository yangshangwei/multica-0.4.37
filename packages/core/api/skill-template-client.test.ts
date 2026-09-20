// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import type { CreateSkillRequest } from "../types";
import { ApiClient, ApiError, SkillCreationUnconfirmedError } from "./index";

const BASE_URL = "https://api.example.test";
const SKILL = {
  id: "skill-copy-1",
  workspace_id: "ws-original",
  name: "multica-code-review-copy",
};
const CREATE: CreateSkillRequest = {
  name: SKILL.name,
  description: "Review a change",
  content: "# Review\n",
};

function stubJson(body: unknown, status = 200) {
  const fetchMock = vi.fn<typeof fetch>().mockImplementation(async () =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => {
  setCurrentWorkspace(null, null);
  vi.unstubAllGlobals();
});

describe("ApiClient skill templates", () => {
  it("loads the catalog in the captured workspace with its cancellation signal", async () => {
    setCurrentWorkspace("another-tab", "ws-another");
    const signal = new AbortController().signal;
    const template = {
      name: "multica-code-review",
      version: 2,
      description: "Review changes",
      content: "---\nname: multica-code-review\n---\n# 审查\n",
      files: [{ path: "references/checks.md", content: "Checks" }],
    };
    const fetchMock = stubJson({ templates: [template] });

    await expect(
      new ApiClient(BASE_URL).listSkillTemplates("ws-original", signal),
    ).resolves.toEqual([template]);
    expect(fetchMock).toHaveBeenCalledWith(
      `${BASE_URL}/api/skills/templates`,
      expect.objectContaining({
        signal,
        headers: expect.objectContaining({
          "X-Workspace-ID": "ws-original",
          "X-Workspace-Slug": "",
        }),
      }),
    );
  });

  it("degrades malformed catalog entries to an empty catalog", async () => {
    stubJson({ templates: [{ name: "multica-code-review" }] });
    await expect(
      new ApiClient(BASE_URL).listSkillTemplates("ws-original"),
    ).resolves.toEqual([]);
  });

  it("keeps a template whose presentation metadata is malformed, dropping only those fields", async () => {
    const base = {
      name: "multica-code-review",
      version: 2,
      description: "Review changes",
      content: "---\nname: multica-code-review\n---\n# 审查\n",
      files: [],
    };
    stubJson({
      templates: [
        { ...base, category: "engineering", icon: "git-pull-request" },
        { ...base, name: "typed-wrong", category: 3, icon: ["bug"] },
        { ...base, name: "absent" },
      ],
    });

    const templates = await new ApiClient(BASE_URL).listSkillTemplates("ws-original");
    expect(templates.map((t) => t.name)).toEqual(["multica-code-review", "typed-wrong", "absent"]);
    expect(templates[0]).toMatchObject({ category: "engineering", icon: "git-pull-request" });
    expect(templates[1]?.category).toBeUndefined();
    expect(templates[1]?.icon).toBeUndefined();
    expect(templates[2]?.category).toBeUndefined();
  });

  it.each([400, 404])("keeps old-backend HTTP %s errors available to the caller", async (status) => {
    stubJson({ error: "invalid skill id" }, status);
    await expect(
      new ApiClient(BASE_URL).listSkillTemplates("ws-original"),
    ).rejects.toMatchObject({ name: "ApiError", status });
  });
});

describe("ApiClient skill creation confirmation", () => {
  it("parses a successful response and sends the explicit workspace and signal", async () => {
    setCurrentWorkspace("another-tab", "ws-another");
    const signal = new AbortController().signal;
    const fetchMock = stubJson({ ...SKILL, future_field: true }, 201);

    const result = await new ApiClient(BASE_URL).createSkill(CREATE, {
      workspaceId: "ws-original",
      signal,
    });

    expect(result).toEqual({
      ...SKILL,
      description: "",
      content: "",
      config: {},
      created_by: null,
      created_at: "",
      updated_at: "",
      files: [],
      future_field: true,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      `${BASE_URL}/api/skills`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(CREATE),
        signal,
        headers: expect.objectContaining({
          "X-Workspace-ID": "ws-original",
          "X-Workspace-Slug": "",
          "Content-Type": "application/json",
        }),
      }),
    );
  });

  it.each([
    null,
    {},
    { ...SKILL, id: 42 },
    { ...SKILL, id: "" },
    { ...SKILL, id: "   " },
    { ...SKILL, workspace_id: "" },
    { ...SKILL, workspace_id: "   " },
    { ...SKILL, workspace_id: "ws-another" },
    { ...SKILL, files: "not-an-array" },
    { ...SKILL, content: { invalid: true } },
  ])("rejects unconfirmed successful creation %#", async (response) => {
    stubJson(response, 201);
    await expect(
      new ApiClient(BASE_URL).createSkill(CREATE, { workspaceId: "ws-original" }),
    ).rejects.toBeInstanceOf(SkillCreationUnconfirmedError);
  });

  it.each([201, 204])("rejects HTTP %s without a readable successful identity", async (status) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(status === 204 ? null : "not-json", { status }),
    ));
    await expect(
      new ApiClient(BASE_URL).createSkill(CREATE, { workspaceId: "ws-original" }),
    ).rejects.toBeInstanceOf(SkillCreationUnconfirmedError);
  });

  it("preserves the old create signature and checks the workspace captured before awaiting", async () => {
    setCurrentWorkspace("original-tab", "ws-original");
    let finish!: (response: Response) => void;
    const response = new Promise<Response>((resolve) => { finish = resolve; });
    const fetchMock = vi.fn<typeof fetch>().mockReturnValue(response);
    vi.stubGlobal("fetch", fetchMock);
    const created = new ApiClient(BASE_URL).createSkill(CREATE);

    setCurrentWorkspace("another-tab", "ws-another");
    finish(new Response(JSON.stringify(SKILL), { status: 201 }));

    await expect(created).resolves.toMatchObject(SKILL);
    expect(fetchMock.mock.calls[0]?.[1]?.headers).toMatchObject({
      "X-Workspace-Slug": "original-tab",
    });
  });

  it("validates an ambient workspace even when the caller does not pass options", async () => {
    setCurrentWorkspace("another-tab", "ws-another");
    stubJson(SKILL, 201);
    await expect(
      new ApiClient(BASE_URL).createSkill(CREATE),
    ).rejects.toBeInstanceOf(SkillCreationUnconfirmedError);
  });

  it.each([400, 409, 500])("preserves HTTP %s failure semantics", async (status) => {
    stubJson({ error: "Creation failed" }, status);
    const creation = new ApiClient(BASE_URL).createSkill(CREATE, {
      workspaceId: "ws-original",
    });
    await expect(creation).rejects.toBeInstanceOf(ApiError);
    await expect(creation).rejects.toMatchObject({ status });
  });

  it.each([
    new TypeError("Failed to fetch"),
    new DOMException("Aborted", "AbortError"),
  ])("preserves transport failures for uncertain-result recovery", async (error) => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(error));
    await expect(
      new ApiClient(BASE_URL).createSkill(CREATE, {
        workspaceId: "ws-original",
        signal: new AbortController().signal,
      }),
    ).rejects.toBe(error);
  });
});

describe("ApiClient skill recovery reads", () => {
  it("sends the explicit workspace and signal on both list and detail reads", async () => {
    setCurrentWorkspace("another-tab", "ws-another");
    const signal = new AbortController().signal;
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(new Response(JSON.stringify([SKILL])))
      .mockResolvedValueOnce(new Response(JSON.stringify(SKILL)));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient(BASE_URL);

    const list = await client.listSkills({ workspaceId: "ws-original", signal });
    const detail = await client.getSkill(SKILL.id, { workspaceId: "ws-original", signal });

    expect(list[0]).toMatchObject({ ...SKILL, config: {}, created_by: null });
    expect(list[0]).not.toHaveProperty("content");
    expect(detail).toMatchObject({ ...SKILL, content: "", files: [] });
    for (const [, init] of fetchMock.mock.calls) {
      expect(init).toMatchObject({
        signal,
        headers: {
          "X-Workspace-ID": "ws-original",
          "X-Workspace-Slug": "",
        },
      });
    }
  });

  it("preserves list and detail calls without options", async () => {
    setCurrentWorkspace("original-tab", "ws-original");
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(new Response(JSON.stringify([SKILL])))
      .mockResolvedValueOnce(new Response(JSON.stringify(SKILL)));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient(BASE_URL);

    await expect(client.listSkills()).resolves.toEqual([expect.objectContaining(SKILL)]);
    await expect(client.getSkill(SKILL.id)).resolves.toMatchObject(SKILL);
    for (const [, init] of fetchMock.mock.calls) {
      expect(init?.headers).toMatchObject({ "X-Workspace-Slug": "original-tab" });
      expect(init?.signal).toBeUndefined();
    }
  });

  it("does not expose malformed read responses as skill identities", async () => {
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(new Response(JSON.stringify([{ id: 4 }])))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ...SKILL, id: 4 })));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient(BASE_URL);

    await expect(client.listSkills()).resolves.toEqual([]);
    await expect(client.getSkill(SKILL.id)).resolves.toMatchObject({ id: "", workspace_id: "" });
  });
});
