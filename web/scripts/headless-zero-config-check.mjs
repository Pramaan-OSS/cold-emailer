import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdirSync} from 'node:fs';
const root=process.env.COLD_EMAIL_CHECK_ROOT ?? new URL('../../',import.meta.url).pathname;
const noKey=process.argv.includes('--no-key');
const browser=await chromium.launch({headless:true});
try {
 const context=await browser.newContext();
 const page=await context.newPage();
 const errors=[],outside=[];
 page.on('pageerror',error=>errors.push(error.message));
 await context.route('**/*',route=>{const u=new URL(route.request().url());if(['http:','https:'].includes(u.protocol)&&!['localhost','127.0.0.1'].includes(u.hostname)){outside.push(u.hostname);return route.abort();}return route.continue();});
 if(noKey){
  await page.goto('http://localhost:5173/auth/login');
  await page.getByText('Add PRAMAAN_API_KEY to .env',{exact:true}).waitFor();
  assert.equal((await context.request.get('http://localhost:8080/health')).status(),200);
 } else {
  const logs=execFileSync('docker',['compose','-f','docker-compose.dev.yml','logs','--no-color','backend'],{cwd:root,encoding:'utf8'});
  const link=logs.match(/http[^\s]*\/setup\?token=[a-f0-9]+/g)?.at(-1);
  assert.ok(link,'a fresh instance must provide its private setup link');
  await page.goto(link);
  const fallback=page.getByText('Use local sign-in',{exact:true});if(await fallback.isVisible())await fallback.click();
  const inputs=page.getByTestId('local-setup').locator('input');
  await inputs.nth(0).fill('Local');await inputs.nth(1).fill('local-check@example.test');await inputs.nth(2).fill('Local-Check-9274!');await inputs.nth(3).fill('Local-Check-9274!');
  await page.getByRole('button',{name:'Create owner account',exact:true}).click();
  await page.waitForURL(/\/onboarding/,{timeout:30000});
  await page.locator('input[name=first_name]').fill('Local');await page.locator('input[name=last_name]').fill('Check');
  await page.getByRole('button',{name:'Continue',exact:true}).click();
  await page.locator('input[name=workspace]').fill('Local workspace');await page.getByRole('button',{name:'Continue',exact:true}).click();
  for(const name of ['Founder','Just me','Reddit'])await page.getByRole('button',{name,exact:true}).click();
  await page.getByRole('button',{name:'Get started',exact:true}).click();
  await page.waitForURL(/\/app\//,{timeout:30000});
  await page.getByRole('link',{name:'Connections',exact:true}).click();
  await page.getByRole('heading',{name:'Connections',exact:true}).waitFor();
  await page.getByPlaceholder('Add a contact or update a spreadsheet').fill('contacts');
  await page.getByRole('button',{name:'Find actions',exact:true}).click();
  await page.getByText('Create or update a contact.',{exact:true}).waitFor();
 }
 mkdirSync(new URL('../../.scratch/zero-config/',import.meta.url),{recursive:true});
 await page.screenshot({path:new URL(`../../.scratch/zero-config/${noKey?'no-key':'connections'}.png`,import.meta.url).pathname});
 assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);
 console.log(JSON.stringify({noKey,mainPage:true,connections:!noKey,errors,outside}));
}finally{await browser.close();}
