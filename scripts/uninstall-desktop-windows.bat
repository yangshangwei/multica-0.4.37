@echo off
rem ===========================================================================
rem  Multica Desktop - complete uninstall for Windows
rem ===========================================================================
rem  Removes the Multica Desktop app and every file, registry key, background
rem  daemon and workspace it owns, for the CURRENT USER.
rem
rem  Usage:
rem    uninstall-desktop-windows.bat [/y] [/dry-run] [/keep-workspaces] [/purge-cli]
rem
rem    /y                Do not ask for confirmation.
rem    /dry-run          Print what would be removed, remove nothing.
rem    /keep-workspaces  Keep agent task workspaces (they can be many GB, but
rem                      they may also hold uncommitted agent work).
rem    /purge-cli        ALSO delete %USERPROFILE%\.multica in full. That
rem                      directory belongs to the separately installed terminal
rem                      CLI (config.json with its token, daemon.log, hand-made
rem                      profiles, and the self-host checkout under
rem                      .multica\server). Desktop never owns those files, so
rem                      they are kept unless you pass this flag.
rem
rem  What Desktop owns, and what this script removes by default:
rem    [install dir]                               resolved from the registry,
rem                                                default %LOCALAPPDATA%\Programs\Multica
rem    %APPDATA%\Multica                           Electron userData: device
rem                                                identity, window state,
rem                                                updater prefs, caches,
rem                                                downloaded CLI in bin\
rem    %APPDATA%\Multica Canary*                   dev-build userData
rem    %LOCALAPPDATA%\@multicadesktop-updater      electron-updater download cache
rem    %USERPROFILE%\.multica\desktop.json         Desktop runtime config
rem    %USERPROFILE%\.multica\desktop_prefs.json   Desktop daemon preferences
rem    %USERPROFILE%\.multica\profiles\desktop-*   Desktop-owned CLI profiles
rem                                                (config.json holds a PAT)
rem    %USERPROFILE%\multica_workspaces_desktop-*  agent task workspaces
rem    %TEMP%\multica-task-*                       agent task temp dirs
rem    HKCU\Software\[guid]                        NSIS install key
rem    HKCU\...\Uninstall\[guid]                   Add/Remove Programs entry
rem    HKCU\Software\Classes\multica               multica:// protocol handler,
rem                                                registered by the app at runtime
rem    Start Menu and Desktop shortcuts
rem
rem  Run this as the user who installed the app. Elevation is only required if
rem  the app was installed for all users (HKLM + Program Files).
rem ===========================================================================

setlocal EnableExtensions EnableDelayedExpansion

set "APP_NAME=Multica"
rem UUIDv5(appId "ai.multica.desktop", electron-builder namespace
rem 50e065bc-3134-11e6-9bab-38c9862bdaf3). This is how electron-builder derives
rem the NSIS install / uninstall registry key when nsis.guid is unset. If the
rem appId in apps/desktop/electron-builder.yml ever changes, this changes too.
set "APP_GUID=fe66bde5-e02e-57e3-a45c-807ddac8c5f7"
rem electron-updater's cache dir name comes from the npm package name
rem (@multica/desktop becomes @multicadesktop), not from productName.
set "UPDATER_CACHE=@multicadesktop-updater"
set "PROTOCOL=multica"

set "ASSUME_YES=0"
set "DRY_RUN=0"
set "KEEP_WORKSPACES=0"
set "PURGE_CLI=0"
set "REMOVED=0"
set "FAILED=0"

:parse
if "%~1"=="" goto parsed
if /i "%~1"=="/y" (
  set "ASSUME_YES=1"
  shift
  goto parse
)
if /i "%~1"=="/dry-run" (
  set "DRY_RUN=1"
  shift
  goto parse
)
if /i "%~1"=="/keep-workspaces" (
  set "KEEP_WORKSPACES=1"
  shift
  goto parse
)
if /i "%~1"=="/purge-cli" (
  set "PURGE_CLI=1"
  shift
  goto parse
)
if /i "%~1"=="/?" goto usage
if /i "%~1"=="-h" goto usage
if /i "%~1"=="--help" goto usage
echo Unknown option: %~1
goto usage

:parsed

echo.
echo === Multica Desktop uninstall ===
if "%DRY_RUN%"=="1" echo Mode: DRY RUN - nothing will be deleted.
echo.

