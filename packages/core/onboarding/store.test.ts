// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  scope: "original-session",
  user: { id: "original-user" },
  patch: vi.fn(),
  setUser: vi.fn(),
  analytics: vi.fn(),
}));
vi.mock("../api", () => ({ api: { getSessionScope: () => state.scope, patchOnboarding: state.patch } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: state.user, setUser: state.setUser }) } }));
vi.mock("../analytics", () => ({ setPersonProperties: state.analytics }));
import { saveQuestionnaire } from "./store";

beforeEach(() => {
  vi.clearAllMocks();
  state.scope = "original-session";
  state.user = { id: "original-user" };
});

it.each(["credential", "user"])("rejects a late questionnaire response after %s replacement", async (replacement) => {
  let finish!: (user: { id: string }) => void;
  state.patch.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
  const saving = saveQuestionnaire({ role: "engineer" });
  if (replacement === "credential") state.scope = "replacement-session";
  else state.user = { id: "replacement-user" };
  finish({ id: "original-user" });
  await expect(saving).rejects.toMatchObject({ name: "AbortError" });
  expect(state.setUser).not.toHaveBeenCalled();
  expect(state.analytics).not.toHaveBeenCalled();
});

it("updates the current user's questionnaire and cohort properties", async () => {
  const user = { id: "original-user", onboarding_questionnaire: { role: "engineer" } };
  state.patch.mockResolvedValueOnce(user);
  await saveQuestionnaire({ role: "engineer" });
  expect(state.setUser).toHaveBeenCalledWith(user);
  expect(state.analytics).toHaveBeenCalledWith({ role: "engineer" });
});
