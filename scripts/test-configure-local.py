"""Check first-run persistence and migration without using any service."""
import contextlib
import importlib.util
import io
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('configure', Path(__file__).with_name('configure-local.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class StartupTests(unittest.TestCase):
    def test_missing_key_and_private_values_are_stable(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / '.env.example').write_text('PRAMAAN_API_KEY=\n# SOCIAL_CLIENT_ID=\n')
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                module.configure(root)
            self.assertEqual(output.getvalue(), 'Add PRAMAAN_API_KEY to .env\n')
            first = (root / '.env').read_text()
            self.assertEqual((root / '.env').stat().st_mode & 0o777, 0o600)
            with contextlib.redirect_stdout(io.StringIO()):
                module.configure(root)
            self.assertEqual(first, (root / '.env').read_text())
            self.assertIn('# SOCIAL_CLIENT_ID=\n', first)
            self.assertEqual(first.count('AUTH_SECRET='), 1)

    def test_existing_and_legacy_keys_are_not_rotated(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / '.env').write_text('PRAMAAN_API_KEY=synthetic\nAUTH_SECRET=keep-this-value\n')
            (root / '.data').mkdir()
            (root / '.data' / 'instance.env').write_text('KMS_LOCAL_MASTER_KEY=legacy-local-key\n')
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                module.configure(root)
            text = (root / '.env').read_text()
            self.assertEqual(output.getvalue(), '')
            self.assertIn('AUTH_SECRET=keep-this-value\n', text)
            self.assertIn('KMS_LOCAL_MASTER_KEY=legacy-local-key\n', text)
            self.assertIn('PRAMAAN_API_KEY=synthetic\n', text)


if __name__ == '__main__':
    unittest.main()
