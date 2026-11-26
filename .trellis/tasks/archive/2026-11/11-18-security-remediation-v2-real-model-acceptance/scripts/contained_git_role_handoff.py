#!/usr/bin/env python3
"""Run every model-owned Git read/transfer inside a no-network role container.

The existing bundle verifier runs unchanged inside this container, including its
staging clone and all Git subprocesses. Host code never executes Git against a
model-owned configuration, include, filter, fsmonitor or hook path.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import uuid

IMAGE='sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f'
LABEL='multica.task=secrem-v2-cli-20261120'
HERE=Path(__file__).resolve().parent
INSPECT_COMMANDS={'rev-parse','status','diff','log','show','ls-tree','cat-file','rev-list','merge-base','bundle'}


class Refused(ValueError):pass


def canonical(path):
    value=Path(path).absolute()
    if value.resolve(strict=True)!=value or not value.is_dir():raise Refused('exact directory required')
    return value


def role_root(path):
    repo=canonical(path);git=repo/'.git'
    if git.is_symlink() or not git.is_dir() or (git/'commondir').exists() or (git/'objects/info/alternates').exists():raise Refused('owned full role clone required')
    return repo


def safe_object(value):
    if not re.fullmatch('[a-f0-9]{40,64}',value):raise Refused('full object hash required')
    return value


def run_container(repo,arguments,mounts=(),writable=False,timeout=90):
    repo=role_root(repo);name='secrem-v2-contained-git-'+uuid.uuid4().hex[:10]
    args=['docker','run','--rm','--pull=never','--name',name,'--label',LABEL,'--network=none','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',f'--user={os.getuid()}:{os.getgid()}','--pids-limit=64','--memory=256m','--tmpfs=/tmp:rw,nosuid,size=128m',f'--tmpfs=/home/node:rw,nosuid,size=16m,uid={os.getuid()},gid={os.getgid()}','-e','HOME=/home/node','-e','GIT_CONFIG_NOSYSTEM=1','-e','GIT_CONFIG_GLOBAL=/dev/null','-e','GIT_OPTIONAL_LOCKS=0','--workdir=/repo','--mount',f'type=bind,src={repo},dst=/repo'+('' if writable else ',readonly')]
    for source,target,read_only in mounts:args+=['--mount',f'type=bind,src={source},dst={target}'+(',readonly' if read_only else '')]
    args+=['--entrypoint',arguments[0],IMAGE,*arguments[1:]]
    try:
        process=subprocess.run(args,capture_output=True,timeout=timeout)
    except subprocess.TimeoutExpired:
        subprocess.run(['docker','rm','-f',name],capture_output=True,timeout=15)
        raise Refused('contained Git operation timed out') from None
    return process


def inspect(repo,args):
    if not args or args[0] not in INSPECT_COMMANDS or any('\0' in a for a in args):raise Refused('read-only Git inspection command rejected')
    # These optional actions are writes despite their parent command names.
    if args[0]=='bundle' and args[1:2] not in [['verify'],['list-heads']]:raise Refused('bundle inspection must be read-only')
    return run_container(repo,['git','-c','core.hooksPath=/dev/null','--no-pager',*args])


def export(repo,base,head,output,actor,task,role,allowed,protected):
    repo=role_root(repo);safe_object(base);safe_object(head)
    output=Path(output).absolute()
    if output.exists() or output.parent.resolve()!=output.parent or not output.parent.is_dir():raise Refused('fresh handoff path required')
    # A private temporary container-visible parent lets the canonical exporter
    # create its own fresh handoff directory without exposing neighbouring data.
    output.mkdir(mode=0o700)
    with tempfile.TemporaryDirectory(prefix='secrem-git-input-',dir=output.parent) as temp:
        inputs=Path(temp).resolve();p=inputs/'protected.json';p.write_text(json.dumps(protected));p.chmod(0o644)
        args=['python3','/tools/git_role_handoff.py','export','--repo','/repo','--base',base,'--head',head,'--output','/export/handoff','--actor-id',actor,'--task-id',task,'--role',role,'--protected-json','/inputs/protected.json']
        for path in allowed:args+=['--allow',path]
        process=run_container(repo,args,[(HERE/'git_role_handoff.py','/tools/git_role_handoff.py',True),(output,'/export',False),(inputs,'/inputs',True)])
    if process.returncode:return process
    # Files are consumed by later container operations, never by host Git.
    handoff = output/'handoff'
    if handoff.is_symlink() or not handoff.is_dir() or handoff.resolve() != handoff:
        raise Refused('handoff output directory aliases host data')
    for name in ['manifest.json','handoff.bundle']:
        source=handoff/name
        if source.is_symlink() or not source.is_file():raise Refused('handoff output not regular')
        source.replace(output/name)
    (output/'handoff').rmdir()
    return process


def import_bundle(repo,handoff,expected):
    repo=role_root(repo);handoff=canonical(handoff)
    if not re.fullmatch('[a-f0-9]{64}',expected):raise Refused('manifest hash required')
    for name in ['manifest.json','handoff.bundle']:
        p=handoff/name
        if p.is_symlink() or not p.is_file():raise Refused('handoff file is not regular')
    return run_container(repo,['python3','/tools/git_role_handoff.py','import','--repo','/repo','--handoff','/handoff','--manifest-sha256',expected],[(HERE/'git_role_handoff.py','/tools/git_role_handoff.py',True),(handoff,'/handoff',True)],writable=True)


def main():
    parser=argparse.ArgumentParser();sub=parser.add_subparsers(dest='operation',required=True)
    query=sub.add_parser('inspect');query.add_argument('--repo',required=True);query.add_argument('git_args',nargs=argparse.REMAINDER)
    out=sub.add_parser('export');out.add_argument('--repo',required=True);out.add_argument('--base',required=True);out.add_argument('--head',required=True);out.add_argument('--output',required=True);out.add_argument('--actor-id',required=True);out.add_argument('--task-id',required=True);out.add_argument('--role',required=True);out.add_argument('--allow',action='append',required=True);out.add_argument('--protected-json',required=True)
    into=sub.add_parser('import');into.add_argument('--repo',required=True);into.add_argument('--handoff',required=True);into.add_argument('--manifest-sha256',required=True)
    args=parser.parse_args()
    if args.operation=='inspect':
        argv=args.git_args[1:] if args.git_args[:1]==['--'] else args.git_args
        result=inspect(args.repo,argv)
    elif args.operation=='export':result=export(args.repo,args.base,args.head,args.output,args.actor_id,args.task_id,args.role,args.allow,json.loads(Path(args.protected_json).read_text()))
    else:result=import_bundle(args.repo,args.handoff,args.manifest_sha256)
    sys.stdout.buffer.write(result.stdout);sys.stderr.buffer.write(result.stderr)
    return result.returncode

if __name__=='__main__':
    try:raise SystemExit(main())
    except (OSError,ValueError) as error:
        print('Contained role Git refused: '+type(error).__name__,file=sys.stderr);raise SystemExit(125)
