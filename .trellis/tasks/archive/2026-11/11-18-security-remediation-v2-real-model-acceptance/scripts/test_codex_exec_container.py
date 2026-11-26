"""Pure controls for direct CLI admission and truthful native event decoding."""
import json
import hashlib
from pathlib import Path
import tempfile
import unittest

import codex_exec_container as cli


class ExecControls(unittest.TestCase):
    def test_exact_direct_or_bound_resume_arguments(self):
        self.assertEqual(cli.direct_args(["exec", "--json", "--skip-git-repo-check", "-"])[1], None)
        session = "11111111-1111-4111-8111-111111111111"
        self.assertEqual(cli.direct_args(["exec", "resume", "--json", "--skip-git-repo-check", session, "-"])[1], session)
        for args in [["app-server", "--listen", "stdio://"], ["exec", "--dangerously-bypass-approvals-and-sandbox", "-"], ["exec", "resume", "--last"], ["exec", "--json", "--skip-git-repo-check", "changed prompt"]]:
            with self.assertRaises(ValueError):
                cli.direct_args(args)

    def good(self):
        return [{"type":"thread.started","thread_id":"11111111-1111-4111-8111-111111111111"},
                {"type":"item.completed","item":{"type":"agent_message","text":"native final"}},
                {"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":5}}]

    def decode(self, events):
        return cli.native_terminal(("\n".join(json.dumps(row) for row in events)+"\n").encode())

    def test_native_terminal_retains_actual_usage_and_text(self):
        parsed=self.decode(self.good())
        self.assertTrue(parsed["valid"])
        self.assertEqual(parsed["usage"],{"input_tokens":20,"output_tokens":5})
        self.assertEqual(parsed["last_message"],"native final")

    def test_failed_missing_or_multiple_terminal_never_pass(self):
        for events in [self.good()[:-1], self.good()+[self.good()[-1]], self.good()+[{"type":"turn.failed","error":{"message":"synthetic"}}], self.good()+[{"type":"error","message":"synthetic"}]]:
            self.assertFalse(self.decode(events)["valid"])

    def test_invalid_json_or_item_or_thread_never_pass(self):
        self.assertFalse(cli.native_terminal(b'not-json\n')["valid"])
        self.assertFalse(self.decode(self.good()+[{"type":"item.completed","item":None}])["valid"])
        for value in [None,"",123,"arbitrary"]:
            events=self.good();events[0]["thread_id"]=value
            self.assertFalse(self.decode(events)["valid"])

    def test_no_repository_requires_actual_empty_sources_and_exact_assets(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()
            (root/'AGENTS.md').write_text('unaltered instructions')
            (root/'.multica').mkdir();(root/'.multica/daemon_task_context.json').write_text('{}')
            (root/'drill.json').write_text('{"synthetic":true}')
            manifest={'no_repository':True,'repo_count':0,'project_resource_count':0,'immutable_files':{'drill.json':hashlib.sha256((root/'drill.json').read_bytes()).hexdigest()}}
            assets=cli.scratch_assets(root,manifest)
            self.assertIn(root/'AGENTS.md',assets)
            self.assertIn(root/'drill.json',assets)
            self.assertIn(root/'.multica/daemon_task_context.json',assets)
            for changed in [{**manifest,'repo_count':1},{**manifest,'project_resource_count':1},{**manifest,'immutable_files':{'../outside':'0'*64}}]:
                with self.assertRaises(ValueError):cli.scratch_assets(root,changed)
            (root/'source.py').write_text('actual source')
            with self.assertRaises(ValueError):cli.scratch_assets(root,manifest)
            (root/'source.py').unlink();(root/'.git').mkdir();(root/'.git/HEAD').write_text('ref: refs/heads/main')
            with self.assertRaises(ValueError):cli.scratch_assets(root,manifest)

    def test_scratch_aliases_cannot_replace_immutable_input(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();target=root/'source';target.write_text('outside')
            (root/'AGENTS.md').symlink_to(target)
            with self.assertRaises(ValueError):
                cli.scratch_assets(root,{'no_repository':True,'repo_count':0,'project_resource_count':0})


if __name__ == "__main__":
    unittest.main()
