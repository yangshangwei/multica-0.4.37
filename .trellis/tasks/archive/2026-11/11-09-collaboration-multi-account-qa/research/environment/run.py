#!/usr/bin/env python3
"""Run one command using this QA allocation's explicit private environment."""
import json, os, sys
from pathlib import Path
repo=next(p for p in Path(__file__).resolve().parents if (p/'artifacts/qa/collaboration-multi-account-20261109/environment/allocation.json').is_file())
a=json.loads((repo/'artifacts/qa/collaboration-multi-account-20261109/environment/allocation.json').read_text())
env=json.loads(Path(a['env_file']).read_text())
args=sys.argv[1:]
if args and args[0]=='--provider':
    provider=args[1];args=args[2:]
    names={'closure':'closure-provider','host':'host-provider','memory':'memory-provider'}
    if provider not in names: raise SystemExit('Unknown provider')
    env['COLLABORATION_HOST_PROVIDER']=str(Path(a['private_root'])/'bin'/names[provider])
if args and args[0]=='--': args=args[1:]
if not args: raise SystemExit('Expected command')
os.chdir(a['snapshot'])
os.execvpe(args[0],args,env)
