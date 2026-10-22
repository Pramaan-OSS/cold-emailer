import {afterEach,describe,expect,it,vi} from 'vitest';
import {cleanup,render,screen} from '@testing-library/react';
import {foldEvent} from './AgentPanel';
import {ActivityTrace} from './AgentActivity';
afterEach(cleanup);
describe('assistant failures',()=>{
 it('marks an unfinished action failed, never completed, and leaves earlier successes intact',()=>{
  let turns=foldEvent([],{type:'tool_start',tool:'find_contact'});
  turns=foldEvent(turns,{type:'tool_result',tool:'find_contact',result:'Found'});
  turns=foldEvent(turns,{type:'tool_start',tool:'create_task'});
  turns=foldEvent(turns,{type:'error',code:'run_failed',message:'Action create_task failed. Check before retrying.'});
  const steps=turns[0].blocks.flatMap(b=>b.kind==='tool'?[b.step]:[]);
  expect(steps[0].done).toBe(true);expect(steps[1].done).toBe(false);expect(steps[1].failed).toBe(true);
  render(<ActivityTrace steps={steps} live={false} onOpen={vi.fn()}/>);
  expect(screen.getByText('1 failed, 1 completed')).toBeInTheDocument();
 });
});
