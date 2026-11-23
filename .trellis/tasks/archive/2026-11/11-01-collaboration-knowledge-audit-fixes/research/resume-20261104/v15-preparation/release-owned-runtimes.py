"""Release only the recorded retired task groups; retain their data and logs."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import time


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PROJECT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
HERE = Path(__file__).resolve().parent
OWNER = '01a1a3c9-2459-7de1-8244-55640c5f6143'
EXPECTED = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'
PROOF = PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/progress-recheck-20261103-01a1a1c6-97cddb08/v15-native-capture/live-proof.json'
OLD_RENDERER = PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261103-2100/candidate-v14/renderer-ready.json'


def processes():
    raw = subprocess.check_output(['ps', '-axo', 'pid=,ppid=,pgid=,lstart=,command='], text=True)
    rows = []
    for line in raw.splitlines():
        fields = line.strip().split(None, 8)
        if len(fields) == 9:
            rows.append({'pid': int(fields[0]), 'ppid': int(fields[1]), 'pgid': int(fields[2]),
                         'started': ' '.join(fields[3:8]), 'command': fields[8]})
    return rows


def save(name, record):
    descriptor = os.open(HERE / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as target:
        json.dump(record, target, indent=2)
        target.write('\n')


assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip() == EXPECTED
assert not subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).strip()
proof = json.loads(PROOF.read_text())
renderer = json.loads(OLD_RENDERER.read_text())
assert renderer['pid'] == renderer['process_group'] == 35776
assert renderer['renderer_directory'] == str(ROOT / 'apps/desktop/out/renderer')
assert proof['renderer']['actual_pid'] == 35776
assert '99298' in proof['actual_API_process'] and str(ROOT) in proof['actual_API_process']
assert '99782' in proof['actual_web_process']
before = processes()
index = {row['pid']: row for row in before}
groups = {
    35776: {'parent': 35776, 'cwd': ROOT, 'contains': 'http.server 55132 --bind 127.0.0.1 --directory ' + str(ROOT / 'apps/desktop/out/renderer'),
            'start': 'Tue Nov 3 21:52:21 2026'},
    98904: {'parent': 98904, 'cwd': ROOT / 'server', 'contains': 'go run -ldflags -X main.commit=d0b132fff ./cmd/server',
            'start': 'Wed Nov 4 00:27:46 2026'},
    99782: {'parent': 99782, 'cwd': ROOT, 'contains': '--dir ' + str(ROOT) + ' --filter @multica/web exec next start --port 13740',
            'start': 'Wed Nov 4 00:29:30 2026'},
}
selected = []
for pgid, expected in groups.items():
    row = index.get(expected['parent'])
    if row is None:
        assert not any(item['pgid'] == pgid for item in before), 'Retired parent absent but group still live'
        selected.append({'pgid': pgid, 'state': 'already_exited'})
        continue
    assert row['pgid'] == pgid and row['started'] == expected['start'] and expected['contains'] in row['command'], 'Recorded parent identity changed'
    cwd = subprocess.check_output(['/usr/sbin/lsof', '-a', '-p', str(row['pid']), '-d', 'cwd', '-Fn'], text=True)
    assert '\nn' + str(expected['cwd']) + '\n' in '\n' + cwd, 'Recorded parent checkout differs'
    children = [item for item in before if item['pgid'] == pgid]
    if pgid == 98904:
        assert index[99298]['pgid'] == pgid and index[99298]['ppid'] == 98904
        assert str(ROOT / '.omx/private/v15-native-diagnostic/tmp') in index[99298]['command']
        assert all(item['pid'] in (98904, 99298) for item in children)
    elif pgid == 35776:
        assert len(children) == 1
    else:
        assert all(item['pid'] == 99782 or ('pnpm.cjs' in item['command'] or 'next-server' in item['command'] or '/esbuild' in item['command']) for item in children)
    selected.append({'pgid': pgid, 'state': 'matching_owned_retired_group', 'cwd': cwd, 'processes': children})

record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'owner': OWNER, 'candidate_sha': EXPECTED,
          'live_proof_sha256': hashlib.sha256(PROOF.read_bytes()).hexdigest(),
          'renderer_proof_sha256': hashlib.sha256(OLD_RENDERER.read_bytes()).hexdigest(),
          'groups': selected, 'signals': [], 'data_or_logs_removed': False, 'other_runtime_modified': False}
save('retired-runtime-release-before.json', record)
for group in selected:
    if group['state'] == 'matching_owned_retired_group':
        os.killpg(group['pgid'], signal.SIGTERM)
        record['signals'].append({'pgid': group['pgid'], 'signal': 'SIGTERM'})
deadline = time.monotonic() + 20
while time.monotonic() < deadline:
    remaining = [row for row in processes() if row['pgid'] in groups]
    if not remaining:
        break
    time.sleep(0.2)
record.update(finished_at=datetime.now(timezone.utc).isoformat(), remaining=remaining,
              state='released' if not remaining else 'graceful_release_incomplete')
save('retired-runtime-release-after.json', record)
assert not remaining, 'Graceful release incomplete; no force signal sent'
print(json.dumps({'state': record['state'], 'groups': [item['pgid'] for item in record['signals']], 'data_or_logs_removed': False}))
