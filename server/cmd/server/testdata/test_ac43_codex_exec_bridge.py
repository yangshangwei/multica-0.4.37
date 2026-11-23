import json
import os
from pathlib import Path
import select
import signal
import subprocess
import sys
import tempfile
import time
import unittest

BRIDGE = Path(__file__).with_name("ac43-codex-exec-bridge.py")
ACTUAL = "1e19baf0-a779-47d6-8d2c-3a45464f9b99"
FAKE = r'''#!/usr/bin/env python3
import json,os,sys,time
from pathlib import Path
if sys.argv[1:] == ["--version"]:
 print("codex-cli 0.165.0");sys.exit(0)
p=Path(os.environ["FAKE_CAPTURE"])
p.write_text(json.dumps({"argv":sys.argv[1:],"cwd":os.getcwd(),"CODEX_HOME":os.environ["CODEX_HOME"],"prompt":sys.stdin.read(),"pid":os.getpid(),"group":os.getpgrp()}))
def out(value): print(json.dumps(value),flush=True)
out({"type":"thread.started","thread_id":"1e19baf0-a779-47d6-8d2c-3a45464f9b99"})
if os.environ.get("FAKE_MODE")=="warning":
 out({"type":"item.completed","item":{"id":"item_warn","type":"error","message":"Codex is ignoring 1 unrecognized configuration setting. Check for typos or deprecated settings.\n  user (/private/test/config.toml): `disable_response_storage` is ignored."}})
if os.environ.get("FAKE_MODE")=="truncated_warning":
 out({"type":"item.completed","item":{"id":"item_warn","type":"error","message":"Codex is ignoring 1 unrecognized configuration setting. Check for typos or deprecated settings.\n  user (/private/var/folders/task/environment-2/workspaces/session/codex-home…: `disable_response_storage` is ignored."}})
if os.environ.get("FAKE_MODE")=="different_warning":
 out({"type":"item.completed","item":{"id":"item_warn","type":"error","message":"Codex is ignoring 1 unrecognized configuration setting. Check for typos or deprecated settings.\n  user (/private/test/config.toml): `other_setting` is ignored."}})
out({"type":"turn.started"})
mode=os.environ.get("FAKE_MODE", "success")
if mode=="active_warning":
 out({"type":"item.completed","item":{"id":"active_warn","type":"error","message":"Codex is ignoring 1 unrecognized configuration setting. Check for typos or deprecated settings.\n  user (/private/test/config.toml): `disable_response_storage` is ignored."}});sys.exit(0)
if mode=="block":
 time.sleep(60);sys.exit(0)
if mode=="unknown":
 out({"type":"item.completed","item":{"type":"command_execution","id":"command1","command":"echo forbidden"}});sys.exit(0)
if mode=="error":
 out({"type":"error","message":"sk-private-test-value-do-not-leak"});sys.exit(7)
out({"type":"item.completed","item":{"type":"agent_message","id":"item_0","text":"AC43_REAL_OUTPUT"}})
if mode!="missing_terminal":out({"type":"turn.completed","usage":{"input_tokens":13,"cached_input_tokens":3,"output_tokens":5}})
if mode=="nonzero":sys.exit(7)
if mode=="slow_exit":time.sleep(.3)
'''


class BridgeTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory(prefix="ac43-bridge-test-")
  self.addCleanup(self.temp.cleanup)
  self.root=Path(self.temp.name)
  self.fake=self.root/"fake-codex"
  self.fake.write_text(FAKE);self.fake.chmod(0o700)
  self.evidence=self.root/"evidence";self.evidence.mkdir(mode=0o700)
  self.cwd=self.root/"work";self.cwd.mkdir()
  self.home=self.root/"codex";self.home.mkdir()
  self.capture=self.root/"capture.json"
  self.env={**os.environ,"TEST_CODEX_EXEC_PATH":str(self.fake),"TEST_CODEX_EXEC_EVIDENCE":str(self.evidence),"CODEX_HOME":str(self.home),"FAKE_CAPTURE":str(self.capture)}

 def start(self,mode="success"):
  self.env["FAKE_MODE"]=mode
  self.p=subprocess.Popen([sys.executable,str(BRIDGE),"app-server","--listen","stdio://"],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,env=self.env,start_new_session=True)
  self.addCleanup(self.stop)
  self.send(1,"initialize",{})
  self.assertEqual(self.read()["id"],1)
  self.send(2,"thread/start",{"cwd":str(self.cwd),"model":"test-model"})
  reply=self.read()
  self.handle=reply["result"]["thread"]["id"]
  self.assertTrue(reply["result"]["logical_handle"])
  self.assertTrue(self.handle.startswith("ac43-exec-bridge-"))
  self.assertNotEqual(self.handle,ACTUAL)
  self.assertFalse(self.capture.exists(),"thread setup must not invoke exec")

 def stop(self):
  if self.p.poll() is None:
   os.killpg(self.p.pid,signal.SIGKILL);self.p.wait(timeout=3)
  for stream in (self.p.stdin,self.p.stdout,self.p.stderr):stream.close()

 def send(self,identity,method,params):
  self.p.stdin.write(json.dumps({"id":identity,"method":method,"params":params})+"\n");self.p.stdin.flush()

 def read(self):
  # Test scenarios produce bounded single-line messages and must never hang.
  ready,_,_=select.select([self.p.stdout],[],[],5)
  if not ready:self.fail("bridge response timeout")
  line=self.p.stdout.readline()
  if not line:self.fail("bridge exited early: "+self.p.stderr.read())
  return json.loads(line)

 def run_turn(self,mode):
  self.start(mode)
  before=time.monotonic()
  self.send(3,"turn/start",{"threadId":self.handle,"input":[{"type":"text","text":"exact input\nsecond line","text_elements":[]}]})
  result=[]
  while True:
   event=self.read();result.append(event)
   if event.get("method")=="turn/completed":break
  return result,time.monotonic()-before

 def test_success_uses_actual_output_usage_identity_and_waits(self):
  events,elapsed=self.run_turn("slow_exit")
  self.assertGreaterEqual(elapsed,.3)
  completed=events[-1]["params"]["turn"]
  self.assertEqual(completed["status"],"completed")
  self.assertEqual(completed["usage"],{"input_tokens":13,"cached_input_tokens":3,"output_tokens":5})
  answer=[x for x in events if x.get("method")=="item/completed"][0]["params"]["item"]["text"]
  self.assertEqual(answer,"AC43_REAL_OUTPUT")
  captured=json.loads(self.capture.read_text())
  self.assertEqual(captured["prompt"],"exact input\nsecond line")
  self.assertEqual(Path(captured["cwd"]).resolve(),self.cwd.resolve());self.assertEqual(captured["CODEX_HOME"],str(self.home))
  self.assertEqual(captured["argv"],["exec","--json","--skip-git-repo-check","--sandbox","read-only","--model","test-model","-"])
  self.assertEqual(captured["group"],self.p.pid)
  facts=json.loads(next(self.evidence.glob("*/launch.json")).read_text())
  self.assertEqual(facts["provider_actual_thread_id"],ACTUAL)
  self.assertEqual(facts["adapter_session_handle"],self.handle)
  self.assertEqual(facts["exit_code"],0);self.assertEqual(facts["physical_exec_children"],1)

 def test_known_pre_turn_configuration_warning_is_retained(self):
  events,_=self.run_turn("warning")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"completed")
  facts=json.loads(next(self.evidence.glob("*/launch.json")).read_text())
  self.assertEqual(facts["pre_turn_config_warnings"],1)

 def test_truncated_pre_turn_configuration_path_is_accepted(self):
  events,_=self.run_turn("truncated_warning")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"completed")
  facts=json.loads(next(self.evidence.glob("*/launch.json")).read_text())
  self.assertEqual(facts["pre_turn_config_warnings"],1)

 def test_other_configuration_setting_warning_is_rejected(self):
  events,_=self.run_turn("different_warning")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"failed")

 def test_same_warning_inside_active_turn_is_not_ignored(self):
  events,_=self.run_turn("active_warning")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"failed")

 def test_nonzero_exit_cannot_be_success(self):
  events,_=self.run_turn("nonzero")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"failed")
  facts=json.loads(next(self.evidence.glob("*/launch.json")).read_text())
  self.assertEqual(facts["exit_code"],7)

 def test_missing_terminal_cannot_be_success(self):
  events,_=self.run_turn("missing_terminal")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"failed")

 def test_unmapped_side_effect_event_fails(self):
  events,_=self.run_turn("unknown")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"failed")
  self.assertIn("unsupported exec item",events[-1]["params"]["turn"]["error"]["message"])

 def test_provider_error_detail_is_private(self):
  events,_=self.run_turn("error")
  self.assertEqual(events[-1]["params"]["turn"]["status"],"failed")
  self.assertNotIn("sk-private",json.dumps(events))

 def test_resume_and_unknown_operation_are_refused_without_exec(self):
  self.start()
  for identity,method in ((3,"thread/resume"),(4,"thread/read")):
   self.send(identity,method,{"threadId":ACTUAL})
   self.assertEqual(self.read()["error"]["code"],-32601)
  self.assertFalse(self.capture.exists())

 def test_stop_process_group_reaches_child(self):
  self.start("block")
  self.send(3,"turn/start",{"threadId":self.handle,"input":[{"type":"text","text":"wait"}]})
  self.read();self.read()
  child=json.loads(self.capture.read_text())["pid"]
  os.killpg(self.p.pid,signal.SIGTERM)
  done=self.read()
  self.assertEqual(done["params"]["turn"]["status"],"failed")
  with self.assertRaises(ProcessLookupError):os.kill(child,0)

 def test_version_is_real_executable_version(self):
  result=subprocess.run([sys.executable,str(BRIDGE),"--version"],capture_output=True,text=True,env=self.env,check=True)
  self.assertEqual(result.stdout,"codex-cli 0.165.0\n")
  self.assertFalse(self.capture.exists())


if __name__=="__main__":unittest.main()
