// @vitest-environment node
import { expect, it } from "vitest";
import { adminLoginUrl, isAdminDestination } from "./admin-path";

it.each(["/admin", "/admin?time_from=today&status=queued", "/admin/tasks?status=queued#selected"])("recognizes the complete admin destination %s", (path) => {
  expect(isAdminDestination(path)).toBe(true);
  expect(new URL(adminLoginUrl(path), "https://multica.test").searchParams.get("next")).toBe(path);
});

it.each([null, "/administrator", "/acme/issues", "https://other.test/admin", "//other.test/admin"])("does not treat %s as an administration path", (path) => {
  expect(isAdminDestination(path)).toBe(false);
});
