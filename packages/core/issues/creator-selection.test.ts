// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveQuickCreateCreator } from "./creator-selection";
const a = (id: string) => ({ type: "agent" as const, id });
const s = (id: string) => ({ type: "squad" as const, id });
const base = { callers: [], draftActor: null, defaultActor: null, lastActor: null,
  agents: [{id:"a1"},{id:"a2"}], squads:[{id:"s1"}], agentsKnown:true, squadsKnown:true };
describe("creator seed precedence", () => {
  it("uses caller, draft, default, accepted history, then first eligible agent", () => {
    expect(resolveQuickCreateCreator({...base,callers:[s("s1")],draftActor:a("a1"),defaultActor:a("a2")})).toEqual(s("s1"));
    expect(resolveQuickCreateCreator({...base,draftActor:a("a1"),defaultActor:a("a2"),lastActor:s("s1")})).toEqual(a("a1"));
    expect(resolveQuickCreateCreator({...base,defaultActor:a("a2"),lastActor:s("s1")})).toEqual(a("a2"));
    expect(resolveQuickCreateCreator({...base,lastActor:s("s1")})).toEqual(s("s1"));
    expect(resolveQuickCreateCreator(base)).toEqual(a("a1"));
  });
  it.each(["callers","draftActor","defaultActor","lastActor"] as const)("waits for unknown squad data at %s", (field) => {
    const input={...base,squads:[],squadsKnown:false,[field]:field==="callers"?[s("s1")]:s("s1")};
    expect(resolveQuickCreateCreator(input)).toBeNull();
  });
  it("skips a known inaccessible ref but never treats an ID from the other actor type as valid", () => {
    expect(resolveQuickCreateCreator({...base,defaultActor:s("a2"),lastActor:a("a2")})).toEqual(a("a2"));
  });
  it("waits for unknown agent permissions before defaulting", () => {
    expect(resolveQuickCreateCreator({...base,agentsKnown:false})).toBeNull();
    expect(resolveQuickCreateCreator({...base,agents:[]})).toBeNull();
  });
});


it("ignores malformed persisted actor kinds instead of treating them as squads", () => {
  const invalid = {type:"member",id:"s1"} as unknown as {type:"agent"|"squad";id:string};
  expect(resolveQuickCreateCreator({...base, defaultActor:invalid})).toEqual(a("a1"));
});
