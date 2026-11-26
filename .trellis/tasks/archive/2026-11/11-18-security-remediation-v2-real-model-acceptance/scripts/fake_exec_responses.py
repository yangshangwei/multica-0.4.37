#!/usr/bin/env python3
"""Credential-free Responses peer for the actual official exec CLI probe.

This program runs only inside a network=none probe container. It scripts a tool
request and final text, so its result is plumbing evidence, never model evidence.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import threading
import time
import urllib.request
import shlex
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

home = Path(os.environ["CODEX_HOME"])
issue = os.environ.get("SECREM_PROBE_ISSUE", "")
scratch = (Path.cwd() / "drill.json").exists()
# The model-side environment intentionally does not receive task secrets.
request_records = []
lock = threading.Lock()


def event(stream, name, value):
    stream.write(("event: "+name+"\ndata: "+json.dumps({"type":name,**value})+"\n\n").encode())
    stream.flush()


class Handler(BaseHTTPRequestHandler):
    protocol_version="HTTP/1.1"
    def log_message(self,*args):
        pass
    def do_GET(self):
        self.send_response(200);self.send_header("Content-Type","application/json");self.send_header("Connection","close");self.end_headers();self.wfile.write(b'{"data":[]}');self.close_connection=True
    def do_POST(self):
        payload=json.loads(self.rfile.read(int(self.headers.get("Content-Length","0"))))
        with lock:
            request_records.append({"path":self.path,"headers":{k:v for k,v in self.headers.items() if k.lower() not in {"authorization","x-openai-actor-authorization"}},"body":payload})
            index=len(request_records)
            (home/"fake-responses.json").write_text(json.dumps(request_records,indent=2)+"\n")
        self.send_response(200);self.send_header("Content-Type","text/event-stream");self.send_header("Connection","close");self.end_headers();self.close_connection=True
        response_id="resp_synthetic_"+str(index)
        response={"id":response_id,"object":"response","created_at":int(time.time()),"status":"in_progress","model":payload.get("model"),"output":[]}
        event(self.wfile,"response.created",{"response":response})
        protected = ["AGENTS.md", ".agent_context/issue_context.md", ".multica/daemon_task_context.json", "drill.json"] if scratch else ["code.py", "AGENTS.md"]
        file_probe = "import pathlib,os,json;root=pathlib.Path.cwd();home=pathlib.Path(os.environ['CODEX_HOME']);out={}\nfor p in [root/n for n in "+repr(protected)+"]+[home/'config.toml',pathlib.Path('/etc/codex/requirements.toml')]:\n try:p.write_text('MUTATED');out[str(p)]='BYPASS'\n except OSError:out[str(p)]='DENIED'\nprint(json.dumps(out))"
        network = "python3 -c \"import socket\ntry:s=socket.socket();s.settimeout(1);s.connect(('127.0.0.1',39091));print('BYPASS')\nexcept OSError:print('DENIED')\""
        commands=["multica issue get "+issue+" --output json", network, "python3 -c "+shlex.quote(file_probe), "echo ESCALATED_BYPASS"]
        escalation = 4
        if scratch:
            write = "from pathlib import Path;p=Path('.multica/comments/handoff.md');p.parent.mkdir(parents=True,exist_ok=True);p.write_text('合成移交内容\\n',encoding='utf-8');print('SCRATCH_WRITTEN')"
            commands = ["multica issue get "+issue+" --output json", "multica issue comment add --help", "multica squad list --output json", "python3 -c "+shlex.quote(write), "multica issue comment add "+issue+" --content-file .multica/comments/handoff.md --output json", network, "python3 -c "+shlex.quote(file_probe), "echo ESCALATED_BYPASS"]
            escalation = 8
        tool=None
        for value in payload.get("tools",[]):
            if value.get("name") in {"exec_command","shell_command","shell"}:
                tool=value["name"];break
        if index<=len(commands) and tool:
            command=commands[index-1]
            args={"cmd":command} if tool=="exec_command" else ({"command":command} if tool=="shell_command" else {"command":["bash","-lc",command]})
            if index == escalation:
                args["sandbox_permissions"] = "require_escalated"
                args["justification"] = "Synthetic forbidden escalation probe"
            item={"type":"function_call","id":"fc_synthetic_"+str(index),"call_id":"call_synthetic_"+str(index),"name":tool,"arguments":json.dumps(args),"status":"completed"}
            event(self.wfile,"response.output_item.added",{"output_index":0,"item":{**item,"arguments":"","status":"in_progress"}})
            event(self.wfile,"response.function_call_arguments.delta",{"item_id":item["id"],"output_index":0,"delta":item["arguments"]})
            event(self.wfile,"response.function_call_arguments.done",{"item_id":item["id"],"output_index":0,"arguments":item["arguments"]})
            event(self.wfile,"response.output_item.done",{"output_index":0,"item":item})
        else:
            text="Synthetic official CLI transport completed; this is not a model result."
            item={"id":"msg_synthetic_"+str(index),"type":"message","status":"completed","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":text,"annotations":[]}]}
            event(self.wfile,"response.output_item.added",{"output_index":0,"item":{**item,"status":"in_progress","content":[]}})
            event(self.wfile,"response.content_part.added",{"item_id":item["id"],"output_index":0,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}})
            event(self.wfile,"response.output_text.delta",{"item_id":item["id"],"output_index":0,"content_index":0,"delta":text})
            event(self.wfile,"response.output_text.done",{"item_id":item["id"],"output_index":0,"content_index":0,"text":text})
            event(self.wfile,"response.output_item.done",{"output_index":0,"item":item})
        event(self.wfile,"response.completed",{"response":{**response,"status":"completed","output":[item],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}})

server=ThreadingHTTPServer(("127.0.0.1",39091),Handler)
threading.Thread(target=server.serve_forever,daemon=True).start()
with urllib.request.urlopen("http://127.0.0.1:39091/model/models", timeout=2) as response:
    (home / "fake-peer-control.json").write_text(json.dumps({"status": response.status, "body": response.read().decode()})+"\n")
try:
    result=subprocess.run(["codex",*sys.argv[1:]])
finally:
    server.shutdown();server.server_close()
raise SystemExit(result.returncode)
