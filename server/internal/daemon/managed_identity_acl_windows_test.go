//go:build windows

package daemon

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Exercise inherited ACLs under an actual Windows user profile, not POSIX mode
// bits or the runner's potentially shared work directory. Only the newly
// created fixture directory is touched; no existing identity is read.
func TestManagedIdentityWindowsProfileACL(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.MkdirTemp(home, ".multica-acl-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(fixture); err != nil {
			t.Error(err)
		}
	})
	identity, err := LoadManagedInstallation(fixture, managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.SaveEnrollment(managedInstallation, "1"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fixture, ".multica", "management", managedDeployment, "installation.json")
	shell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	script := `$ErrorActionPreference='Stop'; $acl=Get-Acl -LiteralPath $env:MULTICA_ACL_TEST_FILE;
$danger=[System.Security.AccessControl.FileSystemRights]::ReadData -bor [System.Security.AccessControl.FileSystemRights]::WriteData -bor [System.Security.AccessControl.FileSystemRights]::AppendData -bor [System.Security.AccessControl.FileSystemRights]::ChangePermissions -bor [System.Security.AccessControl.FileSystemRights]::TakeOwnership;
$public=@('S-1-1-0','S-1-5-11','S-1-5-32-545'); $broad=@();
foreach($rule in $acl.Access){$sid=$rule.IdentityReference.Translate([System.Security.Principal.SecurityIdentifier]).Value; if($rule.AccessControlType -eq 'Allow' -and $public -contains $sid -and ($rule.FileSystemRights -band $danger) -ne 0){$broad+=$sid}}
@{broad_allow_sids=$broad; owner_sid=([System.Security.Principal.NTAccount]::new($acl.Owner)).Translate([System.Security.Principal.SecurityIdentifier]).Value} | ConvertTo-Json -Compress`
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-Command", script)
	// A pwsh parent exports its module search path. Windows PowerShell 5.1
	// must load its own built-in modules instead of PowerShell 7 assemblies.
	cmd.Env = append(os.Environ(), "PSModulePath="+filepath.Join(filepath.Dir(shell), "Modules"), "MULTICA_ACL_TEST_FILE="+path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect fixture ACL: %v: %s", err, output)
	}
	var result struct {
		Broad []string `json:"broad_allow_sids"`
		Owner string   `json:"owner_sid"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result.Owner == "" || len(result.Broad) > 0 {
		t.Fatalf("managed identity ACL has unexpected public access: owner=%q broad=%v", result.Owner, result.Broad)
	}
}
