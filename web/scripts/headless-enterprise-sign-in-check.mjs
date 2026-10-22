import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
import {mkdirSync} from 'node:fs';
mkdirSync(new URL('../../.data/enterprise/',import.meta.url),{recursive:true});
const browser=await chromium.launch({headless:true});
try{
 const baseline=await (await fetch('http://127.0.0.1:59090/fixture/enterprise')).json();const label='Synthetic organisation sign-in '+crypto.randomUUID().slice(0,8);
 const context=await browser.newContext({viewport:{width:1142,height:732}});const page=await context.newPage();const errors=[],outside=[],callbacks=[];
 page.on('pageerror',error=>errors.push(error.message));
 await context.route('**/*',async route=>{
  const url=new URL(route.request().url());
  if(url.origin==='https://api.pramaan.dev'&&url.pathname.startsWith('/connect/link/')){
   const fixture=await (await fetch('http://127.0.0.1:59090/fixture/enterprise/link/'+url.pathname.split('/').at(-1))).json();
   const callback='http://127.0.0.1:58080/v1/auth/enterprise/callback?'+new URLSearchParams({code:fixture.code,state:fixture.state});callbacks.push({url:callback,state:fixture.state});
   return route.fulfill({contentType:'text/html',body:`<!doctype html><title>Synthetic organisation consent</title><a href="${callback.replaceAll('&','&amp;')}">Complete organisation sign-in</a>`});
  }
  if(['http:','https:'].includes(url.protocol)&&!['localhost','127.0.0.1'].includes(url.hostname)){outside.push(url.hostname);return route.abort();}return route.continue();
 });
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(/\/app\//,{timeout:30000});
 await page.goto('http://127.0.0.1:5417/app/settings/security',{waitUntil:'domcontentloaded'});
 await page.getByLabel('Connection name').fill(label);await page.getByLabel('Exact issuer address').fill('https://issuer.example.test');await page.getByLabel('Application client ID').fill('synthetic-public-client');
 await page.getByRole('button',{name:'Review sign-in connection',exact:true}).click();const dialog=page.getByRole('alertdialog');await dialog.waitFor();assert.equal((await (await fetch('http://127.0.0.1:59090/fixture/enterprise')).json()).connections,baseline.connections);await dialog.getByRole('button',{name:'Confirm',exact:true}).click();await page.getByText('Connection created. Members must link their own identity from their local session before using it to sign in.',{exact:true}).waitFor();
 const uuid=/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/;
 const org=(await page.getByText(/^Workspace reference:/).innerText()).match(uuid)[0];const connection=(await page.getByText(/^Connection reference:/).innerText()).match(uuid)[0];
 await dialog.waitFor({state:'hidden'});
 await page.screenshot({path:new URL('../../.data/enterprise/settings.png',import.meta.url).pathname});
 await page.setViewportSize({width:400,height:732});const layout=await page.evaluate(()=>({width:innerWidth,pageWidth:document.documentElement.scrollWidth}));assert.equal(layout.width,layout.pageWidth);await page.setViewportSize({width:1142,height:732});
 await page.getByRole('button',{name:'Link my organisation identity',exact:true}).click();await dialog.waitFor();await dialog.getByRole('button',{name:'Confirm',exact:true}).click();await page.waitForURL('https://api.pramaan.dev/connect/link/**');
 const wrong=await context.request.get('http://127.0.0.1:58080/v1/auth/enterprise/callback?code=synthetic&state=wrong',{maxRedirects:0});assert.equal(wrong.status(),302);assert.ok(wrong.headers().location.includes('sso_error='));
 await page.getByRole('link',{name:'Complete organisation sign-in'}).click();await page.waitForURL(/127\.0\.0\.1:5417\/app\//,{timeout:30000});
 assert.equal((await (await fetch('http://127.0.0.1:59090/fixture/enterprise')).json()).exchanges,baseline.exchanges+1);
 await page.evaluate(()=>{localStorage.clear();sessionStorage.clear();});await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});await page.getByText('Organisation sign-in',{exact:true}).click();await page.getByLabel('Workspace reference',{exact:true}).fill(org);await page.getByLabel('Organisation connection reference',{exact:true}).fill(connection);await page.getByRole('button',{name:'Continue with organisation sign-in',exact:true}).click();await page.waitForURL('https://api.pramaan.dev/connect/link/**');await page.getByRole('link',{name:'Complete organisation sign-in'}).click();await page.waitForURL(/127\.0\.0\.1:5417\/app\//,{timeout:30000});
 assert.equal((await (await fetch('http://127.0.0.1:59090/fixture/enterprise')).json()).exchanges,baseline.exchanges+2);
 const old=callbacks[1];await context.addCookies([{name:'enterprise_sign_in_state',value:old.state,domain:'127.0.0.1',path:'/v1/auth/enterprise',httpOnly:true,sameSite:'Lax'}]);const replay=await context.request.get(old.url,{maxRedirects:0});assert.ok(replay.headers().location.includes('sso_error='));assert.equal((await (await fetch('http://127.0.0.1:59090/fixture/enterprise')).json()).exchanges,baseline.exchanges+2);
 await page.goto('http://127.0.0.1:5417/app/settings/security',{waitUntil:'domcontentloaded'});await page.getByRole('button',{name:'Check sign-in connections',exact:true}).click();const card=page.locator('.enterprise-sign-in .card').filter({has:page.getByRole('heading',{name:label,exact:true})});await card.getByRole('button',{name:'Delete sign-in connection',exact:true}).waitFor();await card.getByRole('button',{name:'Delete sign-in connection',exact:true}).click();await page.getByRole('alertdialog').getByRole('button',{name:'Confirm',exact:true}).click();await page.getByText('Connection deleted.',{exact:true}).waitFor();
 assert.equal((await (await fetch('http://127.0.0.1:59090/fixture/enterprise')).json()).connections,baseline.connections);assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);
 console.log(JSON.stringify({approvedCreate:true,memberLink:true,localSession:true,knownIdentitySignIn:true,wrongStateRefused:true,replayRefused:true,adminListDelete:true,exchanges:2,errors,outside}));
}finally{await browser.close();}
