// @vitest-environment node
import { expect, it } from "vitest";
import { collaborationRecipientSections } from "./recipient-sections";

const account = (n: number, external: boolean | null = false) => ({ user_id: `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`, name: `Account ${n}`, avatar_url: "", external });
const [self, picked, recent, other, last] = [account(1), account(2, true), account(3, null), account(4), account(5)];
const ids = (sections: ReturnType<typeof collaborationRecipientSections>) => sections.map(({ key, accounts }) => [key, accounts.map((item) => item.name)]);

it("orders self, selected, recent and all, listing each account only in its first section", () => {
  expect(ids(collaborationRecipientSections({ self, selected: [self, picked], recent: [picked, recent, self], pages: [{ recipients: [recent, other, picked] }, { recipients: [other, last, self] }] }))).toEqual([
    ["self", ["Account 1"]], ["selected", ["Account 2"]], ["recent", ["Account 3"]], ["all", ["Account 4", "Account 5"]],
  ]);
});

it("keeps every section in order when some are empty", () => {
  expect(ids(collaborationRecipientSections({ selected: [], recent: [], pages: [] }))).toEqual([["self", []], ["selected", []], ["recent", []], ["all", []]]);
  expect(ids(collaborationRecipientSections({ self: null, selected: [other], recent: [], pages: [{ recipients: [other] }] }))).toEqual([["self", []], ["selected", ["Account 4"]], ["recent", []], ["all", []]]);
});

it("treats UUID spellings of one account as the same person and keeps the first snapshot", () => {
  const renamed = { ...picked, user_id: picked.user_id.toUpperCase(), name: "Renamed" };
  const sections = collaborationRecipientSections({ selected: [picked], recent: [renamed], pages: [{ recipients: [renamed, renamed] }] });
  expect(sections.flatMap((section) => section.accounts)).toEqual([picked]);
  expect(collaborationRecipientSections({ selected: [], recent: [], pages: [{ recipients: [other] }, { recipients: [other, last] }] })[3]!.accounts).toEqual([other, last]);
});
