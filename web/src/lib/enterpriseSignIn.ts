import Request from '@/lib/api/client/Request';
import {SSO_BINDING_KEY} from '@/lib/api/client/auth/beginSSO';
export type EnterpriseConnection={id:string;organization_id:string;name:string;created_at:string};
export type EnterprisePage={results:EnterpriseConnection[];nextCursor?:string;redirectUri?:string};
export type EnterpriseSettings={name:string;issuer:string;clientId:string;clientSecret?:string;tokenAuthMethod:'none'|'client_secret_basic'|'client_secret_post';endpointHosts?:string[];approved:boolean};
export const enterpriseAdmin=(role?:string)=>role==='owner'||role==='admin';
export const enterpriseSignIn={
 list:(cursor?:string)=>Request<EnterprisePage>({method:'GET',url:'/auth/enterprise/connections',authorization:true,params:cursor?{cursor}:undefined}),
 choices:()=>Request<EnterprisePage>({method:'GET',url:'/auth/enterprise/choices',authorization:true}),
 create:(input:EnterpriseSettings,key:string)=>Request<EnterpriseConnection>({method:'POST',url:'/auth/enterprise/connections',authorization:true,data:input,headers:{'Idempotency-Key':key}}),
 remove:(id:string,key:string)=>Request<{deleted:true}>({method:'DELETE',url:`/auth/enterprise/connections/${encodeURIComponent(id)}`,authorization:true,data:{approved:true},headers:{'Idempotency-Key':key}}),
 begin:async(org:string,id:string,link=false)=>{
  const reply=await Request<{url:string;binding:string}>({method:'POST',url:link?'/auth/enterprise/link/begin':'/auth/enterprise/begin',authorization:link,withCredentials:true,data:{organization_id:org,connection_id:id,approved:link}});
  const parsed=new URL(reply.url);if(parsed.protocol!=='https:'||parsed.username||parsed.password||!parsed.pathname.startsWith('/connect/link/'))throw new Error('The organisation sign-in link is invalid. Start again.');
  // Fail before navigating if this tab cannot preserve its handoff binding.
  sessionStorage.setItem(SSO_BINDING_KEY,reply.binding);
  return reply.url;
 }
};
