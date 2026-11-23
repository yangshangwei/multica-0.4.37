const { _electron }=require('@playwright/test');
const {mkdtempSync,readFileSync,mkdirSync}=require('fs');
const {resolve,join}=require('path');
const {tmpdir}=require('os');
(async()=>{
 const root=process.cwd();const task=__dirname;
 mkdirSync(join(task,'screenshots'),{recursive:true});
 const app=await _electron.launch({executablePath:resolve('apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),args:[join(task,'preview-electron.cjs')],env:{...process.env,DOCS_CHECK_PROFILE:mkdtempSync(join(tmpdir(),'multica-docs-visual-'))}});
 try{
  const page=await app.firstWindow(); page.on("pageerror",e=>console.log("PAGEERROR",e.message)); page.on("console",m=>{if(m.type()==="error") console.log("CONSOLE",m.text());});
  await page.goto('http://localhost:5666/docs-visual-check.html?slug=concepts&baseline=1');
  await page.getByRole('heading',{name:'核心概念',exact:true}).waitFor({timeout:10000}).catch(async e=>{console.log(await page.locator('body').innerText());throw e;});
  await page.screenshot({path:join(task,'screenshots/baseline-reader.png')});
  console.log('Baseline screenshot saved');
 } finally{await app.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
