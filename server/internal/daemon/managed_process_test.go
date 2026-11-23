package daemon

import "testing"

const (
	testWindowsMarked = `MULTICA_IDENTITY:{"host":"E9F42A64-BF8E-4739-A7CD-3AFE6EBD3AD1","boot":"2026-10-28T01:02:03.4567890Z"}`
	// Written to stdout by a DLP hook DLL injected into powershell.exe on an affected intranet host.
	testWindowsHookLine = "[0x7FFF1DA6F990] ANOMALY: meaningless REX prefix used"
)

// Desktop's managed-process.test.ts pins the same digests: lock owner recovery
// compares host and boot IDs written by both processes.
var testWindowsIdentity = managementProcessInfo{
	hostID: "d2229f218bb4a985010fa17309d8f6d745c4c62f2cfcfe45a7dc97402f8a20dd",
	bootID: "539f8d036b71470fea1fd61b4ef852fe043661af4ae70451c7953fc35b083a1e",
}

// Desktop's managed-process.test.ts pins the same string byte for byte.
func TestWindowsIdentityCommandMatchesDesktop(t *testing.T) {
	const want = `$o=Get-CimInstance Win32_OperatingSystem;$h=(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Cryptography').MachineGuid;'MULTICA_IDENTITY:'+(@{host=$h;boot=$o.LastBootUpTime.ToUniversalTime().ToString('o')}|ConvertTo-Json -Compress)`
	if windowsIdentityCommand != want {
		t.Fatalf("windowsIdentityCommand = %q; want %q", windowsIdentityCommand, want)
	}
}

func TestParseWindowsProcessIdentityReadsMarkedLine(t *testing.T) {
	for name, raw := range map[string]string{
		"clean output":                     testWindowsMarked,
		"a hook line after the identity":   testWindowsMarked + "\r\n" + testWindowsHookLine,
		"a hook line before the identity":  testWindowsHookLine + "\r\n" + testWindowsMarked,
		"LF line endings around the value": testWindowsHookLine + "\n" + testWindowsMarked + "\n" + testWindowsHookLine,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseWindowsProcessIdentity(raw)
			if err != nil || got != testWindowsIdentity {
				t.Fatalf("parseWindowsProcessIdentity() = %+v, %v; want %+v", got, err, testWindowsIdentity)
			}
		})
	}
}

func TestParseWindowsProcessIdentityRejectsWithoutEchoingOutput(t *testing.T) {
	for name, raw := range map[string]string{
		"unmarked JSON":       `{"host":"E9F42A64-BF8E-4739-A7CD-3AFE6EBD3AD1","boot":"2026-10-28T01:02:03.4567890Z"}`,
		"two marked lines":    testWindowsMarked + "\r\n" + testWindowsMarked,
		"malformed JSON":      `MULTICA_IDENTITY:{"host":"E9F42A64-BF8E-4739-A7CD-3AFE6EBD3AD1"`,
		"a missing boot time": `MULTICA_IDENTITY:{"host":"E9F42A64-BF8E-4739-A7CD-3AFE6EBD3AD1","boot":""}`,
		"no output":           "",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseWindowsProcessIdentity(raw)
			if err == nil || err.Error() != "OS boot identity unavailable" {
				t.Fatalf("parseWindowsProcessIdentity() error = %v; want OS boot identity unavailable", err)
			}
		})
	}
}
