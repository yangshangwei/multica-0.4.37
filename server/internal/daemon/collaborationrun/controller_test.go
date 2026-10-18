package collaborationrun

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
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
