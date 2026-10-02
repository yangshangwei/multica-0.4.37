# Electron registers this protocol at runtime; NSIS does not consume protocols.
# Preserve a handler claimed by another installation and preserve it on upgrade.
!macro customUnInstall
  ${ifNot} ${isUpdated}
    Push $R0
    ReadRegStr $R0 HKCU "Software\Classes\multica\shell\open\command" ""
    StrCmp $R0 '"$INSTDIR\${APP_EXECUTABLE_FILENAME}" "%1"' 0 multica_protocol_done
    DeleteRegKey HKCU "Software\Classes\multica"
    multica_protocol_done:
    Pop $R0
  ${endif}
!macroend
