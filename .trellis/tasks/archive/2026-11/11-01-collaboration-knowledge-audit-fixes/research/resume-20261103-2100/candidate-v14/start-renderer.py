from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import os
import socket
import subprocess
import time
import urllib.request

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v14'
PYTHON = '/opt/homebrew/opt/python@3.14/bin/python3.14'

def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

allocation = json.loads((HERE / 'resource-allocation-redis.json').read_text())
install = json.loads((HERE.parent / 'desktop-v14-install.json').read_text())
ready = json.loads((HERE / 'staged-desktop-ready.json').read_text())
assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip() == allocation['candidate_sha']
assert not subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).strip()
renderer = ROOT / 'apps/desktop/out/renderer'
for row in ready['desktop']['renderer']['files']:
    assert digest(renderer / row['path']) == row['sha256']
port = allocation['renderer_port']
with socket.socket() as sock:
    sock.bind(('127.0.0.1', port))
home = OUT / 'renderer-home-v14'
home.mkdir(mode=0o700)
env = {'PATH': '/usr/bin:/bin', 'HOME': str(home), 'LC_ALL': 'C', 'PYTHONDONTWRITEBYTECODE': '1'}
cmd = [PYTHON, '-u', '-m', 'http.server', str(port), '--bind', '127.0.0.1', '--directory', str(renderer)]
with (OUT / 'renderer-v14.log').open('x') as log:
    child = subprocess.Popen(cmd, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    record = {'recorded_at': datetime.now(timezone.utc).isoformat(), 'candidate_sha': allocation['candidate_sha'],
              'candidate_root': str(ROOT), 'coordinator': allocation['coordinator'], 'pid': child.pid,
              'process_group': os.getpgid(child.pid), 'command': cmd, 'cwd': str(ROOT), 'url': allocation['renderer_url'],
              'renderer_directory': str(renderer), 'index_sha256': digest(renderer / 'index.html'),
              'install_record_sha256': digest(HERE.parent / 'desktop-v14-install.json'),
              'allocation_sha256': digest(HERE / 'resource-allocation-redis.json'), 'script_sha256': digest(__file__),
              'model_or_database_environment_forwarded': False, 'state': 'starting'}
    with (HERE / 'renderer-starting.json').open('x') as target:
        json.dump(record, target, indent=2)
        target.write('\n')
body = None
for attempt in range(30):
    assert child.poll() is None, 'Owned renderer exited'
    try:
        with urllib.request.urlopen(allocation['renderer_url'], timeout=2) as response:
            body = response.read()
        break
    except OSError:
        time.sleep(0.1)
assert body is not None and hashlib.sha256(body).hexdigest() == record['index_sha256']
record.update(state='serving', http_index_sha256=hashlib.sha256(body).hexdigest(), ownership_verified=True,
              process_observation=subprocess.check_output(['ps', '-p', str(child.pid), '-o', 'pid=,ppid=,pgid=,command='], text=True).strip(),
              cwd_observation=subprocess.check_output(['/usr/sbin/lsof', '-a', '-p', str(child.pid), '-d', 'cwd', '-Fn'], text=True).strip(),
              listener_observation=subprocess.check_output(['/usr/sbin/lsof', '-nP', '-a', '-p', str(child.pid), '-iTCP:' + str(port), '-sTCP:LISTEN'], text=True).strip())
with (HERE / 'renderer-ready.json').open('x') as target:
    json.dump(record, target, indent=2)
    target.write('\n')
print(json.dumps({key: record[key] for key in ['state', 'candidate_sha', 'pid', 'process_group', 'url', 'index_sha256', 'ownership_verified']}))
