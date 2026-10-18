//go:build linux

package collaborationrun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
	"golang.org/x/sys/unix"
)

func testController(t *testing.T, args ...string) *NamespaceController {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The only provider executable used by this test is this fresh fixture copy.
	raw, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fake-agent")
	if err := os.WriteFile(fake, raw, 0700); err != nil {
		t.Fatal(err)
	}
	c := &NamespaceController{Executable: fake, HelperExecutable: bin, Directory: filepath.Join(t.TempDir(), "sandboxes"), Arguments: append([]string{"--collaboration-test-agent"}, args...)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Probe(ctx); err != nil {
		if os.Getenv("MULTICA_REQUIRE_COLLAB_NAMESPACE") == "1" {
			t.Fatalf("required Linux kernel containment unavailable: %v", err)
		}
		t.Skipf("Linux kernel/static fixture cannot provide containment: %v", err)
	}
	return c
}

func runTestAgent() int {
	mode := os.Args[2]
	if mode == "exit" {
		return 0
	}
	if mode == "success" || mode == "failure" || mode == "legacy_outcome" {
		var input ProviderInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil || len(input.Inputs) != 2 {
			return 48
		}
		outcome := protocol.CollaborationBusinessOutcomeSuccess
		if mode == "failure" {
			outcome = protocol.CollaborationBusinessOutcomeFailure
		} else if mode == "legacy_outcome" {
			outcome = "succeeded"
		}
		if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"outcome": outcome, "body": "two immutable inputs"}); err != nil {
			return 48
		}
		return 0
	}
	if mode == "boundary" {
		if b, _ := os.ReadFile("/proc/1/environ"); strings.Contains(string(b), "SUPERVISOR_SENTINEL") {
			return 40
		}
		if _, err := os.ReadFile(os.Args[3]); err == nil {
			return 41
		}
		if n, err := unix.PrctlRetInt(unix.PR_GET_NO_NEW_PRIVS, 0, 0, 0, 0); err != nil || n != 1 {
			return 42
		}
		var caps [2]unix.CapUserData
		if err := unix.Capget(&unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}, &caps[0]); err != nil || caps[0].Effective != 0 || caps[1].Effective != 0 || caps[0].Permitted != 0 || caps[1].Permitted != 0 {
			return 43
		}
		if err := unix.Chroot("/"); err == nil {
			return 44
		}
		if err := unix.Mount("proc", "/proc", "proc", 0, ""); err == nil {
			return 45
		}
		if _, err := os.Readlink("/proc/1/fd/5"); err == nil {
			return 46
		}
		fds, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return 49
		}
		for _, fd := range fds {
			if fd.Name() == "0" || fd.Name() == "1" || fd.Name() == "2" {
				continue
			}
			link, _ := os.Readlink("/proc/self/fd/" + fd.Name())
			if strings.HasPrefix(link, "pipe:") {
				return 49
			}
		}
		var input ProviderInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil || len(input.Inputs) != 2 {
			return 47
		}
		if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"outcome": protocol.CollaborationBusinessOutcomeSuccess, "body": "two immutable inputs"}); err != nil {
			return 47
		}
		return 0
	}
	if mode == "tree" {
		child := exec.Command("/agent", "--collaboration-test-agent", "intermediate", os.Args[3])
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Run(); err != nil {
			return 50
		}
	} else if mode == "intermediate" {
		child := exec.Command("/agent", "--collaboration-test-agent", "orphan", os.Args[3])
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Start(); err != nil {
			return 51
		}
		return 0
	}
	conn, err := net.Dial("tcp", os.Args[3])
	if err != nil {
		return 52
	}
	defer conn.Close()
	// The detached orphan closes all stdio before announcing itself. The root
	// also closes stdio, so a pipe reader cannot stand in for the controller.
	_ = unix.Close(0)
	_ = unix.Close(1)
	_ = unix.Close(2)
	fmt.Fprintln(conn, mode)
	var b [1]byte
	_, _ = conn.Read(b[:])
	return 0
}

func TestLinuxNamespaceKillsDetachedDoubleForkAndEarlyStdio(t *testing.T) {
	for _, natural := range []bool{false, true} {
		t.Run(fmt.Sprintf("natural=%t", natural), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			c := testController(t, "tree", listener.Addr().String())
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			proc, err := c.Start(ctx, ProviderInput{})
			if err != nil {
				t.Fatal(err)
			}
			defer proc.Stop()
			connections := make(map[string]net.Conn)
			for range 2 {
				if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Fatal(err)
				}
				conn, err := listener.Accept()
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Fatal(err)
				}
				role, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil {
					t.Fatal(err)
				}
				connections[strings.TrimSpace(role)] = conn
			}
			if connections["tree"] == nil || connections["orphan"] == nil {
				t.Fatalf("missing detached process: %v", connections)
			}
			finished := make(chan error, 1)
			go func() { _, err := proc.Wait(); finished <- err }()
			select {
			case err := <-finished:
				t.Fatalf("stdio close falsely proved exit: %v", err)
			default:
			}
			if natural {
				_, err = connections["tree"].Write([]byte("x"))
			} else {
				err = proc.Stop()
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("namespace did not terminate")
			}
			var b [1]byte
			if _, err := connections["orphan"].Read(b[:]); err != io.EOF {
				t.Fatalf("detached double-fork descendant survived namespace: %v", err)
			}
		})
	}
}

func TestLinuxNamespaceHidesSupervisorFilesAndDropsPrivileges(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "supervisor-secret")
	if err := os.WriteFile(secret, []byte("SUPERVISOR_SENTINEL"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTICA_TEST_SUPERVISOR_SECRET", "SUPERVISOR_SENTINEL")
	c := testController(t, "boundary", secret)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	proc, err := c.Start(ctx, ProviderInput{Identity: testIdentity(), Inputs: []protocol.CollaborationRunInput{{Body: "first"}, {Body: "second"}}, BusinessToken: "only-business-token"})
	if err != nil {
		t.Fatal(err)
	}
	defer proc.Stop()
	result, err := proc.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || !strings.Contains(string(result.Output), "two immutable inputs") {
		t.Fatalf("boundary fixture failed: exit=%d output=%s", result.ExitCode, result.Output)
	}
}
