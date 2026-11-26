// No model turn: verify native app-server command/exec against an offline tool policy.
const fs = require('node:fs');
const http = require('node:http');
const { spawn } = require('node:child_process');

async function main() {
  let apiCalls = 0;
  const peer = http.createServer((req, res) => {
    if (req.url !== '/api/issues/synthetic' || req.headers.authorization !== 'Bearer synthetic-task') {
      res.writeHead(403); res.end(); return;
    }
    apiCalls++;
    res.end(JSON.stringify({ synthetic: true, actor: 'synthetic-task', issue: 'synthetic' }));
  });
  await new Promise((resolve) => peer.listen(0, '127.0.0.1', resolve));
  const port = peer.address().port;
  const child = spawn('codex', ['app-server', '--listen', 'stdio://', '-c', 'sandbox_mode="workspace-write"', '-c', 'approval_policy="never"', '-c', 'sandbox_workspace_write.network_access=false', '-c', 'mcp_servers.synthetic.command="node"', '-c', 'mcp_servers.synthetic.args=["/probe/synthetic_mcp_api.js"]', '-c', `mcp_servers.synthetic.env={SYNTHETIC_API="http://127.0.0.1:${port}"}`], { stdio: ['pipe', 'pipe', 'pipe'] });
  let buffer = '', stderr = '', next = 1;
  const pending = new Map();
  child.stdout.on('data', (chunk) => {
    buffer += chunk;
    let end;
    while ((end = buffer.indexOf('\n')) >= 0) {
      const line = buffer.slice(0, end); buffer = buffer.slice(end + 1);
      if (!line.trim()) continue;
      const message = JSON.parse(line);
      if (pending.has(message.id)) { pending.get(message.id)(message); pending.delete(message.id); }
    }
  });
  child.stderr.on('data', (chunk) => { stderr += chunk; });
  child.once('exit', (code) => {
    if (pending.size) {
      console.error(`app-server exited ${code}; stderr=${stderr}`);
      for (const resolve of pending.values()) resolve({ error: { code, message: 'process exited' } });
      pending.clear();
    }
  });
  const timeout = setTimeout(() => child.kill('SIGKILL'), 15000);
  function request(method, params) {
    return new Promise((resolve, reject) => {
      const id = next++;
      const timer = setTimeout(() => { pending.delete(id); reject(new Error(`RPC ${method} timed out; stderr=${stderr}`)); }, 5000);
      pending.set(id, (message) => { clearTimeout(timer); resolve(message); });
      child.stdin.write(JSON.stringify({ id, method, params }) + '\n');
    });
  }
  const initialized = await request('initialize', { clientInfo: { name: 'secrem_offline_probe', version: '1' }, capabilities: { experimentalApi: true } });
  child.stdin.write(JSON.stringify({ method: 'initialized' }) + '\n');
  const requirements = await request('configRequirements/read', {});
  const script = `const n=require('node:net'); const s=n.connect(${port},'127.0.0.1'); s.setTimeout(1000); s.on('connect',()=>{console.log('BYPASS');process.exit(2)});s.on('error',()=>{console.log('DENIED');process.exit(0)});s.on('timeout',()=>{console.log('DENIED');process.exit(0)});`;
  const ordinary = await request('command/exec', { command: ['node', '-e', script], cwd: '/tmp', timeoutMs: 2000 });
  const widened = await request('command/exec', { command: ['node', '-e', script], cwd: '/tmp', timeoutMs: 2000, sandboxPolicy: { type: 'dangerFullAccess' } });
  const networkWidened = await request('command/exec', { command: ['node', '-e', script], cwd: '/tmp', timeoutMs: 2000, sandboxPolicy: { type: 'workspaceWrite', networkAccess: true } });
  const thread = await request('thread/start', { model: 'synthetic-model', cwd: '/tmp', ephemeral: true });
  const threadId = thread.result?.thread?.id;
  let mcp, outside;
  if (threadId) {
    mcp = await request('mcpServer/tool/call', { threadId, server: 'synthetic', tool: 'read_issue', arguments: { issue: 'synthetic' } });
    outside = await request('mcpServer/tool/call', { threadId, server: 'synthetic', tool: 'read_issue', arguments: { issue: 'outside' } });
  }
  child.stdin.end(); child.kill('SIGTERM'); clearTimeout(timeout); peer.close();
  console.log(JSON.stringify({ initialized: !initialized.error, requirements, ordinary, widened, networkWidened, threadStarted: !!threadId, threadError: thread.error, mcp, outside, apiCalls, modelTurns: 0, stderr }));
}
main().catch((error) => { console.error(error.stack); process.exit(1); });
