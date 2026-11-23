# Execution coordinator: 01a1a3c9-2459-7de1-8244-55640c5f6143; template only, no execution authorization.
from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import os
import subprocess
import sys

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT = ROOT / '.omx/private/final-check-v16'
EVIDENCE = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261104/v16-preparation/candidate-setup')
GO_ROOT = Path('/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64')
GO = GO_ROOT / 'bin/go'
NODE = Path('/Users/artisan/.nvm/versions/node/v22.23.2/bin/node')
EXPECTED = 'eda0d68fb07852b714f3092aa02ca0523d3ad312'
allocation = json.loads((OUT / 'build-temp-allocation-v16.json').read_text())
TEMP = Path(allocation['directory'])
phase = sys.argv[1]
if phase not in ['cli', 'desktop']:
    raise SystemExit('Expected cli or desktop build')

def checksum(path):
    with path.open('rb') as file:
        return hashlib.file_digest(file, 'sha256').hexdigest()

def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT, text=True).strip()

if git('rev-parse', 'HEAD') != EXPECTED or git('status', '--porcelain'):
    raise SystemExit('Build candidate drift')
if (EVIDENCE / f'{phase}-build-v16.json').exists():
    raise SystemExit('Preserve previous build evidence')
environment = {
    'PATH': f'{NODE.parent}:{GO.parent}:{ROOT / "apps/desktop/node_modules/.bin"}:{ROOT / "node_modules/.bin"}:/usr/bin:/bin:/usr/sbin:/sbin',
    'HOME': str(OUT / f'build-home-{phase}'), 'TMPDIR': str(TEMP / f'{phase}-build'),
    'LC_ALL': 'C', 'CI': '1', 'GOCACHE': str(OUT / 'go-build-cache'),
    'GOMODCACHE': '/Users/artisan/go/pkg/mod', 'GOPATH': str(OUT / 'go-path'),
    'GOROOT': str(GO_ROOT), 'GOTOOLCHAIN': 'local', 'GOENV': 'off',
    'GOPROXY': 'off', 'GOSUMDB': 'off', 'GOWORK': 'off', 'GOMAXPROCS': '2',
    'GOFLAGS': '-p=1 -mod=readonly', 'CGO_ENABLED': '0', 'GOTELEMETRY': 'off',
    'GIT_OPTIONAL_LOCKS': '0', 'NEXT_TELEMETRY_DISABLED': '1',
    'COREPACK_ENABLE_NETWORK': '0', 'PNPM_DISABLE_SELF_UPDATE_CHECK': '1',
    'NPM_CONFIG_USERCONFIG': '/dev/null', 'NPM_CONFIG_GLOBALCONFIG': '/dev/null',
    'NPM_CONFIG_CACHE': str(OUT / 'node-cache'),
}
if phase == 'desktop':
    environment['NODE_OPTIONS'] = '--max-old-space-size=4096'
outputs = [OUT, TEMP]
profile = '(version 1)\n(allow default)\n(deny network*)\n(deny file-write*)\n'
profile += '(allow file-write* (literal "/dev/null") (literal "/dev/tty")\n'
profile += ''.join(f'  (subpath {json.dumps(str(path))})\n' for path in outputs) + ')\n'
profile_path = OUT / f'{phase}-build-offline.sb'
profile_path.write_text(profile)
prefix = ['/usr/bin/sandbox-exec', '-f', str(profile_path)]
started = datetime.now(timezone.utc).isoformat()
version_result = subprocess.run(prefix + [str(GO), 'version'], env=environment, capture_output=True, text=True, check=True)
if 'go1.26.6' not in version_result.stdout:
    raise SystemExit('Wrong retained Go toolchain')
metadata = {'candidate_sha': EXPECTED, 'started_at': started, 'phase': phase,
            'launcher_path': __file__, 'launcher_sha256': checksum(Path(__file__)),
            'environment': environment, 'profile_path': str(profile_path), 'profile_sha256': checksum(profile_path),
            'Go_version': version_result.stdout.strip(), 'Go_binary_sha256': checksum(GO),
            'Node_binary_sha256': checksum(NODE), 'network': 'denied by sandbox',
            'write_scope': [str(path) for path in outputs], 'real_agent_model_environment_forwarded': False}
