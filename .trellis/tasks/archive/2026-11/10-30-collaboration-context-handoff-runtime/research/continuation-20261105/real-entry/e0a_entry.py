#!/usr/bin/env python3
"""One E0a invocation through the unchanged launcher, with passive CLI capture."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import threading
import time
from datetime import datetime, timezone
from urllib.parse import urlsplit

sys.dont_write_bytecode = True

ROOT = Path('/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104')
PRIVATE = ROOT / '.omx/private/runtime-real-20261105'
HERE = ROOT / '.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1'
LAUNCHER = HERE / 'run_rk1.py'
GO = Path('/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go')
SOURCE = '6b342ba23c2f5ae6826e8841794d086d1258274d3a39eac84983bfb9f429bdea'
FRAMEWORK = '5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9'
LAUNCHER_HASH = '3bc7962c996143200b1834e9246ec215e8a0d3e03fc38e832abd2eddb1769c99'
STRIP_PREFIX = ('MULTICA_', 'RK1_', 'RK1K_', 'DATABASE_', 'POSTGRES_', 'PG',
                'PLAYWRIGHT_', 'COLLABORATION_', 'E2E_', 'NEXT_PUBLIC_', 'REDIS_')
STRIP_KEYS = {'APP_ENV', 'PORT', 'FRONTEND_PORT', 'FRONTEND_ORIGIN', 'REMOTE_API_URL',
              'CORS_ALLOWED_ORIGINS', 'ALLOWED_ORIGINS', 'ALLOW_SIGNUP',
              'DISABLE_WORKSPACE_CREATION', 'ENV_FILE', 'JWT_SECRET', 'ELECTRON_RUN_AS_NODE'}


def now():
    return datetime.now(timezone.utc).isoformat()


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def sha(path):
    return digest(Path(path).read_bytes())


def obj_hash(value):
    return digest(json.dumps(value, sort_keys=True, separators=(',', ':')).encode())


def save(path, value):
    with Path(path).open('x') as out:
        json.dump(value, out, indent=2)
        out.write('\n'); out.flush(); os.fsync(out.fileno())


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def private_json(path):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and not info.st_mode & 0o077,
            'configuration/permit must be an owned private regular file')
    return json.loads(path.read_text())


def module():
    require(sha(LAUNCHER) == LAUNCHER_HASH, 'launcher changed')
    spec = importlib.util.spec_from_file_location('rk1_existing', LAUNCHER)
    loaded = importlib.util.module_from_spec(spec); spec.loader.exec_module(loaded)
    return loaded


def environment(config, launcher):
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(STRIP_PREFIX) and k not in STRIP_KEYS | set(launcher.SESSION_ONLY_ENV)}
    require(env.get('HOME') == '/Users/artisan', 'operator HOME differs')
    original_path = env.get('PATH', '')
    require(not any(x in original_path for x in ['agent-cli-guard', 'check-bin', 'adapter-v']), 'foreign PATH adapter')
    tmp = Path(config['temp_root']).resolve()
    require(str(tmp).startswith('/private/tmp/') and tmp.is_dir(), 'external private temp root required')
    require(tmp.stat().st_uid == os.getuid() and not tmp.stat().st_mode & 0o077, 'temp ownership/mode')
    probe = subprocess.run(['git', '-C', str(tmp), 'rev-parse', '--show-toplevel'], capture_output=True)
    require(probe.returncode != 0, 'temp is in a git checkout')
    database = config['database']
    require(database.startswith('multica_rk1_') and urlsplit(config['DATABASE_URL']).path == '/' + database,
            'database allocation name differs')
    env.update({'PATH': str(GO.parent) + ':' + original_path, 'GOROOT': str(GO.parent.parent),
                'GOCACHE': '/Volumes/artisan/dev-cache/go-build', 'GOMODCACHE': '/Users/artisan/go/pkg/mod',
                'GOPATH': str(PRIVATE / 'gopath'), 'GOTOOLCHAIN': 'local', 'GOENV': 'off', 'GOWORK': 'off',
                'GOPROXY': 'off', 'GOSUMDB': 'off', 'GOTELEMETRY': 'off', 'GOMAXPROCS': '2', 'GOFLAGS': '-p=2',
                'TMPDIR': str(tmp), 'TMP': str(tmp), 'TEMP': str(tmp), 'GOTMPDIR': str(tmp),
                'DATABASE_URL': config['DATABASE_URL']})
    return env, tmp


def binding(env, config_path, launcher):
    previous = dict(os.environ)
    try:
        os.environ.clear(); os.environ.update(env)
        operator = launcher.operator_facts()
        source = launcher.source_binding()
    finally:
        os.environ.clear(); os.environ.update(previous)
    status = subprocess.check_output(['git', '-C', str(ROOT), 'status', '--porcelain=v1', '--untracked-files=all'], text=True)
    reg = json.loads((HERE / 'framework-registration.json').read_text())
    require(source['source_id'] == SOURCE and not source['dirty_product_paths'], 'product identity differs')
    require(launcher.framework_sha256() == FRAMEWORK, 'framework changed')
    git = Path(shutil.which('git', path=env['PATH'])).resolve()
    require(sha(GO) == 'a1c83801d1756c3eca78366c6b585f2c21c20694fb1c7eb92c446a0580420412', 'Go changed')
    require(operator == reg.get('operator_configuration'), 'operator configuration changed')
    require(sha(reg['provider_executable']) == reg.get('provider_executable_sha256'), 'Provider bytes changed')
    return {'head': source['head'], 'source_id': SOURCE, 'framework_sha256': FRAMEWORK,
            'launcher_sha256': LAUNCHER_HASH, 'helper_sha256': sha(__file__),
            'config_sha256': sha(config_path), 'environment_sha256': obj_hash(env),
            'operator_configuration_sha256': obj_hash(operator), 'git': str(git), 'git_sha256': sha(git),
            'python': str(Path(sys.executable).resolve()), 'python_sha256': sha(sys.executable),
            'go_sha256': sha(GO), 'provider_sha256': sha(reg['provider_executable']),
            'registration_sha256': sha(HERE / 'framework-registration.json'),
            'addendum_sha256': sha(HERE / 'runtime-addendum.json'), 'whole_status': status}


def stable_bytes(path, boundary):
    """Reject symlinks, foreign owners and unstable reads; never execute candidates."""
    relative = path.relative_to(boundary)
    current = boundary
    for part in relative.parts[:-1]:
        current /= part
        info = current.lstat()
        require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.getuid(), 'unsafe capture ancestor')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        before = os.fstat(fd)
        require(stat.S_ISREG(before.st_mode) and before.st_uid == os.getuid() and 0 < before.st_size < (256 << 20),
                'unsafe capture file')
        with os.fdopen(fd, 'rb', closefd=False) as src:
            data = src.read()
        after = os.fstat(fd); visible = path.lstat()
        key = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_mode, s.st_uid)
        require(key(before) == key(after) == key(visible) and len(data) == before.st_size, 'unstable capture')
        return data, {'inode': before.st_ino, 'device': before.st_dev, 'size': len(data),
                      'mtime_ns': before.st_mtime_ns, 'owner': before.st_uid}
    finally:
        os.close(fd)


def observe(tmp, output, stop, observations, errors):
    patterns = ['TestRK1RealProvider*/001/multica',
                'TestRK1RealProvider*/*/host-work/*/runs/*/bin/multica']
    stable = {}; captured_identity = {}
    while not stop.is_set():
        for pattern in patterns:
            for path in tmp.glob(pattern):
                try:
                    info = path.lstat()
                    fingerprint = (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_mode, info.st_uid)
                    if captured_identity.get(str(path)) == fingerprint:
                        continue
                    if stable.get(str(path)) != fingerprint:
                        stable[str(path)] = fingerprint; continue
                    raw, identity = stable_bytes(path, tmp)
                    require((identity['device'], identity['inode'], identity['size'], identity['mtime_ns'], identity['owner'])
                            == (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_uid), 'capture changed since observation')
                    target = output / ('capture-%03d-multica' % len(observations))
                    with target.open('xb') as copied:
                        copied.write(raw); copied.flush(); os.fsync(copied.fileno())
                    observations.append({'path': str(path), 'copy': str(target), 'sha256': digest(raw),
                                         'at': now(), **identity})
                    captured_identity[str(path)] = fingerprint
                except (OSError, ValueError) as exc:
                    item = {'path': str(path), 'error_type': type(exc).__name__}
                    if item not in errors:
                        errors.append(item)
        stop.wait(0.05)


def command(argv, env, prefix):
    with (PRIVATE / (prefix + '.log')).open('xb') as log:
        child = subprocess.Popen(argv, cwd=ROOT / 'server', env=env, stdout=log, stderr=subprocess.STDOUT)
        save(PRIVATE / (prefix + '-process.json'), {'pid': child.pid, 'argv': argv, 'started_at': now()})
        return child.wait()


def build_info(path, env):
    result = subprocess.run([str(GO), 'version', '-m', str(path)], env=env, capture_output=True, text=True)
    require(result.returncode == 0, 'Go build metadata unreadable')
    return result.stdout


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('mode', choices=['describe', 'build', 'check', 'run'])
    parser.add_argument('--config', required=True, type=Path)
    parser.add_argument('--permit', required=True, type=Path)
    args = parser.parse_args(); os.umask(0o077)
    PRIVATE.mkdir(mode=0o700, parents=True, exist_ok=True)
    require(not PRIVATE.is_symlink() and not PRIVATE.stat().st_mode & 0o077, 'private root permissions')
    config, permit = private_json(args.config), private_json(args.permit)
    launcher = module(); env, tmp = environment(config, launcher)
    facts = binding(env, args.config, launcher)
    if args.mode == 'describe':
        save(PRIVATE / 'binding-proposal.json', facts); print('Binding proposal recorded; no execution.'); return 0
    require(permit.get('allow_' + args.mode) is True, 'root permit remains closed')
    require(permit.get('binding') == facts and not facts['whole_status'], 'root binding or clean VCS state differs')
    expires = datetime.fromisoformat(permit['window_expires_at'])
    require(datetime.now(timezone.utc) < expires and permit.get('window_id'), 'window missing or expired')
    if args.mode == 'build':
        save(PRIVATE / 'build-attempt.json', {'at': now(), 'binding': facts})
        target = PRIVATE / 'upfront-multica'
        require(not target.exists(), 'upfront output already exists')
        code = command([str(GO), 'build', '-o', str(target), './cmd/multica'], env, 'build')
        result = {'exit_code': code, 'binding_before': facts, 'binding_after': binding(env, args.config, launcher)}
        if code == 0:
            result.update({'cli_sha256': sha(target), 'size': target.stat().st_size, 'build_info': build_info(target, env)})
        save(PRIVATE / 'build-result.json', result); return code
    build = json.loads((PRIVATE / 'build-result.json').read_text())
    require(build['exit_code'] == 0 and build['binding_before'] == facts == build['binding_after'], 'upfront binding differs')
    require(sha(PRIVATE / 'upfront-multica') == build['cli_sha256'], 'upfront CLI changed')
    require(permit.get('build_result_sha256') == sha(PRIVATE / 'build-result.json'), 'build result lacks root binding')
    argv = [sys.executable, str(LAUNCHER), 'RK1-E0a', '1']
    if args.mode == 'check':
        code = command(argv + ['--check-only'], env, 'check')
        save(PRIVATE / 'check-result.json', {'exit_code': code, 'binding': facts, 'at': now()}); return code
    check = json.loads((PRIVATE / 'check-result.json').read_text())
    require(check['exit_code'] == 0 and check['binding'] == facts, 'check-only missing or stale')
    require(permit.get('check_result_sha256') == sha(PRIVATE / 'check-result.json'), 'check result lacks root binding')
    require(not list((HERE / 'results').glob('RK1-*.json')) and not list((ROOT / '.omx/private/rk1').glob('*/invocation-*.json')),
            'E0a inventory already exists')
    require(not list(tmp.iterdir()), 'new E0a temp directory is not empty')
    save(PRIVATE / 'run-attempt.json', {'at': now(), 'binding': facts, 'permit_sha256': sha(args.permit)})
    captures = PRIVATE / 'captures'; captures.mkdir(mode=0o700)
    observations, errors, stop = [], [], threading.Event()
    watcher = threading.Thread(target=observe, args=(tmp, captures, stop, observations, errors), daemon=True)
    watcher.start()
    try:
        code = command(argv, env, 'run')
    finally:
        stop.set(); watcher.join()
        save(PRIVATE / 'captures.json', {'observations': observations, 'errors': errors})
    invocations = list((ROOT / '.omx/private/rk1/RK1-E0a').glob('invocation-*.json'))
    public = HERE / 'results/RK1-E0a-t1.json'
    result = {'exit_code': code, 'finished_at': now(), 'binding_before': facts,
              'captures': observations, 'observer_errors': errors,
              'invocation_files': [{'path': str(p), 'sha256': sha(p)} for p in invocations],
              'public_sha256': sha(public) if public.exists() else None, 'no_automatic_retry': True}
    try:
        result['binding_after'] = binding(env, args.config, launcher)
        result['source_stable'] = all(result['binding_after'][k] == facts[k] for k in facts if k != 'whole_status')
        allowed = str((HERE / 'results/RK1-E0a-t1.json').relative_to(ROOT))
        result['final_status_allowed'] = all(line[3:] == allowed for line in result['binding_after']['whole_status'].splitlines())
        result['captured_build_info'] = {o['copy']: build_info(Path(o['copy']), env) for o in observations}
        trial = json.loads(public.read_text())
        result['trial'] = {k: trial.get(k) for k in ['label', 'mode', 'status', 'verdict', 'provider_version', 'requested_model', 'observed_models']}
        result['public_measured_starts'] = sum(r['physical_launches'] for r in trial['runs'] if r['measured'])
        result['private_launch_accounting'] = launcher.launch_accounting([trial])
        nested = [o for o in observations if Path(o['path']).parent.name == '001']
        launch_paths = list((ROOT / '.omx/private/rk1/RK1-E0a/RK1-E0a-t1/provider').glob('*/launch-*/launch.json'))
        expected = []
        for path in launch_paths:
            launch = json.loads(path.read_text())
            if launch.get('mode') == 'real':
                run_id = path.parents[1].name
                expected.append(str(Path(launch['cwd']).parent / 'runs' / run_id / 'bin/multica'))
        prepared = {o['path'] for o in observations if Path(o['path']).parent.name == 'bin'}
        result['prepared_expected_paths'] = expected
        result['capture_hashes_match_upfront'] = (len(nested) == 1 and len(expected) == 1 and set(expected) == prepared
                                                and all(o['sha256'] == build['cli_sha256'] for o in observations))
    except (OSError, ValueError, KeyError) as exc:
        result['evidence_error_type'] = type(exc).__name__
    result['ready_for_independent_review'] = (code == 0 and result.get('source_stable') is True
                                             and result.get('capture_hashes_match_upfront') is True
                                             and result.get('final_status_allowed') is True
                                             and result.get('public_measured_starts') == 1)
    save(PRIVATE / 'run-result.json', result)
    print(json.dumps({'exit_code': code, 'review_ready': result['ready_for_independent_review'], 'retry_allowed': False}))
    return 0 if result['ready_for_independent_review'] else 1


if __name__ == '__main__':
    try:
        sys.exit(main())
    except Exception as exc:
        print('E0a helper refused/failed: ' + type(exc).__name__ + '; inspect private records.', file=sys.stderr)
        sys.exit(2)
