// Research only: shared-session ownership, missing frames, media positive controls.
const { app, BrowserWindow, session } = require('electron');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { pathToFileURL } = require('node:url');
const root=__dirname;
app.setPath('userData',path.join(root,'lifecycle-profile'));
app.setPath('crashDumps',path.join(root,'crash-dumps'));
app.on('window-all-closed',()=>{});
const fake=pathToFileURL(path.join(root,'fake-secret.txt')).href;
const filePDF=pathToFileURL(path.join(root,'fake-document.pdf')).href;
const script = js=>'<body><script>'+js+'</script>';
const windows=new Map();
const installed=new WeakSet();
const events=[];
let installs=0;
function allows(details) {
  try {
    const wc=windows.get(details.webContentsId);
    return !!wc && !wc.isDestroyed() && !!details.frame && !details.frame.detached && details.frame===wc.mainFrame;
  } catch { return false; }
}
function register(win) {
  const wc=win.webContents;
  const id=wc.id;
  windows.set(id,wc);
  wc.once('destroyed',()=>windows.delete(id));
  const ses=wc.session;
  if(installed.has(ses))return;
  installed.add(ses);installs++;
  ses.webRequest.onBeforeRequest({urls:['file://*/*']},(details,callback)=>{
    if(details.url.endsWith('?dispose-probe')) {
      const wc=windows.get(details.webContentsId);
      wc.executeJavaScript(`document.querySelector('#dispose-frame')?.remove()`);
      setTimeout(()=>{
        let state;try{state=details.frame?{detached:details.frame.detached}:null;}catch{state='disposed';}
        events.push({url:details.url,type:details.resourceType,forcedDisposal:true,frame:state,allow:allows(details)});
        callback({cancel:!allows(details)});
      },100);
      return;
    }
    const allow=process.env.PROBE_GUARD==='off'||allows(details);
    let frame;
    try { frame=details.frame?{url:details.frame.url,detached:details.frame.detached,main:details.frame===windows.get(details.webContentsId)?.mainFrame}:null; } catch {frame='disposed';}
    events.push({url:details.url,type:details.resourceType,id:details.webContentsId,frame,allow});
    callback({cancel:!allow});
  });
}
function pdf() {
  const stream='BT /F1 24 Tf 40 130 Td (FAKE PDF CONTROL) Tj ET';
  const objs=['<< /Type /Catalog /Pages 2 0 R >>','<< /Type /Pages /Kids [3 0 R] /Count 1 >>','<< /Type /Page /Parent 2 0 R /MediaBox [0 0 400 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>','<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',`<< /Length ${stream.length} >>\nstream\n${stream}\nendstream`];
  let out='%PDF-1.4\n',offsets=[0];
  for(let i=0;i<objs.length;i++){offsets.push(Buffer.byteLength(out));out+=`${i+1} 0 obj\n${objs[i]}\nendobj\n`;}
  const xref=Buffer.byteLength(out);
  out+=`xref\n0 ${objs.length+1}\n0000000000 65535 f \n`+offsets.slice(1).map(n=>`${String(n).padStart(10,'0')} 00000 n \n`).join('');
  out+=`trailer\n<< /Root 1 0 R /Size ${objs.length+1} >>\nstartxref\n${xref}\n%%EOF`;
  return Buffer.from(out);
}
async function poll(check,ms=4000){const until=Date.now()+ms;let v;do{v=await check();if(v)return v;await new Promise(r=>setTimeout(r,50));}while(Date.now()<until);return v;}
async function frameProbe(win,kind='direct') {
  const js=kind==='detach'?`const w=new Worker(URL.createObjectURL(new Blob([${JSON.stringify(`setTimeout(()=>fetch(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>postMessage({read:true,value})).catch(e=>postMessage({read:false,error:e.name})),10)`) }],{type:'text/javascript'})));w.onmessage=e=>parent.postMessage(e.data,'*');parent.postMessage({remove:true},'*')`:`fetch(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>parent.postMessage({read:true,value},'*')).catch(e=>parent.postMessage({read:false,error:e.name},'*'))`;
  return win.webContents.executeJavaScript(`new Promise(resolve=>{const f=document.createElement('iframe');f.sandbox='allow-scripts';f.srcdoc=${JSON.stringify(script(js))};const handler=e=>{if(e.source!==f.contentWindow)return;if(e.data.remove){f.remove();resolve({removed:true});window.removeEventListener('message',handler)}else{resolve(e.data);f.remove();window.removeEventListener('message',handler)}};window.addEventListener('message',handler);document.body.append(f);setTimeout(()=>resolve({timeout:true}),1000)})`);
}
async function run(){
  await app.whenReady();
  const pdfBytes=pdf();fs.writeFileSync(path.join(root,'fake-document.pdf'),pdfBytes);
  const headers=[];
  const server=http.createServer((req,res)=>{
    headers.push({path:req.url,header:req.headers['x-probe-composition']});
    if(req.url==='/document.pdf'){res.setHeader('Content-Type','application/pdf');res.end(pdfBytes);}
    else if(req.url==='/image.png'){res.setHeader('Content-Type','image/png');res.end(fs.readFileSync(path.join(root,'fake-image.png')));}
    else if(req.url==='/redirect'){res.writeHead(302,{Location:fake});res.end();}
    else {res.setHeader('Access-Control-Allow-Origin','*');res.end('FAKE_HTTP');}
  });
  await new Promise(r=>server.listen(0,'127.0.0.1',r));
  const origin=`http://127.0.0.1:${server.address().port}`;
  const ses=session.defaultSession;
  ses.webRequest.onBeforeSendHeaders({urls:['http://127.0.0.1/*']},(details,cb)=>{details.requestHeaders['X-Probe-Composition']='preserved';cb({requestHeaders:details.requestHeaders});});
  async function make(){const w=new BrowserWindow({show:false,width:700,height:500,webPreferences:{session:ses,sandbox:true,contextIsolation:true,nodeIntegration:false,webSecurity:false,plugins:true,backgroundThrottling:false}});w.webContents.on('console-message',e=>console.log('renderer-console',e.level,e.message));register(w);await w.loadFile(path.join(root,'parent.html'));return w;}
  const main=await make(),issue=await make();
  const sharedSession=main.webContents.session===issue.webContents.session;
  const probes=await Promise.all([frameProbe(main),frameProbe(issue)]);
  const assets=await Promise.all([main,issue].map(w=>w.webContents.executeJavaScript(`import('./trusted-module.mjs').then(m=>({module:m.marker,script:window.trustedAssetLoaded}))`)));
  main.destroy();
  const recreated=await make();
  const recreatedProbe=await frameProbe(recreated);
  const survivorProbe=await frameProbe(issue);
  const noFrame=await ses.fetch(fake).then(r=>r.text()).then(value=>({read:true,value})).catch(e=>({read:false,error:e.message}));
  const detached=await frameProbe(issue,'detach');
  await issue.webContents.executeJavaScript(`{const f=document.createElement('iframe');f.id='dispose-frame';f.sandbox='allow-scripts';f.srcdoc=${JSON.stringify(script(`fetch(${JSON.stringify(fake+'?dispose-probe')}).catch(()=>{})`))};document.body.append(f);}`);
  await new Promise(r=>setTimeout(r,150));
  const localIframePDF=await issue.webContents.executeJavaScript(`new Promise(resolve=>{let f=document.createElement('iframe');f.src=${JSON.stringify(filePDF)};f.onload=()=>resolve('load');document.body.append(f);setTimeout(()=>resolve('timeout'),400)})`);
  await recreated.webContents.executeJavaScript(`document.body.innerHTML='<iframe id="pdf" style="width:600px;height:400px" src="${origin}/document.pdf"></iframe><img id="image" src="${origin}/image.png">';`);
  const pdfFrame=await poll(()=>recreated.webContents.mainFrame.framesInSubtree.find(f=>f.url.startsWith('chrome-extension:')));
  let pdfState;
  if(pdfFrame){
    pdfState=await poll(async()=>{
      try{return await pdfFrame.executeJavaScript(`(()=>{const v=document.querySelector('pdf-viewer');if(!v?.shadowRoot)return null;return {loadProgress:v.loadProgress_,documentLoaded:v.documentLoaded_,body:document.body.innerText.slice(0,200),keys:Object.keys(v).filter(k=>/load|document|page/i.test(k)).slice(0,30),plugin:!!v.shadowRoot?.querySelector('embed')}})()`);}catch{return null;}
    });
    await new Promise(r=>setTimeout(r,500));
    pdfState=await pdfFrame.executeJavaScript(`(()=>{const v=document.querySelector('pdf-viewer');return v?{loadProgress:v.loadProgress_,documentLoaded:v.documentLoaded_,pageCount:v.docLength_,body:document.body.innerText.slice(0,200),plugin:!!v.shadowRoot?.querySelector('embed'),html:v.outerHTML,shadow:v.shadowRoot?.innerHTML.slice(-3500),keys:Object.getOwnPropertyNames(v)}:null})()`);
  }
  const pdfFrames=recreated.webContents.mainFrame.framesInSubtree.map(f=>({url:f.url}));
  fs.writeFileSync(path.join(root,'pdf-control.png'),(await recreated.webContents.capturePage()).toPNG());
  const image=await recreated.webContents.executeJavaScript(`({complete:document.querySelector('#image').complete,width:document.querySelector('#image').naturalWidth})`);
  const redirect=await recreated.webContents.executeJavaScript(`fetch('${origin}/redirect').then(r=>r.text()).then(value=>({read:true,value})).catch(e=>({read:false,error:e.name}))`);
  const missingFrame=allows({webContentsId:recreated.webContents.id,frame:null});
  const missingWindow=allows({webContentsId:999999,frame:recreated.webContents.mainFrame});
  const throwingFrame=allows({webContentsId:recreated.webContents.id,get frame(){throw new Error('disposed')}});
  const result={electron:process.versions.electron,chrome:process.versions.chrome,sharedSession,installs,probes,assets,recreatedProbe,survivorProbe,noFrame,detached,localIframePDF,pdfState,pdfFrames,image,redirect,missingFrame,missingWindow,throwingFrame,headers,events};
  fs.writeFileSync(path.join(root,process.env.PROBE_LIFECYCLE_OUTPUT||'lifecycle-results.json'),JSON.stringify(result,null,2));
  console.log(JSON.stringify({...result,events:events.length},null,2));
  for(const w of [issue,recreated])w.destroy();server.close();app.quit();
}
run().catch(e=>{console.error(e);app.exit(1)});
