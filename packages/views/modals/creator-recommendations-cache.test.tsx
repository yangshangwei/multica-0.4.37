import { createRef } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enModals from "../locales/en/modals.json";
import { CreatorRecommendations } from "./creator-recommendations";
const call=vi.hoisted(()=>vi.fn());
vi.mock("@multica/core/api",async(importOriginal)=>({...await importOriginal<typeof import("@multica/core/api")>(),api:{recommendIssueCreators:call}}));
vi.mock("@multica/core/issues/stores/quick-create-store",()=>({captureQuickCreateScope:()=>({workspaceId:"ws"}),isQuickCreateScopeCurrent:()=>true}));
it("does not retain private recommendation variables in the global mutation cache after settlement",async()=>{
 const client=new QueryClient();
 const editorRef=createRef<{getMarkdown:()=>string;flushPendingUpdate:()=>string}>();
 editorRef.current={getMarkdown:()=>"Private task text",flushPendingUpdate:()=>"Private task text"};
 call.mockResolvedValue({recommendations:[{actor_type:"agent",actor_id:"a",reason:"Diagnose failures"}]});
 const view=render(<QueryClientProvider client={client}><I18nProvider locale="en" resources={{en:{modals:enModals}}}>
  <CreatorRecommendations wsId="ws" actor={null} projectId={null} identityKey="ws:user:0" value="Private task text" editorRef={editorRef}
   actors={[{type:"agent",id:"a",name:"Ada",description:"Diagnose failures"}]} onPick={vi.fn()}/>
 </I18nProvider></QueryClientProvider>);
 try {
  fireEvent.click(screen.getByRole("button",{name:"Help me choose"}));await screen.findByRole("button",{name:"Use Ada"});
  await waitFor(()=>expect(client.getMutationCache().getAll()).toHaveLength(0));
  expect(screen.getByRole("button",{name:"Use Ada"})).toBeInTheDocument();
 } finally {view.unmount();client.clear();}
});
