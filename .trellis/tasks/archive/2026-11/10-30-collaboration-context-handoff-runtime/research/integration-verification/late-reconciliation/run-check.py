import datetime, hashlib, json, os, pathlib, platform, subprocess, sys, time
root = pathlib.Path('/Volumes/artisan/code/2026/mc-runtime-integration-20261031')
out = pathlib.Path(__file__).resolve().parent
label, *command = sys.argv[1:]
start = datetime.datetime.now(datetime.timezone.utc).isoformat()
head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
diff = subprocess.check_output(['git', 'diff', '--binary', 'HEAD'], cwd=root)
(out / (label + '.source.diff')).write_bytes(diff)
names = subprocess.check_output(['git', 'ls-files', '-c', '-o', '--exclude-standard', '--', 'server', 'packages', 'apps', '.trellis/spec'], cwd=root, text=True).splitlines()
inventory = {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in sorted(set(names)) if (root / name).is_file()}
(out / (label + '.source.json')).write_text(json.dumps(inventory, indent=2) + '\n')
tick = time.monotonic()
with (out / (label + '.log')).open('wb') as log:
    result = subprocess.run(command, cwd=root, stdout=log, stderr=subprocess.STDOUT)
record = {'label': label, 'started_at': start, 'duration_seconds': time.monotonic() - tick, 'command': command, 'cwd': str(root), 'head': head, 'tracked_diff_sha256': hashlib.sha256(diff).hexdigest(), 'source_inventory': label + '.source.json', 'platform': platform.platform(), 'database': 'not used; HTTP fakes only', 'real_provider': False, 'exit_code': result.returncode}
(out / (label + '.json')).write_text(json.dumps(record, indent=2) + '\n')
print(json.dumps(record, indent=2))
print((out / (label + '.log')).read_text(errors='replace')[-5000:])
sys.exit(result.returncode)
