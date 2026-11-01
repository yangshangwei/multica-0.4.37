import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import test from "node:test";
import { InstalledMainClient } from "./collaboration-installed-main.ts";

test("native controller shutdown preserves a failed close command after its owned child exits", async () => {
  const client = Object.create(InstalledMainClient.prototype);
  const failure = new Error("owned close failure");
  client.nativeMode = true;
  client.ended = false;
  client.command = async () => { throw failure; };
  const child = new EventEmitter();
  child.stdin = { end: () => { client.ended = true; child.emit("close", 1); } };
  client.child = child;
  await assert.rejects(client.close(), error => error === failure);
});

test("already closed controller does not send another command or touch Docker", async () => {
  const client = Object.create(InstalledMainClient.prototype);
  client.ended = true;
  client.command = () => { throw new Error("must not send"); };
  await client.close();
});