if phase == 'cli':
    destination = OUT / 'multica'
    if destination.exists():
        raise SystemExit('Never overwrite an existing CLI artifact')
    short = git('rev-parse', '--short', 'HEAD')
    try:
        version = git('describe', '--tags', '--match', 'v[0-9]*', '--dirty')
    except subprocess.CalledProcessError:
        version = f'v0.0.0-0-g{short}'
    date = datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
    flags = f'-X main.version={version} -X main.commit={short} -X main.date={date}'
    command = prefix + [str(GO), 'build', '-p=1', '-mod=readonly', '-ldflags', flags, '-o', str(destination), './cmd/multica']
    cwd = ROOT / 'server'
    metadata.update(version=version, commit=short, date=date, ldflags=flags, output=str(destination))
else:
    destination = OUT / 'artifacts-build'
    if destination.exists():
        raise SystemExit('Never overwrite an existing desktop build')
    config = OUT / 'electron.vite.config.cts'
    source_config = ROOT / 'apps/desktop/electron.vite.config.ts'
    if checksum(config) != checksum(source_config):
        raise SystemExit('Private build config differs from tracked source')
    vite_cli = ROOT / 'apps/desktop/node_modules/electron-vite/bin/electron-vite.js'
    command = prefix + [str(NODE), str(vite_cli), 'build', '--config', str(config), '--outDir', str(destination)]
    cwd = ROOT / 'apps/desktop'
    metadata.update(output=str(destination), config_source=str(source_config), config_copy=str(config),
                    config_source_sha256=checksum(source_config), config_copy_sha256=checksum(config),
                    electron_vite_cli=str(vite_cli), electron_vite_cli_sha256=checksum(vite_cli))

metadata.update(command=command, cwd=str(cwd))
print(json.dumps({'phase': phase, 'command': command, 'cwd': str(cwd), 'source_sha': EXPECTED}, indent=2), flush=True)
log_path = EVIDENCE / f'{phase}-build-v16.log'
with log_path.open('x') as log:
    result = subprocess.run(command, cwd=cwd, env=environment, stdout=log, stderr=subprocess.STDOUT)
metadata.update(finished_at=datetime.now(timezone.utc).isoformat(), exit_code=result.returncode,
                log_path=str(log_path), log_sha256=checksum(log_path))
if result.returncode == 0 and phase == 'cli':
    report = subprocess.run(prefix + [str(destination), '--version'], env=environment, cwd=ROOT, capture_output=True, text=True)
    metadata.update(binary_sha256=checksum(destination), binary_bytes=destination.stat().st_size,
                    version_probe_exit=report.returncode, version_probe_stdout=report.stdout, version_probe_stderr=report.stderr)
    if report.returncode or version not in report.stdout or short not in report.stdout:
        raise SystemExit('Current CLI version/commit observation failed')
    module = subprocess.run(prefix + [str(GO), 'version', '-m', str(destination)], env=environment, capture_output=True, text=True)
    metadata.update(binary_module_metadata=module.stdout, binary_module_metadata_exit=module.returncode)
if result.returncode == 0 and phase == 'desktop':
    for name in ['main/index.js', 'preload/index.js', 'renderer/index.html']:
        path = destination / name
        if not path.is_file():
            raise SystemExit('Current desktop output missing: ' + name)
    metadata['required_files'] = {name: {'sha256': checksum(destination / name), 'bytes': (destination / name).stat().st_size}
                                  for name in ['main/index.js', 'preload/index.js', 'renderer/index.html']}
metadata['source_head_after'] = git('rev-parse', 'HEAD')
metadata['source_status_after'] = git('status', '--porcelain')
with (EVIDENCE / f'{phase}-build-v16.json').open('x') as file:
    json.dump(metadata, file, indent=2)
    file.write('\n')
print(json.dumps({key: metadata[key] for key in ['phase', 'exit_code', 'output', 'log_sha256', 'source_status_after']}, indent=2), flush=True)
raise SystemExit(result.returncode)
