import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest';
const {token,createAuthClient}=vi.hoisted(()=>{const token=vi.fn();return{token,createAuthClient:vi.fn(()=>({token}))};});
vi.mock('better-auth/client',()=>({createAuthClient}));
import {managedSignInToken} from './pramaan';
beforeEach(()=>{vi.resetAllMocks();createAuthClient.mockReturnValue({token});token.mockResolvedValue({data:{token:'verified-token'}});window.history.replaceState({},'', '/auth/sso?managed=1');});
afterEach(()=>vi.unstubAllGlobals());
describe('Managed social callback completion',()=>{
 it('finishes the cookie partition before requesting a token and removes the verifier',async()=>{
  window.history.replaceState({},'','/auth/sso?managed=1&neon_auth_session_verifier=one-time');
  const fetcher=vi.fn(async(_input:RequestInfo|URL,_options?:RequestInit)=>{expect(token).not.toHaveBeenCalled();expect(window.location.search).not.toContain('one-time');return new Response(JSON.stringify({session:{id:'session'}}),{status:200});});vi.stubGlobal('fetch',fetcher);
  expect(await managedSignInToken('https://signin.example.test/auth')).toBe('verified-token');
  expect(String(fetcher.mock.calls[0][0])).toBe('https://signin.example.test/auth/get-session?neon_auth_session_verifier=one-time');
  expect(fetcher.mock.calls[0][1]).toMatchObject({method:'GET',credentials:'include',redirect:'error'});
  expect(token).toHaveBeenCalledTimes(1);expect(window.location.search).toBe('?managed=1');
 });
 it('does not request a token or retry a refused callback',async()=>{
  window.history.replaceState({},'','/auth/sso?managed=1&neon_auth_session_verifier=refused');const fetcher=vi.fn(async()=>new Response('{}',{status:403}));vi.stubGlobal('fetch',fetcher);
  await expect(managedSignInToken('https://signin.example.test/auth')).rejects.toThrow('could not be verified');expect(token).not.toHaveBeenCalled();expect(fetcher).toHaveBeenCalledTimes(1);expect(window.location.search).not.toContain('verifier');
 });
 it('leaves the email sign-in token flow unchanged',async()=>{const fetcher=vi.fn();vi.stubGlobal('fetch',fetcher);expect(await managedSignInToken('https://signin.example.test/auth')).toBe('verified-token');expect(fetcher).not.toHaveBeenCalled();});
});
