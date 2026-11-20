#!/usr/bin/env python3
"""Transfer actual role commits as verified Git bundles; never author a patch.

The coordinator supplies identities from platform records. Git author strings
are retained evidence, never the authority used to infer who ran the task.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import tempfile


class Refused(ValueError):pass


def digest(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def git(repo,*args,input=None):
    environment={k:v for k,v in os.environ.items() if not k.startswith('GIT_')}
    environment['GIT_CONFIG_NOSYSTEM']='1';environment['GIT_CONFIG_GLOBAL']='/dev/null'
    result=subprocess.run(['git','-c','core.hooksPath=/dev/null','-c','protocol.file.allow=always','-C',str(repo),*args],input=input,env=environment,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    if result.returncode:raise Refused('Git operation refused: '+args[0])
    return result.stdout


def repository(path):
    path=Path(path).absolute()
    if path.resolve()!=path or not path.is_dir():raise Refused('repository path rejected')
    top=Path(git(path,'rev-parse','--show-toplevel').decode().strip())
    if top!=path:raise Refused('repository root required')
    if (path/'.git').is_symlink():raise Refused('symlink Git directory refused')
    if git(path,'status','--porcelain=v1','--untracked-files=all').strip():raise Refused('handoff requires a clean committed role tree')
    return path


def object_id(repo,value):
    if not isinstance(value,str) or not re.fullmatch('[0-9a-f]{40,64}',value):raise Refused('full object ID required')
    actual=git(repo,'rev-parse','--verify',value+'^{commit}').decode().strip()
    if actual!=value:raise Refused('commit identity mismatch')
    return actual


def relative(path):
    pure=PurePosixPath(path)
    if pure.is_absolute() or '..' in pure.parts or not pure.parts or pure.parts[0]=='.git' or '\\' in path or '\0' in path:raise Refused('unsafe repository path')
    return str(pure)


def blob(repo,commit,path):
    path=relative(path)
    entry=git(repo,'ls-tree','-z',commit,'--',path).split(b'\0')[0]
    if not entry:raise Refused('protected file missing')
    modekind,sep,entrypath=entry.partition(b'\t')
    mode,kind,oid=modekind.split()
    if mode not in {b'100644',b'100755'} or kind!=b'blob' or entrypath.decode()!=path:raise Refused('protected file is not a regular blob')
    data=git(repo,'cat-file','blob',oid.decode())
    return {'mode':mode.decode(),'git_blob':oid.decode(),'sha256':hashlib.sha256(data).hexdigest()}


def export_bundle(repo,base,head,output,actor_id,task_id,role,allowed_paths,protected):
    repo=repository(repo);base=object_id(repo,base);head=object_id(repo,head)
    if git(repo,'rev-parse','HEAD').decode().strip()!=head:raise Refused('export must bind actual role HEAD')
    git(repo,'merge-base','--is-ancestor',base,head)
    if base==head:raise Refused('no new role commit')
    if git(repo,'rev-list','--merges',base+'..'+head).strip():raise Refused('merge commits need separate handoff review')
    paths=[p.decode() for p in git(repo,'diff','--name-only','-z',base,head).split(b'\0') if p]
    allowed=[relative(p) for p in allowed_paths]
    for path in paths:
        relative(path)
        if not any(path==a or path.startswith(a.rstrip('/')+'/') for a in allowed):raise Refused('commit changed a path outside role ownership')
    for path,expected in protected.items():
        if blob(repo,head,path)['sha256']!=expected:raise Refused('protected verifier test changed')
    # No submodule/symlink can smuggle a filesystem edge into the receiver.
    for raw in git(repo,'ls-tree','-r','-z',head).split(b'\0'):
        if not raw:continue
        mode=raw.split(b' ',1)[0]
        if mode not in {b'100644',b'100755'}:raise Refused('handoff tree contains nonregular file mode')
    output=Path(output).absolute()
    if output.exists() or output.resolve()!=output:raise Refused('output must be a fresh absolute path')
    output.mkdir(parents=True,mode=0o700)
    bundle=output/'handoff.bundle'
    # HEAD retains the exact source commits; no cherry-pick/rewritten author.
    git(repo,'bundle','create',str(bundle),'HEAD','^'+base)
    records=git(repo,'log','--format=%H%x00%P%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00%s','--reverse',base+'..'+head).decode().splitlines()
    manifest={'schema_version':1,'evidence_type':'actual-git-artifact-transfer','actor_id':actor_id,'task_id':task_id,'role':role,'base_commit':base,'head_commit':head,'tree':git(repo,'rev-parse',head+'^{tree}').decode().strip(),'bundle_sha256':digest(bundle),'changed_paths':paths,'allowed_paths':allowed,'protected':protected,'commit_records':[row.split('\0') for row in records]}
    (output/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    return manifest


def validate_manifest(manifest):
    required={'schema_version','evidence_type','actor_id','task_id','role','base_commit','head_commit','tree','bundle_sha256','changed_paths','allowed_paths','protected','commit_records'}
    if not isinstance(manifest,dict) or set(manifest)!=required or manifest['schema_version']!=1:
        raise Refused('handoff manifest schema rejected')
    for key in ['base_commit','head_commit','tree']:
        if not isinstance(manifest[key],str) or not re.fullmatch('[0-9a-f]{40,64}',manifest[key]):raise Refused('manifest object ID invalid')
    if not re.fullmatch('[0-9a-f]{64}',manifest['bundle_sha256']):raise Refused('manifest bundle digest invalid')
    for key in ['actor_id','task_id','role']:
        if not isinstance(manifest[key],str) or not manifest[key] or len(manifest[key])>256:raise Refused('manifest actor binding invalid')
    if not isinstance(manifest['allowed_paths'],list) or not manifest['allowed_paths']:raise Refused('role paths missing')
    for p in manifest['allowed_paths']+manifest['changed_paths']:relative(p)
    if not isinstance(manifest['protected'],dict):raise Refused('protected map required')
    for path,value in manifest['protected'].items():
        relative(path)
        if not re.fullmatch('[0-9a-f]{64}',value):raise Refused('protected digest invalid')


def import_bundle(destination,handoff,expected_manifest_sha256):
    destination=repository(destination);handoff=Path(handoff).resolve(strict=True);manifestfile=handoff/'manifest.json'
    if digest(manifestfile)!=expected_manifest_sha256:raise Refused('handoff manifest changed')
    manifest=json.loads(manifestfile.read_text());validate_manifest(manifest);bundle=handoff/'handoff.bundle'
    if digest(bundle)!=manifest['bundle_sha256']:raise Refused('handoff bundle changed')
    before=git(destination,'rev-parse','HEAD').decode().strip()
    if before!=manifest['base_commit']:raise Refused('receiver is not on the reviewed baseline')
    git(destination,'bundle','verify',str(bundle))
    # Validate in an isolated staging clone before changing receiver HEAD.
    with tempfile.TemporaryDirectory(prefix='secrem-handoff-') as temporary:
        stage=Path(temporary)/'repo'
        subprocess.run(['git','-c','core.hooksPath=/dev/null','clone','--no-hardlinks','--no-checkout',str(destination),str(stage)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        git(stage,'fetch','--no-tags',str(bundle),'HEAD')
        candidate=git(stage,'rev-parse','FETCH_HEAD').decode().strip()
        if candidate!=manifest['head_commit'] or git(stage,'rev-parse',candidate+'^{tree}').decode().strip()!=manifest['tree']:raise Refused('bundle commit/tree differs')
        git(stage,'merge-base','--is-ancestor',before,candidate)
        actual_paths=[p.decode() for p in git(stage,'diff','--name-only','-z',before,candidate).split(b'\0') if p]
        if actual_paths!=manifest['changed_paths']:raise Refused('changed path inventory differs')
        for path in actual_paths:
            if not any(path==a or path.startswith(a.rstrip('/')+'/') for a in manifest['allowed_paths']):raise Refused('bundle changes unowned path')
        for raw in git(stage,'ls-tree','-r','-z',candidate).split(b'\0'):
            if raw and raw.split(b' ',1)[0] not in {b'100644',b'100755'}:raise Refused('bundle contains filesystem edge')
        for path,expected in manifest['protected'].items():
            if blob(stage,candidate,path)['sha256']!=expected:raise Refused('protected artifact changed')
    git(destination,'fetch','--no-tags',str(bundle),'HEAD')
    git(destination,'merge','--ff-only',manifest['head_commit'])
    after=git(destination,'rev-parse','HEAD').decode().strip()
    if after!=manifest['head_commit']:raise Refused('receiver did not reach exact commit')
    return {'base_commit':before,'head_commit':after,'tree':manifest['tree'],'manifest_sha256':expected_manifest_sha256,'bundle_sha256':manifest['bundle_sha256'],'actor_id':manifest['actor_id'],'task_id':manifest['task_id'],'author_records_preserved':True}


def main():
    parser=argparse.ArgumentParser();sub=parser.add_subparsers(dest='operation',required=True)
    export=sub.add_parser('export');export.add_argument('--repo',required=True);export.add_argument('--base',required=True);export.add_argument('--head',required=True);export.add_argument('--output',required=True);export.add_argument('--actor-id',required=True);export.add_argument('--task-id',required=True);export.add_argument('--role',required=True);export.add_argument('--allow',action='append',required=True);export.add_argument('--protected-json')
    imp=sub.add_parser('import');imp.add_argument('--repo',required=True);imp.add_argument('--handoff',required=True);imp.add_argument('--manifest-sha256',required=True)
    args=parser.parse_args()
    if args.operation=='export':result=export_bundle(args.repo,args.base,args.head,args.output,args.actor_id,args.task_id,args.role,args.allow,json.loads(Path(args.protected_json).read_text()) if args.protected_json else {})
    else:result=import_bundle(args.repo,args.handoff,args.manifest_sha256)
    print(json.dumps(result,indent=2))

if __name__=='__main__':main()
