import { describe, expect, it, vi } from 'vitest';
import { prepareMailbox, runMailbox } from './mailboxOutcome';
import { outcomes } from './pramaan';

function fake() {
 return { get:vi.fn(async()=>({available:true})),estimate:vi.fn(async()=>({ready:true,canRun:true,plan:'mail-plan',priceMaxMicro:4000,steps:[]})),run:vi.fn(async()=>({status:'done',result:{status:'pending',ref:'connection'}})) };
}
const client = (c:ReturnType<typeof fake>)=>c as unknown as typeof outcomes;
describe('Mailbox outcome plans',()=>{
 it('refuses an expired estimate without starting',async()=>{const c=fake();const plan=await prepareMailbox(false,client(c));plan.created-=15*60_000;await expect(runMailbox(plan,client(c))).rejects.toThrow('expired');expect(c.run).not.toHaveBeenCalled();});
 it('never resubmits a failed attempt',async()=>{const c=fake();const plan=await prepareMailbox(false,client(c));c.run.mockRejectedValue(new Error('lost response'));await expect(runMailbox(plan,client(c))).rejects.toThrow('lost response');await expect(runMailbox(plan,client(c))).rejects.toThrow('already submitted');expect(c.run).toHaveBeenCalledTimes(1);});
 it('refuses a budget denial before running',async()=>{const c=fake();c.estimate.mockResolvedValue({ready:true,canRun:false,plan:'mail-plan',priceMaxMicro:4000,steps:[]});await expect(prepareMailbox(false,client(c))).rejects.toThrow('balance');expect(c.run).not.toHaveBeenCalled();});
 it('retains partial connection progress after an unknown step',async()=>{const c=fake();c.run.mockResolvedValue({status:'failed',result:{status:'pending',ref:'connection'},steps:[{effect:'unknown'}]} as never);const plan=await prepareMailbox(false,client(c));expect(await runMailbox(plan,client(c))).toEqual({complete:false,result:{status:'pending',ref:'connection'}});});
});
