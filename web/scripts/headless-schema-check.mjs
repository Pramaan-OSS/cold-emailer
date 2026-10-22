import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
import {mkdirSync} from 'node:fs';
mkdirSync(new URL('../../.data/schema/',import.meta.url),{recursive:true});
const fixture='http://127.0.0.1:59090';
const browser=await chromium.launch({headless:true});
try {
 // Raw malformed requests must fail at the fixture too, without an action.
 for(const capability of ['llm.chat','llm.complete','llm.complete-bulk','document.extract']) {
  const bad={type:'object',properties:{email:{type:'string',pattern:'.+'}}};
  const input=capability==='document.extract'?{schema:bad}:capability==='llm.complete-bulk'?{rows:[{jsonSchema:bad}]}:capability==='llm.chat'?{tools:[{name:'find_contact',parameters:bad}]}:{jsonSchema:bad};
  const response=await fetch(`${fixture}/v1/capabilities/${capability}`,{method:'POST',headers:{'Idempotency-Key':crypto.randomUUID()},body:JSON.stringify(input)});
  assert.equal(response.status,400);const body=await response.json();assert.equal(body.error.effect,'none');assert.match(body.error.message,/#\/properties\/email\/pattern/);
 }
 await fetch(`${fixture}/fixture/unavailable`,{method:'POST',body:JSON.stringify({capabilities:['llm.chat']})});
 const context=await browser.newContext({viewport:{width:1142,height:732}});const page=await context.newPage();const errors=[],outside=[];let sessionId;
 page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(()=>{window.schemaCheckStreams=[];const original=window.fetch;window.fetch=async(...args)=>{const response=await original(...args);if(String(args[0]).includes('/ai/sessions/')&&args[1]?.method==='POST'){response.clone().text().then(text=>window.schemaCheckStreams.push(text)).catch(()=>{});}return response;};});
 page.on('response',response=>{if(response.url().includes('/ai/sessions/')&&response.url().endsWith('/messages')&&response.request().method()==='POST')sessionId=new URL(response.url()).pathname.split('/').at(-2);});
 await context.route('**/*',route=>{const url=new URL(route.request().url());if(['http:','https:'].includes(url.protocol)&&!['127.0.0.1','localhost'].includes(url.hostname)){outside.push(url.hostname);return route.abort();}return route.continue();});
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(/\/app\//,{timeout:30000});
 await page.goto('http://127.0.0.1:5417/app/integrations',{waitUntil:'domcontentloaded'});const composer=page.getByPlaceholder('Ask Pramaan about contacts, campaigns, your inbox…');await composer.fill('Draft a short synthetic introduction.');await page.getByRole('button',{name:'Send message',exact:true}).click();
 await page.getByText('This feature is not available yet. Try again later.',{exact:true}).waitFor({timeout:30000});await page.getByRole('button',{name:'Send message',exact:true}).waitFor();assert.ok(await composer.isEnabled());
 await page.waitForFunction(()=>window.schemaCheckStreams.length>0);const stream=await page.evaluate(()=>window.schemaCheckStreams[0]);assert.match(stream,/"type":"error"/);assert.ok(!stream.includes('"type":"done"'));const saved=await page.evaluate(async id=>{const client=await import('/src/lib/api/client/app/agent/getAgentMessages.ts');return client.default(id)},sessionId);assert.equal(saved.run_status,'failed');assert.ok(saved.run_error);
 await fetch(`${fixture}/fixture/unavailable`,{method:'POST',body:JSON.stringify({capabilities:[]})});
 await composer.fill('Try a fresh draft.');await page.getByRole('button',{name:'Send message',exact:true}).click();await page.getByText('This is the local test assistant. Your workspace is ready.',{exact:true}).waitFor({timeout:30000});
 await page.screenshot({path:new URL('../../.data/schema/assistant-retry.png',import.meta.url).pathname});assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);
 console.log(JSON.stringify({strictFixture:4,failedRun:true,explicitRetry:true,errors,outside}));
} finally {await fetch(`${fixture}/fixture/unavailable`,{method:'POST',body:JSON.stringify({capabilities:[]})});await browser.close();}
