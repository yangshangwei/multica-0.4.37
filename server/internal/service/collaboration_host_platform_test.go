package service

import (
	"testing"
)

func TestCollaborationHostMacOSInstallationMatchesDarwinCapability(t *testing.T) {
	for _, test := range []struct {
		installation, capability string
		accepted                 bool
	}{
		{"macos", "darwin", true}, {"linux", "linux", true}, {"windows", "windows", true},
		{"macos", "linux", false}, {"linux", "darwin", false}, {"windows", "darwin", false},
	} {
		t.Run(test.installation+"/"+test.capability, func(t *testing.T) {
			f := newManagedCollaborationFixture(t, "local")
			f.fx.Exec(t, `UPDATE managed_installation SET os=$2 WHERE id=$1`, f.installation.ID, test.installation)
			p := hostCapabilityRegistration(f)
			p.Capabilities.OS = test.capability
			if test.capability == "windows" {
				p.Capabilities.Host.SupervisionScopes = []string{"windows_job", "direct_process"}
			}
			_, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p)
			if (err == nil) != test.accepted {
				t.Fatalf("installation/platform match: %v", err)
			}
			if !test.accepted {
				return
			}
			f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
			if err != nil {
				t.Fatal(err)
			}
			renewHostConsent(t, f)
			target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			code, err := collaborationRuntimeTargetSupported(f.ctx, f.svc.Queries, target)
			if err != nil || code != "" {
				t.Fatalf("registered native platform cannot execute: %s %v", code, err)
			}
		})
	}
}
