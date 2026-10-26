// Requires the seeded local app and scripts/fake-pramaan.py on the fixed loopback ports below.
// Every external navigation is blocked or fulfilled with a synthetic consent page.
import {createRequire} from 'node:module';import {resolve} from 'node:path';import {writeFileSync,mkdirSync} from 'node:fs';
mkdirSync('.scratch/adopt',{recursive:true});
const require=createRequire(resolve('web/package.json'));const {chromium}=require('@playwright/test');
const browser=await chromium.launch({headless:true});const context=await browser.newContext();const page=await context.newPage();const outside=[],report={};page.setDefaultTimeout(15000);
await context.route('**/*',route=>{const u=new URL(route.request().url());if(['http:','https:','ws:','wss:'].includes(u.protocol)&&!['localhost','127.0.0.1'].includes(u.hostname)){outside.push(u.hostname);return route.abort();}return route.continue();});
const receiverURL=`https://receiver.example.test/events-${Date.now()}`;
const mode=async value=>{await fetch('http://127.0.0.1:59090/fixture/adopt-mode',{method:'POST',body:JSON.stringify({mode:value})});};
try{
 await mode('success');await page.goto('http://localhost:54525/auth/login');await page.getByPlaceholder('name@company.com').fill('dev@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();await page.getByPlaceholder('Enter your password').fill('password123');await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(/\/app\//,{timeout:60000});report.signIn=true;
 await page.goto('http://localhost:54525/app/integrations');await page.getByRole('button',{name:'Mailboxes',exact:true}).click();
 const company=page.getByRole('region',{name:'Whole company connection'});
 await company.getByLabel('Company domain').fill('example.com');await company.getByRole('button',{name:'Connect company',exact:true}).click();await company.getByRole('link',{name:'Open company setup'}).waitFor();report.companyProof=true;
 await company.getByLabel('Mailbox address',{exact:true}).fill(`adopt-${Date.now()}@example.com`);await company.getByRole('button',{name:'Connect company',exact:true}).click();await company.getByLabel('Company connection reference').filter({visible:true}).waitFor();await page.waitForFunction(()=>document.querySelector('[aria-label="Whole company connection"] input:last-of-type')!==null);
 await company.getByRole('button',{name:'Check company connection',exact:true}).click();await company.getByText('Company connection checked. Enter each mailbox address to add it.',{exact:true}).waitFor();
 await company.getByRole('button',{name:'Add company mailbox'}).click();await company.getByRole('status').filter({hasText:'Mailbox connected. Not yet live-verified.'}).waitFor();report.companyEnroll=await company.getByRole('status').innerText();
 const tools=page.getByRole('region',{name:'Mailbox tools'});
 await tools.getByRole('button',{name:'Check mailbox health'}).click();await tools.getByRole('status').filter({hasText:'Sending limits: unknown'}).waitFor();report.diagnose=true;
 await tools.getByRole('button',{name:'Load mailbox folders'}).click();await tools.getByLabel('Destination folder').selectOption('folder_fixture');await tools.getByLabel(/Raw message/).fill('U3ViamVjdDogSGVsbG8NCg0KSGVsbG8=');await tools.getByRole('button',{name:'Save message to folder'}).click();await tools.getByRole('status').filter({hasText:'Message saved; nothing sent'}).waitFor();report.append=true;
 await mode('refused');await tools.getByRole('button',{name:'Check mailbox health'}).click();await tools.getByRole('status').filter({hasText:'could not complete'}).waitFor();report.refusal=true;
 await mode('unavailable');await tools.getByRole('button',{name:'Check mailbox health'}).click();await tools.getByRole('status').filter({hasText:'not available yet'}).waitFor();report.notAvailable=true;
 await company.getByRole('button',{name:'Check company connection'}).click();await company.getByRole('status').filter({hasText:'not available yet'}).waitFor();report.companyNotAvailable=true;
 await mode('success');
 await page.route('https://api.pramaan.dev/connect/company/start/synthetic',route=>route.fulfill({contentType:'text/html',body:'<h1>Synthetic administrator consent</h1>'}));
 await company.getByLabel('Company connection type').selectOption('office');await company.getByRole('button',{name:'Resolve company tenant'}).click();await company.getByRole('status').filter({hasText:'Discovery reference:'}).waitFor();report.tenantDiscovery=true;
 await company.getByLabel('Tenant UUID').fill('12345678-1234-4234-8234-123456789abc');await company.getByRole('button',{name:'Connect company',exact:true}).click();await page.getByRole('heading',{name:'Synthetic administrator consent'}).waitFor();report.organizationConsentStart=true;
 await fetch('http://127.0.0.1:59090/fixture/company-authorize',{method:'POST',body:'{}'});
 await page.goto('http://localhost:54525/app/integrations');await page.getByRole('button',{name:'Mailboxes',exact:true}).click();await company.getByLabel('Company connection reference').fill('company_office_fixture');await company.getByRole('button',{name:'Check company connection',exact:true}).click();await company.getByRole('status').filter({hasText:'Company connection checked.'}).waitFor();report.organizationConnected=true;
 await page.screenshot({path:'.scratch/adopt/mailbox-tools.png'});
 await page.goto('http://localhost:54525/app/settings/webhooks');await page.getByRole('button',{name:'Add endpoint',exact:true}).waitFor();report.webhookScreen=(await page.locator('body').innerText()).slice(-5500);
 await page.getByRole('button',{name:'Add endpoint',exact:true}).click();await page.getByPlaceholder('https://acme.com/webhooks/coldemail').fill(receiverURL);await page.getByRole('button',{name:'Next',exact:true}).click();await page.getByRole('button',{name:'Create endpoint',exact:true}).click();await page.getByText(/Save the signing secret now/).waitFor();report.webhookSecretOnce=true;await page.screenshot({path:'.scratch/adopt/webhooks.png'});await page.getByRole('button',{name:/saved|done/i}).last().click();
 await page.locator('code').filter({hasText:'whsec_'}).waitFor({state:'hidden'});
 await page.getByRole('button').filter({hasText:receiverURL}).first().click();await page.getByRole('button',{name:'Send test event',exact:true}).click();
 let signed=false;for(let attempt=0;attempt<40;attempt++){const fixture=await fetch('http://127.0.0.1:59090/fixture/adopt').then(r=>r.json());signed=fixture.calls.some(c=>c.path.endsWith('/workflow.webhook.signed-call')&&c.input.url===receiverURL);if(signed)break;await new Promise(resolve=>setTimeout(resolve,250));}if(!signed)throw new Error('Signed delivery was not dispatched');report.signedDelivery=true;await page.getByRole('button',{name:'Close',exact:true}).last().click();
 // Render the production message reader with the signed-in app session; image
 // requests still use the real server and fake platform transport.
 const baselineImages=await fetch('http://127.0.0.1:59090/fixture/adopt').then(r=>r.json());
 await page.evaluate(async()=>{const React=await import('/node_modules/.vite/deps/react.js');const {default:ReactDOM}=await import('/node_modules/.vite/deps/react-dom_client.js');const {default:EmailBody}=await import('/src/components/app/unibox/EmailBody.tsx');const el=document.createElement('div');el.id='adopt-image';el.style.cssText='position:fixed;top:100px;left:300px;width:700px;background:white;z-index:99999;padding:20px';document.body.append(el);ReactDOM.createRoot(el).render(React.default.createElement(EmailBody,{html:'<p>Synthetic email</p><img src="https://image.example.test/a.png">',blockRemote:true}));});
 const image=page.locator('#adopt-image');await image.getByRole('button',{name:'Load images'}).waitFor();let before=await fetch('http://127.0.0.1:59090/fixture/adopt').then(r=>r.json());const imageCalls=v=>v.calls.filter(c=>c.path.endsWith('/web.image.read')).length;
 report.imageBlockedBeforeConsent=imageCalls(before)===imageCalls(baselineImages);if(!report.imageBlockedBeforeConsent)throw new Error('Image requested before consent');
 page.once('dialog',d=>d.accept());await image.getByRole('button',{name:'Load images'}).click();await image.getByRole('status').filter({hasText:'Images imported'}).waitFor();await page.waitForFunction(()=>{const img=document.querySelector('#adopt-image iframe')?.contentDocument?.querySelector('img');return img?.complete&&img.naturalWidth>0;});report.imageImported=true;
 await page.screenshot({path:'.scratch/adopt/image-import.png'});
 if(outside.length)throw new Error('Unexpected outside request '+outside.join(','));report.outsideRequests=outside;
 writeFileSync('.scratch/adopt/browser-results.json',JSON.stringify(report,null,2));console.log(JSON.stringify(report));
}catch(e){await page.screenshot({path:'.scratch/adopt/browser-failure.png'});report.error=e.message;report.body=(await page.locator('body').innerText()).slice(-3000);writeFileSync('.scratch/adopt/browser-results.json',JSON.stringify(report,null,2));console.log(JSON.stringify(report));process.exitCode=1;}finally{try{await mode('success');}finally{await browser.close();}}
