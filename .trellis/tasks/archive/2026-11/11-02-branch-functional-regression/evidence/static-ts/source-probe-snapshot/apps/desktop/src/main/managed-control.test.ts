// @vitest-environment node
import { afterEach, expect, it } from "vitest";
import { createServer, type Server } from "node:http";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { ManagedControlConnection, managementResponseMAC, MANAGEMENT_RESPONSE, UntrustedManagementPeerError } from "./managed-control";

const secret = "A".repeat(43);
const servers: Server[] = [];
afterEach(async () => { await Promise.all(servers.splice(0).map(server => new Promise<void>((resolve, reject) => { server.closeAllConnections(); server.close(error => error ? reject(error) : resolve()); }))); });
async function listener(mode: "valid" | "attacker" | "body" | "status" | "path" | "method" | "nonce") {
 const server = createServer((request, response) => {
  expect(request.headers).not.toHaveProperty("x-multica-management-token");
  expect(JSON.stringify(request.headers)).not.toContain(secret);
  const headers = new Headers(); for (const [name, value] of Object.entries(request.headers)) if (typeof value === "string") headers.set(name, value);
  let body = Buffer.from(JSON.stringify({ accepted: true }));
  if (mode !== "attacker") {
   if (mode === "nonce") headers.set("X-Multica-Management-Nonce", "stale");
   const proof = managementResponseMAC(secret, mode === "method" ? "GET" : "POST", mode === "path" ? "/management/handoff" : "/shutdown", headers, mode === "status" ? 201 : 202, body);
   response.setHeader(MANAGEMENT_RESPONSE, proof);
  }
  if (mode === "body") body = Buffer.from(JSON.stringify({ accepted: false }));
  response.writeHead(202, { "Content-Type": "application/json" }); response.end(body);
 });
 servers.push(server); await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
 const address = server.address(); if (!address || typeof address === "string") throw new Error("Missing test port");
 return new ManagedControlConnection(address.port, secret, new AbortController().signal);
}
it("authenticates a valid local response without transmitting its secret", async () => { const connection = await listener("valid"); expect(await connection.request("POST", "/shutdown", "{}")).toEqual({ accepted: true }); });
it.each(["attacker", "body", "status", "path", "method", "nonce"] as const)("rejects %s response before trusting local control", async mode => { const connection = await listener(mode); await expect(connection.request("POST", "/shutdown", "{}")).rejects.toBeInstanceOf(UntrustedManagementPeerError); });
it("matches the shared Node/Go response HMAC vector", async () => {
 const value = JSON.parse(await readFile(join(__dirname, "../../../../server/internal/daemon/testdata/managed-control.json"), "utf8"));
 const headers = new Headers({ "X-Multica-Management-Nonce": value.nonce, "X-Multica-Management-Time": value.timestamp });
 expect(managementResponseMAC(value.secret, value.method, value.path, headers, value.status, Buffer.from(value.response_body))).toBe(value.response_mac);
});
