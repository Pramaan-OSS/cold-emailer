import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdirSync} from 'node:fs';
const root=process.env.COLD_EMAIL_CHECK_ROOT??new URL('../../',import.meta.url).pathname;
for(let i=0;i<60;i++){try{if((await fetch('http://localhost:5173')).ok)break;}catch{}if(i===59)throw new Error('The local app did not start.');await new Promise(resolve=>setTimeout(resolve,250));}
const browser=await chromium.launch({headless:true});
try{
 const context=await browser.newContext();const page=await context.newPage();const errors=[];const outside=[];
 page.on('pageerror',e=>errors.push(e.message));
 const claims={sub:'managed-fixture-member',iss:'https://managed-signin.example.test',aud:'https://managed-signin.example.test',email:'managed-check@example.test',name:'Managed Member',emailVerified:false,exp:Math.floor(Date.now()/1000)+600};
 const token=`fixture.${Buffer.from(JSON.stringify(claims)).toString('base64url')}.synthetic-fixture-only`;
 let emailSignIns=0,tokenReads=0;
 await context.route('**/*',async route=>{
  const u=new URL(route.request().url());
  if(u.hostname==='managed-signin.example.test'){
   const headers={'access-control-allow-origin':'http://localhost:5173','access-control-allow-credentials':'true','access-control-allow-headers':'content-type','content-type':'application/json'};
   if(route.request().method()==='OPTIONS')return route.fulfill({status:204,headers});
   if(u.pathname.endsWith('/sign-up/email')){emailSignIns++;return route.fulfill({headers,json:{user:{id:claims.sub,email:claims.email,name:claims.name},token:'opaque-session'}});}
   if(u.pathname.endsWith('/token')){tokenReads++;return route.fulfill({headers,json:{token}});}
   throw new Error(`Unexpected managed sign-in route: ${u.pathname}`);
  }
  if(['http:','https:'].includes(u.protocol)&&!['localhost','127.0.0.1'].includes(u.hostname)){outside.push(u.hostname);return route.abort();}
  return route.continue();
 });
 const logs=execFileSync('docker',['compose','-f','docker-compose.dev.yml','logs','--no-color','backend'],{cwd:root,encoding:'utf8'});
 const link=logs.match(/http[^\s]*\/setup\?token=[a-f0-9]+/g)?.at(-1);assert.ok(link,'private first-owner link exists');
 await page.goto(link);
 const managed=page.getByRole('region',{name:'Managed sign-in'});await managed.waitFor();
 mkdirSync(new URL('../../.scratch/managed/',import.meta.url),{recursive:true});await page.screenshot({path:new URL('../../.scratch/managed/signin-form.png',import.meta.url).pathname});
 await managed.getByRole('button',{name:'Create an account',exact:true}).click();
 await managed.getByLabel('Name',{exact:true}).fill('Managed Member');await managed.getByLabel('Email',{exact:true}).fill(claims.email);await managed.getByLabel('Password',{exact:true}).fill('Synthetic-check-7931!');
 await managed.getByRole('button',{name:'Create managed account',exact:true}).click();
 await page.waitForURL(/\/onboarding|\/app\//,{timeout:60000});
 assert.equal(emailSignIns,1);assert.equal(tokenReads,1);
 const owner=await page.evaluate(async()=>{const token=localStorage.getItem('access_token');const response=await fetch('http://localhost:8080/admin/instance/settings',{headers:{Authorization:`Bearer ${token}`}});return {status:response.status,code:(await response.json()).code};});assert.deepEqual(owner,{status:403,code:'admin_mfa_required'},'managed owner retains admin permissions and the second-factor gate');
 assert.deepEqual(outside,[]);assert.deepEqual(errors,[]);
 mkdirSync(new URL('../../.scratch/managed/',import.meta.url),{recursive:true});await page.screenshot({path:new URL('../../.scratch/managed/signin.png',import.meta.url).pathname});
 console.log(JSON.stringify({managedOwnerClaim:true,emailSignIns,tokenReads,localSession:true,ownerPermissions:true,adminSecondFactorRequired:true,outside,errors}));
}finally{await browser.close();}