rem --- Locate the installation -------------------------------------------
set "INSTALL_DIR="
set "PER_MACHINE=0"
call :readreg "HKCU\Software\%APP_GUID%" InstallLocation INSTALL_DIR
if not defined INSTALL_DIR (
  call :readreg "HKLM\Software\%APP_GUID%" InstallLocation INSTALL_DIR
  if defined INSTALL_DIR set "PER_MACHINE=1"
)
if not defined INSTALL_DIR (
  call :readreg "HKLM\Software\WOW6432Node\%APP_GUID%" InstallLocation INSTALL_DIR
  if defined INSTALL_DIR set "PER_MACHINE=1"
)
if not defined INSTALL_DIR set "INSTALL_DIR=%LOCALAPPDATA%\Programs\%APP_NAME%"

echo Install dir      : %INSTALL_DIR%
if not exist "%INSTALL_DIR%" echo                    ^(not present - leftover cleanup only^)
echo Electron data    : %APPDATA%\%APP_NAME%
echo Updater cache    : %LOCALAPPDATA%\%UPDATER_CACHE%
echo Desktop profiles : %USERPROFILE%\.multica\profiles\desktop-*
if "%KEEP_WORKSPACES%"=="1" (
  echo Task workspaces  : KEPT
) else (
  echo Task workspaces  : %USERPROFILE%\multica_workspaces_desktop-*
)
if "%PURGE_CLI%"=="1" (
  echo.
  echo *** /purge-cli: %USERPROFILE%\.multica will be deleted IN FULL.
  echo *** That removes the terminal CLI's own config.json, its token, its
  echo *** daemon log, every hand-made profile, and .multica\server.
) else (
  echo CLI config       : KEPT ^(%USERPROFILE%\.multica\config.json and friends^)
)
if "%PER_MACHINE%"=="1" (
  net session >nul 2>&1
  if errorlevel 1 (
    echo.
    echo *** This is an all-users installation but this prompt is not elevated.
    echo *** Re-run from an Administrator command prompt, or the install dir and
    echo *** HKLM keys will be left behind.
  )
)
echo.

if "%ASSUME_YES%"=="0" if "%DRY_RUN%"=="0" (
  set "ANSWER="
  set /p "ANSWER=Type YES to remove all of the above: "
  if /i not "!ANSWER!"=="YES" (
    echo Aborted. Nothing was removed.
    exit /b 1
  )
  echo.
)

rem --- Stop the daemon before deleting anything ---------------------------
rem The daemon is a detached process: it outlives the app window, so closing
rem Multica does not stop it, and its open file handles would block the
rem deletes below. Stop it through the CLI first (that shuts down its agent
rem task children too), then kill whatever is left.
echo [1/6] Stopping Multica processes

set "CLI="
if exist "%INSTALL_DIR%\resources\app.asar.unpacked\resources\bin\multica.exe" set "CLI=%INSTALL_DIR%\resources\app.asar.unpacked\resources\bin\multica.exe"
if not defined CLI if exist "%APPDATA%\%APP_NAME%\bin\multica.exe" set "CLI=%APPDATA%\%APP_NAME%\bin\multica.exe"

if defined CLI (
  for /d %%D in ("%USERPROFILE%\.multica\profiles\desktop-*") do (
    echo   daemon stop --profile %%~nxD
    if "%DRY_RUN%"=="0" "!CLI!" daemon stop --profile "%%~nxD" >nul 2>&1
  )
) else (
  echo   no bundled CLI found - skipping graceful daemon shutdown
)

rem Flattened out of an if-block on purpose: cmd.exe mis-parses parentheses
rem that appear inside a quoted argument within a parenthesised block, and the
rem PowerShell filters below rely on both.
if "%DRY_RUN%"=="1" goto after_kill
taskkill /F /T /IM "%APP_NAME%.exe" >nul 2>&1
rem Kill only Desktop-owned daemons. A bare taskkill /IM multica.exe would also
rem kill the daemon the user started from a terminal on the default profile,
rem which Desktop does not own.
where powershell.exe >nul 2>&1
if errorlevel 1 goto after_kill
powershell.exe -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'multica.exe' -and $_.CommandLine -match '--profile\s+desktop-' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }" >nul 2>&1
:after_kill

