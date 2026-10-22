import {chromium} from '@playwright/test';
import assert from 'node:assert/strict';
const browser=await chromium.launch({headless:true});
try {
 const context=await browser.newContext();const page=await context.newPage();const errors=[],outside=[];let starts=0,cancels=0,cancelled=false;
 page.on('pageerror',error=>errors.push(error.message));
 await context.route('**/*',route=>{const url=new URL(route.request().url());if(['http:','https:'].includes(url.protocol)&&!['localhost','127.0.0.1'].includes(url.hostname)){outside.push(url.hostname);return route.abort();}return route.continue();});
 await context.route('**/auth/pramaan/v1/**',async route=>{
  const path=new URL(route.request().url()).pathname;
  const reply=(body,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
  if(path.endsWith('/outcomes/lead.find/estimate'))return reply({plan:'synthetic-plan',ready:true,can_run:true,price_max_micro:4200,steps:[{label:'Find leads',status:'will_run'}]});
  if(path.endsWith('/outcomes/lead.find')){
   if(route.request().method()==='GET')return reply({id:'lead.find',available:true,steps:[]});
   starts++;return reply({job_id:'synthetic-long-job',status:'running'},202);
  }
  if(path.endsWith('/jobs/synthetic-long-job/cancel')){cancels++;cancelled=true;return reply({job_id:'synthetic-long-job',status:'cancelled'});}
  if(path.endsWith('/jobs/synthetic-long-job'))return reply(cancelled?{job_id:'synthetic-long-job',status:'cancelled',result:{outcome:'lead.find',status:'failed',steps:[{id:'step',status:'failed',effect:'unknown'}],cost:{}}}:{job_id:'synthetic-long-job',status:'running'});
  return route.continue();
 });
 await page.goto('http://127.0.0.1:5417/auth/login',{waitUntil:'domcontentloaded'});
 await page.locator('input[type=email]').fill('reviewer@example.test');await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.locator('input[type=password]').fill('ColdEmail-Local-Test-9274!');await page.getByRole('button',{name:'Sign in',exact:true}).click();
 await page.waitForURL(/\/app\//,{timeout:30000});await page.goto('http://127.0.0.1:5417/app/integrations',{waitUntil:'domcontentloaded'});
 await page.getByRole('button',{name:'Lead data',exact:true}).click();
 await page.getByLabel('Describe the companies').fill('Synthetic software companies');await page.getByLabel('Contact roles (comma-separated, up to three)').fill('Sales');await page.getByLabel('Stated basis for contact research').selectOption('consent');
 await page.getByRole('button',{name:'Review lead search',exact:true}).click();const dialog=page.getByRole('alertdialog');await dialog.waitFor();assert.equal(starts,0);
 await dialog.getByRole('button',{name:'Confirm',exact:true}).click();
 await page.getByRole('button',{name:'Cancel lead search',exact:true}).click();
 await page.getByText('A step has an unknown result. Check before retrying; this run will not be repeated automatically.',{exact:true}).waitFor({timeout:15000});
 assert.equal(starts,1);assert.equal(cancels,1);assert.deepEqual(errors,[]);assert.deepEqual(outside,[]);
 console.log(JSON.stringify({approvalBeforeRun:true,cancelVisible:true,starts,cancels,unknownRequiresChecking:true,errors,outside}));
} finally {await browser.close();}
