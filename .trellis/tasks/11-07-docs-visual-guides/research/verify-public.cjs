const {chromium,expect}=require('@playwright/test'); const fs=require('fs'); const {join,resolve}=require('path'); const task=__dirname; const shots=join(task,'screenshots'); const slugs=['concepts','how-multica-works','developers/architecture','tasks','collaboration','collaboration-knowledge','squads','mcp','triage','iterations']; const counts={collaboration:2,'developers/architecture':2};const results={public:[],errors:[]}; let browser; (async()=>{try{
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

console.log(JSON.stringify(results));}finally{fs.writeFileSync(join(task,'public-results.json'),JSON.stringify(results,null,2));if(browser)await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
