"""COW-clone only the retired V15 Go cache into a fresh owned V16 cache."""

import ctypes
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import sys


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
HERE = Path(__file__).resolve().parent
SOURCE = ROOT / '.omx/private/final-check-v15/go-build-cache'
DESTINATION = ROOT / '.omx/private/final-check-v16/go-build-cache'
EXPECTED = 'eda0d68fb07852b714f3092aa02ca0523d3ad312'
DONOR_CANDIDATE = 'd41573e3a4eb3bd6c36feed3f0d542a0e066ee0f'


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def save(name, value):
    descriptor = os.open(HERE / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        json.dump(value, output, indent=2)
        output.write('\n')


def inventory(directory):
    require(directory.is_dir() and not directory.is_symlink(), 'Cache directory must be real')
    root_info = directory.stat()
    require(root_info.st_uid == os.geteuid() and stat.S_IMODE(root_info.st_mode) == 0o700,
            'Cache root must be owned mode0700')
    rows, identities = [], {}
    for path in sorted(directory.rglob('*')):
        info = path.lstat()
        require(not stat.S_ISLNK(info.st_mode) and info.st_uid == os.geteuid(), 'Cache member is redirected or unowned')
        relative = str(path.relative_to(directory))
        mode = stat.S_IMODE(info.st_mode)
        if stat.S_ISDIR(info.st_mode):
            rows.append({'path': relative, 'kind': 'directory', 'mode': mode})
        else:
            require(stat.S_ISREG(info.st_mode), 'Cache member is not a regular file')
            rows.append({'path': relative, 'kind': 'file', 'mode': mode, 'bytes': info.st_size, 'sha256': digest(path)})
            identities[relative] = {'device': info.st_dev, 'inode': info.st_ino, 'bytes': info.st_size,
                                    'mtime_ns': info.st_mtime_ns, 'mode': mode}
    require(any(row['kind'] == 'file' for row in rows), 'Donor cache is empty')
    return {'path': str(directory), 'uid': root_info.st_uid, 'mode': 0o700,
            'device': root_info.st_dev, 'inode': root_info.st_ino, 'entries': rows,
            'fingerprint': hashlib.sha256(json.dumps(rows, sort_keys=True, separators=(',', ':')).encode()).hexdigest(),
            'file_identities': identities}


def main():
    require(sys.platform == 'darwin', 'This task uses the documented Darwin COW clone operation only')
    require('PENDING_ROOT_' not in EXPECTED, 'Root must materialize the frozen final SHA first')
    git_env = {**os.environ, 'GIT_OPTIONAL_LOCKS': '0'}
    require(subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, env=git_env, text=True).strip() == EXPECTED,
            'Candidate differs from the final cache-seeding binding')
    require(not subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, env=git_env, text=True).strip(),
            'Candidate must be clean before cache seeding')
    require(SOURCE.resolve() == SOURCE and not SOURCE.is_symlink(), 'Do not redirect the old cache')
    require(DESTINATION.parent.is_dir() and DESTINATION.parent.resolve() == DESTINATION.parent,
            'New cache namespace must already exist without redirection')
    require(not DESTINATION.exists() and not DESTINATION.is_symlink(), 'Never overwrite an existing cache or partial seed')
    before = inventory(SOURCE)
    require(before['device'] == DESTINATION.parent.stat().st_dev, 'COW cache copy requires the same filesystem')
    save('cache-seeding-before.json', {'recorded_at': datetime.now(timezone.utc).isoformat(),
         'candidate_sha': EXPECTED, 'donor_candidate_sha': DONOR_CANDIDATE, 'source': before,
         'destination': str(DESTINATION), 'destination_absent': True,
         'method': 'Darwin fclonefileat; no fallback, symlink, hardlink or source write'})

    # Apple SDK sys/clonefile.h declares fclonefileat(int,int,const char*,uint32_t).
    # A successful kernel clone establishes COW; do not substitute byte copying.
    clone = ctypes.CDLL(None, use_errno=True).fclonefileat
    clone.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint32]
    clone.restype = ctypes.c_int
    completed = 0
    DESTINATION.mkdir(mode=0o700)
    try:
        for row in before['entries']:
            source = SOURCE / row['path']
            target = DESTINATION / row['path']
            if row['kind'] == 'directory':
                target.mkdir(mode=row['mode'])
                target.chmod(row['mode'])
                continue
            source_fd = os.open(source, os.O_RDONLY | os.O_NOFOLLOW)
            try:
                parent_fd = os.open(target.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
                try:
                    info = os.fstat(source_fd)
                    identity = before['file_identities'][row['path']]
                    actual = {'device': info.st_dev, 'inode': info.st_ino, 'bytes': info.st_size,
                              'mtime_ns': info.st_mtime_ns, 'mode': stat.S_IMODE(info.st_mode)}
                    require(stat.S_ISREG(info.st_mode) and actual == identity, 'Donor changed before its COW clone')
                    result = clone(source_fd, parent_fd, os.fsencode(target.name), 0)
                    if result != 0:
                        error = ctypes.get_errno()
                        raise OSError(error, os.strerror(error), str(target))
                    target.chmod(row['mode'])
                    target_info = target.lstat()
                    require(stat.S_ISREG(target_info.st_mode) and target_info.st_ino != info.st_ino,
                            'COW clone must be a distinct regular inode')
                    completed += 1
                finally:
                    os.close(parent_fd)
            finally:
                os.close(source_fd)
        after = inventory(SOURCE)
        target = inventory(DESTINATION)
        require(after['fingerprint'] == before['fingerprint'] and after['inode'] == before['inode']
                and after['device'] == before['device'], 'Old cache changed during seeding')
        require(target['entries'] == before['entries'], 'New cache bytes/modes differ from the donor snapshot')
        save('cache-seeding-result.json', {'recorded_at': datetime.now(timezone.utc).isoformat(),
             'candidate_sha': EXPECTED, 'source_before_fingerprint': before['fingerprint'],
             'source_after': after, 'destination': target, 'successful_COW_clones': completed,
             'state': 'complete', 'old_cache_content_mode_inventory_unchanged': True,
             'old_cache_written_or_redirected': False, 'normal_copy_fallbacks': 0,
             'knowledge_budget_allocated': 0, 'knowledge_real_calls': 0, 'production_default_enabled': False})
        print(json.dumps({'state': 'complete', 'successful_COW_clones': completed,
                          'source_fingerprint': before['fingerprint'], 'destination_fingerprint': target['fingerprint']}))
    except (OSError, RuntimeError) as error:
        save('cache-seeding-failure.json', {'recorded_at': datetime.now(timezone.utc).isoformat(),
             'candidate_sha': EXPECTED, 'error': str(error), 'successful_COW_clones': completed,
             'source_before_fingerprint': before['fingerprint'], 'partial_destination': str(DESTINATION),
             'partial_destination_preserved': True, 'old_cache_written_or_redirected': False,
             'normal_copy_fallbacks': 0, 'state': 'failed'})
        raise


if __name__ == '__main__':
    main()
