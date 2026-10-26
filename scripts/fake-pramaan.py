"""Local QA fixture: synthetic records, no outgoing network calls."""
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import json,uuid,hashlib
import subprocess
import os
from pathlib import Path

# Use the production validator; fixture schemas cannot silently drift.
root = Path(__file__).resolve().parent.parent
schema_checker = Path(os.environ.get("PRAMAAN_SCHEMA_CHECKER", root / ".data" / "schema-check"))
if not os.environ.get("PRAMAAN_SCHEMA_CHECKER"):
 schema_checker.parent.mkdir(exist_ok=True)
 subprocess.run(["go", "build", "-o", str(schema_checker), "./cmd/schema-check"], cwd=root, check=True)
state={'header_path':'full','adopt_mode':'success','adopt_calls':[], 'company_refs':[], 'signing_policies':{}, 'generation_calls':0, 'generation_mode':'complete', 'unavailable':[], 'inboxes':[{'address':'cold-email-review@example.test','ref':'inbox-local-review','displayName':'Cold Email review','status':'active'}],'domains':[], 'registrations':[], 'domain_creates':0, 'domain_mode':'available', 'domain_name':'synthetic-outreach.example', 'plans':{}, 'jobs':{}, 'outcome_runs':0, 'outcome_estimates':0,'remote_connectors':[], 'remote_calls':0, 'remote_mode':'unknown', 'enterprise_connections':[], 'enterprise_starts':{}, 'enterprise_used':[], 'enterprise_exchanges':0, 'connections':[{'app':'connected-crm','status':'active'}]}
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def reply(self, body, status=200):
  self.send_response(status);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps(body).encode())
 def do_GET(self):
  if self.path=='/fixture/adopt':return self.reply({'calls':state['adopt_calls'],'mode':state['adopt_mode']})
  if self.path.startswith('/v1/companies/mail/'):
   ref=self.path.split('/')[-1]
   if state['adopt_mode']=='unavailable':return self.reply({'error':{'code':'not_found','effect':'none'}},404)
   if ref not in state['company_refs']:return self.reply({'error':{'code':'not_found','effect':'none'}},404)
   return self.reply({'companyRef':ref,'status':'connected'})
  if self.path=='/fixture/image.png':
   import base64
   data=base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jC1sAAAAASUVORK5CYII=')
   self.send_response(200);self.send_header('Content-Type','image/png');self.end_headers();return self.wfile.write(data)
  detail={'id':'lead.find' ,'description':'Find company leads','available':True,'effect_class':'read_only','mode':'async','personal_data':True,'steps':[]}
  if self.path=='/fixture/generation':return self.reply({'calls':state['generation_calls']})
  if self.path=='/v1/outcomes':return self.reply({'results':[detail]})
  if self.path=='/v1/outcomes/lead.find':return self.reply(detail)
  if self.path.startswith('/v1/jobs/'):
   job=state['jobs'].get(self.path.split('/')[-1]);return self.reply(job or {'error':{'code':'job_not_found','effect':'none'}},200 if job else 404)
  if self.path.startswith('/fixture/enterprise/link/'):
   item=state['enterprise_starts'].get(self.path.split('/')[-1]);return self.reply(item or {},200 if item else 404)
  if self.path=='/fixture/enterprise':return self.reply({'connections':len(state['enterprise_connections']),'exchanges':state['enterprise_exchanges']})
  if self.path=='/fixture/connectors':return self.reply({'calls':state['remote_calls'],'registered':len(state['remote_connectors'])})
  if self.path=='/fixture/domains':return self.reply({'creates':state['domain_creates'],'registrations':state['registrations']})
  if self.path=='/fixture/outcomes':return self.reply({'runs':state['outcome_runs'],'estimates':state['outcome_estimates']})
  return self.reply({'error':{'code':'not_found','effect':'none'}},404)
 def do_DELETE(self):
  if self.path.startswith('/v1/companies/mail/'):
   if state['adopt_mode']=='unavailable':return self.reply({'error':{'code':'not_found','effect':'none'}},404)
   state['company_refs']=[ref for ref in state['company_refs'] if ref!=self.path.split('/')[-1]];return self.reply({'revoked':True})
  return self.reply({'error':{'code':'not_found','effect':'none'}},404)
 def do_POST(self):
  cap=self.path.split('/')[-1];data=json.loads(self.rfile.read(int(self.headers.get('Content-Length','0'))) or '{}');out={}
  if self.path.startswith('/v1/capabilities/') and not self.path.endswith('/estimate') and not self.headers.get('Idempotency-Key','').strip():
   return self.reply({'error':{'code':'invalid_request','effect':'none','message':'Idempotency-Key header is required for this capability.'}},400)
  if self.path=='/fixture/header-path':
   state['header_path']=data.get('path','full');return self.reply({'set':True})
  if self.path=='/fixture/company-authorize':
   state['company_refs'].append('company_office_fixture');return self.reply({'companyRef':'company_office_fixture'})
  if self.path=='/fixture/adopt-mode':
   state['adopt_mode']=data.get('mode','success');return self.reply({'set':True})
  adoption_ids=('email.message.append','email.account.protocol-diagnose','email.folders.list','email.directory.tenant.resolve','web.image.read','workflow.webhook.signing-policy.create','workflow.webhook.signing-policy.delete','workflow.webhook.signed-call','ai.judgment.calibrated')
  if cap in adoption_ids or self.path.startswith('/v1/companies/mail/') or (cap in ('email.message.send','email.send') and data.get('headers')):
   state['adopt_calls'].append({'path':self.path,'input':data})
   mode=state['adopt_mode']
   if mode=='unavailable':return self.reply({'error':{'code':'not_found','effect':'none'}},404)
   if mode=='refused':return self.reply({'error':{'code':'invalid_request','effect':'none','message':'The selected path refused these headers.' if data.get('headers') else 'Request refused.'}},400)
   if mode=='uncertain':return self.reply({'error':{'code':'upstream_unavailable','effect':'unknown'}},502)
   if self.path=='/v1/companies/mail/connect':
    if data.get('kind')=='office':return self.reply({'authorizationUrl':'https://api.pramaan.dev/connect/company/start/synthetic'})
    if data.get('kind')=='workspace' and not data.get('mailbox'):return self.reply({'setupUrl':'https://api.pramaan.dev/connect/company/setup','proofName':'_proof.example.test','proofValue':'synthetic-proof'})
    ref='company_fixture';state['company_refs'].append(ref);return self.reply({'companyRef':ref})
   if cap=='email.message.append':
    import base64
    try:valid=data.get('app') in ('gmail','outlook') and data.get('ref')=='folder_fixture' and len(base64.b64decode(data.get('content',''),validate=True))<=10*1024*1024
    except Exception:valid=False
    if not valid:return self.reply({'error':{'code':'invalid_request','effect':'none'}},400)
    return self.reply({'status':'saved','ref':'message_saved_fixture'})
   if cap=='email.account.protocol-diagnose':return self.reply({'mailboxType':'label_based' if data.get('app')=='gmail' else 'folder_based','protocol':'mailbox_api','health':'connected','features':['send','folders','raw_append'],'sendingLimits':[]})
   if cap=='email.folders.list':return self.reply({'results':[{'ref':'folder_fixture','name':'Sent','system':True}]})
   if cap=='email.directory.tenant.resolve':return self.reply({'domain':data.get('domain'),'discovered':True,'ref':'tenant_fixture'})
   if cap=='web.image.read':
    if data.get('consent') is not True:return self.reply({'error':{'code':'invalid_request','effect':'none'}},400)
    return self.reply({'fileId':'file_image_fixture','url':f'http://127.0.0.1:{self.server.server_port}/fixture/image.png','contentType':'image/png','sizeBytes':68,'expiresAt':'2026-10-27T12:00:00Z'})
   if cap=='workflow.webhook.signing-policy.create':
    ref='policy_'+uuid.uuid4().hex;state['signing_policies'][ref]=True;return self.reply({'ref':ref,'secret':'whsec_'+('A'*43)+'='})
   if cap=='workflow.webhook.signing-policy.delete':
    state['signing_policies'].pop(data.get('ref'),None);return self.reply({'deleted':True})
   if cap=='workflow.webhook.signed-call':
    if data.get('signaturePolicyRef') not in state['signing_policies']:return self.reply({'error':{'code':'not_found','effect':'none'}},404)
    job=uuid.uuid4().hex;state['jobs'][job]={'job_id':job,'status':'done','result':{'eventId':data['eventId'],'status':'accepted','receivedAt':'2026-10-26T12:00:00Z'}};return self.reply({'job_id':job,'status':'running'},202)
   if cap=='ai.judgment.calibrated':return self.reply({'results':[{'optionId':o['id'],'probability':1/len(data['options'])} for o in data['options']]})
   if cap in ('email.message.send','email.send') and state['header_path']=='diagnostic-only' and any(k.lower().startswith('list-') for k in data.get('headers',{})):return self.reply({'error':{'code':'invalid_request','effect':'none','message':'The selected path accepts only diagnostic X- headers; List-Unsubscribe headers were refused.'}},400)
   if cap in ('email.message.send','email.send'):return self.reply({'ref':'message_fixture','status':'sent','messageId':'<fixture@example.test>'})
  if cap=='billing.price.get':return self.reply({'unitPrice':2000,'currency':'usd','renewalIntervalUnit':'month','renewalIntervalCount':1,'category':'subscription','scheme':'standard'})
  if cap=='payments.checkout.create':return self.reply({'ref':'checkout_fixture','checkoutUrl':'https://checkout.example.test/pay'})
  if cap=='payments.checkout.get':
   if data.get('ref')!='checkout_fixture':return self.reply({'error':{'code':'not_found','effect':'none','message':'Checkout not found.'}},404)
   return self.reply({'status':'unknown','payments':[]})
  if cap=='auth.user.delete':
   if not data.get('userId') or data.get('confirm') is not True:return self.reply({'error':{'code':'invalid_request','effect':'none','message':'Deletion requires a user reference and confirmation.'}},400)
   return self.reply({'deleted':True})
  if cap=='db.postgres.provision':return self.reply({'error':{'code':'upstream_unavailable','effect':'none'}},502)
  if cap=='auth.token.verify':
   import base64
   try:
    raw=data.get('token','');parts=raw.split('.');claims=json.loads(base64.urlsafe_b64decode(parts[1]+'='*(-len(parts[1])%4)));valid=parts[2]=='synthetic-fixture-only' and claims.get('sub')=='managed-fixture-member' and claims.get('email')=='managed-check@example.test'
   except Exception:valid=False;claims={}
   return self.reply({'valid':True,'userId':claims['sub']} if valid else {'valid':False})
  if cap=='auth.backend.provision':return self.reply({'status':'ready','authUrl':'https://managed-signin.example.test/auth','jwksUrl':'https://managed-signin.example.test/auth/.well-known/jwks.json'})
  if cap=='auth.backend.domains.set':return self.reply({'allowedDomains':data.get('domains',[])})
  if cap in ('llm.chat','llm.complete','llm.complete-bulk','document.extract','email.send','email.message.send','email.message.reply','email.send-bulk'):
   checked=subprocess.run([str(schema_checker)],input=json.dumps({'capability':cap,'input':data}),text=True,capture_output=True)
   if checked.returncode:return self.reply({'error':{'code':'invalid_request','effect':'none','message':checked.stderr.strip()}},400)
  if cap in ('email.send','email.message.send','email.message.reply','email.send-bulk'):return self.reply({'ref':'email_fixture','status':'queued'})
  if cap in ('llm.chat','llm.complete'):state['generation_calls']+=1
  if self.path=='/fixture/generation-mode':
   state['generation_mode']=data.get('mode','complete');return self.reply({'set':True})
  if cap in ('llm.chat','llm.complete') and state['generation_mode']=='limit':return self.reply({'error':{'code':'invalid_request','effect':'none','message':'maxTokens is too small'}},400)
  if cap in ('llm.chat','llm.complete') and state['generation_mode']=='length':return self.reply({'text':'Synthetic partial reply','finishReason':'length','usage':{'inputTokens':10,'outputTokens':20}})
  if self.path=='/fixture/unavailable':
   state['unavailable']=data.get('capabilities',[]);return self.reply({'set':True})
  if cap in state['unavailable']:return self.reply({'error':{'code':'upstream_unavailable','effect':'none'}},502)
  if cap=='auth.enterprise-connection.create':
   import datetime
   row={'ref':'enterprise-'+uuid.uuid4().hex,'organizationRef':data['organizationRef'],'name':data['name'],'status':'ready','createdAt':datetime.datetime.now(datetime.timezone.utc).isoformat()};state['enterprise_connections'].append(row);return self.reply(row)
  if cap=='auth.enterprise-connections.list':return self.reply({'results':[row for row in state['enterprise_connections'] if row['organizationRef']==data.get('organizationRef')]})
  if cap=='auth.enterprise-connection.delete':
   state['enterprise_connections']=[row for row in state['enterprise_connections'] if row['ref']!=data['ref']];return self.reply({'deleted':True})
  if cap=='auth.enterprise.start':
   if not any(row['ref']==data['connectionRef'] for row in state['enterprise_connections']):return self.reply({'error':{'code':'not_found','effect':'none'}},404)
   token=uuid.uuid4().hex;state['enterprise_starts'][token]={**data,'code':'synthetic-code-'+token,'used':False}
   import datetime
   return self.reply({'authorizationUrl':'https://api.pramaan.dev/connect/link/'+token,'expiresAt':(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(minutes=10)).isoformat()})
  if cap=='auth.social.exchange' and data.get('connectionRef'):
   row=next((row for row in state['enterprise_starts'].values() if row['state']==data.get('state')),None)
   if not row or row['used'] or row['connectionRef']!=data.get('connectionRef') or row['code']!=data.get('code') or row['redirectUri']!=data.get('redirectUri') or row['nonce']!=data.get('expectedNonce'):return self.reply({'error':{'code':'forbidden','effect':'none'}},403)
   import base64
   challenge=base64.urlsafe_b64encode(hashlib.sha256(data.get('pkceVerifier','').encode()).digest()).rstrip(b'=').decode()
   if challenge!=row['codeChallenge']:return self.reply({'error':{'code':'forbidden','effect':'none'}},403)
   row['used']=True;state['enterprise_exchanges']+=1
   return self.reply({'providerKey':'enterprise','subjectId':'synthetic-enterprise-member','email':'untrusted-label@example.test','emailVerified':False,'displayName':'Synthetic member'})
  if self.path.startswith('/fixture/connector-authorize/'):
   ref=self.path.split('/')[-1]
   for row in state['remote_connectors']:
    if row['connectorRef']==ref:row['state']='ready'
   return self.reply({'authorized':True})
  if cap=='connector.connection.register':
   row={'connectorRef':'cn_'+uuid.uuid4().hex,'state':'pending-auth' if data['auth']['kind']=='oauth' else 'ready','authKind':data['auth']['kind'],'label':data.get('label','Synthetic tools'),'createdAt':'2026-10-12T10:00:00Z','updatedAt':'2026-10-12T10:00:00Z','owner':data['owner']};state['remote_connectors'].append(row);out={k:v for k,v in row.items() if k!='owner'}
   if row['state']=='pending-auth':out.update({'connectUrl':'https://example.test/connector-sign-in/'+row['connectorRef'],'expiresAt':'2026-10-12T10:10:00Z'})
   return self.reply(out)
  if cap=='connector.connections.list':return self.reply({'results':[{k:v for k,v in row.items() if k!='owner'} for row in state['remote_connectors'] if row['owner']==data['owner']]})
  if cap=='connector.tools.list':return self.reply({'results':[{'toolRef':'tl_'+'a'*48,'schemaDigest':'synthetic_digest','description':'Find synthetic resources','inputSchema':{'type':'object','properties':{'query':{'type':'string'}}},'approvalRequired':True}],'discoveredAt':'2026-10-12T10:00:05Z'})
  if cap=='connector.connection.update':
   row=next(row for row in state['remote_connectors'] if row['connectorRef']==data['connectorRef'])
   if 'label' in data:row['label']=data['label']
   return self.reply({k:v for k,v in row.items() if k!='owner'})
  if cap=='connector.connection.disconnect':
   state['remote_connectors']=[row for row in state['remote_connectors'] if row['connectorRef']!=data['connectorRef']];return self.reply({'connectorRef':data['connectorRef'],'status':'disconnected','disconnectedAt':'2026-10-12T10:00:00Z'})
  if cap=='connector.tool.call':
   state['remote_calls']+=1;return self.reply({'status':'unknown','output':{},'errorCode':'completion-unknown'})
  if self.path=='/fixture/domain-mode':
   state['domain_mode']=data.get('mode','available');state['domain_name']=data.get('domain',state['domain_name']);return self.reply({'mode':state['domain_mode']})
  if self.path=='/fixture/reset-connections':
   state['connections']=[{'app':'connected-crm','status':'active'}];return self.reply({'reset':True})
  if self.path.startswith('/v1/jobs/') and self.path.endswith('/cancel'):
   job=state['jobs'].get(self.path.split('/')[-2]);
   if not job:return self.reply({'error':{'code':'job_not_found','effect':'none'}},404)
   job['status']='cancelled';return self.reply(job)
  if self.path=='/v1/outcomes/lead.find/estimate':
   state['outcome_estimates']+=1;plan=hashlib.sha256(json.dumps(data,sort_keys=True).encode()).hexdigest();state['plans'][plan]=data
   return self.reply({'outcome':'lead.find','plan':plan,'ready':True,'can_run':True,'price_max_micro':4200,'steps':[{'id':'companies','label':'Company search','capability':'companies.search','status':'will_run'},{'id':'people','label':'Contact match and email check','capability':'people.search','status':'will_run'}]})
  if self.path=='/v1/outcomes/lead.find':
   plan=self.headers.get('Pramaan-Plan');known=state['plans'].get(plan)
   if not plan:return self.reply({'error':{'code':'forbidden','reason':'approval_required','effect':'none'}},403)
   if known!=data:return self.reply({'error':{'code':'conflict','effect':'none'}},409)
   state['outcome_runs']+=1;job=str(uuid.uuid4());state['jobs'][job]={'job_id':job,'status':'done','result':{'outcome':'lead.find','plan':plan,'status':'done','completeness':'complete','result':{'leads':[{'company':{'name':'Synthetic software team','domain':'example.test'},'person':{'fullName':'Synthetic reviewer','title':'Head of sales'},'email':'reviewer@example.test','emailStatus':'verified'}]},'steps':[],'evidence':{},'missing':[],'cost':{'charged_micro':1000,'hold_micro':4200}}}
   return self.reply({'job_id':job,'status':'running'},202)
  if self.path=='/v1/capabilities/companies.search/estimate':return self.reply({'price_max_micro':1000,'can_run':True})
  if self.path=='/v1/capabilities/email.domain.create/estimate':return self.reply({'price_max_micro':2600000,'can_run':True})
  if cap.startswith('domains.registration.') and state['domain_mode']=='unavailable':return self.reply({'error':{'code':'upstream_unavailable','effect':'none'}},502)
  if cap=='domains.registration.search':return self.reply({'results':[{'domain':state['domain_name'],'available':True,'premium':False,'purchaseMode':'registration','years':1,'price':{'amount':1500,'currency':'USD'}}]})
  if cap=='domains.registration.quote':return self.reply({'domain':data['domain'],'action':'register','years':data.get('years',1),'premium':False,'price':{'amount':1500,'currency':'USD'}})
  if cap=='domains.registration.create':
   state['domain_creates']+=1;out={'domain':data['domain'],'ref':'registration-'+str(uuid.uuid4()),'state':'active','expiresAt':'2027-10-12T00:00:00Z','verification':{'state':'pending'},'price':data['price']};state['registrations'].append(out)
   if state['domain_mode']=='unknown':return self.reply({'error':{'code':'upstream_unavailable','effect':'unknown'}},502)
   return self.reply(out)
  if cap=='domains.registration.list':return self.reply({'results':state['registrations']})
  if cap=='domains.registration.get':return self.reply(next(row for row in state['registrations'] if row['ref']==data['ref']))
  if cap=='domains.dns-records.list':return self.reply({'records':[{'type':'TXT','name':'@','value':'synthetic-verification','ttl':300}]})
  if cap=='domains.dns-records.apply':return self.reply({'results':[{'operation':'set','record':row,'status':'created'} for row in data['set']],'changed':len(data['set'])})
  if cap=='email.domain.create':
   out={'domain':data['domain'],'ref':'mail-domain-'+str(uuid.uuid4()),'status':'pending','dnsRecords':[{'type':'TXT','name':'_proof.'+data['domain'],'value':'synthetic-verification'}]};state['domains'].append(out);return self.reply(out)
  if cap=='email.domain.get':return self.reply(next(row for row in state['domains'] if row['domain']==data['domain']))
  if cap=='companies.search':return self.reply({'results':[{'name':'Synthetic software team','domain':'example.test'}]})
  if cap=='email.inbox.create':
   out={'address':data.get('username','review')+'@example.test','displayName':data.get('displayName',''),'ref':'inbox-'+str(uuid.uuid4()),'status':'active'};state['inboxes'].append(out)
  elif cap=='email.inboxes.list':out={'results':state['inboxes']}
  elif cap in ('email.messages.list','email.threads.list','email.messages.search'):out={'results':[]}
  elif cap=='email.accounts.list':out={'results':[]}
  elif cap=='messaging.opt-out.check':out={'blocked':False}
  elif cap=='apps.catalog.search' and data.get('query')=='read synthetic file':
   out={'results':[{'app':'fixture_app','action':'FILE_DOWNLOAD','description':'Read synthetic document','inputSchema':{'type':'object','properties':{}}}]} if data.get('cursor') else {'results':[{'app':'fixture_app','action':'FILE_METADATA','description':'Inspect synthetic document','inputSchema':{'type':'object','properties':{}}}],'nextCursor':'fixture-next'}
  elif cap=='apps.catalog.search':out={'results':[{'app':'connected-crm','action':'CONTACT_UPSERT','description':'Create or update a contact.','inputSchema':{'type':'object','properties':{'email':{'type':'string','title':'Contact email'}},'required':['email']}}]}
  elif cap=='apps.connection.revoke':
   before=len(state['connections']);state['connections']=[row for row in state['connections'] if row['app']!=data.get('app')];out={'revoked':before-len(state['connections'])}
  elif cap=='apps.connections.list':out={'results':state['connections']}
  elif cap=='apps.action.execute' and data.get('action')=='FILE_DOWNLOAD':out={'status':'succeeded','output':{'downloaded_file_content':{'fileId':'file_synthetic_document','url':'https://example.test/unused-staging','contentType':'application/pdf','sizeBytes':20}}}
  elif cap=='document.parse':out={'text':'Synthetic parsed file text.','truncated':False}
  elif cap=='apps.action.execute':out={'status':'succeeded','output':{'results':[{'name':'Synthetic saved contact'}],'display_url':'https://example.test/result'}}
  elif cap=='people.email.verify':out={'email':data['email'],'status':'valid'}
  elif cap=='security.dns.lookup':out={'records':[]}
  elif cap=='llm.chat':out={'text':'This is the local test assistant. Your workspace is ready.','usage':{'inputTokens':10,'outputTokens':12}}
  elif cap=='apps.trigger-deployments.list':out={'results':[]}
  else:
   self.send_response(501);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(b'{"error":"Unavailable in the local fixture."}');return
  self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps(out).encode())
if __name__ == '__main__':ThreadingHTTPServer(('127.0.0.1',59090),Handler).serve_forever()
