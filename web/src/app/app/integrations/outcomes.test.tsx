vi.mock('@/lib/api/hooks/auth/useAuthConfig',()=>({default:()=>({config:{providers:[],pramaan_configured:true,hosting_configured:true,enterprise_configured:true,connector_oauth_configured:true}})}));
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import IntegrationsPage from './page';
import { OutcomeError } from '@/lib/pramaan';
const mocks = vi.hoisted(()=>({get:vi.fn(),estimate:vi.fn(),run:vi.fn(),capability:vi.fn(),quote:vi.fn(),show:vi.fn()}));
vi.mock('@/hooks/context/confirm',()=>({useConfirm:()=>({show:mocks.show})}));
vi.mock('@/lib/pramaan',async(importOriginal)=>({...await importOriginal<typeof import('@/lib/pramaan')>(),outcomes:{get:mocks.get,estimate:mocks.estimate,run:mocks.run,list:vi.fn()},capability:mocks.capability,cataloguePage:(input:unknown)=>mocks.capability('apps.catalog.search',input),parseConnectedFile:(fileId:string)=>mocks.capability('document.parse',{fileId,ocr:false}),capabilityEstimate:mocks.quote}));
beforeEach(()=>{vi.clearAllMocks();mocks.get.mockResolvedValue({available:true});mocks.estimate.mockResolvedValue({ready:true,canRun:true,plan:'plan-ui',priceMaxMicro:4200,steps:[{status:'will_run',label:'Company search'},{status:'will_run',label:'Contact match'}]});mocks.run.mockResolvedValue({status:'done',result:{leads:[{company:{name:'Example team'},person:{fullName:'Synthetic reviewer'},email:'reviewer@example.test',emailStatus:'verified'}]}});mocks.quote.mockResolvedValue({canRun:true,priceMaxMicro:1000});mocks.capability.mockResolvedValue({results:[{name:'Company-only match'}]});});
async function prepare(){render(<IntegrationsPage/>);fireEvent.click(screen.getByRole('button',{name:'Lead data'}));fireEvent.change(screen.getByLabelText('Describe the companies'),{target:{value:'Small software companies'}});fireEvent.change(screen.getByLabelText('Contact roles (comma-separated, up to three)'),{target:{value:'Head of sales'}});fireEvent.change(screen.getByLabelText('Stated basis for contact research'),{target:{value:'consent'}});fireEvent.click(screen.getByRole('button',{name:'Review lead search'}));await waitFor(()=>expect(mocks.show).toHaveBeenCalledTimes(1));}
describe('Existing lead-search confirmation flow',()=>{
 it('shows steps, input, basis and maximum total before running, then renders returned leads',async()=>{await prepare();expect(mocks.show.mock.calls[0][0]).toContain('Maximum total: $0.004200');expect(mocks.show.mock.calls[0][0]).toContain('Contact match');expect(mocks.show.mock.calls[0][0]).toContain('Small software companies');expect(mocks.show.mock.calls[0][0]).toContain('consent');expect(mocks.run).not.toHaveBeenCalled();await act(async()=>mocks.show.mock.calls[0][1]());await screen.findByText(/Synthetic reviewer/);expect(mocks.run).toHaveBeenCalledTimes(1);expect(mocks.run.mock.calls[0][2].plan).toBe('plan-ui');});
 it('cancel does not run anything',async()=>{await prepare();await act(async()=>mocks.show.mock.calls[0][2]());expect(mocks.run).not.toHaveBeenCalled();expect(mocks.capability).not.toHaveBeenCalled();await screen.findByText('Lead search was cancelled; nothing ran.');});
 it('rejects an input edit after estimating',async()=>{await prepare();fireEvent.change(screen.getByLabelText('Describe the companies'),{target:{value:'Different software companies'}});await act(async()=>mocks.show.mock.calls[0][1]());await screen.findByText('The input changed. Review a new estimate.');expect(mocks.run).not.toHaveBeenCalled();});
 it('requires a second confirmation for the quoted fallback after an unavailable run',async()=>{mocks.run.mockRejectedValue(new OutcomeError('upstream_unavailable',502,'none'));await prepare();await act(async()=>mocks.show.mock.calls[0][1]());await waitFor(()=>expect(mocks.show).toHaveBeenCalledTimes(2));expect(mocks.show.mock.calls[1][0]).toContain('Maximum total: $0.001000');expect(mocks.capability).not.toHaveBeenCalled();await act(async()=>mocks.show.mock.calls[1][1]());await screen.findByText('Company-only match');expect(mocks.capability).toHaveBeenCalledTimes(1);expect(mocks.run).toHaveBeenCalledTimes(1);});
});

