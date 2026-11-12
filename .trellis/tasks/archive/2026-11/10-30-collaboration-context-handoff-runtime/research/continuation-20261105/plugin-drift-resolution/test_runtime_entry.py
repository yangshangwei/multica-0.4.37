"""Small local controls: no operator scan, Go, database or Provider execution."""
import importlib.util
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('entry', Path(__file__).with_name('runtime_entry.py'))
entry = importlib.util.module_from_spec(spec); spec.loader.exec_module(entry)
launcher_spec = importlib.util.spec_from_file_location('launcher', Path(__file__).parents[2] / 'real-model-rk1/run_rk1.py')
launcher = importlib.util.module_from_spec(launcher_spec); launcher_spec.loader.exec_module(launcher)


class SuccessorControls(unittest.TestCase):
    def test_admission_drift_refused(self):
        with self.assertRaisesRegex(ValueError, 'operator configuration changed'):
            entry.operator_gate({'plugins': 'new'}, {'plugins': 'old'}, True)

    def test_post_drift_is_observed_without_changing_failed_trial(self):
        drift = entry.operator_gate({'plugins': 'new'}, {'plugins': 'old'}, False)
        plan = {'labels':['RK1-H1-t1'], 'launches_per_trial':1, 'paid_launches_expected':1}
        trial = {'label':'RK1-H1-t1','mode':'real','status':'incomplete','verdict':'runtime_failure',
                 'runs':[{'measured':True,'physical_launches':1}]}
        result = entry.trial_accounting(plan,[trial])
        self.assertEqual(drift, {'plugins': {'before':'old','after':'new'}})
        self.assertFalse(result['trial_contract_complete'])
        self.assertEqual(result['trials'][0]['verdict'], 'runtime_failure')
        self.assertEqual(result['public_measured_starts'], 1)

    def test_inventory_matches_original_and_tracks_link_identity_without_following(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); (root/'empty').write_bytes(b''); (root/'file').write_bytes(b'first')
            (root/'nested').mkdir(); (root/'nested'/'x').write_bytes(b'content')
            (root/'alias').symlink_to('file'); (root/'dirlink').symlink_to('nested',target_is_directory=True)
            before = entry.plugin_inventory(root)
            self.assertEqual(before['tree_sha256'],launcher.tree_sha256(root))
            self.assertEqual(before['entries']['alias'],{'kind':'link','value':'file'})
            self.assertNotIn('dirlink/x',before['entries'])
            (root/'alias').unlink(); (root/'alias').symlink_to('empty')
            (root/'file').write_bytes(b'second'); (root/'empty').unlink(); (root/'added').write_bytes(b'new')
            after = entry.plugin_inventory(root)
            self.assertEqual(after['tree_sha256'],launcher.tree_sha256(root))
            self.assertEqual(set(entry.changed_fields(before['entries'],after['entries'])),{'alias','file','empty','added'})

    def test_absent_or_unreadable_inventory_refuses(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            with self.assertRaises(ValueError):entry.plugin_inventory(root/'absent')
            (root/'file').write_bytes(b'content')
            with patch.object(entry,'sha',side_effect=PermissionError),self.assertRaises(PermissionError):
                entry.plugin_inventory(root)

    def test_selected_metadata_bytes_match_inventory_and_drift_refuses(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)/'plugins';root.mkdir()
            file=root/'known_marketplaces.json';file.write_bytes(b'{"version":1}')
            inventory=entry.plugin_inventory(root)
            copies=entry.retain_metadata(root,inventory,Path(directory)/'before')
            self.assertEqual(Path(copies[file.name]['path']).read_bytes(),file.read_bytes())
            self.assertEqual(copies[file.name]['sha256'],inventory['entries'][file.name]['value'])
            self.assertEqual(copies['plugin-directory-cache-v2.json'],{'state':'absent'})
            file.write_bytes(b'{"version":2}')
            with self.assertRaisesRegex(ValueError,'metadata bytes differ from inventory'):
                entry.retain_metadata(root,inventory,Path(directory)/'mismatched')

    def test_unreadable_subdirectory_refuses_instead_of_omitting(self):
        with tempfile.TemporaryDirectory() as directory:
            def unreadable(root, onerror=None):
                if onerror: onerror(PermissionError('synthetic subtree denied'))
                return iter(())
            with patch.object(entry.os,'walk',side_effect=unreadable),self.assertRaises(PermissionError):
                entry.plugin_inventory(Path(directory))

    def test_h1_three_measured_starts_with_three_seed_copies(self):
        addendum={'runtime_scenarios':[{'id':'RK1-H1','trials':3,'launches':3}]}
        plan=entry.scenario_plan(addendum,'RK1-H1',1)
        trials=[{'label':label,'mode':'real','status':'finished','verdict':'pass',
                 'runs':[{'measured':True,'physical_launches':1},{'measured':False,'physical_launches':1}]} for label in plan['labels']]
        result=entry.trial_accounting(plan,trials)
        self.assertTrue(result['trial_contract_complete']);self.assertEqual(result['public_measured_starts'],3)
        self.assertEqual(result['public_seed_starts'],3)

    def test_h6_pairs_and_partial_invocation_do_not_free_budget(self):
        addendum={'runtime_scenarios':[{'id':'RK1-H6','trials':3,'launches':6}]}
        plan=entry.scenario_plan(addendum,'RK1-H6',2)
        self.assertEqual(plan['paid_launches_expected'],4)
        trials=[{'label':label,'mode':'real','status':'finished','verdict':'manual_review_required',
                 'runs':[{'measured':True,'physical_launches':1},{'measured':True,'physical_launches':1}]} for label in plan['labels']]
        self.assertTrue(entry.trial_accounting(plan,trials)['trial_contract_complete'])
        partial=entry.trial_accounting(plan,trials[:1])
        self.assertFalse(partial['trial_contract_complete']);self.assertEqual(partial['public_measured_starts'],2)

    def test_canary_and_duplicate_labels_never_complete(self):
        plan={'labels':['RK1-H5-t1'],'launches_per_trial':1,'paid_launches_expected':1}
        trial={'label':'RK1-H5-t1','mode':'real','status':'finished','verdict':'blocking_review',
               'runs':[{'measured':True,'physical_launches':1}]}
        self.assertFalse(entry.trial_accounting(plan,[trial])['trial_contract_complete'])
        trial['verdict']='pass';trial['label']='RK1-H5-t2'
        self.assertFalse(entry.trial_accounting(plan,[trial])['trial_contract_complete'])

    def test_unregistered_or_exhausted_scenario_refused(self):
        addendum={'runtime_scenarios':[{'id':'RK1-H1','trials':3,'launches':3}]}
        for scenario,start in [('RK1-K1',1),('RK1-H1',4),('RK1-H1',0)]:
            with self.assertRaises(ValueError):entry.scenario_plan(addendum,scenario,start)

    def test_actual_recorder_keeps_canary_and_budget_when_post_binding_raises(self):
        with tempfile.TemporaryDirectory(dir='/private/tmp') as directory:
            root=Path(directory); here=root/'framework';here.mkdir();(root/'server').mkdir()
            temp=root/'temp';temp.mkdir(mode=0o700)
            base=root/'private'; run=base/'RK1-H5-from-1';run.mkdir(parents=True,mode=0o700)
            config=root/'config.json'; permit=root/'permit.json'
            addendum={'runtime_scenarios':[{'id':'RK1-H5','trials':1,'launches':1}]}
            (here/'runtime-addendum.json').write_text(json.dumps(addendum))
            facts={'whole_status':'','scenario_plan':entry.scenario_plan(addendum,'RK1-H5',1)}
            (run/'upfront-multica').write_bytes(b'fake binary never executed')
            build={'exit_code':0,'binding_before':facts,'binding_after':facts,'cli_sha256':entry.sha(run/'upfront-multica')}
            (run/'build-result.json').write_text(json.dumps(build))
            (run/'check-result.json').write_text(json.dumps({'exit_code':0,'binding':facts}))
            config.write_text('{}');config.chmod(0o600)
            permit.write_text(json.dumps({'allow_run':True,'binding':facts,'window_id':'synthetic-test',
                'window_expires_at':'2099-01-01T00:00:00+00:00',
                'build_result_sha256':entry.sha(run/'build-result.json'),
                'check_result_sha256':entry.sha(run/'check-result.json')}));permit.chmod(0o600)
            def bindings(*args,**kwargs):
                if kwargs.get('admission') is False:raise ValueError('synthetic post scan failure')
                return facts
            def command(*args):
                (here/'results').mkdir()
                trial={'label':'RK1-H5-t1','mode':'real','status':'finished','verdict':'blocking_review',
                       'runs':[{'measured':True,'physical_launches':1}]}
                (here/'results/RK1-H5-t1.json').write_text(json.dumps(trial))
                return 1
            fake_launcher=type('FakeLauncher',(),{'launch_accounting':staticmethod(lambda records:{'consumed':1})})()
            argv=['runtime_entry.py','run','--scenario','RK1-H5','--start-trial','1','--config',str(config),'--permit',str(permit)]
            old_umask=os.umask(0o077)
            try:
                with patch.multiple(entry,ROOT=root,HERE=here,PRIVATE=base),patch.object(sys,'argv',argv),\
                     patch.object(entry,'module',return_value=fake_launcher),\
                     patch.object(entry,'environment',return_value=({},temp)),\
                     patch.object(entry,'binding',side_effect=bindings),patch.object(entry,'command',side_effect=command),\
                     patch.object(entry,'observe',return_value=None):
                    self.assertEqual(entry.main(),1)
            finally:os.umask(old_umask)
            result=json.loads((run/'run-result.json').read_text())
            self.assertEqual(result['exit_code'],1)
            self.assertEqual(result['binding_error_type'],'ValueError')
            self.assertEqual(result['trials'][0]['verdict'],'blocking_review')
            self.assertEqual(result['public_measured_starts'],1)
            self.assertEqual(result['private_launch_accounting']['consumed'],1)
            self.assertTrue(result['operator_drift_requires_root_review'])
            self.assertFalse(result['ready_for_independent_review'])
            self.assertFalse(result['next_scenario_authorized'])


if __name__ == '__main__':
    unittest.main()
