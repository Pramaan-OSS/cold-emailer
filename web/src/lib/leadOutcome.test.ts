import { describe, it, expect, vi } from 'vitest';
import { prepareLeadSearch, runLeadSearch, type LeadSearchClient } from './leadOutcome';
import { OutcomeError } from './pramaan';
const input = { description: 'Small software companies', roles: ['sales'], basis: { ground: 'consent' as const } };
function client() {
 return { outcomes: { get: vi.fn(async()=>({available:true})), estimate:vi.fn(async()=>({ready:true,canRun:true,plan:'plan-1',priceMaxMicro:3000,steps:[{status:'will_run',label:'Company search'}]})), run:vi.fn(async()=>({status:'done',result:{leads:[]},completeness:'complete'})) }, capability:vi.fn(async()=>({results:[]})), capabilityEstimate:vi.fn(async()=>({canRun:true,priceMaxMicro:1000})) };
}
const asClient = (c: ReturnType<typeof client>) => c as unknown as LeadSearchClient;
describe('Lead outcome approval and fallback',()=>{
 it('only estimates until approved, snapshots inputs and passes the plan with the identical body',async()=>{const c=client();const data=structuredClone(input);const plan=await prepareLeadSearch(data,asClient(c));data.roles[0]='changed';expect(c.outcomes.run).not.toHaveBeenCalled();expect(plan.priceMaxMicro).toBe(3000);await runLeadSearch(plan,asClient(c));expect(c.outcomes.run).toHaveBeenCalledWith('lead.find',{description:input.description,roles:['sales']},expect.objectContaining({plan:'plan-1',idempotencyKey:plan.key,basis:input.basis,options:{max:5,verifyEmails:true,fit:false}}));expect(c.capability).not.toHaveBeenCalled();await expect(runLeadSearch(plan,asClient(c))).rejects.toThrow('already submitted');expect(c.outcomes.run).toHaveBeenCalledTimes(1);});
 it('uses a quoted company-only fallback when the description is unavailable',async()=>{const c=client();c.outcomes.get.mockResolvedValue({available:false});const plan=await prepareLeadSearch(input,asClient(c));expect(plan.source).toBe('capability');expect(c.outcomes.estimate).not.toHaveBeenCalled();expect(c.capability).not.toHaveBeenCalled();await runLeadSearch(plan,asClient(c));expect(c.capability).toHaveBeenCalledTimes(1);expect(c.outcomes.run).not.toHaveBeenCalled();});
 it('a 502 refusal produces a new quote to approve, never an automatic capability run',async()=>{const c=client();const plan=await prepareLeadSearch(input,asClient(c));c.outcomes.run.mockRejectedValue(new OutcomeError('upstream_unavailable',502,'none'));const out=await runLeadSearch(plan,asClient(c));expect(out.fallback?.source).toBe('capability');expect(c.capability).not.toHaveBeenCalled();await runLeadSearch(out.fallback!,asClient(c));expect(c.capability).toHaveBeenCalledTimes(1);expect(c.outcomes.run).toHaveBeenCalledTimes(1);});
 it('checks availability again after approval',async()=>{const c=client();const plan=await prepareLeadSearch(input,asClient(c));c.outcomes.get.mockResolvedValue({available:false});expect((await runLeadSearch(plan,asClient(c))).fallback).toBeTruthy();expect(c.outcomes.run).not.toHaveBeenCalled();});
 for(const e of [new OutcomeError('forbidden',403,'none'),new OutcomeError('conflict',409,'none'),new OutcomeError('upstream_unavailable',502),new OutcomeError('internal',500)]) it(`does not fall back or replay ${e.code}/${e.status}/${e.effect}`,async()=>{const c=client();const plan=await prepareLeadSearch(input,asClient(c));c.outcomes.run.mockRejectedValue(e);await expect(runLeadSearch(plan,asClient(c))).rejects.toThrow();expect(c.capability).not.toHaveBeenCalled();expect(c.outcomes.run).toHaveBeenCalledTimes(1);});
 it('preserves a failed report rather than treating an uncertain step as unavailable',async()=>{const c=client();const plan=await prepareLeadSearch(input,asClient(c));c.outcomes.run.mockResolvedValue({status:'failed',result:{leads:[]},completeness:'complete'});await expect(runLeadSearch(plan,asClient(c))).rejects.toThrow('recorded steps');expect(c.capability).not.toHaveBeenCalled();});
 it('requires a fresh estimate after the plan expires',async()=>{const c=client();const plan=await prepareLeadSearch(input,asClient(c));plan.created-=15*60_000;await expect(runLeadSearch(plan,asClient(c))).rejects.toThrow('expired');expect(c.outcomes.run).not.toHaveBeenCalled();});
 it('does not use fallback to bypass a failed budget check',async()=>{const c=client();c.outcomes.estimate.mockResolvedValue({ready:true,canRun:false,plan:'p',priceMaxMicro:3000,steps:[]});await expect(prepareLeadSearch(input,asClient(c))).rejects.toThrow('balance');expect(c.capability).not.toHaveBeenCalled();});
});

it('refuses to present a report with an unknown step as completed and locks the original plan',async()=>{
 const c=client();const plan=await prepareLeadSearch(input,asClient(c));
 c.outcomes.run.mockResolvedValue({status:'done',result:{leads:[]},completeness:'complete',steps:[{effect:'unknown'}]} as never);
 await expect(runLeadSearch(plan,asClient(c))).rejects.toThrow('Check before retrying');
 await expect(runLeadSearch(plan,asClient(c))).rejects.toThrow('already submitted');
 expect(c.outcomes.run).toHaveBeenCalledTimes(1);expect(c.capability).not.toHaveBeenCalled();
});
