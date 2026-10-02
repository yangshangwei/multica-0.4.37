import test from 'node:test';
import assert from 'node:assert/strict';
import {verifyGoTestEvents} from './verify-go-test-events.mjs';
const encode=rows=>rows.map(x=>JSON.stringify(x)).join('\n');
const good=[{Action:'run',Test:'TestNative'},{Action:'pass',Test:'TestNative'},{Action:'pass',Package:'daemon'}];
test('requires actual named native execution and package completion',()=>assert.equal(verifyGoTestEvents(encode(good),['TestNative']).status,'passed'));
test('rejects missing test, skipped test, package failure and invalid events',()=>{
 assert.throws(()=>verifyGoTestEvents(encode(good),['TestMissing']),/Missing test/);
 assert.throws(()=>verifyGoTestEvents(encode([{Action:'run',Test:'TestNative'},{Action:'skip',Test:'TestNative'},{Action:'pass'}]),['TestNative']),/without skip/);
 assert.throws(()=>verifyGoTestEvents(encode([...good,{Action:'fail',Package:'daemon'}]),['TestNative']),/failure/);
 assert.throws(()=>verifyGoTestEvents(encode([...good,{Action:'skip',Test:'TestNative/child'}]),['TestNative']),/without skip/);
 assert.throws(()=>verifyGoTestEvents(encode(good.slice(0,2)),['TestNative']),/package completion/);
 assert.throws(()=>verifyGoTestEvents('not JSON',['TestNative']));
});
