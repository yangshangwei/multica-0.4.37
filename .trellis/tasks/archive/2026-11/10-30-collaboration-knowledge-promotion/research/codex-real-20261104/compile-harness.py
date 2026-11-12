#!/usr/bin/env python3
"""Bind a compile-only tagged test binary to unchanged source bytes."""
import argparse,hashlib,json,pathlib,subprocess,sys
p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--record',required=True);p.add_argument('--race',action='store_true');a=p.parse_args()
root=pathlib.Path.cwd();sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def source():
 names=subprocess.check_output(['git','ls-files','-co','--exclude-standard','-z','--','server'],cwd=root).split(b'\0')
 return {name.decode():sha(root/name.decode()) for name in sorted(set(names)) if name and (root/name.decode()).is_file()}
before=source();binary=pathlib.Path(a.binary).resolve();binary.parent.mkdir(parents=True,exist_ok=True)
cmd=['go','-C','server','test','-c','-tags=agentintegration','-o',str(binary)]
if a.race:cmd.append('-race')
cmd.append('./cmd/server');status=subprocess.run(cmd).returncode;after=source()
record={'command':cmd,'exit':status,'source_stable':before==after,'source':before,'binary_sha256':sha(binary) if status==0 else None}
pathlib.Path(a.record).write_text(json.dumps(record,indent=2)+'\n');print(json.dumps({k:v for k,v in record.items() if k!='source'}));sys.exit(status if status else 0 if before==after else 3)