rem --- Collect custom workspace roots before the profiles are deleted -----
rem A profile's config.json may point workspaces_root somewhere other than the
rem default %USERPROFILE%\multica_workspaces_[profile].
set "WSLIST=%TEMP%\multica-uninstall-ws.txt"
if exist "%WSLIST%" del /f /q "%WSLIST%" >nul 2>&1
if "%KEEP_WORKSPACES%"=="1" goto after_wsroots
where powershell.exe >nul 2>&1
if errorlevel 1 goto after_wsroots
powershell.exe -NoProfile -Command "Get-ChildItem -Directory (Join-Path $env:USERPROFILE '.multica\profiles') -Filter 'desktop-*' -ErrorAction SilentlyContinue | ForEach-Object { $c = Join-Path $_.FullName 'config.json'; if (Test-Path $c) { try { $r = (Get-Content -Raw $c | ConvertFrom-Json).workspaces_root; if ($r) { $r } } catch { } } }" > "%WSLIST%" 2>nul
:after_wsroots

rem --- Let the bundled uninstaller do its part -----------------------------
echo [2/6] Running the bundled uninstaller
set "UNINST=%INSTALL_DIR%\Uninstall %APP_NAME%.exe"
if exist "%UNINST%" (
  if "%DRY_RUN%"=="1" (
    echo   [dry-run] "%UNINST%" /S --delete-app-data
  ) else (
    rem _?= keeps the uninstaller from copying itself to %TEMP%, which is what
    rem makes `start /wait` actually wait for it.
    start "" /wait "%UNINST%" /S --delete-app-data "_?=%INSTALL_DIR%"
    call :waitproc "Un_A.exe"
    call :waitproc "Uninstall %APP_NAME%.exe"
  )
) else (
  echo   not found - continuing with manual cleanup
)

rem --- Files ---------------------------------------------------------------
echo [3/6] Removing application files and data
call :rmtree "%INSTALL_DIR%"
call :rmtree "%APPDATA%\%APP_NAME%"
call :rmtree "%APPDATA%\@multicadesktop"
call :rmtree "%LOCALAPPDATA%\%APP_NAME%"
call :rmtree "%LOCALAPPDATA%\%UPDATER_CACHE%"
for /d %%D in ("%APPDATA%\%APP_NAME% Canary*") do call :rmtree "%%~fD"

echo [4/6] Removing Desktop-owned CLI state
call :delfile "%USERPROFILE%\.multica\desktop.json"
call :delfile "%USERPROFILE%\.multica\desktop_prefs.json"
for /d %%D in ("%USERPROFILE%\.multica\profiles\desktop-*") do call :rmtree "%%~fD"

if "%KEEP_WORKSPACES%"=="0" (
  echo   agent task workspaces
  for /d %%D in ("%USERPROFILE%\multica_workspaces_desktop-*") do call :rmtree "%%~fD"
  if exist "%WSLIST%" (
    for /f "usebackq delims=" %%R in ("%WSLIST%") do call :rmtree "%%~R"
  )
  for /d %%D in ("%TEMP%\multica-task-*") do call :rmtree "%%~fD"
)
if exist "%WSLIST%" del /f /q "%WSLIST%" >nul 2>&1

rem --- Shortcuts ----------------------------------------------------------
echo [5/6] Removing shortcuts
call :delfile "%APPDATA%\Microsoft\Windows\Start Menu\Programs\%APP_NAME%.lnk"
call :delfile "%ProgramData%\Microsoft\Windows\Start Menu\Programs\%APP_NAME%.lnk"
call :delfile "%USERPROFILE%\Desktop\%APP_NAME%.lnk"
if defined PUBLIC call :delfile "%PUBLIC%\Desktop\%APP_NAME%.lnk"

rem --- Registry -----------------------------------------------------------
echo [6/6] Removing registry entries
call :delregkey "HKCU\Software\%APP_GUID%"
call :delregkey "HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\%APP_GUID%"
call :delregkey "HKLM\Software\%APP_GUID%"
call :delregkey "HKLM\Software\WOW6432Node\%APP_GUID%"
call :delregkey "HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\%APP_GUID%"
call :delregkey "HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\%APP_GUID%"
rem The multica:// handler is written by the app at runtime, not by the
rem installer, so the bundled uninstaller leaves it behind.
call :delregkey "HKCU\Software\Classes\%PROTOCOL%"
call :delregkey "HKLM\Software\Classes\%PROTOCOL%"

rem Catch an Add/Remove Programs entry under a different key name (an older
rem build, or a custom nsis.guid). Matches DisplayName exactly, nothing else.
for /f "delims=" %%K in ('reg query "HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall" /s /v DisplayName /f "%APP_NAME%" /d /e 2^>nul ^| findstr /b /i /c:"HKEY_"') do call :delregkey "%%K"
for /f "delims=" %%K in ('reg query "HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall" /s /v DisplayName /f "%APP_NAME%" /d /e 2^>nul ^| findstr /b /i /c:"HKEY_"') do call :delregkey "%%K"