import {redirectToConnection} from '@/lib/connectionAccess';
vi.mock('@/lib/connectionAccess',async(importOriginal)=>({...await importOriginal<typeof import('@/lib/connectionAccess')>(),redirectToConnection:vi.fn()}));
it('shows the access grant and actual product use before leaving for the hosted connection',async()=>{
 mocks.capability.mockImplementation(async(id:string)=>id==='apps.catalog.search'?{results:[{app:'gmail',action:'RETURNED_ACTION',description:'Selected mail action',inputSchema:{type:'object',properties:{}}}]}:{connectUrl:'https://example.test/hosted-connect'});
 render(<IntegrationsPage/>);fireEvent.change(screen.getByLabelText('What do you want to do?'),{target:{value:'Selected mail action'}});fireEvent.click(screen.getByRole('button',{name:'Find actions'}));await screen.findByRole('button',{name:'Choose action'});fireEvent.click(screen.getByRole('button',{name:'Choose action'}));
 vi.mocked(redirectToConnection).mockImplementation(()=>{expect(screen.getByTestId('connection-access-notice').textContent).toContain('full access to your Gmail');expect(screen.getByTestId('connection-access-notice').textContent).toContain('actions you select or enable');});
 fireEvent.click(screen.getByRole('button',{name:'Connect your account'}));await waitFor(()=>expect(redirectToConnection).toHaveBeenCalledWith('https://example.test/hosted-connect'));
});

it('loads catalogue pages on demand and removes load-more at the last page',async()=>{
 mocks.capability.mockResolvedValueOnce({results:[{app:'fixture_app',action:'FIRST',description:'First action',inputSchema:{type:'object'}}],nextCursor:'next-page'}).mockResolvedValueOnce({results:[{app:'fixture_app',action:'SECOND',description:'Second action',inputSchema:{type:'object'}}]});
 render(<IntegrationsPage/>);fireEvent.change(screen.getByLabelText('What do you want to do?'),{target:{value:'read file'}});fireEvent.click(screen.getByRole('button',{name:'Find actions'}));await screen.findByText('First action');expect(mocks.capability).toHaveBeenCalledTimes(1);fireEvent.click(screen.getByRole('button',{name:/Load more actions/}));await screen.findByText('Second action');expect(mocks.capability).toHaveBeenLastCalledWith('apps.catalog.search',{query:'read file',limit:50,cursor:'next-page'});expect(screen.queryByRole('button',{name:/Load more actions/})).not.toBeInTheDocument();expect(mocks.capability).toHaveBeenCalledTimes(2);
});

it('keeps a succeeded action visible when its file copy failed without parsing or repeating it',async()=>{
 mocks.capability.mockImplementation(async(id:string)=>id==='apps.catalog.search'?{results:[{app:'fixture_app',action:'SELECTED_DOWNLOAD',description:'Download selected document',inputSchema:{type:'object',properties:{}}}]}:{status:'succeeded',output:{downloaded_file_content:{stored:false,note:'copy failed'},other:'kept'}});
 render(<IntegrationsPage/>);fireEvent.change(screen.getByLabelText('What do you want to do?'),{target:{value:'Download selected document'}});fireEvent.click(screen.getByRole('button',{name:'Find actions'}));fireEvent.click(await screen.findByRole('button',{name:'Choose action'}));fireEvent.click(screen.getByRole('button',{name:'Run this action'}));expect(mocks.capability).toHaveBeenCalledTimes(1);await act(async()=>mocks.show.mock.calls[0][1]());await screen.findByText('The action ran, but this file copy was not saved. Do not run the action again.');expect(screen.getByText('succeeded',{exact:true})).toBeInTheDocument();expect(screen.queryByRole('button',{name:/Read downloaded file/})).not.toBeInTheDocument();expect(mocks.capability.mock.calls.map(call=>call[0])).toEqual(['apps.catalog.search','apps.action.execute']);
});

