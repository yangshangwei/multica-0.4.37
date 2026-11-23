from pathlib import Path
import hashlib
import json
import os
import subprocess
import sys
from datetime import datetime, timezone

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103')
PRIVATE = ROOT / '.omx/private/owned-workspace-links'
EVIDENCE = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/owned-workspace-links')
NODE = Path('/Users/artisan/.nvm/versions/node/v22.23.2/bin/node')
PNPM = PRIVATE / 'pnpm-10.28.2/bin/pnpm.cjs'

def checksum(path):
    with path.open('rb') as file:
        return hashlib.file_digest(file, 'sha256').hexdigest()

head = subprocess.run(['git', 'rev-parse', 'HEAD'], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip()
if head != '304f74dd3ba4ddea862c94917c5bb0770355a351':
    raise SystemExit('Candidate drift')
if not PNPM.is_file() or not Path('/usr/bin/sandbox-exec').is_file():
    raise SystemExit('Copied pinned pnpm or offline sandbox unavailable')
if json.loads((PRIVATE / 'pnpm-10.28.2/package.json').read_text())['version'] != '10.28.2':
    raise SystemExit('Copied package manager mismatch')

outputs = [ROOT / 'node_modules', PRIVATE]
for group in ['apps', 'packages']:
    for package in sorted((ROOT / group).iterdir()):
        if (package / 'package.json').is_file():
            outputs.append(package / 'node_modules')
profile = '(version 1)\n(allow default)\n(deny network*)\n(deny file-write*)\n'
profile += '(allow file-write* (literal "/dev/null") (literal "/dev/tty")\n'
profile += ''.join(f'  (subpath {json.dumps(str(path))})\n' for path in outputs) + ')\n'
profile_path = PRIVATE / 'offline-install.sb'
profile_path.write_text(profile)

# HOME retains the existing process value; all mutable pnpm outputs have explicit
# candidate-owned locations. No account/provider or ambient Node preload values
# are forwarded, and npm user/global configuration is selected as /dev/null.
environment = {
    'PATH': f'{NODE.parent}:/usr/bin:/bin:/usr/sbin:/sbin',
    'HOME': os.environ['HOME'],
    'TMPDIR': str(PRIVATE / 'tmp'),
    'PNPM_HOME': str(PRIVATE / 'pnpm-home'),
    'NPM_CONFIG_USERCONFIG': '/dev/null',
    'NPM_CONFIG_GLOBALCONFIG': '/dev/null',
    'COREPACK_ENABLE_NETWORK': '0',
    'PNPM_DISABLE_SELF_UPDATE_CHECK': '1',
    'CI': '1',
    'LC_ALL': 'C',
}
prefix = ['/usr/bin/sandbox-exec', '-f', str(profile_path), str(NODE), str(PNPM)]
version = subprocess.run(prefix + ['--version'], cwd=ROOT, env=environment, capture_output=True, text=True)
if version.returncode or version.stdout.strip() != '10.28.2':
    raise SystemExit(f'Pinned version verification failed: {version.returncode}: {version.stdout}: {version.stderr}')
arguments = prefix + [
    'install', '--offline', '--frozen-lockfile', '--ignore-scripts',
    '--package-import-method=copy', '--side-effects-cache=false',
    '--manage-package-manager-versions=false',
    f'--store-dir={PRIVATE / "pnpm-store"}',
    f'--cache-dir={PRIVATE / "pnpm-cache"}',
    f'--state-dir={PRIVATE / "pnpm-state"}',
    f'--global-dir={PRIVATE / "pnpm-global"}',
    '--reporter=append-only',
]
started = datetime.now(timezone.utc).isoformat()
print(json.dumps({'phase': 'offline-frozen-install-start', 'candidate_sha': head,
                  'pnpm_version': version.stdout.strip(), 'network': 'denied by sandbox',
                  'writes': [str(path) for path in outputs]}, indent=2), flush=True)
log_path = EVIDENCE / 'offline-install.log'
with log_path.open('x') as log:
    result = subprocess.run(arguments, cwd=ROOT, env=environment, stdout=log, stderr=subprocess.STDOUT)
record = {
    'schema_version': 1, 'candidate_sha': head, 'started_at': started,
    'finished_at': datetime.now(timezone.utc).isoformat(), 'command': arguments,
    'exit_code': result.returncode, 'log_path': str(log_path), 'log_sha256': checksum(log_path),
    'environment': environment, 'profile_path': str(profile_path), 'profile_sha256': checksum(profile_path),
    'network': 'sandbox denies network* for pnpm and children; pnpm --offline',
    'file_writes': 'sandbox denies file-write* except candidate node_modules/private artifacts and /dev/null,/dev/tty',
    'lifecycle_scripts': 'disabled', 'side_effects_cache': 'disabled',
    'actual_pnpm_version': version.stdout.strip(), 'copied_pnpm_bin_sha256': checksum(PNPM),
    'copied_pnpm_bundle_sha256': checksum(PRIVATE / 'pnpm-10.28.2/dist/pnpm.cjs'),
    'node_binary_sha256': checksum(NODE), 'node_version': subprocess.run([str(NODE), '--version'],
                             capture_output=True, text=True, check=True).stdout.strip(),
    'tracked_lock_sha256_after': checksum(ROOT / 'pnpm-lock.yaml'),
}
with (EVIDENCE / 'offline-install.json').open('x') as file:
    json.dump(record, file, indent=2)
    file.write('\n')
print(json.dumps({key: record[key] for key in ['exit_code', 'log_path', 'log_sha256', 'actual_pnpm_version',
                  'tracked_lock_sha256_after']}, indent=2), flush=True)
raise SystemExit(result.returncode)
