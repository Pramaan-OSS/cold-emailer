import {useRef,useState} from 'react';
import {useAppStore} from '@/stores';
import {companyMail,capability} from '@/lib/pramaan';
import {redirectToConnection} from '@/lib/connectionAccess';

export default function CompanyMailConnect(){
 const role=useAppStore(s=>s.currentOrganization?.role),admin=role==='owner'||role==='admin';
 const [kind,setKind]=useState('workspace'),[domain,setDomain]=useState(''),[tenant,setTenant]=useState(''),[ref,setRef]=useState(''),[mailbox,setMailbox]=useState(''),[message,setMessage]=useState(''),[busy,setBusy]=useState(false),[proof,setProof]=useState<Record<string,unknown>>();
 const locked=useRef(false);
 async function run(action:'connect'|'status'|'revoke'|'enroll'){
  if(locked.current||!admin)return;locked.current=true;setBusy(true);setMessage('');
  try{
   const input=action==='connect'?kind==='workspace'?{kind,domain,...(mailbox?{mailbox}:{})}:{kind,tenant}:{companyRef:ref,...(action==='enroll'?{mailbox}:{})};
   const out=await companyMail(action,input);
   if(typeof out.companyRef==='string')setRef(out.companyRef);
   setProof(out);
   if(typeof out.authorizationUrl==='string'){redirectToConnection(out.authorizationUrl);setMessage('Complete administrator consent, then enter the returned connection reference and check it.');}
   else setMessage(action==='enroll'?typeof out.localRegistration==='string'?out.localRegistration:'Mailbox connected. Not yet live-verified.':action==='revoke'?'Company connection revoked. Remove the administrator grant in your company console too.':out.companyRef?'Company connection checked. Enter each mailbox address to add it.':'Add the domain proof and permissions shown on the setup page, then enter an administrator mailbox and connect again.');
  }catch(e){setMessage(e instanceof Error?e.message:'Check the connection before retrying.');}finally{locked.current=false;setBusy(false);}
 }
 return <section className="card" aria-label="Whole company connection"><h2>Connect a whole company</h2><p>Recommended for multiple mailboxes: one administrator approval, then add mailbox addresses. Not yet live-verified. Mailbox discovery is unavailable; enter addresses you are authorized to use.</p>
 {!admin&&<p>Only a workspace owner or admin can connect or revoke a company.</p>}
 <fieldset disabled={!admin||busy}>
 <label className="field">Company connection type<select className="in" value={kind} onChange={e=>setKind(e.target.value)}><option value="workspace">Whole Workspace domain</option><option value="office">Whole organization</option></select></label>
 <label className="field">Company domain<input className="in" value={domain} onChange={e=>setDomain(e.target.value)} placeholder="example.com"/></label>
 {kind==='office'&&<><button className="btn" disabled={!domain} onClick={()=>{void capability<{ref:string}>('email.directory.tenant.resolve',{domain}).then(out=>{setMessage(`Discovery reference: ${out.ref}. This grants no access and does not prove domain ownership. Enter the tenant UUID supplied by your administrator below.`);}).catch(e=>setMessage(e.message));}}>Resolve company tenant</button><label className="field">Tenant UUID<input className="in" value={tenant} onChange={e=>setTenant(e.target.value)}/></label><p>Consent requires a Global Administrator or Privileged Role Administrator.</p></>}
 <label className="field">Mailbox address<input className="in" type="email" value={mailbox} onChange={e=>setMailbox(e.target.value)} placeholder="admin@example.com"/></label>
 <button className="btn" disabled={kind==='workspace'?!domain:!tenant} onClick={()=>void run('connect')}>Connect company</button>
 {typeof proof?.setupUrl==='string'&&<a className="btn" href={proof.setupUrl} target="_blank" rel="noreferrer">Open company setup</a>}
 {typeof proof?.proofName==='string'&&<p>Domain proof: {proof.proofName} = {String(proof.proofValue)}</p>}
 <label className="field">Company connection reference<input className="in" value={ref} onChange={e=>setRef(e.target.value)}/></label>
 <button className="btn" disabled={!ref} onClick={()=>void run('status')}>Check company connection</button>
 <button className="btn" disabled={!ref||!mailbox} onClick={()=>void run('enroll')}>Add company mailbox</button>
 <button className="btn" disabled={!ref} onClick={()=>{if(window.confirm('Revoke company access for this workspace?'))void run('revoke');}}>Revoke company connection</button>
 </fieldset>{message&&<p role="status">{message}</p>}</section>;
}
