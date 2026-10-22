import {useState} from 'react';
import {startManagedSignIn, managedCallbackURL} from '@/lib/managedSignIn';

export default function ManagedSignIn({authUrl,setupToken='',allowSignup=true}:{authUrl?:string;setupToken?:string;allowSignup?:boolean}) {
 const [email,setEmail]=useState(''),[password,setPassword]=useState(''),[name,setName]=useState(''),[signup,setSignup]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('');
 if (!authUrl) return null;
 async function submit(social=false) {
  setBusy(true);setError('');
  try {
   const client=await startManagedSignIn(authUrl!,setupToken);
   const result=social ? await client.signIn.social({provider:'google',callbackURL:managedCallbackURL()}) : signup ? await client.signUp.email({email,password,name,callbackURL:managedCallbackURL()}) : await client.signIn.email({email,password,callbackURL:managedCallbackURL()});
   if (result.error) throw new Error('Sign-in was not accepted. Check your details and try again.');
   if(!social) window.location.assign(managedCallbackURL());
  } catch {setError('Managed sign-in could not complete. Check your details and try again.');setBusy(false);}
 }
 return <section aria-label="Managed sign-in" className="my-5 space-y-3 rounded-lg border border-slate-200 p-4">
 <h2 className="font-semibold">Managed sign-in</h2>
 <p className="text-sm text-slate-500">Sign in through Pramaan. Local sign-in remains available below.</p>
 <button type="button" disabled={busy} onClick={()=>void submit(true)} className="w-full rounded-md bg-slate-900 px-3 py-2 text-white">Continue with social sign-in</button>
 <form onSubmit={e=>{e.preventDefault();void submit();}} className="space-y-3">
 {signup&&<label className="block text-sm">Name<input required value={name} onChange={e=>setName(e.target.value)} autoComplete="name" className="mt-1 block w-full rounded-md border p-2"/></label>}
 <label className="block text-sm">Email<input required type="email" value={email} onChange={e=>setEmail(e.target.value)} autoComplete="email" className="mt-1 block w-full rounded-md border p-2"/></label>
 <label className="block text-sm">Password<input required type="password" minLength={8} value={password} onChange={e=>setPassword(e.target.value)} autoComplete={signup?'new-password':'current-password'} className="mt-1 block w-full rounded-md border p-2"/></label>
 <button disabled={busy} className="w-full rounded-md border px-3 py-2">{busy?'Please wait…':signup?'Create managed account':'Sign in with email'}</button>
 </form>
 {allowSignup&&<button type="button" disabled={busy} onClick={()=>setSignup(!signup)} className="text-sm underline">{signup?'Already have an account?':'Create an account'}</button>}
 {error&&<p role="alert" className="text-sm text-red-700">{error}</p>}
 </section>;
}
