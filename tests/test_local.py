import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import zipfile

from tools import local


class LocalCommands(unittest.TestCase):
    def test_failed_build_stops_all_before_tests_or_packaging(self):
        with patch.object(local, 'require'), patch.object(local, 'run') as run:
            run.side_effect = subprocess.CalledProcessError(1, 'build')
            self.assertEqual(local.main(['all']), 1)
            self.assertEqual(run.call_count, 1)
            self.assertEqual(run.call_args.args[0][-1], 'tools/build.py')

    def test_tests_use_host_target_and_enable_linux_race_detector(self):
        cross = {'GOOS': 'linux', 'GOARCH': 'arm', 'GOARM': '7', 'CGO_ENABLED': '0'}
        with patch.dict(os.environ, cross), patch.object(local, 'require'), \
                patch.object(local, 'build'), patch.object(local, 'run') as run, \
                patch.object(local.platform, 'system', return_value='Linux'):
            local.test()
        go_calls = [call for call in run.call_args_list if call.args[0][0] == 'go']
        self.assertEqual(len(go_calls), 3)
        for call in go_calls:
            for name in ('GOOS', 'GOARCH', 'GOARM'):
                self.assertNotIn(name, call.kwargs['env'])
        self.assertEqual(go_calls[-1].args[0], ['go', 'test', '-tags=nethttpomithttp2', '-race', './...'])
        self.assertEqual(go_calls[-1].kwargs['env']['CGO_ENABLED'], '1')

    def test_windows_runs_checks_without_linux_race_test(self):
        with patch.object(local, 'require'), patch.object(local, 'build'), \
                patch.object(local, 'run') as run, \
                patch.object(local.platform, 'system', return_value='Windows'):
            local.test()
        commands = [call.args[0] for call in run.call_args_list]
        self.assertFalse(any('-race' in command for command in commands))
        self.assertIn(['node', '--check', 'controller/web/app.js'], commands)
        self.assertIn(['node', '--check', 'controller/web/settings.js'], commands)

    def test_source_archive_contains_head_not_local_edits_or_untracked_files(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)

            def git(*args):
                subprocess.run(['git', *args], cwd=root, check=True,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE)

            git('init')
            (root / 'LICENSE').write_text('committed license')
            git('add', 'LICENSE')
            git('-c', 'user.name=Local Test', '-c', 'user.email=test@example.invalid',
                '-c', 'commit.gpgsign=false', 'commit', '-m', 'fixture')
            (root / 'LICENSE').write_text('uncommitted edit')
            (root / 'private.txt').write_text('untracked')
            with patch.object(local, 'ROOT', root):
                self.assertEqual(local.main(['source']), 0)
            with zipfile.ZipFile(root / 'dist/kdbox-xbt-source.zip') as archive:
                self.assertEqual(archive.namelist(), ['kdbox-xbt/', 'kdbox-xbt/LICENSE'])
                self.assertEqual(archive.read('kdbox-xbt/LICENSE'), b'committed license')


if __name__ == '__main__':
    unittest.main()
