from http.server import ThreadingHTTPServer,BaseHTTPRequestHandler
import pathlib,json,urllib.request,urllib.error,time,os,hashlib,threading
p=pathlib.Path(__file__).parent
roles={hashlib.sha256(json.loads((p/(r+'-trial.json')).read_text())['env']['MULTICA_TOKEN'].encode()).hexdigest():r for r in ['reporter','reviewer']}
lock=threading.Lock()
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args): pass
 def forward(self):
  body=self.rfile.read(int(self.headers.get('Content-Length','0')))
  headers={k:v for k,v in self.headers.items() if k.lower() not in ['host','connection','accept-encoding']}
  req=urllib.request.Request('http://localhost:18251'+self.path,data=body if body else None,method=self.command,headers=headers)
  start=time.time()
  try:
   with urllib.request.urlopen(req,timeout=30) as response:status=response.status;data=response.read();outheaders=dict(response.headers)
  except urllib.error.HTTPError as response:status=response.code;data=response.read();outheaders=dict(response.headers)
  except Exception:status=502;data=b'{"error":"owned API proxy connection failure"}';outheaders={'Content-Type':'application/json'}
  digest=hashlib.sha256(self.headers.get('Authorization','').removeprefix('Bearer ').encode()).hexdigest()
  role=roles.get(digest,'unknown')
  record={'time':time.time(),'role':role,'method':self.command,'path':self.path,'status':status,'elapsed_ms':round((time.time()-start)*1000),'request_body_sha256':hashlib.sha256(body).hexdigest(),'response_sha256':hashlib.sha256(data).hexdigest()}
  with lock:
   with (p/'live-http-trace.jsonl').open('a') as f:f.write(json.dumps(record)+'\n')
  self.send_response(status)
  for k,v in outheaders.items():
   if k.lower() not in ['content-length','transfer-encoding','connection','content-encoding']:self.send_header(k,v)
  self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
 do_GET=forward
 do_POST=forward
 do_PUT=forward
 do_PATCH=forward
 do_DELETE=forward
 do_OPTIONS=forward
srv=ThreadingHTTPServer(('127.0.0.1',0),Handler)
(p/'trace-proxy.json').write_text(json.dumps({'pid':os.getpid(),'port':srv.server_address[1],'upstream':'http://localhost:18251'})+'\n')
print('Owned transparent tracing proxy ready',flush=True)
srv.serve_forever()
