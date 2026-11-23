import assert from 'node:assert/strict';
import http from 'node:http';

// Task-owned transport faults only. No request bodies, headers or tokens enter evidence.
export async function startFaultProxy(upstream, port = 0) {
  const target = new URL(upstream);
  assert.equal(target.protocol, 'http:');
  assert(['127.0.0.1', 'localhost'].includes(target.hostname));
  const sockets = new Set(), websocketSockets = new Set(), blockedTasks = new Set(), droppedReceiptResponses = new Set();
  const evidence = { http_requests: 0, receipt_failures: {}, receipt_successes: {}, receipt_response_drops: {}, websocket_connections: 0, daemon_websocket_connections: 0, websocket_disconnects: 0 };
  const server = http.createServer((request, response) => {
    evidence.http_requests++;
    const match = /^\/api\/daemon\/tasks\/([a-f0-9-]{36})\/cancel-ack(?:\?|$)/.exec(request.url);
    if (match && blockedTasks.has(match[1])) {
      evidence.receipt_failures[match[1]] = (evidence.receipt_failures[match[1]] || 0) + 1;
      request.resume();
      response.writeHead(503, { 'content-type': 'application/json', 'connection': 'close' });
      response.end('{"error":"isolated_native_receipt_fault"}');
      return;
    }
    const remote = http.request(new URL(request.url, target), {
      method: request.method, headers: { ...request.headers, host: target.host },
    }, incoming => {
      if (match && incoming.statusCode >= 200 && incoming.statusCode < 300) {
        evidence.receipt_successes[match[1]] = (evidence.receipt_successes[match[1]] || 0) + 1;
        if (droppedReceiptResponses.delete(match[1])) {
          evidence.receipt_response_drops[match[1]] = (evidence.receipt_response_drops[match[1]] || 0) + 1;
          incoming.resume(); response.destroy(); return;
        }
      }
      response.writeHead(incoming.statusCode, incoming.headers);
      incoming.pipe(response);
    });
    remote.on('error', () => { if (!response.headersSent) response.writeHead(502); response.end(); });
    request.on('aborted', () => remote.destroy());
    request.pipe(remote);
  });
  server.on('connection', socket => { sockets.add(socket); socket.on('close', () => sockets.delete(socket)); });
  server.on('upgrade', (request, client, head) => {
    const remote = http.request(new URL(request.url, target), {
      method: request.method, headers: { ...request.headers, host: target.host },
    });
    remote.on('upgrade', (incoming, upstreamSocket, upstreamHead) => {
      const pair = { client, upstreamSocket, daemon: new URL(request.url, target).pathname === '/api/daemon/ws' };
      websocketSockets.add(pair); evidence.websocket_connections++;
      if (pair.daemon) evidence.daemon_websocket_connections++;
      const close = () => { websocketSockets.delete(pair); client.destroy(); upstreamSocket.destroy(); };
      client.on('close', close); upstreamSocket.on('close', close);
      client.on('error', close); upstreamSocket.on('error', close);
      const headers = Object.entries(incoming.headers).flatMap(([key, value]) =>
        Array.isArray(value) ? value.map(v => key + ': ' + v) : [key + ': ' + value]);
      client.write('HTTP/1.1 101 Switching Protocols\r\n' + headers.join('\r\n') + '\r\n\r\n');
      if (upstreamHead.length) client.write(upstreamHead);
      if (head.length) upstreamSocket.write(head);
      client.pipe(upstreamSocket); upstreamSocket.pipe(client);
    });
    remote.on('response', incoming => { incoming.resume(); client.end('HTTP/1.1 ' + incoming.statusCode + ' Rejected\r\nConnection: close\r\n\r\n'); });
    remote.on('error', () => client.destroy());
    remote.end();
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(port, '127.0.0.1', resolve); });
  return {
    origin: 'http://127.0.0.1:' + server.address().port,
    evidence,
    blockReceipt(taskId) { assert(/^[a-f0-9-]{36}$/.test(taskId)); blockedTasks.add(taskId); },
    unblockReceipt(taskId) { blockedTasks.delete(taskId); },
    dropReceiptResponse(taskId) { assert(/^[a-f0-9-]{36}$/.test(taskId)); droppedReceiptResponses.add(taskId); },
    disconnectWebSockets(daemonOnly = false) {
      const selected = [...websocketSockets].filter(pair => !daemonOnly || pair.daemon);
      const count = selected.length; evidence.websocket_disconnects += count;
      for (const pair of selected) { pair.client.destroy(); pair.upstreamSocket.destroy(); }
      return count;
    },
    async close() {
      for (const pair of websocketSockets) { pair.client.destroy(); pair.upstreamSocket.destroy(); }
      for (const socket of sockets) socket.destroy();
      await new Promise(resolve => server.close(resolve));
    },
  };
}
