// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { spawn, type ChildProcess } from "node:child_process";
import { ManagedControlConnection } from "./managed-control";
import { transitionManagedDaemon, type ManagedTransitionRecovery } from "./managed-transition";
import type { ManagedDaemonScope } from "./managed-session";

const children: ChildProcess[] = [];
afterEach(async () => { for (const child of children.splice(0)) if (child.exitCode === null && child.signalCode === null) { child.kill("SIGKILL"); await new Promise(resolve => child.once("exit", resolve)); } });
const secret = "A".repeat(43);
async function fakeProcess(scope: { user_id: string; auth_version: string }, active = 0, rejectHandoff = false, dropShutdown = false) {
 const source = `
 const http=require('node:http'),crypto=require('node:crypto');
 const key=Buffer.from(${JSON.stringify(secret)},'base64url');
 let active=${active},intent=null;
 function mac(domain,r,body,status){const f=[domain,r.method,r.url,r.headers['x-multica-management-nonce'],r.headers['x-multica-management-time']];if(status)f.push(String(status));f.push(crypto.createHash('sha256').update(body).digest('hex'));return crypto.createHmac('sha256',key).update(f.join('\\n')).digest('hex');}
 const server=http.createServer(async(r,w)=>{let chunks=[];for await(const chunk of r)chunks.push(chunk);const raw=Buffer.concat(chunks);
 if(r.url==='/health'){w.end(JSON.stringify({status:'running'}));return;}
 if(mac('multica-management-request-v1',r,raw)!==r.headers['x-multica-management-auth']){w.writeHead(401).end();return;}
 const reply=(status,value)=>{const body=Buffer.from(JSON.stringify(value));w.setHeader('X-Multica-Management-Response',mac('multica-management-response-v1',r,body,status));w.writeHead(status,{'Content-Type':'application/json'});w.end(body);};
 if(r.url==='/management/session'){reply(200,{...${JSON.stringify(scope)},active_task_count:active,workspace_ids:[],drain_intent_id:intent});return;}
 if(r.url==='/management/handoff'){reply(200,{accepted:${!rejectHandoff}});return;}
 if(r.url==='/shutdown'){const input=JSON.parse(raw);if(intent!==input.intent_id&&intent!==input.expected_intent_id){reply(409,{accepted:false});return;}intent=input.intent_id;process.send({event:'drain',intent});if(${dropShutdown}){w.destroy();}else{reply(202,{accepted:true,intent_id:intent,status:'draining'});}if(!active)setTimeout(()=>server.close(()=>process.exit(0)),5);return;}
 reply(404,{});
 });
 process.on('message',message=>{if(message==='finish'){active=0;if(intent)server.close(()=>process.exit(0));}});
 server.listen(0,'127.0.0.1',()=>process.send({event:'ready',port:server.address().port}));
 `;
 const child = spawn(process.execPath, ["-e", source], { stdio: ["ignore", "ignore", "pipe", "ipc"] }); children.push(child);
 let error = ""; child.stderr!.on("data", chunk => { error += String(chunk); });
 const port = await new Promise<number>((resolve, reject) => { child.on("message", (value: unknown) => { if (value && typeof value === "object" && "event" in value && value.event === "ready" && "port" in value) resolve(Number(value.port)); }); child.once("exit", code => reject(new Error(`Fake process exited ${code}: ${error}`))); });
 const connection = new ManagedControlConnection(port, secret, new AbortController().signal);
 const stopped = () => child.exitCode !== null || child.signalCode !== null;
 const waitForStop = async () => { for (let i = 0; i < 40 && !stopped(); i++) await new Promise(resolve => setTimeout(resolve, 5)); return stopped(); };
 const exited = new Promise<void>(resolve => child.once("exit", () => resolve()));
 return { child, port, connection, stopped, waitForStop, exited };
}

