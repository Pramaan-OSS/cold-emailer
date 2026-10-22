import {beforeEach,describe,expect,it,vi} from 'vitest';
vi.mock('@/lib/information',()=>({API_BASE_URL:'http://localhost:18080/v1',TOKEN_KEY:'auth_token'}));
import {capability} from './pramaan';
const token={access_token:'current-app-session',refresh_token:'test-refresh',access_token_expires_at:'2030-01-01T00:00:00Z',refresh_token_expires_at:'2030-02-01T00:00:00Z'};
beforeEach(()=>{vi.mocked(localStorage.getItem).mockImplementation(key=>key==='auth_token'?JSON.stringify(token):key==='access_token'?'stale-legacy-session':null);});
describe('Pramaan app-session bridge',()=>{
 it('uses the current app session and preserves the action key',async()=>{const fetcher=vi.fn(async()=>new Response(JSON.stringify({saved:true}),{status:200,headers:{'content-type':'application/json'}}));vi.stubGlobal('fetch',fetcher);await capability('apps.action.execute',{action:'fake-action'},'11111111-1111-4111-8111-111111111111');const [url,options]=fetcher.mock.calls[0] as unknown as [string,RequestInit];expect(url).toBe('http://localhost:18080/v1/auth/pramaan/v1/capabilities/apps.action.execute');const headers=new Headers(options.headers);expect(headers.get('authorization')).toBe('Bearer current-app-session');expect(headers.get('idempotency-key')).toBe('11111111-1111-4111-8111-111111111111');vi.unstubAllGlobals();});
 it('does not retry a failed write or show remote service details',async()=>{const fetcher=vi.fn(async()=>new Response(JSON.stringify({error:{message:'provider-secret $99'}}),{status:500,headers:{'content-type':'application/json'}}));vi.stubGlobal('fetch',fetcher);await expect(capability('apps.action.execute',{action:'fake-action'})).rejects.not.toThrow('provider-secret');expect(fetcher).toHaveBeenCalledTimes(1);vi.unstubAllGlobals();});
});

import {outcomes, OutcomeError} from './pramaan';
import {afterEach} from 'vitest';
afterEach(()=>vi.unstubAllGlobals());
const response=(body:unknown,status=200)=>new Response(JSON.stringify(body),{status,headers:{'content-type':'application/json'}});
describe('Outcome SDK app-session transport',()=>{
 it('lists and describes runtime availability, estimates the exact body and runs with approval headers',async()=>{
  const fetcher=vi.fn(async(url:RequestInfo|URL)=>{const path=String(url);if(path.endsWith('/estimate'))return response({plan:'plan-1',ready:true,can_run:true,price_max_micro:100,steps:[]});if(path.endsWith('/outcomes'))return response({results:[{id:'lead.find',available:true}]});return response({id:'lead.find',available:true,outcome:'lead.find',status:'done',result:{leads:[]},steps:[],cost:{}})});vi.stubGlobal('fetch',fetcher);
  expect((await outcomes.list())[0].available).toBe(true);expect((await outcomes.get('lead.find')).available).toBe(true);
  const input={description:'Software companies',roles:['sales']};const opts={basis:{ground:'consent' as const},options:{max:5}};const estimate=await outcomes.estimate('lead.find',input,opts);await outcomes.run('lead.find',input,{...opts,plan:estimate.plan,idempotencyKey:'request-1'});
  const calls=fetcher.mock.calls as unknown as [string,RequestInit][];expect(JSON.parse(String(calls[2][1].body))).toEqual(JSON.parse(String(calls[3][1].body)));const headers=new Headers(calls[3][1].headers);expect(headers.get('Pramaan-Plan')).toBe('plan-1');expect(headers.get('Authorization')).toBe('Bearer current-app-session');for(const h of ['Pramaan-Account','Pramaan-End-User','Pramaan-Max-Price-Micro'])expect(headers.get(h)).toBeNull();
 });
 it('waits for a 202 job without resubmitting the outcome',async()=>{const fetcher=vi.fn(async(url:RequestInfo|URL)=>String(url).includes('/jobs/')?response({job_id:'job-1',status:'done',result:{outcome:'lead.find',status:'done',result:{leads:[]},steps:[],cost:{}}}):response({job_id:'job-1',status:'running'},202));vi.stubGlobal('fetch',fetcher);expect((await outcomes.run('lead.find',{}, {plan:'p',idempotencyKey:'same-key'})).status).toBe('done');expect(fetcher.mock.calls.filter(c=>String(c[0]).includes('/outcomes/'))).toHaveLength(1);expect(fetcher.mock.calls.some(c=>String(c[0]).endsWith('/jobs/job-1'))).toBe(true);});
 for(const [status,code] of [[403,'forbidden'],[409,'conflict'],[502,'upstream_unavailable']] as const) it(`preserves ${status} safely without retries`,async()=>{const fetcher=vi.fn(async()=>response({error:{code,effect:'none',message:'private-upstream-details'}},status));vi.stubGlobal('fetch',fetcher);try{await outcomes.run('lead.find',{}, {plan:'p'});throw new Error('expected refusal');}catch(e){expect(e).toBeInstanceOf(OutcomeError);expect(e).toMatchObject({code,status,effect:'none'});expect((e as Error).message).not.toContain('private-upstream-details');}expect(fetcher).toHaveBeenCalledTimes(1);});
});

