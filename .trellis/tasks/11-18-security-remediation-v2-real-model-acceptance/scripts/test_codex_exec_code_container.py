"""No-provider role policy controls; use only local synthetic Git repositories."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import codex_exec_code_container as code


class CodePolicy(unittest.TestCase):
    def setUp(self):
        evidence=Path('/Volumes/artisan/code/2026/secrem-v2-cli-evidence-20261120')
        evidence.mkdir(exist_ok=True)
        self.tmp=tempfile.TemporaryDirectory(prefix='code-policy-test-',dir=evidence);self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name).resolve();(self.root/'tests').mkdir()
        (self.root/'app.py').write_text('value = 1\n');(self.root/'tests/test_base.py').write_text('def test_base(): pass\n')
        for args in [['init','-q'],['config','user.name','Synthetic'],['config','user.email','synthetic@example.invalid'],['add','.'],['commit','-qm','Synthetic baseline']]:
            subprocess.run(['git','-C',str(self.root),*args],check=True,capture_output=True)
        self.head=subprocess.check_output(['git','-C',str(self.root),'rev-parse','HEAD'],text=True).strip()
        self.manifest=dict(no_repository=False,role_mode='implementer',role='implementer',is_leader_task=False,repo_root=str(self.root),baseline_commit=self.head,protected_test_dir='tests',protected_test_sha256={'tests/test_base.py':hashlib.sha256((self.root/'tests/test_base.py').read_bytes()).hexdigest()})

    def test_registered_baseline_and_role(self):
        mode,tests,protected=code.code_policy(self.root,self.manifest)
        self.assertEqual(mode,'implementer');self.assertEqual(tests,self.root/'tests');self.assertTrue(protected)
        for change in [{'role':'security-reviewer'},{'is_leader_task':True},{'baseline_commit':'0'*40},{'repo_root':str(self.root.parent)},{'no_repository':True}]:
            with self.subTest(change=change),self.assertRaises(ValueError):code.code_policy(self.root,{**self.manifest,**change})

    def test_shared_git_or_modified_test_refused(self):
        (self.root/'.git/commondir').write_text('/outside')
        with self.assertRaises(ValueError):code.code_policy(self.root,self.manifest)
        (self.root/'.git/commondir').unlink();(self.root/'tests/test_base.py').write_text('changed')
        with self.assertRaises(ValueError):code.code_policy(self.root,self.manifest)

    def test_post_run_head_does_not_execute_git_or_follow_alias(self):
        self.assertEqual(code.retained_head(self.root),self.head)
        (self.root/'.git/HEAD').write_text('ref: refs/heads/outside\n')
        (self.root/'.git/refs/heads/outside').symlink_to(self.root/'app.py')
        with self.assertRaises(ValueError):code.retained_head(self.root)

    def test_model_fifo_head_is_rejected_without_open_wait(self):
        head=self.root/'.git/HEAD';head.unlink();os.mkfifo(head)
        with self.assertRaises(ValueError):code.retained_head(self.root)

    def test_leader_scratch_uses_prepared_set_without_rewriting_claim(self):
        outside=self.root.parent/(self.root.name+'-project');outside.mkdir();self.addCleanup(outside.rmdir)
        manifest={'role_mode':'leader_scratch','is_leader_task':True,'leader_ignores_project_resources':True,'prepared_repo_count':0,'no_repository':True,'repo_count':0,'project_resource_count':1,'ignored_project_paths':[str(outside)]}
        with patch.object(code.scratch_transport,'prepare',return_value='prepared') as prepare:
            self.assertEqual(code.prepare({},manifest,self.root,self.root,self.root),'prepared')
            self.assertEqual(prepare.call_args.args[1]['project_resource_count'],0)
            self.assertEqual(manifest['project_resource_count'],1)
        for change in [{'is_leader_task':False},{'prepared_repo_count':1},{'leader_ignores_project_resources':False}]:
            with self.assertRaises(ValueError):code.prepare({},dict(manifest,**change),self.root,self.root,self.root)

    def test_leader_project_paths_cannot_overlap_scratch(self):
        manifest={'is_leader_task':True,'leader_ignores_project_resources':True,'prepared_repo_count':0,'no_repository':True,'repo_count':0,'project_resource_count':1}
        for paths in [[],[str(self.root)],[str(self.root.parent)],[str(self.root/'tests')],['relative']]:
            with self.subTest(paths=paths),self.assertRaises(ValueError):code.leader_prepared_manifest(self.root,{**manifest,'ignored_project_paths':paths})

    def test_leader_postcheck_does_not_read_unmounted_project_tests(self):
        (self.root/'AGENTS.md').write_text('actual prepared instructions')
        manifest={'role_mode':'leader_scratch','protected_test_sha256':{'tests/not-mounted.py':'f'*64}}
        evidence=code.post_filesystem_evidence(self.root,manifest,hashlib.sha256((self.root/'AGENTS.md').read_bytes()).hexdigest())
        self.assertTrue(evidence['agents_unchanged'])
        self.assertEqual(evidence['protected_tests_status'],'not_mounted')
        self.assertIsNone(evidence['protected_tests_unchanged'])
        self.assertEqual(manifest['protected_test_sha256'],{'tests/not-mounted.py':'f'*64})

    def test_analysis_policy_binds_actual_product_analyst(self):
        manifest={**self.manifest,'role_mode':'analysis','role':'product-analyst'}
        self.assertEqual(code.code_policy(self.root,manifest)[0],'analysis')
        with self.assertRaises(ValueError):code.code_policy(self.root,{**manifest,'role':'implementer'})

if __name__=='__main__':unittest.main()
