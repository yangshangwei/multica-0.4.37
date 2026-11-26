"""No providers: context retention must never follow untrusted filesystem edges."""
import os
from pathlib import Path
import tempfile
import unittest

from codex_container import copy_regular_tree, retain_sessions, registration_from_driver
import runtime_guard as guard
import json
import time
from unittest.mock import patch


class ContextCopy(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name).resolve();self.src=self.root/'source';self.src.mkdir();self.dst=self.root/'dest'
        self.secret=self.root/'host-secret';self.secret.write_text('synthetic host-only sentinel')

    def test_regular_transcript_preserved(self):
        (self.src/'day').mkdir();(self.src/'day'/'rollout.jsonl').write_text('synthetic\n')
        copy_regular_tree(self.src,self.dst)
        self.assertEqual((self.dst/'day'/'rollout.jsonl').read_text(),'synthetic\n')

    def test_source_absolute_host_symlink_refused(self):
        (self.src/'rollout.jsonl').symlink_to(self.secret)
        with self.assertRaises(ValueError):copy_regular_tree(self.src,self.dst)
        self.assertFalse((self.dst/'rollout.jsonl').exists())
        self.assertEqual(self.secret.read_text(),'synthetic host-only sentinel')

    def test_destination_alias_refused(self):
        self.dst.mkdir();(self.src/'rollout.jsonl').write_text('write')
        (self.dst/'rollout.jsonl').symlink_to(self.secret)
        with self.assertRaises(ValueError):copy_regular_tree(self.src,self.dst)
        self.assertEqual(self.secret.read_text(),'synthetic host-only sentinel')

    def test_rejected_retention_stops_durable_ledger(self):
        (self.src/'rollout.jsonl').symlink_to(self.secret)
        result=retain_sessions(self.src,self.dst,'completed',0)
        self.assertEqual(result,('evidence_failed',125,True))
        self.root.chmod(0o700)
        registration={'ledger':str(self.root/'ledger.json'),'registration_sha256':'a'*64}
        with guard.ledger(registration) as ledger:
            ledger['starts'].append({'sequence':1,'started_at':1,'finished_at':None})
        guard.finish_start(registration,1,result[0],result[1])
        ledger=json.loads((self.root/'ledger.json').read_text())
        self.assertTrue(ledger['stopped'])
        self.assertEqual(ledger['starts'][0]['outcome'],'evidence_failed')

    def test_driver_private_opt_in_survives_filtered_multica_env(self):
        executable=self.root/'fake-provider';executable.write_text('#!/bin/sh\nexit 0\n');executable.chmod(0o700)
        source=self.root/'source.txt';source.write_text('reviewed')
        hashes={str(source):guard.digest(source)}
        admission=self.root/'admission.json'
        admission.write_text(json.dumps({'verdict':'approved','scope':'P1','verified':['actual_driver','original_task_identity','shell_network_denied','permission_widening_denied','role_filesystem','instruction_complete','platform_records'],'source_hashes':hashes,'model':'synthetic','account_alias':'synthetic-only'}));admission.chmod(0o600)
        registration={'schema_version':1,'mode':'real','enabled':True,'model':'synthetic','account_alias':'synthetic-only','integration_commit':'synthetic','isolation_evidence_sha256':'a'*64,'source_hashes':hashes,'admission_file':str(admission),'admission_sha256':guard.digest(admission),'scenario':'P1','max_starts':1,'max_model_requests':1,'max_seconds':20,'per_start_seconds':10,'expires_at':time.time()+60,'ledger':str(self.root/'ledger.json'),'argv':[str(executable)],'executable_sha256':guard.digest(executable),'cwd':str(self.root),'environment':{}}
        path=self.root/'registration.json';path.write_text(json.dumps(registration));path.chmod(0o600)
        config={'registration_file':str(path),'registration_sha256':guard.digest(path)}
        with patch.dict(os.environ,{},clear=True):
            with self.assertRaises(guard.Refused):registration_from_driver(config)
        with patch.dict(os.environ,{'SECREM_REAL_EXECUTION_AUTHORIZED':'1'},clear=True):
            actual=registration_from_driver(config)
            self.assertEqual(actual['model'],'synthetic')
            self.assertEqual(os.environ['MULTICA_RUN_REAL_AGENT_SMOKE'],'1')
        with patch.dict(os.environ,{'SECREM_REAL_EXECUTION_AUTHORIZED':'true'},clear=True):
            with self.assertRaises(guard.Refused):registration_from_driver(config)
        with patch.dict(os.environ,{'SECREM_REAL_EXECUTION_AUTHORIZED':'1'},clear=True):
            invalid=dict(config,registration_sha256='0'*64)
            with self.assertRaises(guard.Refused):registration_from_driver(invalid)
        registration['mode']='fake';path.write_text(json.dumps(registration))
        fake=dict(config,registration_sha256=guard.digest(path))
        with patch.dict(os.environ,{'SECREM_REAL_EXECUTION_AUTHORIZED':'1'},clear=True):
            with self.assertRaises(guard.Refused):registration_from_driver(fake)

    def test_nested_symlink_and_fifo_refused(self):
        (self.src/'nested').symlink_to(self.root)
        with self.assertRaises(ValueError):copy_regular_tree(self.src,self.dst)
        (self.src/'nested').unlink();os.mkfifo(self.src/'fifo')
        with self.assertRaises(ValueError):copy_regular_tree(self.src,self.dst)

if __name__=='__main__':unittest.main()
