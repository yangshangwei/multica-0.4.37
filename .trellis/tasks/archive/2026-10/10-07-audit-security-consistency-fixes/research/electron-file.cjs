const {app,BrowserWindow}=require('electron');
app.on('window-all-closed',()=>{});app.setPath('userData',process.env.AUDIT_ELECTRON_DATA);
(async()=>{await app.whenReady();const results=[];
for(const webSecurity of [true,false]){
 const win=new BrowserWindow({show:false,webPreferences:{sandbox:true,contextIsolation:true,nodeIntegration:false,webSecurity,plugins:true}});
 await win.loadFile(require('path').join(__dirname,'parent.html'));
 const result=await win.webContents.executeJavaScript("new Promise(resolve=>{\n  window.addEventListener('message',e=>resolve(e.data),{once:true});\n  const frame=document.createElement('iframe');frame.sandbox='allow-scripts';\n  frame.srcdoc=\"<script>fetch(\\\"file:///var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-code-audit-jzu0hkn0/fake-local.txt\\\").then(r=>r.text()).then(value=>parent.postMessage({fileReadable:true,value},'*')).catch(e=>parent.postMessage({fileReadable:false,error:e.name},'*'));</script>\";\n  document.body.appendChild(frame);setTimeout(()=>resolve({timeout:true}),3000);\n })");
 results.push({webSecurity,...result});win.destroy();}
console.log(JSON.stringify(results));app.quit();})().catch(e=>{console.error(e);app.exit(1)});