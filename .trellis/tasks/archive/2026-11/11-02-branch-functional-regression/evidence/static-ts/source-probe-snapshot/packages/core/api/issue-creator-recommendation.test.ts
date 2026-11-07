// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import { setCurrentWorkspace } from "../platform/workspace-storage";
const id = "019ec09d-6222-722b-bdfa-427b105d80be";
const candidate = { actor_type:"agent", actor_id:id, reason:"Locate failures and verify the fix." };
function respond(body:unknown,status=200) {
  const fetcher=vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(body),{status,headers:{"Content-Type":"application/json"}}));
  vi.stubGlobal("fetch",fetcher); return fetcher;
}
afterEach(()=>{vi.unstubAllGlobals();setSchemaLogger(noopLogger);setCurrentWorkspace(null,null);});
describe("creator recommendation API",()=>{
  it("pins workspace and cancellation while sending only the current task text",async()=>{
    setCurrentWorkspace("other","ws-other");
    const fetcher=respond({recommendations:[candidate]}); const controller=new AbortController(); const signal=controller.signal;
    await expect(new ApiClient("https://api.test").recommendIssueCreators("Find the bug",{workspaceId:"ws-one",signal}))
      .resolves.toEqual({recommendations:[candidate]});
    expect(fetcher).toHaveBeenCalledWith("https://api.test/api/issues/recommend-creators",expect.objectContaining({
      method:"POST",body:JSON.stringify({text:"Find the bug"}),signal:expect.objectContaining({aborted:false}),
      headers:expect.objectContaining({"X-Workspace-ID":"ws-one","X-Workspace-Slug":""}),
    }));
    controller.abort("request cancelled");
    expect(fetcher.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
    expect(fetcher.mock.calls[0]?.[1]?.signal?.reason).toBe("request cancelled");
  });
  it("accepts empty results and harmless future fields",async()=>{
    respond({recommendations:[{...candidate,future:true}],future:true});
    await expect(new ApiClient("https://api.test").recommendIssueCreators("task",{workspaceId:"ws"}))
      .resolves.toEqual({recommendations:[candidate]});
    respond({recommendations:[]});
    await expect(new ApiClient("https://api.test").recommendIssueCreators("task",{workspaceId:"ws"}))
      .resolves.toEqual({recommendations:[]});
  });
  it.each([null,{}, {recommendations:null},{recommendations:[{...candidate,actor_type:"member"}]},
    {recommendations:[{...candidate,actor_id:"invented"}]},{recommendations:[{...candidate,reason:" "}]},
    {recommendations:[{...candidate,reason:"界".repeat(241)}]},
    {recommendations:[candidate,candidate]}, {recommendations:Array.from({length:4},()=>candidate)},
  ])("rejects malformed output instead of claiming no matches %#",async body=>{
    respond(body);
    await expect(new ApiClient("https://api.test").recommendIssueCreators("task",{workspaceId:"ws"}))
      .rejects.toMatchObject({status:502,body:{code:"ai_invalid_output"}});
  });
  it("counts evidence length in Unicode codepoints and normalizes IDs",async()=>{
    respond({recommendations:[{...candidate,actor_id:id.toUpperCase(),reason:"😀".repeat(240)}]});
    const result=await new ApiClient("https://api.test").recommendIssueCreators("task",{workspaceId:"ws"});
    expect(result.recommendations[0]).toEqual({...candidate,reason:"😀".repeat(240)});
  });
  it("redacts malformed private provider output and preserves service errors",async()=>{
    const warn=vi.fn();setSchemaLogger({...noopLogger,warn});
    respond({recommendations:[{...candidate,reason:{secret:"PRIVATE_CONTENT"}}]});
    await expect(new ApiClient("https://api.test").recommendIssueCreators("task",{workspaceId:"ws"})).rejects.toThrow();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_CONTENT");
    respond({code:"ai_unavailable",error:"Unavailable"},503);
    await expect(new ApiClient("https://api.test").recommendIssueCreators("task",{workspaceId:"ws"}))
      .rejects.toMatchObject({status:503,body:{code:"ai_unavailable"}});
  });
});