async function fixture(target: { user_id: string; auth_version: string }, active = 1, rejectHandoff = false, dropShutdown = false) {
 const oldScope = { user_id: "account-a", auth_version: "1" };
 const old = await fakeProcess(oldScope, active, rejectHandoff, dropShutdown);
 let current = true; const handoff = JSON.stringify({ ...target, bindings: [{ workspace_id: "workspace" }] });
 const start = vi.fn(async (value: string) => { expect(old.stopped()).toBe(true); const replacement = await fakeProcess(JSON.parse(value)); return { success: replacement.child.pid !== undefined }; });
 const session = {
  connectRunningDaemon: async () => ({ scope: await old.connection.request("GET", "/management/session") as ManagedDaemonScope, connection: old.connection }),
  matchesDaemonScope: (scope: ManagedDaemonScope) => scope.user_id === target.user_id && scope.auth_version === target.auth_version,
  createHandoff: vi.fn(async () => handoff), acknowledgeHandoff: vi.fn(async () => undefined),
 };
 const recovery: ManagedTransitionRecovery = { stopRequested: false };
 const options = { session, port: old.port, recovery, isCurrent: () => current, isStopped: async () => old.stopped(), waitForStop: old.waitForStop, start };
 return { old, options, start, session, recovery, supersede: () => { current = false; } };
}

it.each([{ user_id: "account-b", auth_version: "1" }, { user_id: "account-a", auth_version: "2" }])("drains active work then automatically starts $user_id/$auth_version", async target => {
 const f = await fixture(target);
 expect(await transitionManagedDaemon(f.options)).toEqual({ accepted: false, reason: "busy" });
 expect(f.start).not.toHaveBeenCalled(); expect(f.recovery.stopRequested).toBe(true);
 f.old.child.send("finish"); await f.old.exited;
 expect(await transitionManagedDaemon(f.options)).toEqual({ accepted: true });
 expect(f.start).toHaveBeenCalledOnce(); expect(f.session.acknowledgeHandoff).toHaveBeenCalledOnce();
});
it("does not report accepted:false handoff as a completed session", async () => {
 const f = await fixture({ user_id: "account-a", auth_version: "1" }, 0, true);
 expect(await transitionManagedDaemon(f.options)).toEqual({ accepted: false, reason: "switching" });
 expect(f.start).not.toHaveBeenCalled(); expect(f.session.acknowledgeHandoff).not.toHaveBeenCalled();
});
it("continues after unknown shutdown acknowledgement only after real process exit", async () => {
 const f = await fixture({ user_id: "account-b", auth_version: "1" }, 0, false, true);
 expect(await transitionManagedDaemon(f.options)).toEqual({ accepted: true });
 expect(f.start).toHaveBeenCalledOnce();
});
it("cannot start a superseded session after the old process exits", async () => {
 const f = await fixture({ user_id: "account-b", auth_version: "1" });
 await transitionManagedDaemon(f.options); f.supersede(); f.old.child.send("finish"); await f.old.exited;
 expect(await transitionManagedDaemon(f.options)).toEqual({ accepted: false, reason: "switching" });
 expect(f.start).not.toHaveBeenCalled();
 const fresh = { ...f.options, isCurrent: () => true, session: { ...f.session, createHandoff: async () => JSON.stringify({ user_id: "account-c", auth_version: "3", bindings: [] }) } };
 expect(await transitionManagedDaemon(fresh)).toEqual({ accepted: true });
 expect(JSON.parse(f.start.mock.calls[0][0]).user_id).toBe("account-c");
});
it("does not drain after a newer login supersedes proof preparation", async () => {
 const f = await fixture({ user_id: "account-b", auth_version: "1" });
 f.session.createHandoff.mockImplementation(async () => { f.supersede(); return "{}"; });
 expect(await transitionManagedDaemon(f.options)).toEqual({ accepted: false, reason: "switching" });
 const scope = await f.old.connection.request("GET", "/management/session") as ManagedDaemonScope;
 expect(scope.drain_intent_id).toBeNull(); expect(f.recovery.stopRequested).toBe(false);
});
