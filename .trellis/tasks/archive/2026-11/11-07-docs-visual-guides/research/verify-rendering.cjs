const { _electron,chromium,expect }=require('@playwright/test');
const fs=require('fs');const {resolve,join}=require('path');const {tmpdir}=require('os');
const task=__dirname;
const shots=join(task,'screenshots');fs.mkdirSync(shots,{recursive:true});
const slugs=['concepts','how-multica-works','developers/architecture','tasks','collaboration','collaboration-knowledge','squads','mcp','triage','iterations'];
const counts={collaboration:2,'developers/architecture':2};
const results={native:[],public:[],errors:[]};
(async()=>{
 const app=await _electron.launch({executablePath:resolve('apps/desktop/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron'),args:[join(task,'preview-electron.cjs')],env:{...process.env,DOCS_CHECK_PROFILE:fs.mkdtempSync(join(tmpdir(),'multica-docs-visual-'))}});
 let browser;
 try{
  const page=await app.firstWindow(); page.setDefaultTimeout(20000);page.on('pageerror',e=>results.errors.push('native: '+e.message));
  for(const theme of ['light','dark'])for(const slug of slugs){
   await page.goto(`http://localhost:5666/docs-visual-check.html?slug=${slug}&theme=${theme}`);
   console.log('Native',theme,slug); const diagrams=page.locator('.docs-diagram');await expect(diagrams).toHaveCount(counts[slug]||1,{timeout:30000});
   for(let i=0;i<(counts[slug]||1);i++){
    const diagram=diagrams.nth(i);await diagram.scrollIntoViewIfNeeded();
    const frame=diagram.locator('iframe');await expect(frame).toHaveAttribute('srcdoc',/<svg/,{timeout:20000});
    const source=await frame.getAttribute('srcdoc');if(!source.includes('<text'))throw Error('No SVG text '+slug);
    const bounds=await diagram.boundingBox();
    await frame.contentFrame().locator('svg').waitFor({state:'visible'});
    await app.evaluate(({BrowserWindow},height)=>BrowserWindow.getAllWindows()[0].setSize(1200,Math.max(1000,height+180)),Math.ceil(bounds.height));
    await diagram.scrollIntoViewIfNeeded();
    await expect.poll(()=>frame.contentFrame().locator('svg').evaluate(el=>el.getBoundingClientRect().height)).toBeGreaterThan(0); await page.waitForTimeout(150);
    const metrics=await frame.evaluate(el=>({width:el.offsetWidth,height:el.offsetHeight,sandbox:el.getAttribute('sandbox')}));
    await diagram.screenshot({path:join(shots,`native-${theme}-${slug.replaceAll('/','-')}-${i}.png`)});
    results.native.push({slug,theme,index:i,bounds,metrics});
   }
   const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth+1);if(overflow)throw Error('Page overflow '+slug);
  }
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setSize(520,850));
  for(const theme of ['light','dark']){
   await page.goto(`http://localhost:5666/docs-visual-check.html?slug=collaboration&theme=${theme}`);
   const d=page.locator('.docs-diagram').first();await d.scrollIntoViewIfNeeded();await expect(d.locator('iframe')).toHaveAttribute('srcdoc',/<svg/); await d.frameLocator('iframe').locator('svg').waitFor({state:'visible'}); await page.waitForTimeout(150);
   await page.screenshot({path:join(shots,`native-${theme}-narrow.png`)});
   const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth+1);if(overflow)throw Error('Narrow page overflow');
   const opener=d.getByRole('button',{name:'打开图表查看器'});await opener.focus();await page.keyboard.press('Enter');
   const dialog=page.getByRole('dialog');await expect(dialog).toBeVisible();
   await expect(dialog.locator('iframe')).toHaveAttribute('srcdoc',/<svg/);
   await dialog.frameLocator('iframe').locator('svg').waitFor({state:'visible'});
   await page.screenshot({path:join(shots,`native-${theme}-viewer.png`)});
   const before=await dialog.locator('.zoom-canvas-content').getAttribute('style');
   await dialog.getByRole('button',{name:'放大',exact:true}).click();
   if(before===await dialog.locator('.zoom-canvas-content').getAttribute('style'))throw Error('Zoom did not change');
   await dialog.getByRole('button',{name:'适应窗口',exact:true}).click();
   if(theme==='dark'){ const current=await dialog.locator('iframe').getAttribute('srcdoc'); await page.evaluate(()=>document.documentElement.classList.remove('dark')); await expect(dialog.locator('iframe')).not.toHaveAttribute('srcdoc',current); await expect(dialog).toBeVisible(); await page.evaluate(()=>document.documentElement.classList.add('dark')); await expect(dialog.locator('iframe')).toHaveAttribute('srcdoc',/color-scheme: dark/); }
   await dialog.getByRole('button',{name:'显示源码',exact:true}).click();await expect(dialog.locator('pre')).toBeVisible();
   await page.keyboard.press('Escape');await expect(dialog).not.toBeVisible();await expect(opener).toBeFocused();
  }
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setSize(390,844));
  await page.goto('http://localhost:5666/docs-visual-check.html?slug=how-multica-works&theme=dark');
  const small=page.locator('.docs-diagram').first(); await small.scrollIntoViewIfNeeded(); await small.frameLocator('iframe').locator('svg').waitFor({state:'visible'});
  if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1))throw Error('390px overflow');
  const scroller=small.locator('.mermaid-diagram-scroll');await scroller.evaluate(el=>{el.scrollLeft=el.scrollWidth;});
  if(await scroller.evaluate(el=>el.scrollLeft<=0))throw Error('Diagram did not scroll');
  await page.screenshot({path:join(shots,'native-dark-390-scroll-end.png')});
  await page.goto('http://localhost:5666/docs-visual-check.html?slug=concepts&broken=1');
  await expect(page.getByText('无法渲染 Mermaid 图。',{exact:true})).toBeVisible();
  await expect(page.getByText('this is invalid mermaid',{exact:true})).toBeVisible();
  await expect(page.getByText('后续正文仍然可读。',{exact:true})).toBeVisible();
  await page.screenshot({path:join(shots,'native-error-fallback.png')});
  browser=await chromium.launch({headless:true});
  const web=await browser.newPage({viewport:{width:1280,height:1000}});web.on('pageerror',e=>results.errors.push('public: '+e.message));
  for(const theme of ['light','dark']){
   await web.addInitScript(t=>localStorage.setItem('theme',t),theme);
   for(const slug of slugs){
    await web.goto(`http://localhost:4057/docs/zh/${slug}`,{waitUntil:'domcontentloaded'});
    const diagrams=web.getByRole('region',{name:'图示',exact:true});await expect(diagrams).toHaveCount(counts[slug]||1,{timeout:20000});
    for(let i=0;i<(counts[slug]||1);i++){
     const d=diagrams.nth(i);await d.scrollIntoViewIfNeeded();await expect(d.locator('svg')).toBeVisible();
     await d.screenshot({path:join(shots,`public-${theme}-${slug.replaceAll('/','-')}-${i}.png`)});
     results.public.push({slug,theme,index:i,svg:await d.locator('svg').getAttribute('viewBox')});
    }
    if(await web.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1))throw Error('Public overflow '+slug);
   }
  }
  await web.setViewportSize({width:390,height:844});
  await web.goto('http://localhost:4057/docs/zh/how-multica-works');
  await web.getByRole('region',{name:'图示',exact:true}).scrollIntoViewIfNeeded();
  await web.screenshot({path:join(shots,'public-dark-narrow.png')});
  if(await web.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1))throw Error('Public narrow overflow');
  if(results.errors.length)throw Error('Page errors: '+results.errors.join(';'));
  console.log(JSON.stringify({native:results.native.length,public:results.public.length,errors:results.errors}));
 }finally{
  fs.writeFileSync(join(task,'rendering-results.json'),JSON.stringify(results,null,2));
  if(browser)await browser.close();await app.close();
 }
})().catch(e=>{console.error(e);process.exitCode=1;});
