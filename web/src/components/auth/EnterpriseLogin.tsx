import {useState} from 'react';
import {enterpriseSignIn} from '@/lib/enterpriseSignIn';
export default function EnterpriseLogin(){const[org,setOrg]=useState(''),[connection,setConnection]=useState(''),[busy,setBusy]=useState(false),[message,setMessage]=useState('');
 return <details className="enterprise-login mt-4"><summary>Organisation sign-in</summary><form className="space-y-3 mt-3" onSubmit={async event=>{event.preventDefault();if(busy)return;setBusy(true);try{const link=await enterpriseSignIn.begin(org.trim(),connection.trim());window.location.assign(link)}catch{setBusy(false);setMessage('Organisation sign-in is not available or this connection is unconfirmed. Use your local account, or start a fresh sign-in.')}}}>
 <p>Use the workspace and connection references your administrator gave you. First link your organisation identity from your signed-in account’s Security page. Local accounts and second factors remain. SAML is unavailable.</p>
 <label className="field">Workspace reference<input className="in" required value={org} onChange={event=>setOrg(event.target.value)}/></label><label className="field">Organisation connection reference<input className="in" required value={connection} onChange={event=>setConnection(event.target.value)}/></label><button className="btn" disabled={busy}>Continue with organisation sign-in</button>{message&&<p role="alert">{message}</p>}
 </form></details>
}