rem --- Optional: the shared CLI root --------------------------------------
if "%PURGE_CLI%"=="1" (
  echo Purging %USERPROFILE%\.multica in full
  call :rmtree "%USERPROFILE%\.multica"
  for /d %%D in ("%USERPROFILE%\multica_workspaces*") do call :rmtree "%%~fD"
)

echo.
echo === Done ===
echo Removed: %REMOVED%   Failed: %FAILED%
if "%DRY_RUN%"=="1" (
  echo Dry run - nothing was actually deleted.
  goto end
)
if not "%FAILED%"=="0" (
  echo.
  echo Some items could not be removed. Usual causes:
  echo   - a Multica process is still running ^(check Task Manager^)
  echo   - an all-users install needs an elevated prompt
  echo   - a file is open in another program
  echo Re-run this script after closing them.
)
if "%PURGE_CLI%"=="0" (
  echo.
  echo Left in place on purpose ^(not owned by Desktop^):
  echo   %USERPROFILE%\.multica\config.json      terminal CLI config + token
  echo   %USERPROFILE%\.multica\daemon.log       terminal CLI daemon log
  echo   %USERPROFILE%\.multica\profiles\*       profiles you created yourself
  echo   %USERPROFILE%\.multica\hooks            agent hook overrides
  echo   %USERPROFILE%\.multica\server           self-host checkout
  echo   the multica CLI on PATH, and any agent CLI ^(Claude Code, Codex, ...^)
  echo Pass /purge-cli to remove the .multica root as well.
)
goto end

rem ===========================================================================
rem  Helpers
rem ===========================================================================

:usage
echo.
echo Multica Desktop uninstall for Windows
echo.
echo   uninstall-desktop-windows.bat [/y] [/dry-run] [/keep-workspaces] [/purge-cli]
echo.
echo   /y                skip the confirmation prompt
echo   /dry-run          list what would be removed, delete nothing
echo   /keep-workspaces  keep agent task workspaces
echo   /purge-cli        also delete %%USERPROFILE%%\.multica in full
echo.
exit /b 2

rem Read a registry value into the variable named by %3. Empty if absent.
:readreg
set "%~3="
for /f "tokens=2,*" %%A in ('reg query "%~1" /v %~2 2^>nul ^| findstr /r /c:"REG_[A-Z_]*"') do set "%~3=%%B"
goto :eof

:rmtree
if not exist "%~1" goto :eof
if "%DRY_RUN%"=="1" (
  echo   [dry-run] rmdir /s /q "%~1"
  goto :eof
)
rmdir /s /q "%~1" >nul 2>&1
if exist "%~1" (
  echo   [WARN] could not remove "%~1"
  set /a FAILED+=1
) else (
  echo   [ok] "%~1"
  set /a REMOVED+=1
)
goto :eof

:delfile
if not exist "%~1" goto :eof
if "%DRY_RUN%"=="1" (
  echo   [dry-run] del "%~1"
  goto :eof
)
del /f /q "%~1" >nul 2>&1
if exist "%~1" (
  echo   [WARN] could not remove "%~1"
  set /a FAILED+=1
) else (
  echo   [ok] "%~1"
  set /a REMOVED+=1
)
goto :eof

:delregkey
reg query "%~1" >nul 2>&1
if errorlevel 1 goto :eof
if "%DRY_RUN%"=="1" (
  echo   [dry-run] reg delete "%~1" /f
  goto :eof
)
reg delete "%~1" /f >nul 2>&1
if errorlevel 1 (
  echo   [WARN] could not delete %~1
  set /a FAILED+=1
) else (
  echo   [ok] %~1
  set /a REMOVED+=1
)
goto :eof

rem Wait up to 60s for a process to exit. The NSIS uninstaller can outlive the
rem `start /wait` that launched it, and its handles block the deletes.
:waitproc
set "WP_TRIES=0"
:waitproc_loop
tasklist /fi "imagename eq %~1" 2>nul | find /i "%~1" >nul
if errorlevel 1 goto :eof
set /a WP_TRIES+=1
if !WP_TRIES! GEQ 60 (
  echo   [WARN] %~1 is still running - continuing anyway
  goto :eof
)
ping -n 2 127.0.0.1 >nul 2>&1
goto waitproc_loop

:end
endlocal
exit /b 0
