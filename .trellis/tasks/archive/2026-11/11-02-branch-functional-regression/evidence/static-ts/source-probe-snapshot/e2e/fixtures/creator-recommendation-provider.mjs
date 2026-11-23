import { createServer } from 'node:http';
import { setTimeout } from 'node:timers/promises';
let calls = 0;
const server = createServer(async (req, res) => {
  if (req.url === '/health') { res.setHeader('Content-Type','application/json');res.end(JSON.stringify({calls}));return; }
  if (req.method !== 'POST' || !req.url.endsWith('/chat/completions')) {res.writeHead(404);res.end();return;}
  try {
    const chunks=[];for await (const chunk of req) chunks.push(chunk);
    const body=JSON.parse(Buffer.concat(chunks).toString());
    const input=JSON.parse(body.messages.at(-1).content);
    if (!Array.isArray(input.candidates)) throw new Error('Unexpected consumer');
    calls++;
    if (input.text.includes('CREATORSLOW')) await setTimeout(1500);
    if (res.destroyed) return;
    res.setHeader('Content-Type','application/json');
    if (input.text.includes('CREATORFAIL')) {res.writeHead(503);res.end(JSON.stringify({error:{message:'controlled failure'}}));return;}
    const recommendations=input.candidates.slice(0,3).map(candidate=>({ref:candidate.ref,evidence:Array.from(candidate.description_excerpt).slice(0,120).join('')}));
    res.end(JSON.stringify({choices:[{message:{content:JSON.stringify({recommendations})},finish_reason:'stop'}]}));
  } catch { res.writeHead(400);res.end(JSON.stringify({error:{message:'Unsupported test payload'}})); }
});
server.listen(Number(process.env.ACTOR_RECOMMENDATION_PORT) || 0,'127.0.0.1',()=>process.stdout.write(`http://127.0.0.1:${server.address().port}\n`));
