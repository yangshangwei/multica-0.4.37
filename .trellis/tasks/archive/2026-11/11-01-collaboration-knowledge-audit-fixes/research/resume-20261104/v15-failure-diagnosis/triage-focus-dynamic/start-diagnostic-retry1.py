"""Start only the recorded task API/production Web through dev-env up."""
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess


HERE = Path(__file__).resolve().parent
record = json.loads((HERE / 'preparation.json').read_text())
runtime = Path(record['diagnostic_checkout'])
private = Path(record['private'])
environment = json.loads((private / 'startup-environment-retry1.json').read_text())
assert hashlib.sha256(Path(record['env_path']).read_bytes()).hexdigest() == record['env_sha256']
assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=runtime, text=True).strip() == record['candidate']
assert not subprocess.check_output(['git', 'status', '--porcelain'], cwd=runtime, text=True).strip()
command = ['bash', str(runtime / 'scripts/dev-env.sh'), 'up', '--name', record['name'],
           '--components', 'api,web', '--web-mode', 'production', '--ephemeral']
metadata = {'started_at': datetime.now(timezone.utc).isoformat(), 'command': command,
            'cwd': str(runtime), 'env_file_path': record['env_path'], 'env_file_sha256': record['env_sha256'],
            'private_registry': environment['MULTICA_DEV_HOME'], 'database_expected': record['database'],
            'knowledge_calls': 0, 'real_provider_allowed': False, 'scope': 'task-only diagnostic startup; no tests or models'}
log = HERE / 'startup-retry1.log'
with log.open('x') as stream:
    process = subprocess.run(command, cwd=runtime, env=environment, stdin=subprocess.DEVNULL,
                             stdout=stream, stderr=subprocess.STDOUT)
metadata.update(finished_at=datetime.now(timezone.utc).isoformat(), exit_code=process.returncode,
                log_path=str(log), log_sha256=hashlib.sha256(log.read_bytes()).hexdigest(),
                runtime_status=subprocess.check_output(['git', 'status', '--porcelain'], cwd=runtime, text=True))
with (HERE / 'startup-retry1-result.json').open('x') as stream:
    json.dump(metadata, stream, indent=2)
    stream.write('\n')
print(json.dumps({key: metadata[key] for key in ('command', 'exit_code', 'database_expected', 'private_registry', 'runtime_status')}))
raise SystemExit(process.returncode)
