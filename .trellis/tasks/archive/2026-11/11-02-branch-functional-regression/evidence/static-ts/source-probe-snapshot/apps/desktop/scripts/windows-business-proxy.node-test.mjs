import assert from 'node:assert/strict';
import http from 'node:http';
import { once } from 'node:events';
import { randomUUID } from 'node:crypto';
import { test } from 'node:test';
import { startFaultProxy } from './windows-business-proxy.mjs';

test('forwards real HTTP, isolates receipt faults, and restores the same request', async () => {
  const paths = [];
  const upstream = http.createServer((request, response) => { paths.push(request.url); request.resume(); response.end('accepted'); });
  upstream.listen(0, '127.0.0.1'); await once(upstream, 'listening');
  const proxy = await startFaultProxy('http://127.0.0.1:' + upstream.address().port);
  try {
    const task = randomUUID(); proxy.blockReceipt(task);
    const blocked = await fetch(proxy.origin + '/api/daemon/tasks/' + task + '/cancel-ack', { method: 'POST', body: '{"synthetic":true}' });
    assert.equal(blocked.status, 503); await blocked.text(); assert.deepEqual(paths, []);
    const healthy = await fetch(proxy.origin + '/health'); assert.equal(await healthy.text(), 'accepted');
    proxy.unblockReceipt(task);
    const recovered = await fetch(proxy.origin + '/api/daemon/tasks/' + task + '/cancel-ack', { method: 'POST' });
    assert.equal(await recovered.text(), 'accepted');
    assert.equal(proxy.evidence.receipt_failures[task], 1);
    assert.deepEqual(paths, ['/health', '/api/daemon/tasks/' + task + '/cancel-ack']);
  } finally { await proxy.close(); await new Promise(resolve => upstream.close(resolve)); }
});

test('tunnels upgrades and disconnects only its own live WebSocket', async () => {
  const upstream = http.createServer();
  const peers = new Set();
  upstream.on('upgrade', (_request, socket) => {
    peers.add(socket); socket.on('close', () => peers.delete(socket));
    socket.write('HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n');
    socket.on('data', bytes => socket.write(bytes));
  });
  upstream.listen(0, '127.0.0.1'); await once(upstream, 'listening');
  const proxy = await startFaultProxy('http://127.0.0.1:' + upstream.address().port);
  try {
    const request = http.request(proxy.origin + '/api/daemon/ws', { headers: { Connection: 'Upgrade', Upgrade: 'websocket' } });
    request.end(); const [, socket] = await once(request, 'upgrade');
    socket.write('synthetic-frame'); const [bytes] = await once(socket, 'data'); assert.equal(bytes.toString(), 'synthetic-frame');
    const closed = once(socket, 'close'); assert.equal(proxy.disconnectWebSockets(true), 1); await closed;
    assert.equal(proxy.evidence.websocket_connections, 1); assert.equal(proxy.evidence.websocket_disconnects, 1);
    assert.equal(proxy.evidence.daemon_websocket_connections, 1);
  } finally { await proxy.close(); for (const peer of peers) peer.destroy(); await new Promise(resolve => upstream.close(resolve)); }
});

test('drops one successful receipt response after upstream acceptance, then permits retry', async () => {
  let accepted = 0;
  const upstream = http.createServer((request, response) => { accepted++; request.resume(); response.end('{"status":"ok"}'); });
  upstream.listen(0, '127.0.0.1'); await once(upstream, 'listening');
  const proxy = await startFaultProxy('http://127.0.0.1:' + upstream.address().port);
  try {
    const task = randomUUID(), url = proxy.origin + '/api/daemon/tasks/' + task + '/cancel-ack';
    proxy.dropReceiptResponse(task);
    await assert.rejects(fetch(url, { method: 'POST' })); assert.equal(accepted, 1);
    const retry = await fetch(url, { method: 'POST' }); assert.deepEqual(await retry.json(), { status: 'ok' });
    assert.equal(accepted, 2); assert.equal(proxy.evidence.receipt_response_drops[task], 1); assert.equal(proxy.evidence.receipt_successes[task], 2);
  } finally { await proxy.close(); await new Promise(resolve => upstream.close(resolve)); }
});