it('shows mailbox provisioning unavailable and keeps connection discovery usable',async()=>{
 mocks.capability.mockImplementation(async(id:string)=>{if(id.startsWith('email.'))throw new Error('This feature is not available yet.');return {results:[]};});
 render(<IntegrationsPage/>);fireEvent.click(screen.getByRole('button',{name:'Mailboxes'}));
 fireEvent.change(screen.getByLabelText('Mailbox name'),{target:{value:'Synthetic mailbox'}});fireEvent.change(screen.getByLabelText('Address name'),{target:{value:'synthetic'}});
 fireEvent.click(screen.getByRole('button',{name:'Create mailbox'}));await screen.findByText('This feature is not available yet.');expect(screen.queryByText('Your request is complete.')).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole('button',{name:'Apps'}));fireEvent.change(screen.getByLabelText('What do you want to do?'),{target:{value:'Read a synthetic record'}});expect(screen.getByRole('button',{name:'Find actions'})).not.toBeDisabled();
});

it('does not show a failed connected action as completed',async()=>{
 mocks.capability.mockImplementation(async(id:string)=>id==='apps.catalog.search'?{results:[{app:'fixture_app',action:'SELECTED_ACTION',description:'Synthetic reviewed action',inputSchema:{type:'object',properties:{}}}]}:{status:'failed',output:{retained:'Failure detail'}});
 render(<IntegrationsPage/>);fireEvent.change(screen.getByLabelText('What do you want to do?'),{target:{value:'Synthetic action'}});fireEvent.click(screen.getByRole('button',{name:'Find actions'}));fireEvent.click(await screen.findByRole('button',{name:'Choose action'}));fireEvent.click(screen.getByRole('button',{name:'Run this action'}));await act(async()=>mocks.show.mock.calls[0][1]());await screen.findByText('The action failed. Check its result before retrying; it was not repeated.');expect(screen.queryByText('Your request is complete.')).not.toBeInTheDocument();expect(mocks.capability).toHaveBeenCalledTimes(2);
});

it('narrows broad keyword results by the selected app without automatic paid discovery',async()=>{
 mocks.capability.mockResolvedValue({results:[{app:'wanted_app',action:'SELECTED_ACTION',description:'Specific app result',inputSchema:{type:'object',properties:{}}}]});
 render(<IntegrationsPage/>);
 fireEvent.change(screen.getByLabelText('What do you want to do?'),{target:{value:'read records'}});
 fireEvent.change(screen.getByLabelText('App filter (optional)'),{target:{value:'wanted_app'}});
 expect(mocks.capability).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole('button',{name:'Find actions'}));
 await screen.findByText('Specific app result');
 expect(mocks.capability).toHaveBeenCalledWith('apps.catalog.search',{query:'read records',app:'wanted_app',limit:50});
 expect(screen.getByText('App: wanted_app')).toBeInTheDocument();
});

it.each([0, 2100])('shows typical price %s prominently before approval', async (priceTypicalMicro) => { mocks.estimate.mockResolvedValue({ready:true,canRun:true,plan:'plan-ui',priceMaxMicro:4200,priceTypicalMicro,steps:[]}); await prepare(); render(<>{mocks.show.mock.calls[0][0]}</>); expect(screen.getByText(`usually about $${(priceTypicalMicro/1_000_000).toFixed(6)}`).tagName).toBe('STRONG'); expect(screen.getByText('at most $0.004200').tagName).toBe('SMALL'); expect(mocks.run).not.toHaveBeenCalled(); });

