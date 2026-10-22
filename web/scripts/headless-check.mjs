import {mkdirSync} from 'node:fs';
mkdirSync(new URL('../../.data/browser-checks/',import.meta.url),{recursive:true});
import {chromium} from '@playwright/test';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const browser=await chromium.launch({headless:true});
try {
 const context=await browser.newContext({viewport:{width:1142,height:732}});
 const page=await context.newPage();const errors=[];const blocked=[];
 page.on('pageerror',e=>errors.push(e.message));
 await context.route('**/*',route=>{const u=new URL(route.request().url());if(['http:','https:'].includes(u.protocol)&&!['127.0.0.1','localhost'].includes(u.hostname)){blocked.push(u.hostname);return route.abort();}return route.continue();});
 const log=fs.readFileSync('../.data/clean-backend.log','utf8');
 const urls=log.match(/http:\/\/127\.0\.0\.1:5418\/[^\s\u001b]+/g)||[];
 const url=urls.find(u=>u.includes('setup')||u.includes('claim'));if(!url)throw new Error('Local setup link not found');
 await page.goto(url,{waitUntil:'networkidle',timeout:60000});
 await page.waitForTimeout(1000);
 const firstRun=await page.getByRole('button',{name:'Create owner account',exact:true}).count();
 if(firstRun){await page.locator('input').nth(0).fill('Headless Reviewer');
 await page.locator('input').nth(1).fill('reviewer-clean@example.test');
 await page.locator('input').nth(2).fill('Local-Headless-Test-9274!');
 await page.locator('input').nth(3).fill('Local-Headless-Test-9274!');
 await page.getByRole('button',{name:'Create owner account',exact:true}).click();
 }else{
 await page.goto('http://127.0.0.1:5418/auth/login',{waitUntil:'networkidle'});
 await page.locator('input[type=email]').fill('reviewer-clean@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.locator('input[type=password]').fill('Local-Headless-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();
 }
 await page.waitForURL(/\/app\//,{timeout:60000});
 await page.goto('http://127.0.0.1:5418/app/integrations',{waitUntil:'networkidle'});
 await page.waitForTimeout(1500);
 if(new URL(page.url()).pathname==='/onboarding'){
 await page.locator('input[name=first_name]').fill('Headless');await page.locator('input[name=last_name]').fill('Reviewer');await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.locator('input[name=workspace]').fill('Headless Review');await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.getByRole('button',{name:'Founder',exact:true}).click();await page.getByRole('button',{name:'Just me',exact:true}).click();await page.getByRole('button',{name:'Reddit',exact:true}).click();await page.getByRole('button',{name:'Get started',exact:true}).click();
 await page.waitForURL(u=>u.pathname.startsWith('/app/'),{timeout:60000});await page.goto('http://127.0.0.1:5418/app/integrations',{waitUntil:'networkidle'});
 }
 await page.getByRole('heading',{name:'Connections',exact:true}).waitFor({timeout:60000}).catch(async error=>{console.error(JSON.stringify({path:new URL(page.url()).pathname,text:(await page.locator('body').innerText()).slice(0,1800),errors,blocked}));throw error;});
 assert.deepEqual(errors,[]);assert.deepEqual(blocked,[]);
 console.log(JSON.stringify({title:await page.title(),text:(await page.locator('body').innerText()).slice(0,1200),errors,blocked},null,2));
 await page.screenshot({path:new URL('../../.data/browser-checks/clean-setup-headless.png',import.meta.url).pathname});
} finally {await browser.close();}
