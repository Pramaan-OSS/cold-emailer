import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
import {mkdirSync} from 'node:fs';
mkdirSync(new URL('../../.data/schema/',import.meta.url),{recursive:true});
const fixture='http://127.0.0.1:59090';const ids=['email.inbox.create','email.account.connect.start','email.domain.create','companies.enrich','domains.registration.search'];
const browser=await chromium.launch({headless:true});
try {
 await fetch(`${fixture}/fixture/unavailable`,{method:'POST',body:JSON.stringify({capabilities:ids})});
 const context=await browser.newContext({viewport:{width:1142,height:732}});const page=await context.newPage();const errors=[],outside=[];
 page.on('pageerror',e=>errors.push(e.message));await context.route('**/*',route=>{const u=new URL(route.request().url());if(['http:','https:'].includes(u.protocol)&&!['127.0.0.1','localhost'].includes(u.hostname)){outside.push(u.hostname);return route.abort();}return route.continue();});
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(/\/app\//,{timeout:30000});await page.goto('http://127.0.0.1:5417/app/integrations',{waitUntil:'domcontentloaded'});
 const panel=page.locator('.capabilities-view');const message=panel.getByText('This feature is not available yet.',{exact:true});
 await panel.getByRole('button',{name:'Mailboxes',exact:true}).click();await panel.getByLabel('Mailbox name').fill('Synthetic unavailable mailbox');await panel.getByLabel('Address name').fill('synthetic-unavailable');await panel.getByRole('button',{name:'Create mailbox',exact:true}).click();await message.waitFor();assert.equal(await panel.getByText('Your request is complete.',{exact:true}).count(),0);
 await panel.getByRole('button',{name:'Connect your mailbox',exact:true}).click();await message.waitFor();assert.equal(new URL(page.url()).hostname,'127.0.0.1');
 await panel.getByRole('button',{name:'Your domain',exact:true}).click();await panel.getByLabel('Your domain',{exact:true}).fill('synthetic-owned.example');await panel.getByRole('button',{name:'Attach your domain',exact:true}).click();await message.waitFor();
 await panel.getByRole('button',{name:'Find and buy a domain',exact:true}).click();await panel.getByLabel('Domain search').fill('synthetic');await panel.getByRole('button',{name:'Search domains',exact:true}).click();await panel.getByText('Domain purchase is not available yet; use a domain you own',{exact:true}).waitFor();assert.ok(await panel.getByRole('button',{name:'Attach your domain',exact:true}).isEnabled());
 await panel.getByRole('button',{name:'Lead data',exact:true}).click();await panel.getByLabel('Company domain').fill('synthetic.example');await panel.getByRole('button',{name:'Find company details',exact:true}).click();await message.waitFor();
 await panel.getByRole('button',{name:'Apps',exact:true}).click();await panel.getByLabel('What do you want to do?').fill('Synthetic contact');await panel.getByRole('button',{name:'Find actions',exact:true}).click();await panel.getByRole('button',{name:'Choose action',exact:true}).waitFor();
 await page.screenshot({path:new URL('../../.data/schema/availability-page.png',import.meta.url).pathname});assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);console.log(JSON.stringify({unavailableScreens:5,restOfPageWorks:true,errors,outside}));
} finally {await fetch(`${fixture}/fixture/unavailable`,{method:'POST',body:JSON.stringify({capabilities:[]})});await browser.close();}
