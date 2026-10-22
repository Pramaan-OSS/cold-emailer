import {mkdirSync as ensureEvidenceFolder} from 'node:fs';
ensureEvidenceFolder(new URL('../../.data/browser-checks/', import.meta.url), {recursive:true});
import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
const browser=await chromium.launch({headless:true});
try {
 const context=await browser.newContext({viewport:{width:1142,height:732}});const page=await context.newPage();const errors=[],outside=[];
 page.on('pageerror',e=>errors.push(e.message));
 await context.route('**/*',route=>{const u=new URL(route.request().url());if(['http:','https:'].includes(u.protocol)&&!['127.0.0.1','localhost'].includes(u.hostname)){outside.push(u.hostname);return route.abort();}return route.continue();});
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});
 await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();
 await page.waitForURL(/\/app\//,{timeout:30000});await page.goto('http://127.0.0.1:5417/app/integrations',{waitUntil:'domcontentloaded'});
 await page.getByRole('heading',{name:'Connections',exact:true}).waitFor();
 await page.screenshot({path:new URL('../../.data/browser-checks/connections-desktop-headless.png',import.meta.url).pathname});
 const layout=await page.evaluate(()=>({width:innerWidth,pageWidth:document.documentElement.scrollWidth,threadCount:document.querySelectorAll('[data-thread]').length,rightRatio:document.querySelector('.right').getBoundingClientRect().width/document.querySelector('main').getBoundingClientRect().width,crumbLinks:document.querySelectorAll('.crumb a').length,cards:[...document.querySelectorAll('.capabilities-view .card')].map(e=>getComputedStyle(e).borderRadius)}));
 await page.getByRole('button',{name:'Mailboxes',exact:true}).click();
 if(!await page.getByText('headless-native-check@example.test',{exact:true}).count()){await page.getByLabel('Mailbox name').fill('Headless mailbox');await page.getByLabel('Address name').fill('headless-native-check');await page.getByRole('button',{name:'Create mailbox',exact:true}).click();await page.getByText('headless-native-check@example.test',{exact:true}).waitFor({timeout:30000});}
 await page.evaluate(()=>document.documentElement.classList.add('dark'));await page.screenshot({path:new URL('../../.data/browser-checks/mailboxes-dark-headless.png',import.meta.url).pathname});
 await page.setViewportSize({width:400,height:732});await page.screenshot({path:new URL('../../.data/browser-checks/mailboxes-mobile-headless.png',import.meta.url).pathname});const mobile=await page.evaluate(()=>({width:innerWidth,pageWidth:document.documentElement.scrollWidth}));
 await page.goto('http://127.0.0.1:5417/app/emails',{waitUntil:'domcontentloaded'});
 await page.locator('table tbody tr').first().waitFor({timeout:30000});
 assert.equal(await page.locator('a[href*="coldemail-cloud"]').count(),0);
 await page.screenshot({path:new URL('../../.data/browser-checks/local-mailboxes-mobile-headless.png',import.meta.url).pathname});
 assert.equal(layout.width,layout.pageWidth);assert.equal(mobile.width,mobile.pageWidth);assert.ok(layout.rightRatio>=0.6);assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);
 console.log(JSON.stringify({layout,mobile,mailboxSaved:true,localMailboxList:true,errors,outside},null,2));
} finally {await browser.close();}
