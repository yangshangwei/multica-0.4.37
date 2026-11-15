import fs from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
const wt='/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108';
const out='/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification/visual';
const require=createRequire(wt+'/apps/web/package.json');
const postcss=require('postcss');
const tailwind=require('@tailwindcss/postcss');
const esbuild=require('esbuild');
const {chromium}=require('@playwright/test');
const entry=wt+'/apps/web/app/globals.css';
const fixture=`import React from 'react'; import {createRoot} from 'react-dom/client';
import {Input} from '${wt}/packages/ui/components/ui/input.tsx';
import {Textarea} from '${wt}/packages/ui/components/ui/textarea.tsx';
import {Button} from '${wt}/packages/ui/components/ui/button.tsx';
import {Checkbox} from '${wt}/packages/ui/components/ui/checkbox.tsx';
import {Switch} from '${wt}/packages/ui/components/ui/switch.tsx';
function Demo(){return <main style={{maxWidth:580,margin:'0 auto',padding:32,fontFamily:'Arial,sans-serif'}}>
<h1 className="text-title font-semibold" style={{marginBottom:8}}>Task details</h1><p className="text-body text-muted-foreground" style={{marginBottom:24}}>Form controls · shared Web and Desktop styles</p>
<section style={{display:'grid',gap:20}}>
<label style={{display:'grid',gap:8}}>Title<Input id="normal" defaultValue="Review collaboration changes"/></label>
<label style={{display:'grid',gap:8}}>Owner<Input id="focused" placeholder="Search accounts"/></label>
<label style={{display:'grid',gap:8}}>Workspace<Input id="disabled" disabled defaultValue="Engineering"/></label>
<label style={{display:'grid',gap:8}}>Description<Textarea id="description" placeholder="Add context for the team"/></label>
<div style={{display:'flex',alignItems:'center',gap:12}}><Checkbox id="check"/><label htmlFor="check">Include linked resources</label></div>
<div style={{display:'flex',alignItems:'center',gap:12}}><Switch id="switch"/><label htmlFor="switch">Email notifications</label></div>
<div style={{display:'flex',gap:12,flexWrap:'wrap'}}><Button id="outline" variant="outline">Cancel</Button><Button id="brand" variant="brand">Save changes</Button><Button disabled variant="outline">Unavailable</Button></div>
</section></main>};createRoot(document.getElementById('app')).render(<Demo/>);`;
for(const mode of ['baseline','candidate']){
 let css=await fs.readFile(entry,'utf8');
 if(mode==='baseline'){
  css=css.replace('../../../packages/ui/styles/tokens.css',out+'/baseline/packages/ui/styles/tokens.css').replace('../../../packages/ui/styles/base.css',out+'/baseline/packages/ui/styles/base.css');
  css+='\n@source "'+out+'/baseline/packages/ui";';
 }
 const built=await postcss([tailwind({base:path.dirname(entry)})]).process(css,{from:path.join(path.dirname(entry),'.merge-visual-'+mode+'.css')});
 await fs.writeFile(out+'/'+mode+'.css',built.css+'\nbody{margin:0;background:var(--background);color:var(--foreground)}');
 const result=await esbuild.build({stdin:{contents:fixture,loader:'tsx',resolveDir:wt+'/apps/web'},bundle:true,write:false,format:'iife',platform:'browser',jsx:'automatic',define:{'process.env.NODE_ENV':'"production"'},plugins:mode==='baseline'?[{name:'baseline-ui',setup(build){build.onLoad({filter:/\/packages\/ui\/.*\.(tsx?|jsx?)$/},args=>({contents:execFileSync('git',['show','e541cfb43:'+path.relative(wt,args.path)],{cwd:wt,encoding:'utf8'}),loader:args.path.endsWith('.tsx')?'tsx':'ts',resolveDir:path.dirname(args.path)}));}}]:[]});
 await fs.writeFile(out+'/'+mode+'.js',result.outputFiles[0].contents);
 await fs.writeFile(out+'/'+mode+'.html','<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="'+mode+'.css"></head><body><div id="app"></div><script src="'+mode+'.js"></script></body></html>');
}
const browser=await chromium.launch({headless:true});
const page=await browser.newPage();const evidence=[];
for(const theme of ['light','dark'])for(const width of [720,375])for(const mode of ['baseline','candidate']){
 await page.setViewportSize({width,height:800});await page.goto('file://'+out+'/'+mode+'.html');await page.waitForSelector('#normal');
 await page.addStyleTag({content:'*,*::before,*::after{transition:none!important;animation:none!important}'});
 await page.evaluate(t=>document.documentElement.className=t==='dark'?'dark':'',theme);
 await page.locator('#focused').focus();await page.keyboard.press('Tab');await page.keyboard.press('Shift+Tab');
 await page.screenshot({path:out+'/'+mode+'-'+theme+'-'+width+'.png',fullPage:true});
 const metrics=await page.evaluate(()=>{const selectors=['#normal','#focused','#disabled','#outline','#brand','[data-slot="switch"]'];return {overflow:document.documentElement.scrollWidth>innerWidth,controls:Object.fromEntries(selectors.map(s=>{const e=document.querySelector(s);const c=getComputedStyle(e);const r=e.getBoundingClientRect();return [s,{border:c.borderColor,background:c.backgroundColor,color:c.color,shadow:c.boxShadow,width:r.width,height:r.height}]}))}});
 evidence.push({theme,width,mode,...metrics});
}
await browser.close();await fs.writeFile(out+'/metrics.json',JSON.stringify(evidence,null,2));
console.log(JSON.stringify({screenshots:8,overflowCases:evidence.filter(x=>x.overflow),out}));
