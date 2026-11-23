import {test, expect} from "__SOURCE_ROOT__/node_modules/@playwright/test/index.mjs";
import pg from "__SOURCE_ROOT__/node_modules/pg/lib/index.js";
import {seedRuntimeHandoff, assertRuntimeHandoff} from "__SOURCE_ROOT__/e2e/fixtures/collaboration-runtime-handoff.ts";

// Acceptance gap ENV-1: live context must not claim a newer document was the startup version.
// Retained execution rows are seeded; memory edit and reads use the actual API, and UI runs on production Web.
test('ENV-1 a live latest memory v1 must not be labelled as the startup version of a run frozen at v0', async ({page}, info) => {
  test.setTimeout(120_000);
  const f = await seedRuntimeHandoff();
  const db = new pg.Client(process.env.DATABASE_URL);
  await db.connect();
  try {
    const updated=await db.query("UPDATE collaboration_run SET memory_mode='supported',memory_base_version=0 WHERE id=$1", [f.runId]); expect(updated.rowCount).toBe(1);
  } finally { await db.end(); }
  const written = await f.creator.client.requestJSON(`${f.root}/memory`, {method:'PUT',headers:{'Idempotency-Key':crypto.randomUUID()},body:{expected_version:'0',content:'Latest decision written after the retained run started.'}});
  const live = await f.recipient.client.requestJSON(f.handoffPath);
  const verify = new pg.Client(process.env.DATABASE_URL); await verify.connect();
  let persisted;
  try {persisted=(await verify.query('SELECT memory_mode,memory_base_version::text FROM collaboration_run WHERE id=$1',[f.runId])).rows[0];}finally{await verify.end();}
  expect(persisted).toEqual({memory_mode:'supported',memory_base_version:'0'});
  expect(written.version).toBe('1');
  await f.recipient.authenticateWeb(page);
  await page.goto(f.path);
  const {handoff} = await assertRuntimeHandoff(page,f);
  await info.attach('ENV-1-api-and-persisted-facts',{contentType:'application/json',body:JSON.stringify({issue_id:f.issueId,run_id:f.runId,persisted,latest_memory_written:written.version,live_memory:live.memory,scope:'seeded retained run; real memory API and production UI; no provider execution'},null,2)});
  await info.attach('ENV-1-live-handoff',{body:await handoff.screenshot({animations:'disabled'}),contentType:'image/png'});
  // This assertion is intentionally the product invariant, not the observed wrong behavior.
  await expect(handoff.getByRole('link',{name:'Started from v1',exact:true})).toHaveCount(0);
});
