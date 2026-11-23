const {app,BrowserWindow,session}=require('electron');
app.setName('Multica Docs Diagram Check');
app.setPath('userData',process.env.DOCS_CHECK_PROFILE);
app.whenReady().then(async()=>{
 session.defaultSession.webRequest.onBeforeRequest((details,done)=>{
  const url=new URL(details.url);
  done({cancel:['http:','https:','ws:','wss:'].includes(url.protocol)&&!['localhost','127.0.0.1'].includes(url.hostname)});
 });
 const window=new BrowserWindow({width:1200,height:920,show:true,webPreferences:{sandbox:true,contextIsolation:true}});
 await window.loadURL('http://localhost:5666/docs-visual-check.html');
});
app.on('window-all-closed',()=>app.quit());
