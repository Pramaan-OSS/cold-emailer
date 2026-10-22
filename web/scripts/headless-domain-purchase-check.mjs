import {mkdirSync as ensureEvidenceFolder} from 'node:fs';
ensureEvidenceFolder(new URL('../../.data/browser-checks/', import.meta.url), {recursive:true});
import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
const domain=`synthetic-outreach-${Date.now()}.example`;
await fetch('http://127.0.0.1:59090/fixture/domain-mode',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode:'unknown',domain})});
const before=await (await fetch('http://127.0.0.1:59090/fixture/domains')).json();
const browser=await chromium.launch({headless:true});
try{
 const context=await browser.newContext({viewport:{width:1142,height:732}}),page=await context.newPage(),outside=[],errors=[],calls=[];let purchaseRequest;
 page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(r.url().includes('/capabilities/'))calls.push(r.url().split('/capabilities/')[1]);if(r.url().endsWith('/capabilities/domains.registration.create'))purchaseRequest=r;});
 await context.route('**/*',route=>{const url=new URL(route.request().url());if(['http:','https:'].includes(url.protocol)&&!['127.0.0.1','localhost'].includes(url.hostname)){outside.push(url.hostname);return route.abort();}return route.continue();});
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(/\/app\//);
 await page.goto('http://127.0.0.1:5417/app/emails/domains',{waitUntil:'domcontentloaded'});
 await page.getByRole('button',{name:'Find and buy a domain',exact:true}).click();await page.getByLabel('Domain search', {exact:true}).fill('synthetic-outreach');await page.getByRole('button',{name:'Search domains',exact:true}).click();await page.getByRole('button',{name:`Show quote for ${domain}`,exact:true}).click();
 const buy=page.getByRole('button',{name:`Confirm and buy ${domain}`,exact:true});await buy.waitFor();assert.equal(await buy.isEnabled(),false);assert.equal(calls.filter(id=>id==='domains.registration.create').length,0);
 const fields={'First name':'Test','Last name':'Owner','Street address':'1 Test Road','City':'London','State or province':'London','Postal code':'E1 1AA','Country code (two letters)':'GB','International phone number':'+442079460000','Registrant email':'synthetic@example.test'};
 for(const [label,value] of Object.entries(fields))await page.getByLabel(label,{exact:true}).fill(value);
 await page.getByText(/This purchase is not refundable/).waitFor();assert.match(await page.getByText(/I confirm a real purchase/).innerText(),/\$15\.00/);
 await page.getByRole('checkbox',{name:/I confirm a real purchase/}).check();await page.screenshot({path:new URL('../../.data/browser-checks/domain-purchase-confirmation-headless.png',import.meta.url).pathname});await buy.click();await page.getByRole('status').filter({hasText:'result is unknown'}).waitFor();assert.equal(calls.filter(id=>id==='domains.registration.create').length,1);assert.equal(await buy.isEnabled(),false);
 const replay=await context.request.post(purchaseRequest.url(),{data:purchaseRequest.postDataJSON(),headers:purchaseRequest.headers()});assert.equal(replay.status(),502);
 await page.getByRole('button',{name:'Check purchase status',exact:true}).click();await page.getByText(`Registered domain: ${domain}.`,{exact:false}).waitFor();assert.ok(calls.includes('domains.registration.list'));assert.ok(calls.includes('domains.registration.get'));assert.equal(calls.filter(id=>id==='domains.registration.create').length,1);
 await page.getByRole('button',{name:'Show mail-domain estimate',exact:true}).click();await page.getByText(/Attaching costs up to \$2\.60/).waitFor();await page.getByRole('button',{name:'Confirm monthly charge and attach for mail',exact:true}).click();await page.getByRole('button',{name:'Confirm and apply mail DNS records',exact:true}).click();await page.getByRole('status').filter({hasText:'DNS records applied'}).waitFor();assert.ok(calls.includes('domains.dns-records.apply'));assert.ok(calls.includes('email.domain.get'));
 await page.screenshot({path:new URL('../../.data/browser-checks/domain-purchase-recovered-headless.png',import.meta.url).pathname});
 await fetch('http://127.0.0.1:59090/fixture/domain-mode',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode:'unavailable'})});
 await page.getByRole('button',{name:'Search domains',exact:true}).click();await page.getByRole('status').filter({hasText:'Domain purchase is not available yet; use a domain you own'}).waitFor();assert.ok(await page.getByRole('link',{name:'Use a domain you own',exact:true}).isVisible());
 const after=await (await fetch('http://127.0.0.1:59090/fixture/domains')).json();assert.equal(after.creates-before.creates,1);assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);
 console.log(JSON.stringify({search:true,quote:true,priceConfirmation:true,onePurchase:true,unknownRecoveredByGet:true,mailDomainAndDNS:true,unavailableOwnedPath:true,outside,errors}));
}finally{await browser.close();}
