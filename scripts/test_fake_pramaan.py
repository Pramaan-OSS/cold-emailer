"""Transport checks for the loopback capability fixture."""
import importlib.util
import json
import os
from pathlib import Path
import threading
import subprocess
import unittest
from urllib.error import HTTPError
from urllib.request import Request, urlopen

checker = Path(__file__).resolve().parent.parent / '.scratch' / 'fake-client-tests' / 'schema-check'
checker.parent.mkdir(parents=True, exist_ok=True)
subprocess.run(['go', 'build', '-o', str(checker), './cmd/schema-check'], cwd=checker.parents[2], check=True)
os.environ['PRAMAAN_SCHEMA_CHECKER'] = str(checker)
spec = importlib.util.spec_from_file_location('fixture', Path(__file__).with_name('fake-pramaan.py'))
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)

class ActionHeaderTests(unittest.TestCase):
    def test_actions_require_idempotency_header(self):
        server = fixture.ThreadingHTTPServer(('127.0.0.1', 0), fixture.Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            for capability, status in [('auth.backend.provision', 200), ('auth.backend.domains.set', 200), ('db.postgres.provision', 502), ('auth.user.delete', 200), ('email.message.send', 200), ('payments.checkout.create', 200), ('payments.checkout.get', 200)]:
                for supplied in [False, True]:
                    with self.subTest(capability=capability, supplied=supplied):
                        headers = {'Content-Type': 'application/json'}
                        if supplied:
                            headers['Idempotency-Key'] = 'synthetic-stable-attempt'
                        payload = {'userId':'synthetic-member','confirm':True} if capability == 'auth.user.delete' else {'ref':'checkout_fixture'} if capability == 'payments.checkout.get' else {}
                        request = Request(f'http://127.0.0.1:{server.server_port}/v1/capabilities/{capability}', data=json.dumps(payload).encode(), headers=headers)
                        try:
                            response = urlopen(request)
                        except HTTPError as error:
                            response = error
                        with response:
                            body = json.load(response)
                            self.assertEqual(response.status, status if supplied else 400)
                            if not supplied:
                                self.assertEqual(body['error']['code'], 'invalid_request')
                                self.assertIn('Idempotency-Key header is required', body['error']['message'])
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

class EstimateHeaderTests(unittest.TestCase):
    def test_read_only_estimate_needs_no_action_header(self):
        server = fixture.ThreadingHTTPServer(('127.0.0.1', 0), fixture.Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            for capability in ['email.domain.create', 'companies.search']:
                request = Request(f'http://127.0.0.1:{server.server_port}/v1/capabilities/{capability}/estimate', data=b'{}', headers={'Content-Type': 'application/json'})
                with urlopen(request) as response:
                    self.assertEqual(response.status, 200)
                    self.assertTrue(json.load(response)['can_run'])
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

class RecipientTests(unittest.TestCase):
    def test_reserved_recipients_refused_by_fixture(self):
        server = fixture.ThreadingHTTPServer(('127.0.0.1', 0), fixture.Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            for capability in ['email.send', 'email.message.send', 'email.message.reply', 'email.send-bulk']:
                for address, expected in [('synthetic@example.test', 400), ('recipient@pramaan.dev', 200)]:
                    with self.subTest(capability=capability, address=address):
                        payload = {'to': [address]}
                        if capability == 'email.send-bulk': payload = {'rows': [payload]}
                        request = Request(f'http://127.0.0.1:{server.server_port}/v1/capabilities/{capability}', data=json.dumps(payload).encode(), headers={'Content-Type': 'application/json', 'Idempotency-Key': 'synthetic-recipient-check'})
                        try: response = urlopen(request)
                        except HTTPError as error: response = error
                        with response:
                            body = json.load(response)
                            self.assertEqual(response.status, expected)
                            if expected == 400: self.assertIn('reserved', body['error']['message'])
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

if __name__ == '__main__':
    unittest.main()
