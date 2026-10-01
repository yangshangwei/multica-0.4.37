// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
afterEach(() => vi.unstubAllGlobals());
it('parses password-member usernames and safely ignores malformed optional usernames', async () => {
  const member = {id:'m',workspace_id:'ws',user_id:'u',role:'member',name:'Same Name',email:''};
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(JSON.stringify([{...member,username:'alice'},{...member,id:'m2',user_id:'u2',username:17}]))));
  const members = await new ApiClient('http://localhost').listMembers('ws');
  expect(members[0]?.username).toBe('alice');
  expect(members[1]?.username).toBeUndefined();
});
