import {beforeEach,describe,it,expect,vi} from 'vitest';
import {startManagedSignIn,finishManagedSignIn,managedCallbackURL} from './managedSignIn';
import Request from './api/client/Request';
import {managedSignInClient,managedSignInToken} from './pramaan';
vi.mock('./api/client/Request',()=>({default:vi.fn()}));
vi.mock('./pramaan',()=>({managedSignInClient:vi.fn(),managedSignInToken:vi.fn()}));
describe('Managed sign-in browser handoff',()=>{
 beforeEach(()=>{sessionStorage.clear();vi.resetAllMocks();});
 it('keeps the private owner claim and browser binding out of the callback address',async()=>{
  vi.mocked(Request).mockResolvedValueOnce({state:'private-state'}).mockResolvedValueOnce({access_token:'local-session'});
  vi.mocked(managedSignInClient).mockResolvedValue({} as never);vi.mocked(managedSignInToken).mockResolvedValue('signed-token');
  await startManagedSignIn('https://signin.example.test/auth','private-owner-claim');
  expect(managedCallbackURL()).not.toContain('private');
  expect(await finishManagedSignIn()).toEqual({access_token:'local-session'});
  expect(Request).toHaveBeenLastCalledWith(expect.objectContaining({url:'/auth/managed/session',data:{state:'private-state',binding:'private-state',setupToken:'private-owner-claim',token:'signed-token'}}));
  expect(sessionStorage.length).toBe(0);
  await expect(finishManagedSignIn()).rejects.toThrow('Start sign-in in this browser');
 });
 it('never sends an unverified browser token or retries verification',async()=>{
  vi.mocked(Request).mockResolvedValue({state:'state'});vi.mocked(managedSignInClient).mockResolvedValue({} as never);vi.mocked(managedSignInToken).mockRejectedValue(new Error('refused'));
  await startManagedSignIn('https://signin.example.test/auth');
  await expect(finishManagedSignIn()).rejects.toThrow('refused');
  expect(Request).toHaveBeenCalledTimes(1);expect(sessionStorage.length).toBe(0);
 });
});
