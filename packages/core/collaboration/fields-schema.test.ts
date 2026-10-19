// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationFieldOptions, parseCollaborationIssue } from "./schema";
import { collaborationFieldOptionsFixture as options, collaborationIssueFixture as issue } from "./test-fixtures";

const definition = options.properties[0]!;
const propertyId = definition.id;
const actor = "member:00000000-0000-4000-8000-000000000004";

afterEach(() => setSchemaLogger(noopLogger));

it("preserves minimal source references and rejects a malformed source catalog", () => {
  const source = { issue_revision: "9007199254740993", project: null, parent_issue: null, iteration: null, stage: null };
  expect(parseCollaborationIssue({ ...issue, source_structure: source })?.source_structure).toEqual(source);
  expect(parseCollaborationIssue({ ...issue, source_structure: { ...source, issue_revision: 9 } })).toBeNull();
  expect(parseCollaborationFieldOptions({ ...options, source_options: { projects: [], parents: [], iterations: [], parents_truncated: false, iterations_enabled: true } })?.source_options).toEqual({ projects: [], parents: [], iterations: [], parents_truncated: false, iterations_enabled: true });
  expect(parseCollaborationFieldOptions({ ...options, source_options: { projects: [], parents: [], iterations: [], iterations_enabled: true } })).toBeNull();
});

it("preserves missing older-server fields as unknown and explicit empty fields as empty", () => {
  const older = parseCollaborationIssue(issue);
  expect(older).not.toHaveProperty("labels");
  expect(older).not.toHaveProperty("properties");
  expect(older).not.toHaveProperty("property_definitions");
  expect(parseCollaborationIssue({ ...issue, labels: [], properties: {}, property_definitions: [] })).toMatchObject({
    labels: [], properties: {}, property_definitions: [],
  });
  expect(parseCollaborationFieldOptions({ ...options, labels: [], properties: [] })).toEqual({ ...options, labels: [], properties: [] });
});

it("retains exact option identity and strips unrelated source metadata", () => {
  expect(parseCollaborationFieldOptions({
    ...options,
    members: ["PRIVATE_MEMBER_DIRECTORY"],
    labels: options.labels.map((label) => ({ ...label, workspace_id: "PRIVATE_WORKSPACE" })),
    properties: [{ ...definition, description: "PRIVATE_DEFINITION", config: { ...definition.config, members: ["PRIVATE_MEMBER"] } }],
  })).toEqual(options);
});

it.each([
  { type: "text", value: "Current text" },
  { type: "number", value: 0 },
  { type: "checkbox", value: false },
  { type: "date", value: "2028-02-29" },
  { type: "url", value: "https://example.test/path" },
  { type: "select", value: "option-one" },
  { type: "multi_select", value: ["option-one"] },
  { type: "actor", value: actor },
  { type: "multi_actor", value: [actor] },
])("preserves the stored $type value, including an archived definition or departed member", ({ type, value }) => {
  const property = { ...definition, type, archived: true, config: type.includes("select") ? definition.config : {} };
  const detail = { ...issue, labels: options.labels, properties: { [propertyId]: value }, property_definitions: [property] };
  expect(parseCollaborationIssue(detail)).toEqual({
    ...parseCollaborationIssue(issue), labels: options.labels, properties: { [propertyId]: value }, property_definitions: [property],
  });
});

it.each([
  { issue_id: "PRIVATE_ISSUE" },
  { source_workspace_id: "PRIVATE_SOURCE" },
  { collaboration_revision: 1 },
  { collaboration_revision: "9223372036854775808" },
  { labels: undefined },
  { labels: null },
  { labels: [{ ...options.labels[0], id: "PRIVATE_LABEL" }] },
  { labels: [options.labels[0], options.labels[0]] },
  { properties: undefined },
  { properties: null },
  { properties: [{ ...definition, id: "PRIVATE_PROPERTY" }] },
  { properties: [definition, definition] },
  { properties: [{ ...definition, type: "PRIVATE_TYPE" }] },
  { properties: [{ ...definition, archived: true }] },
  { properties: [{ ...definition, archived: undefined }] },
  { properties: [{ ...definition, archived: "false" }] },
  { properties: [{ ...definition, config: undefined }] },
  { properties: [{ ...definition, config: { options: null } }] },
  { properties: [{ ...definition, config: {} }] },
  { properties: [{ ...definition, config: { options: [] } }] },
  { properties: [{ ...definition, config: { options: [definition.config.options[0], definition.config.options[0]] } }] },
  { properties: [{ ...definition, config: { options: [{ id: "", name: "PRIVATE_OPTION", color: "#112233" }] } }] },
])("makes malformed editing options unavailable without logging field values", (patch) => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationFieldOptions({ ...options, ...patch })).toBeNull();
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
});

it.each([
  { type: "text", value: {} },
  { type: "number", value: "1" },
  { type: "number", value: Number.POSITIVE_INFINITY },
  { type: "checkbox", value: "true" },
  { type: "date", value: "2026-02-29" },
  { type: "url", value: ["https://example.test"] },
  { type: "select", value: ["option-one"] },
  { type: "select", value: "PRIVATE_UNKNOWN_OPTION" },
  { type: "multi_select", value: "option-one" },
  { type: "multi_select", value: ["PRIVATE_UNKNOWN_OPTION"] },
  { type: "actor", value: "member:PRIVATE_INVALID_UUID" },
  { type: "actor", value: "agent:00000000-0000-4000-8000-000000000004" },
  { type: "multi_actor", value: [actor, false] },
  { type: "multi_actor", value: [actor, "member:PRIVATE_INVALID_UUID"] },
])("does not discard or coerce a malformed current $type value into editable empty state", ({ type, value }) => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationIssue({
    ...issue, properties: { [propertyId]: value },
    property_definitions: [{ ...definition, type, config: type.includes("select") ? definition.config : {} }],
  })).toBeNull();
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
});

it.each([
  { labels: null },
  { labels: [{ ...options.labels[0], id: "PRIVATE_LABEL" }] },
  { properties: null, property_definitions: [] },
  { properties: { [propertyId]: null }, property_definitions: [definition] },
  { properties: { PRIVATE_PROPERTY_ID: "option-one" }, property_definitions: [definition] },
  { properties: { [propertyId]: "option-one" } },
  { property_definitions: [definition] },
  { properties: { [propertyId]: "option-one" }, property_definitions: [] },
  { properties: {}, property_definitions: [definition] },
  { properties: { [propertyId]: "option-one" }, property_definitions: [definition, definition] },
  { properties: { [propertyId]: "option-one" }, property_definitions: [{ ...definition, type: "PRIVATE_UNKNOWN_TYPE" }] },
])("rejects incomplete or mismatched current property definitions and identities", (patch) => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationIssue({ ...issue, ...patch })).toBeNull();
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
});
