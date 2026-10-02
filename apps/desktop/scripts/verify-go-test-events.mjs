import assert from 'node:assert/strict';
import {readFileSync,writeFileSync} from 'node:fs';
import {pathToFileURL} from 'node:url';

export function verifyGoTestEvents(text, expected) {
  assert(expected.length > 0 && new Set(expected).size === expected.length, 'Expected test names must be unique and nonempty');
  const rows=text.trim().split(/\r?\n/).filter(Boolean).map(line=>JSON.parse(line));
  assert(!rows.some(row=>row.Action==='fail'), 'Go test reported failure');
  const tests=expected.map(name=>{
    const events=rows.filter(row=>row.Test===name);
    assert(events.some(row=>row.Action==='run'), `Missing test execution: ${name}`);
    const skipped = rows.some(row => row.Action === 'skip' && typeof row.Test === 'string' && (row.Test === name || row.Test.startsWith(`${name}/`)));
    assert(events.some(row=>row.Action==='pass') && !skipped, `Test did not pass without skip: ${name}`);
    return {name,status:'passed'};
  });
  assert(rows.some(row=>row.Action==='pass'&&!row.Test), 'Missing successful package completion');
  return {status:'passed',source_commit:process.env.GITHUB_SHA??null,runner_os:process.env.RUNNER_OS??process.platform,tests};
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){
 const [input,output,...names]=process.argv.slice(2);
 assert(input&&output,'Usage: verify-go-test-events.mjs input.jsonl output.json TestName...');
 try{const result=verifyGoTestEvents(readFileSync(input,'utf8'),names);writeFileSync(output,JSON.stringify(result,null,2)+'\n');console.log(`Verified ${result.tests.length} native Go tests without skips.`);}
 catch(error){writeFileSync(output,JSON.stringify({status:'failed',error:error.message},null,2)+'\n');throw error;}
}
