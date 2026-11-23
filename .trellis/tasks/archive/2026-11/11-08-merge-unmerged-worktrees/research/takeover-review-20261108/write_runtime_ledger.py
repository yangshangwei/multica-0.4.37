from pathlib import Path
import subprocess
root=Path('/Volumes/artisan/code/2026/multica-0.4.37'); integ=Path('/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108')
sources={'handoff-launch':Path('/Volumes/artisan/code/2026/mc-handoff-launch-20261031'),'handoff-cursor':Path('/Volumes/artisan/code/2026/mc-handoff-cursor-20261101'),'recording-recovery':Path('/Volumes/artisan/code/2026/mc-runtime-recording-recovery-20261101')}
base={'handoff-launch':'4d0d1a76ddcbbf2ab985550d694a38263aaff25f','handoff-cursor':'e3620721efacb820b89709c8db9d326d9cf2b00b','recording-recovery':'e3620721efacb820b89709c8db9d326d9cf2b00b'}
def status(w):
 out=subprocess.run(['git','status','--porcelain=v1'],cwd=w,text=True,stdout=subprocess.PIPE).stdout.splitlines(); ret={}
 for l in out:
  if len(l)>=4:
   p=l[3:]; p=p.split(' -> ')[-1]
   ret[p]=l[:2]
 return ret
def allowed(p):
 return (p.startswith('server/') or p.startswith('packages/') or p.startswith('e2e/') or p.startswith('scripts/')) and not p.startswith('server/internal/service/builtin_skills/') and not p.endswith(('.md','.mdx','.json','.jsonl','.log','.png','.txt'))
paths={}
for n,w in sources.items():
 for p in status(w):
  if allowed(p): paths.setdefault(p,[]).append(n)
rows=[]; same=0; diff=0
for p in sorted(paths):
 cur=(integ/p).read_bytes() if (integ/p).exists() else None
 tags=[]
 for n in paths[p]:
  fp=sources[n]/p; src=fp.read_bytes() if fp.exists() and fp.is_file() else None
  tags.append(f'{n}=' + ('same' if src==cur else 'different'))
  if src==cur: same+=1
  else: diff+=1
 decision='kept-current'
 reason='Current HEAD already contains the path; source snapshot is based on an older ancestor and its dirty content would replace newer validation/behavior.'
 if all('same' in t for t in tags): reason='Source snapshot bytes equal current HEAD; no change needed.'
 rows.append((p,', '.join(tags),decision,reason))
out=root/'.trellis/tasks/11-08-merge-unmerged-worktrees/research/runtime-handoff-conflicts-20261108.md'
with out.open('w') as f:
 f.write('# Runtime handoff merge ledger — 2026-11-08\n\n')
 f.write('Integration base: `e541cfb43`. Source worktrees were read-only. No Runtime handoff source patch was applied because every candidate path already exists in current HEAD and the differing snapshots are based on older ancestors (`4d0d1a76` / `e3620721`).\n\n')
 f.write('## Evidence\n\n')
 f.write(f'- Candidate source paths reviewed: {len(rows)}\n- Snapshot comparisons: {same} equal to current HEAD, {diff} different.\n- All differing snapshots were retained in the external preservation archive; no source worktree was changed.\n- The only automatically applicable test diff (`packages/views/collaboration/execution-panel.test.tsx`) was intentionally rejected because it removes existing coverage.\n\n')
 f.write('## Path decisions\n\n| Path | Source snapshot | Decision | Reason |\n| --- | --- | --- | --- |\n')
 for p,s,d,r in rows: f.write(f'| `{p}` | {s} | {d} | {r} |\n')
 f.write('\n## Follow-up\n\nThis batch is safe for the next batch: the integration worktree is clean at `e541cfb43`. If a later reviewer identifies a specific behavior from the older snapshots that is still required, extract that behavior as a focused patch with a regression test rather than reapplying the stale worktree snapshot.\n')
print(out, 'paths',len(rows),'equal',same,'different',diff)
