#!/usr/bin/env python3
"""Capture credential-free process identity for this one quiet verification pair."""
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess

root = Path(__file__).parent
at = datetime.now(timezone.utc)
rows = subprocess.check_output(['ps', '-axo', 'pid,ppid,etime,comm'], text=True)
processes = []
for row in rows.splitlines()[1:]:
    fields = row.strip().split(None, 3)
    if len(fields) != 4:
        continue
    pid, parent, elapsed, command = fields
    name = Path(command).name
    if (name in {'go', 'make', 'node', 'pnpm', 'claude', 'multica', 'server', 'Electron'}
            or name.endswith('.test') or 'ms-playwright' in command
            or 'go-build' in command or 'esbuild' in command or 'turbo' in command):
        processes.append({'pid': int(pid), 'ppid': int(parent), 'elapsed': elapsed,
                          'executable': command})
path = root / ('processes-' + at.strftime('%Y%m%dT%H%M%S%fZ') + '.json')
data = {'at': at.isoformat(), 'processes': processes,
        'scope': 'Executable names and process ancestry only; no argv or credentials.',
        'limitation': 'A snapshot cannot prove no process appeared between observations; coordinator release remains required.'}
with path.open('x') as output:
    json.dump(data, output, indent=2)
    output.write('\n')
print(json.dumps({'snapshot': str(path), 'processes': len(processes)}))
