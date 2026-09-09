//go:build windows

package agent

import (
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

const windowsUTF8CodePage = 65001

// acpTerminalShellCommand keeps the existing cmd.exe contract for commands
// without an explicit argv, but makes the shell's output encoding explicit.
// The command still runs when chcp cannot attach to a console (for example in
// a service), so a code-page setup failure cannot turn into a command failure.
func acpTerminalShellCommand(command string) (string, []string) {
	wrapped := "chcp 65001 >nul 2>&1 & " + command
	return "cmd.exe", []string{"/d", "/s", "/c", wrapped}
}

// normalizeTerminalOutput converts legacy Windows console bytes when a
// command ignored the UTF-8 shell setup (for example a tool that writes CP936
// directly to its redirected stdout). Valid UTF-8 is left untouched.
func normalizeTerminalOutput(raw []byte) []byte {
	codePage, err := windows.GetConsoleOutputCP()
	if err != nil || codePage == 0 {
		codePage = windows.GetACP()
	}
	// A child can write legacy bytes directly even after the shell switched to
	// UTF-8. In that case the active console code page is 65001, so use the
	// machine ANSI code page as the fallback decoder instead of returning bytes
	// that the common snapshot path would replace with �.
	if codePage == windowsUTF8CodePage {
		codePage = windows.GetACP()
	}
	return decodeWindowsCodePage(raw, codePage)
}

func decodeWindowsCodePage(raw []byte, codePage uint32) []byte {
	if len(raw) == 0 || utf8.Valid(raw) || codePage == 0 || codePage == windowsUTF8CodePage {
		return raw
	}

	wideLen, err := windows.MultiByteToWideChar(codePage, 0, &raw[0], int32(len(raw)), nil, 0)
	if err != nil || wideLen <= 0 {
		return raw
	}
	wide := make([]uint16, wideLen)
	if _, err := windows.MultiByteToWideChar(codePage, 0, &raw[0], int32(len(raw)), &wide[0], wideLen); err != nil {
		return raw
	}
	return []byte(string(utf16.Decode(wide)))
}
