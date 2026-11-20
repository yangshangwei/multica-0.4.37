"""Synthetic repository tests for the Docker-contained Git trust boundary."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import contained_git_role_handoff as contained


class ContainedGit(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(prefix='contained-git-test-');self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name).resolve();self.repo=self.root/'repo';self.repo.mkdir()
        (self.repo/'tests').mkdir();(self.repo/'app.py').write_text('value = 1\n');(self.repo/'tests/test_base.py').write_text('def test_base(): pass\n')
        self.git('init','-q');self.git('config','user.name','Synthetic');self.git('config','user.email','synthetic@example.invalid');self.git('add','.');self.git('commit','-qm','Synthetic baseline')
        self.base=self.git('rev-parse','HEAD');self.protected={'tests/test_base.py':hashlib.sha256((self.repo/'tests/test_base.py').read_bytes()).hexdigest()}

    def git(self,*args):
        return subprocess.check_output(['git','-C',str(self.repo),*args],text=True).strip()

    def test_exact_commit_export_import_retains_identity(self):
        receiver=self.root/'receiver'
        subprocess.run(['git','clone','--no-hardlinks',str(self.repo),str(receiver)],check=True,capture_output=True)
        (self.repo/'app.py').write_text('value = 2\n');self.git('add','app.py');self.git('commit','-qm','Synthetic actual output');head=self.git('rev-parse','HEAD')
        output=self.root/'export'
        result=contained.export(self.repo,self.base,head,output,'synthetic-actor','synthetic-task','implementer',['app.py'],self.protected)
        self.assertEqual(result.returncode,0,result.stderr.decode())
        manifest=output/'manifest.json';result=contained.import_bundle(receiver,output,hashlib.sha256(manifest.read_bytes()).hexdigest())
        self.assertEqual(result.returncode,0,result.stderr.decode())
        parsed=json.loads(result.stdout);self.assertEqual(parsed['head_commit'],head);self.assertTrue(parsed['author_records_preserved'])

    def test_model_config_program_cannot_run_on_host(self):
        outside=self.root/'host-marker'
        self.git('config','core.fsmonitor','touch '+str(outside))
        result=contained.inspect(self.repo,['status','--porcelain=v1'])
        self.assertFalse(outside.exists())
        self.assertEqual(result.returncode,0,result.stderr.decode())

    def test_host_absolute_git_config_include_is_not_visible(self):
        outside=self.root/'host-git-config'
        outside.write_text('[probe]\nsecret = host-only-sentinel\n')
        self.git('config','include.path',str(outside))
        result=contained.inspect(self.repo,['rev-parse','--show-toplevel'])
        self.assertEqual(result.returncode,0,result.stderr.decode())
        self.assertEqual(result.stdout.decode().strip(),'/repo')
        self.assertNotIn(b'host-only-sentinel',result.stdout+result.stderr)

    def test_git_shared_directory_and_write_inspection_refused(self):
        (self.repo/'.git/commondir').write_text('/outside')
        with self.assertRaises(contained.Refused):contained.inspect(self.repo,['status'])
        (self.repo/'.git/commondir').unlink()
        with self.assertRaises(contained.Refused):contained.inspect(self.repo,['config','user.name','changed'])
        with self.assertRaises(contained.Refused):contained.inspect(self.repo,['bundle','create','bad'])

if __name__=='__main__':unittest.main()
