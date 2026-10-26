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

class AdoptionContractTests(unittest.TestCase):
    def test_new_contracts_success_refusal_and_unavailable(self):
        server = fixture.ThreadingHTTPServer(('127.0.0.1', 0), fixture.Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        def post(path, data):
            request=Request(f'http://127.0.0.1:{server.server_port}'+path,data=json.dumps(data).encode(),headers={'Content-Type':'application/json','Idempotency-Key':'synthetic-adopt'})
            try: response=urlopen(request)
            except HTTPError as error: response=error
            with response:return response.status,json.load(response)
        inputs={
            'email.message.append':{'app':'gmail','endUserRef':'org_fixture','ref':'folder_fixture','content':'U3ViamVjdDogSGVsbG8NCg0KSGVsbG8='},
            'email.account.protocol-diagnose':{'app':'gmail','endUserRef':'org_fixture'},
            'email.directory.tenant.resolve':{'domain':'example.test'},
            'web.image.read':{'url':'https://image.example.test/a.png','consent':True},
            'workflow.webhook.signing-policy.create':{'name':'Synthetic policy'},
            'workflow.webhook.signing-policy.delete':{'ref':'policy_fixture'},
            'workflow.webhook.signed-call':{'url':'https://receiver.example.test/events','signaturePolicyRef':'policy_fixture','eventId':'synthetic-event','payload':{}},
            'ai.judgment.calibrated':{'question':'Which?','context':'Synthetic','options':[{'id':'yes','description':'Yes'},{'id':'no','description':'No'}]},
        }
        try:
            for mode,status in [('success',200),('refused',400),('unavailable',404),('uncertain',502)]:
                post('/fixture/adopt-mode',{'mode':mode})
                fixture.state['signing_policies']['policy_fixture']=True
                for capability,input in inputs.items():
                    with self.subTest(mode=mode,capability=capability):
                        fixture.state['signing_policies']['policy_fixture']=True
                        actual,body=post('/v1/capabilities/'+capability,input)
                        self.assertEqual(actual,202 if mode=='success' and capability=='workflow.webhook.signed-call' else status)
                        if mode=='success':
                            if capability=='email.message.append':self.assertEqual(body['status'],'saved')
                            if capability=='email.account.protocol-diagnose':self.assertEqual(body['sendingLimits'],[])
                            if capability=='workflow.webhook.signing-policy.create':self.assertTrue(body['secret'].startswith('whsec_'))
                            if capability=='ai.judgment.calibrated':self.assertAlmostEqual(sum(row['probability'] for row in body['results']),1)
            post('/fixture/adopt-mode',{'mode':'success'})
            post('/fixture/header-path',{'path':'diagnostic-only'})
            for capability in ['email.message.send','email.send']:
                message={'sender':'domain','from':'sender@fixture.email','to':['recipient@fixture.email'],'text':'Synthetic opt-out body','headers':{'X-Mailtrace-Verify':'nonce'}}
                self.assertEqual(post('/v1/capabilities/'+capability,message)[0],200)
                message['headers'].update({'List-Unsubscribe':'<https://app.example.test/stop>','List-Unsubscribe-Post':'List-Unsubscribe=One-Click'})
                status,body=post('/v1/capabilities/'+capability,message)
                self.assertEqual(status,400)
                self.assertIn('headers',body['error']['message'])
            post('/fixture/header-path',{'path':'full'})
            status,body=post('/v1/capabilities/web.image.read',{'url':'https://example.test/a.png','consent':False})
            self.assertEqual(status,400)
        finally:
            fixture.state['adopt_mode']='success';fixture.state['header_path']='full'
            server.shutdown();server.server_close();thread.join()
