import {mkdirSync as ensureEvidenceFolder} from 'node:fs';
ensureEvidenceFolder(new URL('../../.data/browser-checks/', import.meta.url), {recursive:true});
import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
const browser=await chromium.launch({headless:true});
try {
 const context=await browser.newContext({viewport:{width:1142,height:732}});const page=await context.newPage();const errors=[],outside=[],runs=[];
 page.on('pageerror',e=>errors.push(e.message));
 await context.route('**/*',route=>{const u=new URL(route.request().url());if(['http:','https:'].includes(u.protocol)&&!['127.0.0.1','localhost'].includes(u.hostname)){outside.push(u.hostname);return route.abort();}return route.continue();});
 page.on('request',r=>{if(r.method()==='POST'&&r.url().endsWith('/outcomes/lead.find'))runs.push(r);});
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});
 await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();
 await page.waitForURL(/\/app\//,{timeout:30000});await page.goto('http://127.0.0.1:5417/app/integrations',{waitUntil:'domcontentloaded'});
 await page.getByRole('button',{name:'Lead data',exact:true}).click();
 await page.getByLabel('Describe the companies').fill('Small software companies');await page.getByLabel('Contact roles (comma-separated, up to three)').fill('Head of sales');await page.getByLabel('Stated basis for contact research').selectOption('consent');
 await page.getByRole('button',{name:'Review lead search',exact:true}).click();
 const dialog=page.getByRole('alertdialog');await dialog.waitFor();assert.ok((await dialog.innerText()).includes('Maximum total: $0.004200'));assert.equal(runs.length,0);
 await page.waitForFunction(()=>{const d=document.querySelector('[role=alertdialog]');return d&&Number(getComputedStyle(d).opacity)>0.99&&Number(getComputedStyle(d.parentElement).opacity)>0.99;});
 await page.screenshot({path:new URL('../../.data/browser-checks/outcomes-approval-headless.png',import.meta.url).pathname});
 await dialog.getByRole('button',{name:'Cancel',exact:true}).click();await page.getByText('Lead search was cancelled; nothing ran.',{exact:true}).waitFor();assert.equal(runs.length,0);
 await page.getByRole('button',{name:'Review lead search',exact:true}).click();await dialog.waitFor();await dialog.getByRole('button',{name:'Confirm',exact:true}).click();
 await page.getByText('Email check: verified',{exact:true}).waitFor({timeout:30000});assert.equal(runs.length,1);
 const headers=await runs[0].allHeaders();assert.ok(headers['pramaan-plan']);for(const h of ['pramaan-account','pramaan-end-user','pramaan-max-price-micro'])assert.equal(headers[h],undefined);
 await page.setViewportSize({width:400,height:732});await page.screenshot({path:new URL('../../.data/browser-checks/outcomes-leads-mobile-headless.png',import.meta.url).pathname});
 const layout=await page.evaluate(()=>({width:innerWidth,pageWidth:document.documentElement.scrollWidth}));assert.equal(layout.width,layout.pageWidth);assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);
 console.log(JSON.stringify({estimateShown:true,cancelDidNotRun:true,approvedRunCount:runs.length,backgroundJobCompleted:true,layout,errors,outside},null,2));
} finally {await browser.close();}