import {connectedResultRows,connectedDisplayURL,pollingFetch} from './pramaan';
describe('Connected result parsing and busy outcomes',()=>{
 it('accepts list output and safe display links, rejecting malformed rows and unsafe links',()=>{expect(connectedResultRows({output:{results:[null,'bad',{name:'row'}],display_url:'https://example.test/result'}})).toEqual([{name:'row'}]);expect(connectedDisplayURL({output:{display_url:'https://example.test/result'}})).toBe('https://example.test/result');expect(connectedDisplayURL({output:{display_url:'javascript:alert(1)'}})).toBeUndefined();expect(connectedResultRows({results:'wrong'})).toEqual([]);});
 it('reports a capacity refusal as try later without resubmitting',async()=>{const fetcher=vi.fn(async()=>response({error:{code:'rate_limited',effect:'none'}},429));vi.stubGlobal('fetch',fetcher);await expect(outcomes.run('lead.find',{}, {plan:'p'})).rejects.toThrow('Try again later');expect(fetcher).toHaveBeenCalledTimes(1);});
 it('retries only the same job read after 429',async()=>{const fetcher=vi.fn().mockResolvedValueOnce(response({},429)).mockResolvedValueOnce(response({status:'done'}));vi.stubGlobal('fetch',fetcher);const sleep=vi.fn(async()=>{});await pollingFetch('http://localhost:18080/v1/jobs/job-1',{method:'GET'},sleep);expect(fetcher).toHaveBeenCalledTimes(2);expect(fetcher.mock.calls.every(c=>c[0].endsWith('/jobs/job-1')&&c[1].method==='GET')).toBe(true);});
 it('cancels an owned outcome job once through the SDK bridge',async()=>{const fetcher=vi.fn(async(_url:RequestInfo|URL)=>response({job_id:'job-1',status:'cancelled',result:{status:'declined'}}));vi.stubGlobal('fetch',fetcher);expect((await outcomes.cancel('job-1')).status).toBe('cancelled');expect(fetcher).toHaveBeenCalledTimes(1);expect(String(fetcher.mock.calls[0][0])).toContain('/jobs/job-1/cancel');});
});

import { domainCapability, DomainError } from './pramaan';
describe('Domain SDK transport',()=>{
 for(const effect of ['none','unknown'] as const)it(`keeps ${effect} purchase effects distinct and sends once`,async()=>{const fetcher=vi.fn(async()=>response({error:{code:'upstream_unavailable',effect,message:'private-remote-message'}},502));vi.stubGlobal('fetch',fetcher);try{await domainCapability('domains.registration.create',{},'22222222-2222-4222-8222-222222222222');throw new Error('expected refusal');}catch(e){expect(e).toBeInstanceOf(DomainError);expect((e as DomainError).code).toBe(effect==='none'?'unavailable':'upstream_unavailable');expect((e as Error).message).not.toContain('private-remote-message');}expect(fetcher).toHaveBeenCalledTimes(1);const headers=new Headers((fetcher.mock.calls[0] as unknown as [string,RequestInit])[1].headers);expect(headers.get('Idempotency-Key')).toBe('22222222-2222-4222-8222-222222222222');});
});

