"""No model: retain exact authored commits and refuse artifact boundary drift."""
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

import git_role_handoff as handoff


class GitTransfer(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.root=Path(self.tmp.name).resolve();self.source=self.root/'source';self.source.mkdir()
        subprocess.run(['git','init','-q',str(self.source)],check=True)
        handoff.git(self.source,'config','user.name','Synthetic verifier');handoff.git(self.source,'config','user.email','verifier@example.invalid')
        (self.source/'app.py').write_text('value = 1\n');(self.source/'tests').mkdir();(self.source/'tests/base.py').write_text('baseline = True\n')
        handoff.git(self.source,'add','.');handoff.git(self.source,'commit','-qm','Synthetic baseline');self.base=handoff.git(self.source,'rev-parse','HEAD').decode().strip()
        self.dest=self.root/'receiver';subprocess.run(['git','clone','-q',str(self.source),str(self.dest)],check=True)

    def commit(self,path,text):
        (self.source/path).write_text(text);handoff.git(self.source,'add',path);handoff.git(self.source,'commit','-qm','Actual synthetic role output');return handoff.git(self.source,'rev-parse','HEAD').decode().strip()

    def export(self,head,allowed,protected=None):
        out=self.root/'handoff';manifest=handoff.export_bundle(self.source,self.base,head,out,'agent-verifier','actual-task','verifier',allowed,protected or {});return out,manifest

    def test_bundle_keeps_commit_and_authors(self):
        head=self.commit('tests/repro.py','assert 1 == 1\n');out,manifest=self.export(head,['tests'])
        receipt=handoff.import_bundle(self.dest,out,handoff.digest(out/'manifest.json'))
        self.assertEqual(receipt['head_commit'],head)
        self.assertEqual(handoff.git(self.dest,'show','--format=fuller','--no-patch',head),handoff.git(self.source,'show','--format=fuller','--no-patch',head))
        self.assertEqual((self.dest/'tests/repro.py').read_text(),'assert 1 == 1\n')

    def test_changed_protected_test_refused(self):
        original=hashlib.sha256((self.source/'tests/base.py').read_bytes()).hexdigest();head=self.commit('tests/base.py','baseline = False\n')
        with self.assertRaises(handoff.Refused):self.export(head,['tests'],{'tests/base.py':original})
        self.assertEqual(handoff.git(self.dest,'rev-parse','HEAD').decode().strip(),self.base)

    def test_outside_role_and_symlink_refused(self):
        head=self.commit('app.py','value = 2\n')
        with self.assertRaises(handoff.Refused):self.export(head,['tests'])
        (self.source/'tests/link').symlink_to('/etc/passwd');handoff.git(self.source,'add','tests/link');handoff.git(self.source,'commit','-qm','Unsafe edge');head=handoff.git(self.source,'rev-parse','HEAD').decode().strip()
        with self.assertRaises(handoff.Refused):self.export(head,['tests','app.py'])

    def test_manifest_path_projection_cannot_lie(self):
        head=self.commit('app.py','value = 2\n');out,manifest=self.export(head,['app.py'])
        manifest['changed_paths']=['tests/base.py']
        (out/'manifest.json').write_text(json.dumps(manifest))
        with self.assertRaises(handoff.Refused):handoff.import_bundle(self.dest,out,handoff.digest(out/'manifest.json'))
        self.assertEqual(handoff.git(self.dest,'rev-parse','HEAD').decode().strip(),self.base)

    def test_receiver_baseline_must_match(self):
        head=self.commit('app.py','value = 2\n');out,_=self.export(head,['app.py'])
        handoff.git(self.dest,'config','user.name','Other');handoff.git(self.dest,'config','user.email','other@example.invalid')
        (self.dest/'extra').write_text('other change');handoff.git(self.dest,'add','extra');handoff.git(self.dest,'commit','-qm','Unreviewed receiver change')
        before=handoff.git(self.dest,'rev-parse','HEAD')
        with self.assertRaises(handoff.Refused):handoff.import_bundle(self.dest,out,handoff.digest(out/'manifest.json'))
        self.assertEqual(handoff.git(self.dest,'rev-parse','HEAD'),before)

    def test_bundle_tamper_and_wrong_base_refused(self):
        head=self.commit('app.py','value = 2\n');out,_=self.export(head,['app.py']);manifesthash=handoff.digest(out/'manifest.json')
        with (out/'handoff.bundle').open('ab') as f:f.write(b'tamper')
        with self.assertRaises(handoff.Refused):handoff.import_bundle(self.dest,out,manifesthash)
        self.assertEqual(handoff.git(self.dest,'rev-parse','HEAD').decode().strip(),self.base)

if __name__=='__main__':unittest.main()
