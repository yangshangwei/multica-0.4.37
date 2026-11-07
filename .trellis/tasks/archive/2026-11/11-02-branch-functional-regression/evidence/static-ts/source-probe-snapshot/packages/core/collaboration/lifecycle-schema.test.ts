// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationOperation, parseCollaborationStatusOptions } from "./schema";

const options = {
  action: "change_status",
  status_options_version: "a".repeat(64),
  default_status: "todo",
  options: [{ key: "todo", name: "To do", category: "todo" }],
};
const receipt = { code: "status_options_changed", collaboration_revision: "9007199254740993" };

afterEach(() => setSchemaLogger(noopLogger));

it.each([
  { action: "change_status", category: "blocked" },
  { action: "change_status", category: "in_progress" },
  { action: "change_status", category: "in_review" },
  { action: "change_status", category: "backlog" },
  { action: "reopen", category: "todo" },
  { action: "complete", category: "done" },
  { action: "cancel", category: "cancelled" },
  { action: "request_information", category: "blocked" },
  { action: "return_information", category: "blocked" },
  { action: "return_information", category: "backlog" },
  { action: "submit_acceptance", category: "in_review" },
])("parses $action options with category $category", ({ action, category }) => {
  const value = { ...options, action, options: [{ key: "source_custom", name: "Source status", category }], default_status: "source_custom" };
  expect(parseCollaborationStatusOptions({ ...value, private_extra: "PRIVATE_OPTION_EXTRA" })).toEqual(value);
});

it.each(["action", "status_options_version", "default_status", "options"])("requires status options field %s", (field) => {
  const value: Record<string, unknown> = { ...options };
  delete value[field];
  expect(parseCollaborationStatusOptions(value)).toBeNull();
});

it.each(["key", "name", "category"])("requires each option's %s", (field) => {
  const option: Record<string, unknown> = { ...options.options[0] };
  delete option[field];
  expect(parseCollaborationStatusOptions({ ...options, options: [option] })).toBeNull();
});

it.each(["", "a".repeat(63), "a".repeat(65), "A".repeat(64), "g".repeat(64), `${"a".repeat(64)}\n`, null, 1])("rejects a noncanonical status options digest", (digest) => {
  expect(parseCollaborationStatusOptions({ ...options, status_options_version: digest })).toBeNull();
});

it.each([
  { action: "future_action" },
  { options: [] },
  { options: null },
  { options: "PRIVATE_OPTIONS" },
  { default_status: "" },
  { default_status: "unlisted" },
  { options: [{ key: "", name: "Invalid", category: "todo" }] },
  { options: [{ key: "todo", name: "Unknown", category: "future_category" }] },
  { options: [{ key: "todo", name: "Wrong", category: "done" }] },
  { options: [...options.options, ...options.options] },
  { action: "complete" },
  { action: "cancel" },
  { action: "reopen", options: [{ key: "todo", name: "Wrong", category: "cancelled" }] },
  { action: "request_information" },
  { action: "submit_acceptance" },
  { action: "return_information", options: [{ key: "todo", name: "Wrong", category: "done" }] },
  { action: "return_information", options: [...options.options, { key: "blocked", name: "Unexpected choice", category: "blocked" }] },
])("does not make malformed or inapplicable options actionable", (patch) => {
  expect(parseCollaborationStatusOptions({ ...options, ...patch })).toBeNull();
});

it("retains supplied current options on operation recovery without inventing absent options", () => {
  expect(parseCollaborationOperation({ ...receipt, status_options: options })).toEqual({ ...receipt, status_options: options });
  expect(parseCollaborationOperation(receipt)).toEqual(receipt);
  expect(parseCollaborationOperation({ ...receipt, status_options: null })).toEqual({ ...receipt, status_options: null });
});

it("rejects malformed supplied recovery options and redacts option values", () => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  const malformed = { ...options, default_status: "PRIVATE_DEFAULT", options: [{ key: "PRIVATE_KEY", name: "PRIVATE_NAME", category: "PRIVATE_CATEGORY" }] };
  expect(parseCollaborationStatusOptions(malformed)).toBeNull();
  expect(parseCollaborationOperation({ ...receipt, status_options: malformed })).toBeNull();
  expect(warn).toHaveBeenCalledTimes(2);
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
  expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
});
