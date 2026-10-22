import Request from './api/client/Request';
import type { LoginResult } from './api/models/auth/LoginResult';
import { managedSignInClient, managedSignInToken } from './pramaan';

const key = 'managed_sign_in_attempt';
export async function startManagedSignIn(authUrl: string, setupToken = '') {
    const { state } = await Request<{state:string}>({method:'POST',url:'/auth/managed/begin',authorization:false});
    sessionStorage.setItem(key, JSON.stringify({state,authUrl,setupToken}));
    return managedSignInClient(authUrl);
}
export async function finishManagedSignIn(): Promise<LoginResult> {
    const saved = sessionStorage.getItem(key);
    sessionStorage.removeItem(key);
    if (!saved) throw new Error('Start sign-in in this browser before continuing.');
    const {state,authUrl,setupToken} = JSON.parse(saved) as {state:string;authUrl:string;setupToken:string};
    const token = await managedSignInToken(authUrl);
    return Request<LoginResult>({method:'POST',url:'/auth/managed/session',authorization:false,data:{state,binding:state,token,setupToken}});
}
export function managedCallbackURL() { return `${window.location.origin}/auth/sso?managed=1`; }