import {createCatalogueCache,connectedFiles,parseConnectedFile} from './pramaan';
describe('Paged catalogue and saved connected files',()=>{
 it('loads exactly two requested pages, caches repeats and isolates sessions/workspaces',async()=>{const fetchPage=vi.fn().mockResolvedValueOnce({results:[{action:'FIRST'}],nextCursor:'opaque-next'}).mockResolvedValueOnce({results:[{action:'LAST'}]}).mockResolvedValueOnce({results:[]});const page=createCatalogueCache(fetchPage);const input={query:'Read a file',app:'fixture_app',limit:50};const first=await page(input,'session-one/workspace-one');expect(first.nextCursor).toBe('opaque-next');expect(fetchPage).toHaveBeenCalledTimes(1);first.results.length=0;expect((await page(input,'session-one/workspace-one')).results).toHaveLength(1);const second=await page({...input,cursor:first.nextCursor},'session-one/workspace-one');expect(second.nextCursor).toBeUndefined();expect(fetchPage).toHaveBeenCalledTimes(2);await page({...input,cursor:first.nextCursor},'session-one/workspace-one');expect(fetchPage).toHaveBeenCalledTimes(2);await page(input,'session-two/workspace-two');expect(fetchPage).toHaveBeenCalledTimes(3);expect(fetchPage.mock.calls[1][0]).toEqual({...input,cursor:'opaque-next'});});
 it('takes a returned fileId directly to parse without any staging fetch/upload or repeat execution',async()=>{const fetcher=vi.fn().mockResolvedValueOnce(response({status:'succeeded',output:{downloaded_file_content:{fileId:'file_fixture',url:'https://example.test/staging'}}})).mockResolvedValueOnce(response({text:'Parsed synthetic text',truncated:false}));vi.stubGlobal('fetch',fetcher);const action=await capability('apps.action.execute',{action:'SELECTED_DOWNLOAD',input:{}});const files=connectedFiles(action.output);expect(files).toEqual([{fileId:'file_fixture'}]);expect(await parseConnectedFile(files[0].fileId!)).toEqual({text:'Parsed synthetic text',truncated:false});const requests=fetcher.mock.calls as unknown as [string,RequestInit][];expect(requests).toHaveLength(2);expect(requests[1][0]).toContain('/capabilities/document.parse');expect(JSON.parse(String(requests[1][1].body))).toEqual({fileId:'file_fixture',ocr:false});expect(action.status).toBe('succeeded');});
 it('retains a succeeded action when its file copy failed and has nothing to parse',async()=>{const fetcher=vi.fn().mockResolvedValue(response({status:'succeeded',output:{downloaded_file_content:{stored:false,note:'synthetic copy failure'},other:'retained result'}}));vi.stubGlobal('fetch',fetcher);const result=await capability('apps.action.execute',{action:'SELECTED_DOWNLOAD',input:{}});expect(result.status).toBe('succeeded');expect(connectedFiles(result.output)).toEqual([{stored:false}]);expect((result.output as Record<string,unknown>).other).toBe('retained result');expect(fetcher).toHaveBeenCalledTimes(1);});
});

describe('Long outcome terminal behavior',()=>{
 it('exposes the accepted job for cancellation and preserves unknown steps in a failed report',async()=>{
  const fetcher=vi.fn(async(url:RequestInfo|URL)=>String(url).includes('/jobs/')?response({job_id:'long-1',status:'failed',result:{outcome:'lead.find',status:'failed',steps:[{id:'step-1',effect:'unknown',status:'failed'}],cost:{}}}):response({job_id:'long-1',status:'running'},202));vi.stubGlobal('fetch',fetcher);
  const onJob=vi.fn();const report=await outcomes.run('lead.find',{}, {plan:'p'},onJob);
  expect(onJob).toHaveBeenCalledWith('long-1');expect(report.steps[0].effect).toBe('unknown');expect(fetcher.mock.calls.filter(c=>String(c[0]).includes('/outcomes/'))).toHaveLength(1);
 });
 it('reports a platform timeout as check before retrying and never starts again',async()=>{
  const fetcher=vi.fn(async(url:RequestInfo|URL)=>String(url).includes('/jobs/')?response({job_id:'long-1',status:'failed',error:{code:'upstream_timeout',message:'remote detail'}}):response({job_id:'long-1',status:'running'},202));vi.stubGlobal('fetch',fetcher);
  await expect(outcomes.run('lead.find',{}, {plan:'p'})).rejects.toThrow('Check its recorded steps before retrying');expect(fetcher.mock.calls.filter(c=>String(c[0]).includes('/outcomes/'))).toHaveLength(1);
 });
});