async function prepareMailboxUI(check = false) {
 render(<IntegrationsPage/>);
 fireEvent.click(screen.getByRole('button',{name:'Mailboxes'}));
 fireEvent.click(screen.getByRole('button',{name:check ? 'Check your connection' : 'Connect your mailbox'}));
 await waitFor(()=>expect(mocks.show).toHaveBeenCalledTimes(1));
}
it('quotes mailbox connection, keeps the access notice and opens only the returned hosted link after approval',async()=>{
 mocks.estimate.mockResolvedValue({ready:true,canRun:true,plan:'mail-plan',priceMaxMicro:4200,priceTypicalMicro:2100,steps:[{status:'will_run',label:'Connect mailbox'},{status:'will_run',label:'Test read access'}]});
 mocks.run.mockResolvedValue({status:'done',result:{ref:'connection-one',connectUrl:'https://example.test/mail-signin',status:'pending'}});
 await prepareMailboxUI();
 expect(screen.getByTestId('mailbox-access-notice')).toBeInTheDocument();
 render(<>{mocks.show.mock.calls[0][0]}</>);
 expect(screen.getByText('usually about $0.002100').tagName).toBe('STRONG');
 expect(screen.getByText('at most $0.004200').tagName).toBe('SMALL');
 expect(screen.getByText(/The read test reads at most one message/)).toHaveTextContent('Test read access');
 expect(mocks.run).not.toHaveBeenCalled();
 vi.mocked(redirectToConnection).mockImplementation(()=>{});
 await act(async()=>mocks.show.mock.calls[0][1]());
 expect(mocks.run).toHaveBeenCalledWith('mailbox.connect',{},expect.objectContaining({plan:'mail-plan',idempotencyKey:expect.any(String)}));
 expect(redirectToConnection).toHaveBeenCalledWith('https://example.test/mail-signin');
 expect(mocks.capability).not.toHaveBeenCalled();
});
it('does not connect a mailbox when the estimate is cancelled',async()=>{
 await prepareMailboxUI(); await act(async()=>mocks.show.mock.calls[0][2]());
 await screen.findByText('Mailbox connection was cancelled; nothing ran.');
 expect(mocks.run).not.toHaveBeenCalled(); expect(mocks.capability).not.toHaveBeenCalled();
});
it.each([true,false])('shows the mailbox read test verdict honestly: tested=%s',async(tested)=>{
 mocks.run.mockResolvedValue({status:'done',completeness:tested?'complete':'partial',result:{ref:'connection-one',status:'connected',inbox:'mailbox@pramaan.dev',...(tested?{tested:true}:{})}});
 await prepareMailboxUI(true); await act(async()=>mocks.show.mock.calls[0][1]());
 await screen.findByText(tested?'Mailbox read access was tested. Nothing was sent.':'Mailbox connected; read access is still unconfirmed. Check your connection again later.');
 expect(mocks.run.mock.calls[0][1]).toEqual({check:true});
 expect(mocks.capability).not.toHaveBeenCalled();
});
it('does not retry or use capabilities after mailbox approval_required',async()=>{
 mocks.run.mockRejectedValue(new OutcomeError('forbidden',403,'none'));
 await prepareMailboxUI(); await act(async()=>mocks.show.mock.calls[0][1]());
 await screen.findByText('Approval is required. Review the estimate first.');
 expect(mocks.run).toHaveBeenCalledTimes(1);expect(mocks.capability).not.toHaveBeenCalled();
 expect(screen.getByRole('button',{name:'Connect your mailbox'})).not.toBeDisabled();
 fireEvent.click(screen.getByRole('button',{name:'Connect your mailbox'}));
 await waitFor(()=>expect(mocks.show).toHaveBeenCalledTimes(2));
 expect(mocks.run).toHaveBeenCalledTimes(1);
});
it('refuses unavailable mailbox estimates without running or bypassing the outcome',async()=>{
 mocks.get.mockResolvedValue({available:false});render(<IntegrationsPage/>);
 fireEvent.click(screen.getByRole('button',{name:'Mailboxes'}));fireEvent.click(screen.getByRole('button',{name:'Connect your mailbox'}));
 await screen.findByText('Mailbox connection is not available yet.');
 expect(mocks.estimate).not.toHaveBeenCalled();expect(mocks.run).not.toHaveBeenCalled();expect(mocks.capability).not.toHaveBeenCalled();
});
