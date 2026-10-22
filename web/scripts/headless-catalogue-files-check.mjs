import {mkdirSync as ensureEvidenceFolder} from 'node:fs';
ensureEvidenceFolder(new URL('../../.data/browser-checks/', import.meta.url), {recursive:true});
import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
const browser=await chromium.launch({headless:true});
try{
 const context=await browser.newContext({viewport:{width:1142,height:732}}),page=await context.newPage(),outside=[],errors=[],calls=[];
 page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(r.url().includes('/capabilities/'))calls.push({id:r.url().split('/capabilities/')[1],input:r.postDataJSON()});});
 await context.route('**/*',route=>{const url=new URL(route.request().url());if(['http:','https:'].includes(url.protocol)&&!['127.0.0.1','localhost'].includes(url.hostname)){outside.push(url.hostname);return route.abort();}return route.continue();});
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(/\/app\//);
 await page.goto('http://127.0.0.1:5417/app/integrations',{waitUntil:'domcontentloaded'});
 await page.getByLabel('What do you want to do?',{exact:true}).fill('read synthetic file');await page.getByRole('button',{name:'Find actions',exact:true}).click();await page.getByText('Inspect synthetic document',{exact:true}).waitFor();assert.equal(calls.filter(c=>c.id==='apps.catalog.search').length,1);
 await page.getByRole('button',{name:'Find actions',exact:true}).click();await page.getByText('Catalogue page ready. No more pages were loaded.',{exact:true}).waitFor();assert.equal(calls.filter(c=>c.id==='apps.catalog.search').length,1);
 await page.getByRole('button',{name:'Load more actions (USD 0.001 per new page)',exact:true}).click();await page.getByText('Read synthetic document',{exact:true}).waitFor();const searches=calls.filter(c=>c.id==='apps.catalog.search');assert.equal(searches.length,2);assert.deepEqual(searches[1].input,{query:'read synthetic file',limit:50,cursor:'fixture-next'});assert.equal(await page.getByRole('button',{name:/Load more actions/}).count(),0);
 await page.getByText('Read synthetic document',{exact:true}).locator('..').getByRole('button',{name:'Choose action',exact:true}).click();await page.getByRole('button',{name:'Run this action',exact:true}).click();await page.getByRole('alertdialog').getByRole('button',{name:'Confirm',exact:true}).click();await page.getByRole('button',{name:'Read downloaded file 1 as text (uses credits)',exact:true}).click();await page.getByText('Synthetic parsed file text.',{exact:true}).waitFor();
 assert.equal(calls.filter(c=>c.id==='apps.action.execute').length,1);assert.deepEqual(calls.find(c=>c.id==='document.parse').input,{fileId:'file_synthetic_document',ocr:false});assert.equal(await page.getByRole('button',{name:'Read downloaded file 1 as text (uses credits)',exact:true}).count(),1);
 await page.screenshot({path:new URL('../../.data/browser-checks/catalogue-file-text-headless.png',import.meta.url).pathname});assert.deepEqual(outside,[]);assert.deepEqual(errors,[]);console.log(JSON.stringify({twoPages:true,lastPage:true,cachedRepeat:true,noAutomaticCrawl:true,downloadedFileParsed:true,actionRetained:true,oneExecution:true,outside,errors}));
}finally{await browser.close();}