it('waits beyond 31 minutes for the platform 35-minute terminal timeout without resubmitting',async()=>{
 const {createPramaanClient: Pramaan}=await import('./pramaan');let now=0,posts=0;
 const clock=vi.spyOn(Date,'now').mockImplementation(()=>now);
 try {
  const client=Pramaan({apiKey:'synthetic',baseUrl:'http://localhost:18080',maxRetries:0,sleep:async()=>{now+=8*60_000;},fetch:async(_url,init)=>{
   if(init.method==='POST'){posts++;return response({job_id:'long-1',status:'running'},202);}
   return response(now>=35*60_000?{job_id:'long-1',status:'failed',error:{code:'upstream_timeout',message:'Check before retrying'}}:{job_id:'long-1',status:'running'});
  }});
  await expect(client.outcomes.run('lead.find',{}, {plan:'p'})).rejects.toMatchObject({code:'upstream_timeout'});
  expect(now).toBeGreaterThanOrEqual(35*60_000);expect(posts).toBe(1);
 } finally {clock.mockRestore();}
});

describe('unavailable capability replies',()=>{
 for(const id of ['email.inbox.create','email.domain.create','email.inboxes.list','companies.search','companies.enrich','companies.discovery-search'])it(`shows a plain unavailable message for ${id} and never repeats or returns success`,async()=>{
  const fetcher=vi.fn(async()=>response({error:{code:'upstream_unavailable',effect:'none',message:'private upstream details and stack trace'}},502));vi.stubGlobal('fetch',fetcher);
  await expect(capability(id,{})).rejects.toThrow('This feature is not available yet.');expect(fetcher).toHaveBeenCalledTimes(1);
 });
 it('keeps unknown effects as check before retrying',async()=>{const fetcher=vi.fn(async()=>response({error:{code:'upstream_unavailable',effect:'unknown'}},502));vi.stubGlobal('fetch',fetcher);await expect(capability('email.message.send',{})).rejects.toThrow('check before retrying');expect(fetcher).toHaveBeenCalledTimes(1);});
 it('shows unavailable for mail-domain estimates instead of a fake price',async()=>{const fetcher=vi.fn(async()=>response({error:{code:'upstream_unavailable',effect:'none',message:'private detail'}},502));vi.stubGlobal('fetch',fetcher);const {capabilityEstimate}=await import('./pramaan');await expect(capabilityEstimate('email.domain.create',{domain:'synthetic.example'})).rejects.toThrow('This feature is not available yet.');expect(fetcher).toHaveBeenCalledTimes(1);});
});

it.each([['reply_incomplete',422,'The reply is incomplete.'],['reply_limit',400,'Increase maxTokens']])('shows %s safely and sends once',async(code,status,message)=>{const fetcher=vi.fn(async()=>response({error:{code:code==='reply_limit'?'invalid_request':'upstream_unavailable',reason:code,message:'private detail'}},Number(status)));vi.stubGlobal('fetch',fetcher);await expect(capability('llm.complete',{messages:[{role:'user',content:'Synthetic'}],maxTokens:256})).rejects.toThrow(String(message));expect(fetcher).toHaveBeenCalledTimes(1);});
import {capabilityEstimate} from './pramaan';
it.each([undefined,0,50])('preserves optional typical price %s through the fake transport',async(priceTypicalMicro)=>{
 vi.stubGlobal('fetch',vi.fn(async()=>response({plan:'p',ready:true,can_run:true,price_max_micro:100,...(priceTypicalMicro===undefined?{}:{price_typical_micro:priceTypicalMicro}),steps:[]})));
 for(const estimate of [await capabilityEstimate('companies.search',{}),await outcomes.estimate('lead.find',{})]){expect(estimate.priceMaxMicro).toBe(100);expect(estimate.priceTypicalMicro).toBe(priceTypicalMicro);if(priceTypicalMicro===undefined)expect(estimate).not.toHaveProperty('priceTypicalMicro');}
});
