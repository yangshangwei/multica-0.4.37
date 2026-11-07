"""Start the new uncontrolled-green production window through canonical up."""
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess


HERE = Path(__file__).resolve().parent
prepared = json.loads((HERE / 'preparation.json').read_text())
runtime = Path(prepared['runtime'])
private = Path(prepared['private'])
environment = json.loads(Path(prepared['startup_environment_path']).read_text())
assert hashlib.sha256(Path(prepared['env_path']).read_bytes()).hexdigest() == prepared['env_sha256']
assert hashlib.sha256((runtime / 'e2e/fixtures/triage-duplicate-picker.ts').read_bytes()).hexdigest() == '0cbc13c518022a2af413da1f09b3801e8b0f4a6c7264bbf395544a301a69c029'
command = ['bash', str(runtime / 'scripts/dev-env.sh'), 'up', '--name', prepared['name'],
           '--components', 'api,web', '--web-mode', 'production', '--ephemeral']
record = {'started_at': datetime.now(timezone.utc).isoformat(), 'command': command, 'cwd': str(runtime),
          'source_env_sha256': prepared['env_sha256'], 'expected_database': prepared['database'],
          'scope': 'uncontrolled green only, new production API/Web; no model or extra tests'}
log = HERE / 'startup.log'
with log.open('x') as stream:
    result = subprocess.run(command, cwd=runtime, env=environment, stdout=stream, stderr=subprocess.STDOUT)
record.update(finished_at=datetime.now(timezone.utc).isoformat(), exit=result.returncode,
              log_sha256=hashlib.sha256(log.read_bytes()).hexdigest())
with (HERE / 'startup-result.json').open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
print(json.dumps(record))
raise SystemExit(result.returncode)
