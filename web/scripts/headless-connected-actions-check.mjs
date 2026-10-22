import {mkdirSync as ensureEvidenceFolder} from 'node:fs';
ensureEvidenceFolder(new URL('../../.data/browser-checks/', import.meta.url), {recursive:true});
import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
const reset=await fetch('http://127.0.0.1:59090/fixture/reset-connections',{method:'POST'});assert.ok(reset.ok);
const browser=await chromium.launch({headless:true});
try {
 const context=await browser.newContext({viewport:{width:1142,height:732}}),page=await context.newPage(),outside=[],errors=[];
 page.on('pageerror',error=>errors.push(error.message));
 await context.route('**/*',route=>{const url=new URL(route.request().url());if(['http:','https:'].includes(url.protocol)&&!['127.0.0.1','localhost'].includes(url.hostname)){outside.push(url.hostname);return route.abort();}return route.continue();});
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});
 await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(/\/app\//);
 await page.goto('http://127.0.0.1:5417/app/integrations',{waitUntil:'domcontentloaded'});
 const discovery=[];page.on('request',r=>{if(r.url().includes('/capabilities/apps.'))discovery.push(r.url().split('/capabilities/')[1]);});
 await page.getByLabel('What do you want to do?').fill('Create a contact');await page.getByRole('button',{name:'Find actions',exact:true}).click();await page.getByRole('button',{name:'Choose action',exact:true}).click();await page.getByLabel('Contact email').fill('{{.email}}');await page.getByRole('button',{name:'Save for contact actions',exact:true}).click();await page.getByText('Saved for contact actions.',{exact:false}).waitFor({timeout:30000});
 await page.screenshot({path:new URL('../../.data/browser-checks/connected-action-saved-headless.png',import.meta.url).pathname});
 await page.getByTestId('connection-access-notice').waitFor();
 await page.getByRole('button',{name:'Run this action',exact:true}).click();const approval=page.getByRole('alertdialog');await approval.waitFor();assert.equal(discovery.filter(id=>id==='apps.action.execute').length,0);await approval.getByRole('button',{name:'Confirm',exact:true}).click();await page.getByText('Synthetic saved contact',{exact:true}).waitFor();assert.equal(await page.getByRole('link',{name:'Open result',exact:true}).getAttribute('href'),'https://example.test/result');assert.equal(discovery.filter(id=>id==='apps.action.execute').length,1);
 await page.getByRole('button',{name:'Show connected apps',exact:true}).click();await page.getByRole('button',{name:'Disconnect app',exact:true}).waitFor();
 page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'Disconnect app',exact:true}).click();await page.getByText('Your request is complete.',{exact:true}).waitFor();
 await page.getByRole('button',{name:'Show connected apps',exact:true}).click();await page.getByText('Your request is complete.',{exact:true}).waitFor();assert.equal(await page.getByRole('button',{name:'Disconnect app',exact:true}).count(),0);
 assert.ok(discovery.every(id=>['apps.catalog.search','apps.connections.list','apps.connection.revoke','apps.action.execute'].includes(id)));
 assert.deepEqual(outside,[]);assert.deepEqual(errors,[]);console.log(JSON.stringify({discovery:true,catalogueSchemaSelected:true,contactTemplateSaved:true,connectionListedAndRevoked:true,consentExplained:true,writeConfirmed:true,resultListAndLinkParsed:true,outside,errors}));
}finally{await browser.close();}
