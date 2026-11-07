const { app, BrowserWindow, systemPreferences } = require('electron');
app.setName('Multica Isolated Drag Region Proof');
app.setPath('userData', process.env.DRAG_PROOF_PROFILE);
app.whenReady().then(async () => {
 const win=new BrowserWindow({width:901,height:1042,x:80,y:80,show:true,titleBarStyle:'hiddenInset',trafficLightPosition:{x:16,y:17},webPreferences:{contextIsolation:true,nodeIntegration:false,sandbox:true}});
 global.dragProof={win,trust:systemPreferences.isTrustedAccessibilityClient(false)};
 await win.loadURL(process.env.DRAG_PROOF_URL);win.show();win.focus();
});
app.on('window-all-closed',()=>app.quit());
