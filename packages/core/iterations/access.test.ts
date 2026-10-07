// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { defaultStorage } from "../platform/storage";
import { ApiError } from "../api/client";
import { protectIterationRead } from "./access";
const identity = vi.hoisted(() => ({ session: "actor-a" }));
vi.mock("../api", async (original) => ({ ...await original<typeof import("../api")>(), api: { getSessionScope: () => identity.session, getBaseUrl: () => "server" } }));
describe("iteration permission epochs", () => {
  it.each([403, 404])(
    "rejects late success after workspace access is revoked with %s",
    async (status) => {
      const client = new QueryClient();
      let finish!: (value: { title: string }) => void;
      const pending = protectIterationRead(
        client,
        "w",
        () =>
          new Promise<{ title: string }>((resolve) => {
            finish = resolve;
          }),
      );
      await expect(
        protectIterationRead(client, "w", async () => {
          throw new ApiError("Revoked", status, "Forbidden", {
            code: "workspace_access_denied",
          });
        }),
      ).rejects.toThrow("Revoked");
      finish({ title: "Protected title" });
      await expect(pending).rejects.toThrow("Iteration access revoked");
    },
  );
  it("requires a fresh successful capability check before recovering access", async () => {
    const client = new QueryClient();
    await expect(
      protectIterationRead(client, "w", async () => {
        throw new ApiError("Revoked", 403, "Forbidden");
      }),
    ).rejects.toThrow();
    await expect(
      protectIterationRead(client, "w", async () => "cached"),
    ).rejects.toThrow();
    await expect(
      protectIterationRead(client, "w", async () => "authorized", true),
    ).resolves.toBe("authorized");
    await expect(
      protectIterationRead(client, "w", async () => "fresh"),
    ).resolves.toBe("fresh");
  });
  it("does not erase workspace access for an ordinary missing resource", async () => {
    const client = new QueryClient();
    client.setQueryData(["iterations", "w", "detail", "other"], {
      title: "Allowed",
    });
    await expect(
      protectIterationRead(client, "w", async () => {
        throw new ApiError("Missing", 404, "Not Found", {
          code: "iteration_not_found",
        });
      }),
    ).rejects.toThrow();
    expect(client.getQueryData(["iterations", "w", "detail", "other"])).toEqual(
      { title: "Allowed" },
    );
    await expect(
      protectIterationRead(client, "w", async () => "allowed"),
    ).resolves.toBe("allowed");
  });
});

afterEach(() => vi.restoreAllMocks());
it.each(["success", "denied"])("isolates a delayed %s from a replaced session", async (outcome) => {
  identity.session = "actor-a";
  const remove = vi.spyOn(defaultStorage, "removeItem");
  vi.spyOn(defaultStorage, "keys").mockReturnValue(["multica_iteration_command:server:actor-b:w:create:request"]);
  const client = new QueryClient();
  let finish!: (value: string) => void;
  let fail!: (error: Error) => void;
  const pending = protectIterationRead(client, "w", () => new Promise<string>((resolve, reject) => { finish = resolve; fail = reject; }), true);
  identity.session = "actor-b";
  client.clear();
  client.setQueryData(["iterations", "w", "detail", "b"], { title: "B data" });
  if (outcome === "success") finish("A data");
  else fail(new ApiError("Revoked A", 403, "Forbidden"));
  await expect(pending).rejects.toThrow();
  expect(client.getQueryData(["iterations", "w", "detail", "b"])).toEqual({ title: "B data" });
  expect(client.getQueryData(["iterations", "w", "access"])).toBeUndefined();
  expect(remove).not.toHaveBeenCalled();
});
