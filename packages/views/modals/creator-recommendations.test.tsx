import { createRef, type ComponentProps } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enModals from "../locales/en/modals.json";
import { CreatorRecommendations } from "./creator-recommendations";
const call=vi.hoisted(()=>vi.fn());
const reset=vi.hoisted(()=>vi.fn());
vi.mock("@multica/core/issues/mutations",()=>({useRecommendIssueCreators:()=>({mutateAsync:call,reset})}));
vi.mock("@multica/core/issues/stores/quick-create-store",()=>({captureQuickCreateScope:()=>({workspaceId:"ws"}),isQuickCreateScopeCurrent:()=>true}));
let text="Investigate a failure";
const pick=vi.fn();
const editorRef=createRef<{getMarkdown:()=>string;flushPendingUpdate:()=>string}>();
const actors=[{type:"agent" as const,id:"a",name:"Ada",description:"Investigate failures"}];
function ui(value=text,available=actors,overrides: Partial<ComponentProps<typeof CreatorRecommendations>> = {}) {
 editorRef.current={getMarkdown:()=>text,flushPendingUpdate:()=>text};
 return <I18nProvider locale="en" resources={{en:{modals:enModals}}}>
  <CreatorRecommendations wsId="ws" actor={null} projectId={null} identityKey="ws:user:0" editorRef={editorRef}
   value={value} actors={available} onPick={pick} {...overrides} />
 </I18nProvider>;
}
beforeEach(()=>{text="Investigate a failure";pick.mockReset();call.mockReset().mockResolvedValue({recommendations:[{actor_type:"agent",actor_id:"a",reason:"Investigate failures"}]});});
describe("creator suggestions",()=>{
 it("drops evidence after its saved responsibility changes, even if the quote remains",async()=>{
  const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
  await screen.findByRole("button",{name:"Use Ada"});
  view.rerender(ui(text,[{...actors[0]!,description:"Do not Investigate failures"}]));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();
 });

 it("only requests on click and only selects on explicit adoption",async()=>{
  const needsSpace=vi.fn();render(ui(text,actors,{onNeedsSpace:needsSpace}));expect(call).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
  fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
  expect(call).toHaveBeenCalledTimes(1);expect(needsSpace).toHaveBeenCalledTimes(1);
  await screen.findByText("Investigate failures");expect(pick).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button",{name:"Use Ada"}));
  expect(pick).toHaveBeenCalledWith({type:"agent",id:"a"});
 });
 it("aborts on edited text and ignores a provider that resolves after cancellation",async()=>{
  let resolve!:(value:unknown)=>void;call.mockImplementation(()=>new Promise(r=>{resolve=r}));
  const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
  const signal=call.mock.calls[0]![0].signal as AbortSignal;
  text="Write documentation";view.rerender(ui());expect(signal.aborted).toBe(true);
  await act(async()=>resolve({recommendations:[{actor_type:"agent",actor_id:"a",reason:"Investigate failures"}]}));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();expect(pick).not.toHaveBeenCalled();
 });
 it("checks live editor text before adopting even before the debounced prop changes",async()=>{
  render(ui());fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
  await screen.findByRole("button",{name:"Use Ada"});text="Changed immediately";
  fireEvent.click(screen.getByRole("button",{name:"Use Ada"}));expect(pick).not.toHaveBeenCalled();
 });
 it("omits no-longer-eligible actors and keeps failure distinct from no matches",async()=>{
  const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
  await screen.findByRole("button",{name:"Use Ada"});view.rerender(ui(text,[]));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();
  call.mockRejectedValueOnce(new Error("failed"));fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
  await waitFor(()=>expect(screen.getByRole("alert")).toBeInTheDocument());expect(pick).not.toHaveBeenCalled();
 });
});


it.each([{projectId:"other-project"},{actor:{type:"agent" as const,id:"a"}},{identityKey:"ws:user:next-login"},{wsId:"other-workspace"}])("discards a pending response when its context changes: %j",async(overrides)=>{
 let resolve!:(value:unknown)=>void;call.mockImplementationOnce(()=>new Promise(r=>{resolve=r}));
 const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));
 const signal=call.mock.calls[0]![0].signal as AbortSignal;
 view.rerender(ui(text,actors,overrides));expect(signal.aborted).toBe(true);
 await act(async()=>resolve({recommendations:[{actor_type:"agent",actor_id:"a",reason:"Investigate failures"}]}));
 expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();expect(pick).not.toHaveBeenCalled();
});
