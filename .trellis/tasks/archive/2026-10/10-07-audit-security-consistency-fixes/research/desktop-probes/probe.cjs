// Research only. Uses fake fixtures and a loopback-only HTTP positive control.
const { app, BrowserWindow, session, webContents } = require('electron');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { pathToFileURL } = require('node:url');
const root = __dirname;
app.setPath('userData', path.join(root, 'electron-profile'));
app.setPath('crashDumps', path.join(root, 'crash-dumps'));
app.on('window-all-closed', () => {});
const sleep = ms => new Promise(r => setTimeout(r, ms));
const fake = pathToFileURL(path.join(root, 'fake-secret.txt')).href;
const fakeScript = pathToFileURL(path.join(root, 'fake-script.js')).href;
const fakeWorker = pathToFileURL(path.join(root, 'fake-worker.js')).href;
const fakePage = pathToFileURL(path.join(root, 'fake-page.html')).href;
const fakeImage = pathToFileURL(path.join(root, 'fake-image.png')).href;
const fakeStyle = pathToFileURL(path.join(root, 'fake-style.css')).href;
const parentPath = path.join(root, 'parent.html');
const payloadScript = js => '<script>' + js + '</script>';
const fileRead = label => `fetch(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>top.postMessage({probe:true,label:${JSON.stringify(label)},read:true,value},'*')).catch(e=>top.postMessage({probe:true,label:${JSON.stringify(label)},read:false,error:e.name},'*'))`;
fs.writeFileSync(path.join(root, 'fake-secret.txt'), 'FAKE_ONLY_LOCAL_FILE_MARKER');
fs.writeFileSync(path.join(root, 'fake-script.js'), "top.postMessage({probe:true,label:'file-script',executed:true},'*')");
fs.writeFileSync(path.join(root, 'fake-worker.js'), `fetch(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>postMessage({read:true,value})).catch(e=>postMessage({read:false,error:e.name}))`);
fs.writeFileSync(path.join(root, 'fake-page.html'), payloadScript(fileRead('file-navigation')));
fs.writeFileSync(parentPath, '<!doctype html><title>isolated fake-data probe</title><script src="./trusted-asset.js"></script><body></body>');
fs.writeFileSync(path.join(root, 'trusted-asset.js'), 'window.trustedAssetLoaded = true;');
fs.writeFileSync(path.join(root, 'trusted-module.mjs'), 'export const marker = "TRUSTED_DYNAMIC_MODULE";');
fs.writeFileSync(path.join(root, 'fake-style.css'), 'body { --fake-file-style: LOADED; }');
fs.writeFileSync(path.join(root, 'fake-image.png'), Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aT9sAAAAASUVORK5CYII=', 'base64'));

const csp = "default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' http: https: data: blob:; style-src 'unsafe-inline' http: https:; img-src http: https: data: blob:; font-src http: https: data:; media-src http: https: data: blob:; connect-src http: https: ws: wss: data: blob:; frame-src 'none'; worker-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'";
const escapeAttr = s => s.replaceAll('&', '&amp;').replaceAll('"','&quot;').replaceAll('<','&lt;');
function makeWrapper(html) {
  const encoded = Buffer.from(html, 'utf8').toString('base64');
  return `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="${escapeAttr(csp)}"><style>html,body,iframe{width:100%;height:100%;margin:0;border:0}</style></head><body><script>const child=document.createElement('iframe');child.sandbox='allow-scripts';child.srcdoc=new TextDecoder().decode(Uint8Array.from(atob('${encoded}'), c=>c.charCodeAt(0)));document.body.append(child)</script></body></html>`;
}

async function run() {
  await app.whenReady();
  const server = http.createServer((req, res) => {
    res.setHeader('Access-Control-Allow-Origin', '*');
    if (req.url === '/evil.html') { res.setHeader('Content-Type','text/html');res.end(payloadScript(fileRead('http-document'))); }
    else if (req.url === '/worker.js') { res.setHeader('Content-Type','text/javascript');res.end(fs.readFileSync(path.join(root, 'fake-worker.js'))); }
    else if (req.url === '/script.js') { res.setHeader('Content-Type','text/javascript');res.end("top.postMessage({probe:true,label:'http-script',executed:true},'*')"); }
    else if (req.url === '/file-redirect') { res.writeHead(302, {Location:fake});res.end(); }
    else { res.setHeader('Content-Type','text/plain');res.end('FAKE_HTTP_CONTROL'); }
  });
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  const origin = `http://127.0.0.1:${server.address().port}`;
  const workerCode = `fetch(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>postMessage({read:true,value})).catch(e=>postMessage({read:false,error:e.name}))`;
  const htmlRead = label => payloadScript(fileRead(label));
  const scriptLiteral = text => JSON.stringify(text).replaceAll('<','\\u003c');
  const tasks = {
    direct: htmlRead('direct'),
    xhr: payloadScript(`let x=new XMLHttpRequest();x.open('GET',${JSON.stringify(fake)});x.onload=()=>top.postMessage({probe:true,label:'xhr',read:true,value:x.responseText},'*');x.onerror=()=>top.postMessage({probe:true,label:'xhr',read:false},'*');x.send()`),
    file_script: '<script src="'+fakeScript+'"></script>',
    nested_srcdoc: payloadScript(`let f=document.createElement('iframe');f.srcdoc=${scriptLiteral(htmlRead('nested-srcdoc'))};document.body.append(f)`),
    nested_data: payloadScript(`let f=document.createElement('iframe');f.src='data:text/html;base64,${Buffer.from(htmlRead('nested-data')).toString('base64')}';document.body.append(f)`),
    nested_blob: payloadScript(`let f=document.createElement('iframe');f.src=URL.createObjectURL(new Blob([${scriptLiteral(htmlRead('nested-blob'))}],{type:'text/html'}));document.body.append(f)`),
    worker_blob: payloadScript(`try{let w=new Worker(URL.createObjectURL(new Blob([${scriptLiteral(workerCode)}],{type:'text/javascript'})));w.onmessage=e=>top.postMessage({probe:true,label:'worker-blob',...e.data},'*');w.onerror=e=>top.postMessage({probe:true,label:'worker-blob',workerError:e.message},'*')}catch(e){top.postMessage({probe:true,label:'worker-blob',error:e.name},'*')}`),
    worker_data: payloadScript(`try{let w=new Worker('data:text/javascript;base64,${Buffer.from(workerCode).toString('base64')}');w.onmessage=e=>top.postMessage({probe:true,label:'worker-data',...e.data},'*');w.onerror=e=>top.postMessage({probe:true,label:'worker-data',workerError:e.message},'*')}catch(e){top.postMessage({probe:true,label:'worker-data',error:e.name},'*')}`),
    worker_file: payloadScript(`try{let w=new Worker(${JSON.stringify(fakeWorker)});w.onmessage=e=>top.postMessage({probe:true,label:'worker-file',...e.data},'*');w.onerror=e=>top.postMessage({probe:true,label:'worker-file',workerError:e.message},'*')}catch(e){top.postMessage({probe:true,label:'worker-file',error:e.name},'*')}`),
    worker_http: payloadScript(`try{let w=new Worker('${origin}/worker.js');w.onmessage=e=>top.postMessage({probe:true,label:'worker-http',...e.data},'*');w.onerror=e=>top.postMessage({probe:true,label:'worker-http',workerError:e.message},'*')}catch(e){top.postMessage({probe:true,label:'worker-http',error:e.name},'*')}`),
    import_scripts: payloadScript(`try{let w=new Worker(URL.createObjectURL(new Blob([${scriptLiteral(`try{importScripts(${JSON.stringify(fakeWorker)})}catch(e){postMessage({importError:e.name})}`)}],{type:'text/javascript'})));w.onmessage=e=>top.postMessage({probe:true,label:'import-scripts',...e.data},'*');w.onerror=e=>top.postMessage({probe:true,label:'import-scripts',workerError:e.message},'*')}catch(e){top.postMessage({probe:true,label:'import-scripts',error:e.name},'*')}`),
    remove_meta: payloadScript(`document.querySelectorAll('meta').forEach(m=>m.remove());${fileRead('remove-meta')}`),
    document_open: payloadScript(`document.open();document.write(${scriptLiteral(htmlRead('document-open'))});document.close()`),
    navigate_file: payloadScript(`location.href=${JSON.stringify(fakePage)}`),
    navigate_data: payloadScript(`location.href='data:text/html;base64,${Buffer.from(htmlRead('navigate-data')).toString('base64')}'`),
    navigate_blob: payloadScript(`location.href=URL.createObjectURL(new Blob([${scriptLiteral(htmlRead('navigate-blob'))}],{type:'text/html'}))`),
    navigate_http: payloadScript(`location.href='${origin}/evil.html'`),
    nested_http: payloadScript(`let f=document.createElement('iframe');f.src='${origin}/evil.html';document.body.append(f)`),
    file_image: payloadScript(`let im=new Image();im.onload=()=>{let c=document.createElement('canvas');c.width=c.height=1;c.getContext('2d').drawImage(im,0,0);try{top.postMessage({probe:true,label:'file-image',pixels:[...c.getContext('2d').getImageData(0,0,1,1).data]},'*')}catch(e){top.postMessage({probe:true,label:'file-image',error:e.name},'*')}};im.onerror=()=>top.postMessage({probe:true,label:'file-image',blocked:true},'*');im.src=${JSON.stringify(fakeImage)}`),
    file_style: '<link rel="stylesheet" href="'+fakeStyle+'">'+payloadScript(`setTimeout(()=>top.postMessage({probe:true,label:'file-style',value:getComputedStyle(document.body).getPropertyValue('--fake-file-style')},'*'),150)`),
    file_css_import: '<style>@import url("'+fakeStyle+'");</style>'+payloadScript(`setTimeout(()=>top.postMessage({probe:true,label:'file-css-import',value:getComputedStyle(document.body).getPropertyValue('--fake-file-style')},'*'),150)`),
    file_object: '<object data="'+fakePage+'" type="text/html"></object>',
    file_embed: '<embed src="'+fakePage+'" type="text/html">',
    shared_worker: payloadScript(`try{let w=new SharedWorker(URL.createObjectURL(new Blob([${scriptLiteral(`onconnect=e=>{const port=e.ports[0];fetch(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>port.postMessage({read:true,value})).catch(e=>port.postMessage({read:false,error:e.name}))}`)}],{type:'text/javascript'})));w.port.onmessage=e=>top.postMessage({probe:true,label:'shared-worker',...e.data},'*');w.onerror=()=>top.postMessage({probe:true,label:'shared-worker',workerError:true},'*')}catch(e){top.postMessage({probe:true,label:'shared-worker',error:e.name},'*')}`),
    delegated_parent: payloadScript(`for (let access of ['fetch','Function','document']) { try { let p=top[access];if(access==='fetch')p(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>top.postMessage({probe:true,label:'delegated-parent',access,read:true,value},'*'));else if(access==='Function')p('return fetch')()(${JSON.stringify(fake)}).then(r=>r.text()).then(value=>top.postMessage({probe:true,label:'delegated-parent',access,read:true,value},'*'));else p.open(); } catch(e){top.postMessage({probe:true,label:'delegated-parent',access,error:e.name},'*')} }`),
    frame_spoof: payloadScript(`let f=document.createElement('iframe');f.name='_top';f.sandbox='allow-scripts allow-same-origin';f.srcdoc=${scriptLiteral(htmlRead('frame-spoof'))};document.body.append(f)`),
    parent_access: payloadScript(`try{let d=parent.document;top.postMessage({probe:true,label:'parent-access',allowed:!!d},'*')}catch(e){top.postMessage({probe:true,label:'parent-access',allowed:false},'*')}`),
    positive: '<button onclick="top.postMessage({probe:true,label:\'click\',interactive:true},\'*\')">Control</button><script src="'+origin+'/script.js"></script>'+payloadScript(`document.querySelector('button').click();fetch('${origin}/control').then(r=>r.text()).then(value=>top.postMessage({probe:true,label:'http-fetch',value},'*'))`),
  };
  const results=[];
  const modes = process.env.PROBE_MODES?.split(',') || ['baseline','prefix-csp','wrapper-csp-opaque','wrapper-csp','native-frame-deny','scripts-off'];
  for(const mode of modes) {
    const partition = 'probe-'+mode+'-'+Date.now();
    const ses = session.fromPartition(partition);
    const requests=[];
    ses.webRequest.onBeforeRequest({urls:['file://*/*']}, (details, callback) => {
      const frame=details.frame;
      const wc=details.webContentsId ? webContents.fromId(details.webContentsId) : null;
      const allow=!!frame && !!wc && frame === wc.mainFrame;
      let frameInfo;
      try { frameInfo=frame ? {url:frame.url,top:frame === wc?.mainFrame,detached:frame.detached} : null; } catch { frameInfo='disposed'; }
      requests.push({case:currentCase,url:details.url,resourceType:details.resourceType,webContentsId:details.webContentsId,frame:frameInfo,cancel:mode==='native-frame-deny'&&!allow});
      callback({cancel:mode==='native-frame-deny'&&!allow});
    });
    let currentCase='parent-load';
    const win=new BrowserWindow({show:false,webPreferences:{partition,sandbox:true,contextIsolation:true,nodeIntegration:false,webSecurity:false,plugins:true}});
    const consoleMessages=[];
    win.webContents.on('console-message', (event) => { if(event.level>=2)consoleMessages.push({case:currentCase,message:event.message}); });
    await win.loadFile(parentPath);
    const trusted=await win.webContents.executeJavaScript('window.trustedAssetLoaded === true');
    const trustedChecks=await win.webContents.executeJavaScript(`Promise.all([fetch(${JSON.stringify(fake)}).then(r=>r.text()),import('./trusted-module.mjs').then(m=>m.marker),new Promise(resolve=>{let im=new Image();im.onload=()=>resolve('IMAGE_LOADED');im.onerror=()=>resolve('IMAGE_ERROR');im.src=${JSON.stringify(fakeImage)};document.body.append(im)}),new Promise(resolve=>{let w=new Worker('./fake-worker.js');w.onmessage=e=>{resolve(e.data);w.terminate()};w.onerror=e=>resolve({workerError:e.message});setTimeout(()=>{resolve('WORKER_TIMEOUT');w.terminate()},500)})])`);
    const cases={};
    for(const [name, source] of Object.entries(tasks)) {
      currentCase=name;
      const html='<body>'+source;
      const doc=mode.startsWith('wrapper-csp')?makeWrapper(html):mode==='prefix-csp'?`<!doctype html><meta http-equiv="Content-Security-Policy" content="${escapeAttr(csp)}">`+html:html;
      const sandbox=mode==='scripts-off'?'':mode==='wrapper-csp'?'allow-scripts allow-same-origin':'allow-scripts';
      cases[name]=await win.webContents.executeJavaScript(`new Promise(resolve=>{document.body.innerHTML='';const found=[];function listen(e){if(e.data&&e.data.probe)found.push(e.data)}window.addEventListener('message',listen);let frame=document.createElement('iframe');frame.sandbox=${JSON.stringify(sandbox)};frame.srcdoc=${JSON.stringify(doc)};document.body.append(frame);setTimeout(()=>{window.removeEventListener('message',listen);resolve(found)},350)})`);
    }
    results.push({mode,trusted,trustedChecks,cases,requests,consoleMessages});
    win.destroy();
  }
  server.close();
  fs.writeFileSync(path.join(root,process.env.PROBE_OUTPUT || 'results.json'),JSON.stringify({electron:process.versions.electron,chrome:process.versions.chrome,results},null,2));
  console.log(JSON.stringify(results.map(r=>({mode:r.mode,trusted:r.trusted,cases:r.cases,fileRequests:r.requests.length})),null,2));
  app.quit();
}
run().catch(e=>{console.error(e);app.exit(1)});
