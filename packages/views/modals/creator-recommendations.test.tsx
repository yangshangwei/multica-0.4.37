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
 it("explains the empty-description prerequisite and enables suggestions after typing",()=>{
  text="";
  const view=render(ui());
  const button=screen.getByRole("button",{name:"Suggest an agent"});
  expect(button).toBeDisabled();
  expect(button).toHaveAccessibleDescription("Describe the task first to get suggestions.");
  expect(screen.getByText("Describe the task first to get suggestions.")).toBeVisible();
  text="Investigate a failure";view.rerender(ui());
  expect(button).toBeEnabled();
  expect(screen.queryByText("Describe the task first to get suggestions.")).not.toBeInTheDocument();
  expect(call).not.toHaveBeenCalled();
 });
 it("drops evidence after its saved responsibility changes, even if the quote remains",async()=>{
  const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  await screen.findByRole("button",{name:"Use Ada"});
  view.rerender(ui(text,[{...actors[0]!,description:"Do not Investigate failures"}]));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();
 });

 it("only requests on click and only selects on explicit adoption",async()=>{
  const needsSpace=vi.fn();render(ui(text,actors,{onNeedsSpace:needsSpace}));expect(call).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  expect(call).toHaveBeenCalledTimes(1);expect(needsSpace).toHaveBeenCalledTimes(1);
  await screen.findByText("Investigate failures");expect(pick).not.toHaveBeenCalled();
  // Visible text is the bare verb; the name still says whom.
  expect(screen.getByRole("button",{name:"Use Ada"})).toHaveTextContent(/^Use$/);
  fireEvent.click(screen.getByRole("button",{name:"Use Ada"}));
  expect(pick).toHaveBeenCalledWith({type:"agent",id:"a"});
 });
 it("aborts on edited text and ignores a provider that resolves after cancellation",async()=>{
  let resolve!:(value:unknown)=>void;call.mockImplementation(()=>new Promise(r=>{resolve=r}));
  const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  const signal=call.mock.calls[0]![0].signal as AbortSignal;
  text="Write documentation";view.rerender(ui());expect(signal.aborted).toBe(true);
  expect(screen.getByRole("status")).toHaveTextContent("Something changed. Recommendations were cleared; ask again.");
  await act(async()=>resolve({recommendations:[{actor_type:"agent",actor_id:"a",reason:"Investigate failures"}]}));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();expect(pick).not.toHaveBeenCalled();
 });
 it("checks live editor text before adopting even before the debounced prop changes",async()=>{
  render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  await screen.findByRole("button",{name:"Use Ada"});text="Changed immediately";
  fireEvent.click(screen.getByRole("button",{name:"Use Ada"}));expect(pick).not.toHaveBeenCalled();
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent("Something changed. Recommendations were cleared; ask again.");
 });
 it("omits no-longer-eligible actors and keeps failure distinct from no matches",async()=>{
  const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  await screen.findByRole("button",{name:"Use Ada"});view.rerender(ui(text,[]));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();
  // With nobody left to suggest the button is off, so the failure needs candidates again.
  view.rerender(ui());
  call.mockRejectedValueOnce(new Error("failed"));fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  await waitFor(()=>expect(screen.getByRole("alert")).toBeInTheDocument());expect(pick).not.toHaveBeenCalled();
 });
 it("turns into the cancel control while pending, like the description helper",async()=>{
  let resolve!:(value:unknown)=>void;call.mockImplementation(()=>new Promise(r=>{resolve=r}));
  render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  expect(screen.getByRole("status")).toHaveTextContent("Finding suggestions...");
  expect(screen.queryByRole("button",{name:"Suggest an agent"})).not.toBeInTheDocument();
  expect(screen.getAllByRole("button")).toHaveLength(1);
  fireEvent.click(screen.getByRole("button",{name:"Cancel"}));
  expect((call.mock.calls[0]![0].signal as AbortSignal).aborted).toBe(true);
  expect(screen.getByRole("button",{name:"Suggest an agent"})).toBeEnabled();
  // A deliberate cancel is not a stale result, so nothing is announced.
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
  await act(async()=>resolve({recommendations:[{actor_type:"agent",actor_id:"a",reason:"Investigate failures"}]}));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();
 });
 it("shows a failure in the alert tone with a retry that asks again",async()=>{
  call.mockRejectedValueOnce(new Error("failed"));
  render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  const alert=await screen.findByRole("alert");
  expect(alert).toHaveTextContent("Could not get recommendations. Try again or choose manually.");
  expect(alert.className).toContain("text-destructive");
  fireEvent.click(screen.getByRole("button",{name:"Try again"}));
  expect(call).toHaveBeenCalledTimes(2);
  await screen.findByRole("button",{name:"Use Ada"});
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
 });
 it("says why a shown result disappeared, until the next request, but not for a discarded error",async()=>{
  const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  await screen.findByRole("button",{name:"Use Ada"});
  view.rerender(ui(text,actors,{projectId:"other-project"}));
  expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent("Something changed. Recommendations were cleared; ask again.");
  call.mockRejectedValueOnce(new Error("failed"));
  fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
  await screen.findByRole("alert");
  expect(screen.queryByText("Something changed. Recommendations were cleared; ask again.")).not.toBeInTheDocument();
  view.rerender(ui(text,actors,{projectId:"third-project"}));
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
 });
 it("does not ask for suggestions when nobody can be suggested, and says why",()=>{
  render(ui(text,[]));
  const button=screen.getByRole("button",{name:"Suggest an agent"});
  expect(button).toBeDisabled();
  expect(button).toHaveAccessibleDescription("No agent or squad to suggest.");
  expect(screen.getByText("No agent or squad to suggest.")).toBeVisible();
  fireEvent.click(button);
  expect(call).not.toHaveBeenCalled();
 });
});


it.each([{projectId:"other-project"},{actor:{type:"agent" as const,id:"a"}},{identityKey:"ws:user:next-login"},{wsId:"other-workspace"}])("discards a pending response when its context changes: %j",async(overrides)=>{
 let resolve!:(value:unknown)=>void;call.mockImplementationOnce(()=>new Promise(r=>{resolve=r}));
 const view=render(ui());fireEvent.click(screen.getByRole("button",{name:"Suggest an agent"}));
 const signal=call.mock.calls[0]![0].signal as AbortSignal;
 view.rerender(ui(text,actors,overrides));expect(signal.aborted).toBe(true);
 await act(async()=>resolve({recommendations:[{actor_type:"agent",actor_id:"a",reason:"Investigate failures"}]}));
 expect(screen.queryByRole("button",{name:"Use Ada"})).not.toBeInTheDocument();expect(pick).not.toHaveBeenCalled();
});
