package collaborationrun

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestMain(m *testing.M) {
	if code, handled := runHostProviderFixture(); handled {
		os.Exit(code)
	}
	if code, handled := runPreparedTestFixture(); handled {
		os.Exit(code)
	}
	if len(os.Args) == 2 && os.Args[1] == NamespaceHelperArg {
		os.Exit(NamespaceHelperMain())
	}
	if len(os.Args) > 1 && os.Args[1] == "--collaboration-test-agent" {
		os.Exit(runTestAgent())
	}
	os.Exit(m.Run())
}

func TestUnsupportedPlatformNeverAdvertisesContainment(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux capability requires an actual namespace probe")
	}
	c := &NamespaceController{Executable: "/does-not-exist/test-agent", Directory: t.TempDir()}
	if err := c.Probe(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("platform must refuse before executable lookup: %v", err)
	}
	if _, err := c.Start(context.Background(), ProviderInput{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported launch: %v", err)
	}
}

func TestNamespaceProbeReportsSafeConfigurationFailure(t *testing.T) {
	c := &NamespaceController{Executable: "/private-secret/missing-provider", Directory: t.TempDir()}
	err := c.Probe(t.Context())
	var diagnostic *NamespaceProbeError
	if !errors.Is(err, ErrUnsupported) || !errors.As(err, &diagnostic) {
		t.Fatalf("missing safe refusal: %v", err)
	}
	wantStage := "platform"
	if runtime.GOOS == "linux" {
		wantStage = "prepare_root"
	}
	if diagnostic.Stage != wantStage || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("wrong or unsafe refusal: %v", err)
	}
}

func TestNamespaceProbeDiagnosticPreservesCauseWithoutPrivateText(t *testing.T) {
	for _, cause := range []error{nil, &os.PathError{Op: "fork/exec", Path: "/private-credential/helper", Err: syscall.EPERM}, context.Canceled, context.DeadlineExceeded} {
		diagnostic := namespaceProbeFailure("helper_start", cause)
		if !errors.Is(diagnostic, ErrUnsupported) || (cause != nil && !errors.Is(diagnostic, cause)) || strings.Contains(diagnostic.Error(), "private-credential") {
			t.Fatalf("unsafe or lossy cause: %v", diagnostic)
		}
		for _, unwrapped := range diagnostic.Unwrap() {
			if unwrapped == nil {
				t.Fatal("nil error in Unwrap")
			}
		}
		if errors.Is(cause, syscall.EPERM) && diagnostic.Errno != int(syscall.EPERM) {
			t.Fatal("lost actual clone errno")
		}
	}
	for _, code := range []string{"mount_proc_errno_1", "protect_supervisor", "invalid_preparation"} {
		if namespaceProbeHelperCode(code) != code {
			t.Fatalf("lost known helper refusal %q", code)
		}
	}
	for _, code := range []string{"private-credential", "mount_proc_errno_999999", "mount_proc_errno_01", "mount_proc_errno_1 secret", "mount_proc\nsecret"} {
		if namespaceProbeHelperCode(code) != "invalid_helper_response" {
			t.Fatalf("untrusted helper code escaped: %q", code)
		}
	}
}

func TestNamespaceReadyFailureRetainsConcurrentCancellation(t *testing.T) {
	for _, tc := range []struct{ name, wire, code string }{
		{"closed", "", "no_helper_response"},
		{"malformed", "{private-secret", "malformed_helper_response"},
		{"refused", `{"namespace_id":"","error_code":"mount_proc_errno_1"}`, "mount_proc_errno_1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := readNamespaceReady(strings.NewReader(tc.wire))
			if result.err == nil {
				t.Fatal("invalid readiness admitted")
			}
			failure := namespaceReadyFailure(result, context.Canceled)
			if !errors.Is(failure, context.Canceled) || !errors.Is(failure, ErrUnsupported) || failure.Context != "canceled" || failure.Code != tc.code {
				t.Fatalf("lost boundary observations: %v", failure)
			}
			if tc.name == "closed" && !errors.Is(failure, io.EOF) {
				t.Fatal("lost EOF identity")
			}
			if strings.Contains(failure.Error(), "private-secret") {
				t.Fatal("private response escaped")
			}
		})
	}
}
