from pathlib import Path
import subprocess, tempfile

integration=Path('/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108')
root=Path('/Volumes/artisan/code/2026/multica-0.4.37')
sources={
 'handoff-launch':(Path('/Volumes/artisan/code/2026/mc-handoff-launch-20261031'),'4d0d1a76ddcbbf2ab985550d694a38263aaff25f'),
 'handoff-cursor':(Path('/Volumes/artisan/code/2026/mc-handoff-cursor-20261101'),'e3620721efacb820b89709c8db9d326d9cf2b00b'),
 'recording-recovery':(Path('/Volumes/artisan/code/2026/mc-runtime-recording-recovery-20261101'),'e3620721efacb820b89709c8db9d326d9cf2b00b'),
}
def run(a,cwd): return subprocess.run(a,cwd=cwd,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
def status(wt):
 out=run(['git','status','--porcelain=v1'],wt).stdout.decode().splitlines(); ret={}
 for l in out:
  if len(l)<4: continue
  p=l[3:]
  if ' -> ' in p: p=p.split(' -> ')[-1]
  ret[p]=l[:2]
 return ret
def read(wt,p):
 fp=wt/p
 return fp.read_bytes() if fp.exists() and fp.is_file() else None
def blob(c,p):
 r=run(['git','show',f'{c}:{p}'],root)
 return r.stdout if r.returncode==0 else None
def allowed(p):
 if not (p.startswith('server/') or p.startswith('packages/') or p.startswith('e2e/') or p.startswith('scripts/')): return False
 if p.startswith('server/internal/service/builtin_skills/'): return False
 if p.endswith(('.md','.mdx','.json','.jsonl','.log','.png','.txt')): return False
 return True
def merge(o,b,t):
 if o==t: return o,False,'identical'
 if o==b: return t,False,'source-only'
 if t==b: return o,False,'current-only'
 if any(b'\x00' in (x or b'') for x in (o,b,t)): return None,True,'binary-conflict'
 with tempfile.TemporaryDirectory(prefix='runtime-merge-') as td:
  d=Path(td); (d/'o').write_bytes(o or b''); (d/'b').write_bytes(b or b''); (d/'t').write_bytes(t or b'')
  r=run(['git','merge-file','-p','--diff3',str(d/'o'),str(d/'b'),str(d/'t')],integration)
  if r.returncode==0: return r.stdout,False,'merged'
  return None,True,f'conflict-rc-{r.returncode}'
st={n:status(w) for n,(w,_) in sources.items()}
paths=sorted(p for p in set().union(*(x.keys() for x in st.values())) if allowed(p))
ledger=[]; conflicts=[]; changed=[]
for p in paths:
 xs=[]
 for n,(w,c) in sources.items():
  if p in st[n]: xs.append((n,c,read(w,p),st[n][p]))
 if not xs: continue
 cr=[x for x in xs if x[0] in ('handoff-cursor','recording-recovery')]
 launch=[x for x in xs if x[0]=='handoff-launch']
 source_conflict=None
 if len(cr)==2:
  n1,c1,v1,_=cr[0]; n2,c2,v2,_=cr[1]; base=blob(c1,p)
  candidate,source_conflict,dec=merge(v1,base,v2)
  if source_conflict:
   # recording-recovery is retained as the newer/feature-complete candidate for review,
   # but its conflict is never written into the integration tree.
   candidate=v2; candidate_base=c2; labels=f'{n1}+{n2}'; source_dec='source-conflict-recording-preferred'
  else:
   candidate_base=c1; labels=f'{n1}+{n2}'; source_dec=dec
 elif cr:
  n,c,v,_=cr[0]; candidate=v; candidate_base=c; labels=n; source_dec='single-source'
 elif launch:
  n,c,v,_=launch[0]; candidate=v; candidate_base=c; labels=n; source_dec='launch-only'
 else: continue
 if launch and cr:
  lv=launch[0][2]
  if lv!=candidate: ledger.append((p,'handoff-launch','stale-overlap-skipped','common-base source selected'))
 current=read(integration,p); base=blob(candidate_base,p)
 result,conf,dec=merge(current,base,candidate)
 if source_conflict:
  conflicts.append((p,labels,source_dec))
 if conf:
  conflicts.append((p,labels,dec)); ledger.append((p,labels,source_dec,'kept-current-conflict'))
  continue
 fp=integration/p
 if result is None or result==b'':
  if candidate in (None,b'') and fp.exists(): fp.unlink()
 elif result!=current:
  fp.parent.mkdir(parents=True,exist_ok=True); fp.write_bytes(result); changed.append(p)
 ledger.append((p,labels,source_dec,dec))
ledger_path=root/'.trellis/tasks/11-08-merge-unmerged-worktrees/research/runtime-handoff-conflicts-20261108.md'
with ledger_path.open('w') as f:
 f.write('# Runtime handoff merge ledger — 2026-11-08\n\nBase integration HEAD: `e541cfb43`. Source worktrees were read only.\n\n')
 f.write('## Decisions\n\n| Path | Source snapshot | Source decision | Result |\n| --- | --- | --- | --- |\n')
 for p,s,d,r in ledger: f.write(f'| `{p}` | {s} | {d} | {r} |\n')
 f.write('\n## Conflicts requiring review\n\n')
 if conflicts:
  for p,s,d in sorted(set(conflicts)): f.write(f'- `{p}` from `{s}`: {d}\n')
 else: f.write('None.\n')
 f.write(f'\nChanged paths applied: {len(changed)}\nConflict paths kept at current HEAD: {len(set(p for p,_,_ in conflicts))}\n')
print('paths',len(paths),'changed',len(changed),'conflict_paths',len(set(p for p,_,_ in conflicts)))
for c in sorted(set(conflicts))[:100]: print('CONFLICT',*c,sep=' | ')
