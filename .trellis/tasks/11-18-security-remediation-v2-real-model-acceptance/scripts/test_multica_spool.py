#!/usr/bin/env python3
"""Credential-free transport controls; never runs a provider or live platform."""
import base64
import json
import os
from pathlib import Path
import tempfile
import unittest
import uuid

import multica_spool as spool


class SpoolTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.requests = self.root / '.secrem-requests'
        self.responses = self.root / '.secrem-responses'
        self.requests.mkdir(); self.responses.mkdir()
        self.manifest = dict(task_id='task', issue_id='issue', squad_id='squad', agent_id='agent', workspace_id='workspace', owner_ids=['owner'], role='lead', is_leader_task=True)
        self.executable = self.root / 'fake-cli'
        self.executable.write_text('#!/usr/bin/env python3\nimport os,sys,json\nprint(json.dumps({"argv":sys.argv[1:],"stdin":sys.stdin.read(),"token":os.environ["MULTICA_TOKEN"],"actor":os.environ["MULTICA_AGENT_ID"],"workspace":os.environ["MULTICA_WORKSPACE_ID"]}))\n')
        self.executable.chmod(0o700)
        self.env = dict(PATH=os.environ['PATH'], MULTICA_TOKEN='mat_synthetic', MULTICA_TASK_ID='task', MULTICA_AGENT_ID='agent', MULTICA_WORKSPACE_ID='workspace')
        self.broker = spool.Broker(self.manifest, self.root, self.requests, self.responses, self.executable, self.env, self.root/'audit.jsonl')

    def request(self, argv, stdin=b'', cwd=None):
        name = uuid.uuid4().hex
        path = self.requests/(name+'.json')
        path.write_text(json.dumps(dict(argv=argv, stdin_b64=base64.b64encode(stdin).decode(), cwd=str(cwd or self.root))))
        self.broker.process(path)
        return json.loads((self.responses/(name+'.json')).read_text())

    def test_original_actor_and_stdin_preserved_without_shell(self):
        content = b'$(touch forbidden); `whoami` "quotes"\n'
        reply = self.request(['issue','comment','add','issue','--content-stdin','--parent','parent'], content)
        self.assertEqual(reply['exit_code'],0)
        parsed=json.loads(base64.b64decode(reply['stdout_b64']))
        self.assertEqual(parsed['token'],'mat_synthetic')
        self.assertEqual(parsed['actor'],'agent')
        self.assertEqual(parsed['workspace'],'workspace')
        self.assertEqual(parsed['stdin'],content.decode())
        self.assertFalse((self.root/'forbidden').exists())
        self.assertNotIn('mat_synthetic',(self.root/'audit.jsonl').read_text())

    def test_scope_and_global_override_refused(self):
        for argv in [['issue','get','outside'],['--server','https://outside','issue','get','issue'],['issue','get','issue','--token','human'],['issue','comment','add','issue','--content-file','/etc/passwd'],['issue','update','issue','--assignee-id','other'],['squad','update','squad','--instructions','x'],['issue','subscriber','add','issue','--user-id','outside']]:
            with self.subTest(argv=argv):self.assertEqual(self.request(argv)['exit_code'],126)

    def test_review_scope_preserves_allow_read_post_only(self):
        self.manifest.update(role='security-reviewer',is_leader_task=False)
        self.assertEqual(self.request(['issue','get','issue'])['exit_code'],0)
        self.assertEqual(self.request(['issue','comment','add','issue','--content','result'])['exit_code'],0)
        for argv in [['issue','status','issue','done'],['issue','runs','issue'],['squad','get','squad']]:
            with self.subTest(argv=argv):self.assertEqual(self.request(argv)['exit_code'],126)

    def test_symlink_and_outside_cwd_refused(self):
        link=self.root/'link';link.symlink_to('/tmp')
        self.assertEqual(self.request(['issue','get','issue'],cwd=link)['exit_code'],126)
        self.assertEqual(self.request(['issue','get','issue'],cwd=self.root.parent)['exit_code'],126)
        requestid=uuid.uuid4().hex
        bad=self.requests/(requestid+'.json');bad.symlink_to('/etc/passwd')
        self.broker.process(bad)
        self.assertEqual(json.loads((self.responses/(requestid+'.json')).read_text())['exit_code'],126)

    def test_variant_only_backlog_unassigned(self):
        self.assertEqual(self.request(['issue','create','--title','finding-1','--status','backlog','--description','ref'])['exit_code'],0)
        self.assertEqual(self.request(['issue','create','--title','finding-1','--status','todo'])['exit_code'],126)
        self.assertEqual(self.request(['issue','create','--title','finding-1','--status','backlog','--parent','issue'])['exit_code'],126)

    def test_replayed_request_is_not_executed_twice(self):
        name=uuid.uuid4().hex;path=self.requests/(name+'.json')
        path.write_text(json.dumps(dict(argv=['issue','get','issue'],stdin_b64='',cwd=str(self.root))))
        self.broker.process(path);self.broker.process(path)
        self.assertEqual(len((self.root/'audit.jsonl').read_text().splitlines()),1)


if __name__=='__main__':unittest.main()
