"""Run with: python3 -m unittest discover -s .github/scripts -p 'test_*.py'."""
import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import check_ovs_versions as checker


ROOT = Path(__file__).resolve().parents[2]
TAGS = [
    'v3.8.0', 'v3.7.2', 'v3.7.1', 'v3.6.4', 'v3.6.3', 'v3.5.4',
    'v3.9.0-rc1', 'master', 'not-a-version',
]
EXPECTED = [
    {'ovs_version': 'master', 'tag': 'latest'},
    {'ovs_version': 'v3.8.0', 'tag': '3.8.0'},
    {'ovs_version': 'v3.7.2', 'tag': '3.7.2'},
    {'ovs_version': 'v3.6.4', 'tag': '3.6.4'},
]


class CheckOVSVersionsTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        for name in ('ovs/versions.json', 'Makefile', '.github/workflows/images.yml'):
            target = self.directory / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((ROOT / name).read_bytes())
        # Fixed stale fixtures keep the tests independent of future version bumps.
        self.write_versions([
            {'ovs_version': 'master', 'tag': 'latest'},
            {'ovs_version': 'v3.7.1', 'tag': '3.7.1'},
            {'ovs_version': 'v3.6.3', 'tag': '3.6.3'},
            {'ovs_version': 'v3.5.4', 'tag': '3.5.4'},
        ])
        (self.directory / 'Makefile').write_text('OVS_VERSION ?= v3.7.1\n\nall:\n\t@echo test\n')
        previous = Path.cwd()
        os.chdir(self.directory)
        self.addCleanup(os.chdir, previous)

    def write_versions(self, images):
        (self.directory / 'ovs/versions.json').write_text(json.dumps(images, indent=2) + '\n')

    def snapshot(self):
        return {str(p.relative_to(self.directory)): p.read_bytes()
                for p in self.directory.rglob('*') if p.is_file()}

    def run_checker(self, *args):
        output = io.StringIO()
        with patch.object(checker, 'fetch_ovs_tags', return_value=TAGS), \
                patch('sys.argv', ['check_ovs_versions.py', *args]), \
                contextlib.redirect_stdout(output), \
                self.assertRaises(SystemExit) as result:
            checker.main()
        return result.exception.code, output.getvalue()

    def test_updates_only_data_and_makefile_and_is_idempotent(self):
        before = self.snapshot()
        code, _ = self.run_checker()
        self.assertEqual(code, 2)
        after = self.snapshot()
        changed = {path for path in before if before[path] != after[path]}
        self.assertEqual(changed, {'ovs/versions.json', 'Makefile'})
        self.assertEqual(before.keys(), after.keys())
        self.assertEqual(json.loads(after['ovs/versions.json']), EXPECTED)
        self.assertEqual(after['Makefile'], before['Makefile'].replace(b'v3.7.1', b'v3.8.0'))
        self.assertTrue(after['ovs/versions.json'].endswith(b'\n'))
        code, _ = self.run_checker()
        self.assertEqual(code, 0)
        self.assertEqual(self.snapshot(), after)

    def test_dry_run_reports_changes_without_writing(self):
        before = self.snapshot()
        code, output = self.run_checker('--dry-run')
        self.assertEqual(code, 2)
        self.assertIn('Would update ovs/versions.json', output)
        self.assertIn('Would update Makefile', output)
        self.assertEqual(self.snapshot(), before)

    def test_makefile_only_update(self):
        self.write_versions(EXPECTED)
        before = self.snapshot()
        code, _ = self.run_checker()
        self.assertEqual(code, 2)
        after = self.snapshot()
        self.assertEqual({p for p in before if before[p] != after[p]}, {'Makefile'})

    def test_versions_only_update(self):
        (self.directory / 'Makefile').write_text('OVS_VERSION ?= v3.8.0\n')
        before = self.snapshot()
        code, _ = self.run_checker()
        self.assertEqual(code, 2)
        after = self.snapshot()
        self.assertEqual({p for p in before if before[p] != after[p]}, {'ovs/versions.json'})

    def test_preserves_master_and_existing_release_metadata(self):
        images = [
            {'ovs_version': 'master', 'tag': 'latest', 'metadata': 'keep'},
            {'ovs_version': 'v3.8.0', 'tag': '3.8.0', 'metadata': 'keep'},
        ]
        updated = checker.update_versions(images, checker.get_latest_patches(TAGS))
        self.assertEqual(updated[:2], images)
        self.assertEqual(updated[2:], EXPECTED[2:])

    def test_selects_latest_patch_of_three_newest_minor_releases(self):
        tags = TAGS + ['v3.7.10', 'v3.10.0', 'v3.10.1', 'v3.10.0-rc1']
        self.assertEqual([str(v) for v in checker.get_latest_patches(tags)],
                         ['3.10.1', '3.8.0', '3.7.10'])


if __name__ == '__main__':
    unittest.main()
