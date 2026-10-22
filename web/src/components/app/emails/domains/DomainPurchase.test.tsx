import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import DomainPurchase from './DomainPurchase';
import { domainClient } from '@/lib/domainPurchase';
import { DomainError } from '@/lib/pramaan';
const state=vi.hoisted(()=>({role:'owner'}));
vi.mock('@/stores',()=>({useAppStore:(fn:(s:unknown)=>unknown)=>fn({currentOrganization:{role:state.role}})}));
vi.mock('@tanstack/react-router',()=>({Link:({children}: {children:React.ReactNode})=><a href="/app/integrations">{children}</a>}));
vi.mock('@/lib/pramaan',async()=>{const actual=await vi.importActual<typeof import('@/lib/pramaan')>('@/lib/pramaan');return {...actual,domainCapability:vi.fn(),capability:vi.fn(),capabilityEstimate:vi.fn()};});
afterEach(()=>{cleanup();vi.resetAllMocks();state.role='owner';});
async function search(){fireEvent.click(screen.getByRole('button',{name:'Find and buy a domain'}));fireEvent.change(screen.getByLabelText('Domain search'),{target:{value:'outreach'}});fireEvent.click(screen.getByRole('button',{name:'Search domains'}));}
describe('domain setup screen',()=>{
 it('searches and quotes, shows price and non-refundable confirmation before creating',async()=>{vi.mocked(domainClient.domain).mockResolvedValueOnce({results:[{domain:'outreach.example',available:true,purchaseMode:'registration',years:1}]}).mockResolvedValueOnce({domain:'outreach.example',action:'register',years:1,price:{amount:1500,currency:'USD'},confirmation:'test-quote'}).mockResolvedValueOnce({domain:'outreach.example',ref:'owned',state:'active'});render(<DomainPurchase/>);await search();fireEvent.click(await screen.findByRole('button',{name:'Show quote for outreach.example'}));expect(await screen.findByText(/This purchase is not refundable/)).toHaveTextContent('$15.00');const buy=screen.getByRole('button',{name:'Confirm and buy outreach.example'});expect(buy).toBeDisabled();expect(domainClient.domain).toHaveBeenCalledTimes(2);for(const label of ['First name','Last name','Street address','City','State or province','Postal code','Country code (two letters)','International phone number','Registrant email'])fireEvent.change(screen.getByLabelText(label),{target:{value:label==='Registrant email'?'synthetic@example.test':label==='Country code (two letters)'?'GB':label==='International phone number'?'+442079460000':'Test'}});fireEvent.click(screen.getByRole('checkbox'));fireEvent.click(buy);await screen.findByText(/Registered domain: outreach.example/);expect(domainClient.domain).toHaveBeenCalledTimes(3);expect(vi.mocked(domainClient.domain).mock.calls[2][0]).toBe('domains.registration.create');});
 it('shows switched-off message and leaves owned-domain link available',async()=>{vi.mocked(domainClient.domain).mockRejectedValue(new DomainError('unavailable','none'));render(<DomainPurchase/>);await search();await waitFor(()=>expect(screen.getByRole('status')).toHaveTextContent('Domain purchase is not available yet; use a domain you own'));expect(screen.getByRole('link',{name:'Use a domain you own'})).toHaveAttribute('href','/app/integrations');});
 it('refuses a manager before search or purchase',()=>{state.role='manager';render(<DomainPurchase/>);expect(screen.getByRole('button',{name:'Find and buy a domain'})).toBeDisabled();expect(domainClient.domain).not.toHaveBeenCalled();});
});
it.each([undefined, 0, 500000])('reviews mail estimate with typical %s', async (priceTypicalMicro) => {
 vi.mocked(domainClient.domain).mockResolvedValueOnce({results:[{domain:'outreach.example',available:true,purchaseMode:'registration',years:1}]}).mockResolvedValueOnce({results:[{domain:'outreach.example',ref:'owned',state:'active'}]}).mockResolvedValueOnce({domain:'outreach.example',ref:'owned',state:'active'});
 vi.mocked(domainClient.mailEstimate).mockResolvedValue({canRun:true,priceMaxMicro:1000000,priceTypicalMicro,balanceMicro:2000000});
 render(<DomainPurchase/>); await search(); fireEvent.click(await screen.findByRole('button',{name:'Check purchase status for outreach.example'})); fireEvent.click(await screen.findByRole('button',{name:'Show mail-domain estimate'}));
 if(priceTypicalMicro===undefined) expect(await screen.findByText(/Attaching costs up to/)).toHaveTextContent('$1.00');
 else {expect((await screen.findByText(`usually about $${(priceTypicalMicro/1_000_000).toFixed(6)}`)).tagName).toBe('STRONG');expect(screen.getByText('at most $1.000000').tagName).toBe('SMALL');}
 expect(domainClient.mail).not.toHaveBeenCalled();
});
