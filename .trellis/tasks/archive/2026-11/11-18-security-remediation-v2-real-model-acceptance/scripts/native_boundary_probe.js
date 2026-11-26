// No-model native sandbox probe: a live control-plane peer and an exact Unix broker.
const fs = require('node:fs');
const net = require('node:net');
const http = require('node:http');
const { spawn, spawnSync } = require('node:child_process');

const socketPath = '/tmp/secrem-api.sock';
const requestPipe = '/tmp/secrem-request';
const responsePipe = '/tmp/secrem-response';

function connect(options) {
  return new Promise((resolve) => {
    const connection = net.createConnection(options);
    connection.setTimeout(750);
    connection.once('connect', () => { connection.destroy(); resolve(true); });
    connection.once('error', () => resolve(false));
    connection.once('timeout', () => { connection.destroy(); resolve(false); });
  });
}

async function request(path, token) {
  if (process.env.SECREM_PROBE_FIFO === '1') {
    fs.writeFileSync(requestPipe, JSON.stringify({ path, token }) + '\n', { flag: 'w' });
    return JSON.parse(fs.readFileSync(responsePipe, 'utf8'));
  }
  return new Promise((resolve, reject) => {
    const request = http.get({ socketPath, path, headers: { Authorization: token } }, (response) => {
      let body = '';
      response.on('data', (data) => { body += data; });
      response.once('end', () => resolve({ status: response.statusCode, body }));
    });
    request.once('error', reject);
    request.setTimeout(1000, () => request.destroy(new Error('broker timeout')));
  });
}

async function tool() {
  const port = Number(process.argv[3]);
  if (await connect({ host: '127.0.0.1', port })) throw new Error('sandbox reached control plane');
  if (await connect({ host: '198.51.100.1', port: 443 })) throw new Error('sandbox reached external peer');
  const broker = await request('/api/issues/synthetic', 'Bearer synthetic-task');
  if (broker.status !== 200 || !broker.body.includes('synthetic-task')) throw new Error('broker failed identity');
  const arbitrary = await request('/model/responses', 'Bearer synthetic-task');
  if (arbitrary.status !== 403) throw new Error('broker opened model path');
  const nsenter = spawnSync('nsenter', ['-t', '1', '-n', 'true'], { encoding: 'utf8', timeout: 1000 });
  if (nsenter.status === 0) throw new Error('sandbox reentered provider namespace');
  const child = spawnSync('sh', ['-c', `curl --max-time 1 -s http://127.0.0.1:${port}`], { encoding: 'utf8', timeout: 2000 });
  if (child.status === 0) throw new Error('curl bypassed network boundary');
  console.log(JSON.stringify({ sandbox_tcp_denied: true, external_tcp_denied: true, unix_api_identity: true, unix_arbitrary_route_denied: true, nsenter_denied: true, curl_denied: true, curl_exit: child.status }));
}

async function main() {
  const peer = http.createServer((request, response) => { response.end('synthetic control plane'); });
  await new Promise((resolve) => peer.listen(0, '127.0.0.1', resolve));
  const port = peer.address().port;
  if (!await connect({ host: '127.0.0.1', port })) throw new Error('control-plane positive check failed');
  const broker = http.createServer((request, response) => {
    if (request.url !== '/api/issues/synthetic' || request.headers.authorization !== 'Bearer synthetic-task') {
      response.writeHead(403); response.end(); return;
    }
    response.setHeader('Content-Type', 'application/json');
    response.end(JSON.stringify({ synthetic: true, authorization: request.headers.authorization }));
  });
  await new Promise((resolve) => broker.listen(socketPath, resolve));
  if (process.env.SECREM_PROBE_FIFO === '1') {
    const result = spawnSync('mkfifo', [requestPipe, responsePipe], { encoding: 'utf8' });
    if (result.status !== 0) throw new Error('could not create task pipes');
    const processPipe = () => {
      const stream = fs.createReadStream(requestPipe, { encoding: 'utf8' });
      let content = '';
      stream.on('data', (chunk) => { content += chunk; });
      stream.on('end', () => {
        if (!content.trim()) { processPipe(); return; }
        const input = JSON.parse(content);
        const status = input.path === '/api/issues/synthetic' && input.token === 'Bearer synthetic-task' ? 200 : 403;
        fs.writeFile(responsePipe, JSON.stringify({ status, body: status === 200 ? 'synthetic-task' : '' }), (error) => {
          if (error) throw error;
          if (status === 200) processPipe();
        });
      });
    };
    processPipe();
  }
  const extra = process.env.SECREM_PROBE_UNIX_ALLOW === '1'
    ? ['-c', 'permissions.secrem.extends=":workspace"', '-c', 'permissions.secrem.network.enabled=false', '-c', 'permissions.secrem.network.unix_sockets={"/tmp/secrem-api.sock"="allow"}'] : [];
  const child = spawn('codex', ['sandbox', '-P', extra.length ? 'secrem' : ':workspace', '-c', 'approval_policy="never"', ...extra, '--', 'node', '/probe/native_boundary_probe.js', 'tool', String(port)], { stdio: ['ignore', 'pipe', 'pipe'] });
  let output = '', error = '';
  child.stdout.on('data', (data) => { output += data; });
  child.stderr.on('data', (data) => { error += data; });
  const timeout = setTimeout(() => child.kill('SIGKILL'), 15000);
  const exit = await new Promise((resolve) => child.once('exit', resolve));
  clearTimeout(timeout);
  peer.close(); broker.close();
  if (exit !== 0) throw new Error(`sandbox probe exit ${exit}: ${error}; stdout=${output}`);
  console.log(JSON.stringify({ provider_loopback_positive: true, tool: JSON.parse(output), native_stderr: error }));
}

(process.argv[2] === 'tool' ? tool() : main()).catch((error) => { console.error(error.message); process.exitCode = 1; });
