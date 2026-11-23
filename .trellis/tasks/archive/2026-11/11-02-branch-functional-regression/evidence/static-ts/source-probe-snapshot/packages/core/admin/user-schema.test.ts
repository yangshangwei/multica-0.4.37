// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseAdminUsers, parseAdminUserDetail, parseAdminUserOperation } from "./user-schema";
const id="00000000-0000-4000-8000-000000000001";
const user={id,name:"Account",username:"account",platform_role:null,status:"active",auth_version:1,workspace_count:0,created_at:"2026-10-01T00:00:00Z",allowed_actions:["disable"]};
describe("admin account response boundaries",()=>{
 it("parses directory and excludes unknown fields",()=>{
  expect(parseAdminUsers({items:[{...user,password_hash:"never expose"}],next_cursor:null,scope:id,as_of:user.created_at,data_quality:"complete",registration:{enabled:true,approval_required:false}})?.items[0]).not.toHaveProperty("password_hash");
 });
 it("fails closed on malformed identities and unsafe versions",()=>{
  expect(parseAdminUserDetail({user:{...user,id:"bad"},memberships:[],scope:id})).toBeNull();
  expect(parseAdminUserDetail({user:{...user,auth_version:Number.MAX_SAFE_INTEGER+1},memberships:[],scope:id})).toBeNull();
  expect(parseAdminUserOperation({id:"not-an-operation"})).toBeNull();
 });
 it("uses unknown state without inventing actions",()=>{
  const result=parseAdminUserDetail({user:{...user,status:"future_state",allowed_actions:[]},memberships:[],memberships_truncated:false,scope:id});
  expect(result?.user.status).toBe("unknown");expect(result?.user.allowedActions).toEqual([]);
 });
});
