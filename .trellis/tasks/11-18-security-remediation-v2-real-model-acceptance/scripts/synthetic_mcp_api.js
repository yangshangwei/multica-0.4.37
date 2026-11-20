// Synthetic MCP peer for the no-model boundary probe; not a production API broker.
const http = require('node:http');
const readline = require('node:readline');
const input = readline.createInterface({ input: process.stdin });

input.on('line', async (line) => {
  const message = JSON.parse(line);
  if (!('id' in message)) return;
  let result;
  if (message.method === 'initialize') {
    result = { protocolVersion: '2024-11-05', capabilities: { tools: {} }, serverInfo: { name: 'secrem_fake_api', version: '1' } };
  } else if (message.method === 'tools/list') {
    result = { tools: [{ name: 'read_issue', description: 'Read the one synthetic issue from the fake API', inputSchema: { type: 'object', properties: { issue: { type: 'string' } }, required: ['issue'], additionalProperties: false } }] };
  } else if (message.method === 'tools/call' && message.params.name === 'read_issue') {
    if (message.params.arguments?.issue !== 'synthetic') {
      result = { isError: true, content: [{ type: 'text', text: 'outside fixed issue scope' }] };
    } else {
      const body = await new Promise((resolve, reject) => {
        http.get(`${process.env.SYNTHETIC_API}/api/issues/synthetic`, { headers: { Authorization: 'Bearer synthetic-task' } }, (response) => {
          let data = ''; response.on('data', (chunk) => { data += chunk; }); response.on('end', () => resolve(data));
        }).on('error', reject);
      });
      result = { content: [{ type: 'text', text: body }] };
    }
  } else {
    result = { isError: true, content: [{ type: 'text', text: 'unsupported operation' }] };
  }
  process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: message.id, result }) + '\n');
});
