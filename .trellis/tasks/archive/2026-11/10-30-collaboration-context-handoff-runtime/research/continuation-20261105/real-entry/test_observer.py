"""Credential-free controls; no Go, database, Provider or launcher execution."""
import importlib.util
import os
import sys
from pathlib import Path
import tempfile
import threading
import time
import types
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location('entry', Path(__file__).with_name('e0a_entry.py'))
entry = importlib.util.module_from_spec(spec); spec.loader.exec_module(entry)


class ObserverControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir='/private/tmp')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.file = self.root / 'multica'
        self.file.write_bytes(b'owned test artifact')

    def test_regular_owned_stable_bytes(self):
        raw, info = entry.stable_bytes(self.file, self.root)
        self.assertEqual(raw, b'owned test artifact')
        self.assertEqual(info['owner'], os.getuid())

    def test_file_symlink_refused(self):
        link = self.root / 'link'; link.symlink_to(self.file)
        with self.assertRaises(OSError):
            entry.stable_bytes(link, self.root)

    def test_parent_symlink_refused(self):
        real = self.root / 'real'; real.mkdir(); (real / 'multica').write_bytes(b'x')
        link = self.root / 'link'; link.symlink_to(real, target_is_directory=True)
        with self.assertRaises(ValueError):
            entry.stable_bytes(link / 'multica', self.root)

    def test_foreign_owner_refused(self):
        info = self.file.stat()
        fake = types.SimpleNamespace(st_mode=info.st_mode, st_uid=os.getuid()+1, st_size=info.st_size)
        with patch.object(entry.os, 'fstat', return_value=fake), self.assertRaises(ValueError):
            entry.stable_bytes(self.file, self.root)

    def test_unstable_read_refused(self):
        real = entry.os.fstat; calls = 0
        def changed(fd):
            nonlocal calls
            calls += 1; info = real(fd)
            values = {k:getattr(info,k) for k in ['st_dev','st_ino','st_size','st_mtime_ns','st_mode','st_uid']}
            if calls == 2:
                values['st_mtime_ns'] += 1
            return types.SimpleNamespace(**values)
        with patch.object(entry.os, 'fstat', side_effect=changed), self.assertRaises(ValueError):
            entry.stable_bytes(self.file, self.root)

    def test_only_two_expected_patterns_captured_without_execution(self):
        nested = self.root / 'TestRK1RealProvider123/001/multica'
        prepared = self.root / 'TestRK1RealProvider123/002/host-work/session/runs/run/bin/multica'
        unrelated = self.root / 'TestOther/001/multica'
        for p in [nested, prepared, unrelated]:
            p.parent.mkdir(parents=True, exist_ok=True); p.write_bytes(b'same fake bytes')
        output = self.root / 'captures'; output.mkdir()
        stop = threading.Event(); captures, errors = [], []
        thread = threading.Thread(target=entry.observe, args=(self.root, output, stop, captures, errors))
        with patch.object(entry, 'stable_bytes', wraps=entry.stable_bytes) as reads:
            thread.start()
            try:
                deadline = time.monotonic()+2
                while len(captures) < 2 and time.monotonic() < deadline:
                    time.sleep(.01)
                time.sleep(.25)
            finally:
                stop.set(); thread.join()
            self.assertEqual(reads.call_count, 2, 'unchanged binaries must be read only once across polls')
        self.assertEqual({x['path'] for x in captures}, {str(nested),str(prepared)})
        self.assertEqual({x['sha256'] for x in captures}, {entry.digest(b'same fake bytes')})
        self.assertEqual(errors, [])

    def test_operator_settings_preserved_and_server_state_removed(self):
        config = {'database':'multica_rk1_dummy','DATABASE_URL':'postgres://localhost/multica_rk1_dummy',
                  'temp_root':str(self.root)}
        inherited = {'HOME':'/Users/artisan','PATH':'/usr/bin:/bin','ANTHROPIC_API_KEY':'dummy-test-secret',
                     'CLAUDE_CODE_CUSTOM_SETTING':'keep','CLAUDECODE':'session','DATABASE_MAX_CONNS':'9',
                     'PGOPTIONS':'discard','MULTICA_SOMETHING':'discard','JWT_SECRET':'dummy-server-secret'}
        with patch.dict(os.environ, inherited, clear=True):
            env, _ = entry.environment(config, types.SimpleNamespace(SESSION_ONLY_ENV=['CLAUDECODE']))
        self.assertEqual(env['ANTHROPIC_API_KEY'], inherited['ANTHROPIC_API_KEY'])
        self.assertEqual(env['CLAUDE_CODE_CUSTOM_SETTING'], 'keep')
        self.assertEqual(env['HOME'], '/Users/artisan')
        self.assertNotIn('CLAUDE_CONFIG_DIR', env)
        for name in ['CLAUDECODE','DATABASE_MAX_CONNS','PGOPTIONS','MULTICA_SOMETHING','JWT_SECRET']:
            self.assertNotIn(name,env)


if __name__ == '__main__':
    unittest.main()
