package collaborationrun

import (
	"bufio"
	"bytes"
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

const NamespaceHelperArg = "__multica_collaboration_namespace_init"

// NamespaceController accepts either the original static executable or an
// explicit, validated file bundle. A successful native kernel probe is required
// before either can claim work; neither path discovers ambient providers.
type NamespaceController struct {
	Executable       string
	CLIExecutable    string
	HelperExecutable string
	Directory        string
	Arguments        []string
	Bundle           *ProviderBundleConfig
}

type ProviderInput struct {
	Identity      protocol.CollaborationRunIdentity  `json:"identity"`
	Inputs        []protocol.CollaborationRunInput   `json:"inputs"`
	BusinessToken string                             `json:"-"`
	ServerURL     string                             `json:"-"`
	Preparation   *protocol.CollaborationPreparation `json:"preparation,omitempty"`
	prepared      *PreparedRun
}

type namespaceRequest struct {
	Root            string                 `json:"root"`
	Arguments       []string               `json:"arguments"`
	Input           json.RawMessage        `json:"input"`
	BusinessToken   string                 `json:"business_token"`
	ServerURL       string                 `json:"server_url"`
	Probe           bool                   `json:"probe"`
	ProbeGit        bool                   `json:"probe_git,omitempty"`
	Devices         []string               `json:"devices,omitempty"`
	ProbeGitDevices []string               `json:"probe_git_devices,omitempty"`
	Execution       *preparedExecution     `json:"execution,omitempty"`
	Git             *gitPreparationRequest `json:"git,omitempty"`
}

type namespaceReady struct {
	NamespaceID string `json:"namespace_id"`
	ErrorCode   string `json:"error_code,omitempty"`
}

type ContainmentResult struct {
	ExitCode         int
	Output           []byte
	OutputTruncated  bool
	PreparationFacts []preparationFact
}

// Process observes the namespace init, never an arbitrary PID/PGID. Wait only
// reports proof after the kernel has reaped that init and destroyed its scope.
type Process struct {
	cmd         *exec.Cmd
	liveness    *os.File
	root        string
	namespaceID string
	output      *limitedOutput
	waitOnce    sync.Once
	waitDone    chan struct{}
	result      ContainmentResult
	waitErr     error
	stopOnce    sync.Once
	stopErr     error
	facts       <-chan []preparationFact
}

type limitedOutput struct {
	mu        sync.Mutex
	b         bytes.Buffer
	truncated bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := protocol.CollaborationOutcomeJSONLimit - b.b.Len()
	if len(p) > remaining {
		b.truncated = true
		p = p[:remaining]
	}
	_, _ = b.b.Write(p)
	return n, nil
}

// NamespaceProbeError preserves the refusal boundary without exposing paths,
// helper output or credential-bearing configuration in daemon diagnostics.
type NamespaceProbeError struct {
	Stage    string
	Code     string
	Errno    int
	ExitCode int
	Context  string
	cause    error
}

func (e *NamespaceProbeError) Error() string {
	return fmt.Sprintf("namespace probe refused: stage=%s code=%s errno=%d exit=%d context=%s", e.Stage, e.Code, e.Errno, e.ExitCode, e.Context)
}
func (e *NamespaceProbeError) Unwrap() []error {
	if e.cause == nil {
		return []error{ErrUnsupported}
	}
	return []error{ErrUnsupported, e.cause}
}

func namespaceProbeFailure(stage string, cause error) *NamespaceProbeError {
	e := &NamespaceProbeError{Stage: stage, Code: "unavailable", ExitCode: -1, Context: "none", cause: cause}
	var errno syscall.Errno
	if errors.As(cause, &errno) {
		e.Errno = int(errno)
	}
	switch {
	case errors.Is(cause, context.DeadlineExceeded):
		e.Code, e.Context = "deadline", "deadline"
	case errors.Is(cause, context.Canceled):
		e.Code, e.Context = "canceled", "canceled"
	}
	return e
}

func namespaceProbeHelperCode(code string) string {
	stage, number, hasErrno := strings.Cut(code, "_errno_")
	switch stage {
	case "invalid_helper_request", "invalid_preparation", "namespace_identity", "protect_supervisor",
		"private_mount", "bind_root", "bind_null", "mount_proc", "pivot_root", "root_chdir", "unmount_old_root", "remove_old_root",
		"create_work", "create_tmp", "create_home", "mount_work", "mount_tmp", "mount_home", "readonly_root", "bind_urandom", "entropy_device", "isolate_root":
		if !hasErrno {
			return stage
		}
		if n, err := strconv.ParseUint(number, 10, 12); err == nil && n > 0 && strconv.FormatUint(n, 10) == number {
			return code
		}
	}
	return "invalid_helper_response"
}

type namespaceReadyResult struct {
	response namespaceReady
	err      error
	code     string
}

func readNamespaceReady(reader io.Reader) namespaceReadyResult {
	var result namespaceReadyResult
	raw, err := io.ReadAll(io.LimitReader(reader, 4097))
	switch {
	case err != nil:
		result.err, result.code = err, "helper_read_failed"
	case len(raw) == 0:
		result.err, result.code = io.EOF, "no_helper_response"
	case len(raw) > 4096:
		result.err, result.code = ErrUnsupported, "malformed_helper_response"
	default:
		result.err = decodeStrict(raw, &result.response)
		if result.err != nil {
			result.code = "malformed_helper_response"
		} else if result.response.NamespaceID == "" {
			result.err = ErrUnsupported
			result.code = namespaceProbeHelperCode(result.response.ErrorCode)
		}
	}
	return result
}

func namespaceReadyFailure(result namespaceReadyResult, contextErr error) *NamespaceProbeError {
	failure := namespaceProbeFailure("helper_ready", errors.Join(result.err, contextErr))
	failure.Code = result.code
	return failure
}

func (c *NamespaceController) Probe(ctx context.Context) error {
	if !namespaceSupported() {
		return namespaceProbeFailure("platform", ErrUnsupported)
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, err := c.start(probeCtx, ProviderInput{}, true)
	if err != nil {
		return err
	}
	result, err := p.Wait()
	if err != nil || result.ExitCode != 0 {
		failure := namespaceProbeFailure("helper_wait", errors.Join(err, probeCtx.Err()))
		failure.ExitCode = result.ExitCode
		return failure
	}
	return nil
}

func (c *NamespaceController) Start(ctx context.Context, input ProviderInput) (*Process, error) {
	if !namespaceSupported() {
		return nil, ErrUnsupported
	}
	return c.start(ctx, input, false)
}

func (c *NamespaceController) start(ctx context.Context, input ProviderInput, probe bool) (_ *Process, failure error) {
	stage := "prepare_root"
	if probe {
		defer func() {
			var diagnosed *NamespaceProbeError
			if failure != nil && !errors.As(failure, &diagnosed) {
				failure = namespaceProbeFailure(stage, failure)
			}
		}()
	}
	var root string
	var execution *preparedExecution
	var err error
	if c.Bundle != nil {
		if probe {
			root, err = c.prepareBundleRoot(ctx, true)
		} else if input.prepared == nil || input.Preparation == nil {
			return nil, ErrUnsupported
		} else {
			root, execution = input.prepared.root, &input.prepared.execution
		}
	} else {
		if input.Preparation != nil || input.prepared != nil {
			return nil, ErrUnsupported
		}
		root, err = c.prepareRoot()
	}
	if err != nil {
		return nil, err
	}
	ok := false
	forked := false
	defer func() {
		if !ok && !forked && input.prepared == nil {
			_ = os.RemoveAll(root)
		}
	}()
	stage = "helper_lookup"
	helper := c.HelperExecutable
	if helper == "" {
		helper, err = os.Executable()
		if err != nil {
			return nil, ErrUnsupported
		}
	}
	if !filepath.IsAbs(helper) {
		return nil, ErrUnsupported
	}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	configuration := namespaceRequest{Root: root, Arguments: c.Arguments, Input: body, BusinessToken: input.BusinessToken, ServerURL: input.ServerURL,
		Probe: probe, ProbeGit: probe && c.Bundle != nil && c.Bundle.Git != nil, Execution: execution}
	if c.Bundle != nil {
		configuration.Devices = c.Bundle.preparationDevices()
		if configuration.ProbeGit {
			configuration.ProbeGitDevices = c.Bundle.Git.Devices
		}
	}
	request, err := json.Marshal(configuration)
	if err != nil || len(request) > 2*1024*1024 {
		return nil, errors.New("collaboration batch exceeds local input limit")
	}
	stage = "helper_pipes"
	configR, configW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer configR.Close()
	defer configW.Close()
	readyR, readyW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer readyR.Close()
	defer readyW.Close()
	liveR, liveW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer liveR.Close()
	defer func() {
		if !ok {
			_ = liveW.Close()
		}
	}()
	factsR, factsW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer factsW.Close()
	defer func() {
		if !forked {
			_ = factsR.Close()
		}
	}()
	output := &limitedOutput{}
	cmd := exec.CommandContext(ctx, helper, NamespaceHelperArg)
	cmd.Env = []string{"LANG=C"}
	cmd.ExtraFiles = []*os.File{configR, readyW, liveR, factsW}
	cmd.Stdout, cmd.Stderr = output, io.Discard
	configureNamespace(cmd)
	stage = "helper_start"
	if err := cmd.Start(); err != nil {
		if probe {
			return nil, err
		}
		return nil, ErrUnsupported
	}
	forked = true
	_ = factsW.Close()
	facts := make(chan []preparationFact, 1)
	go func() {
		defer factsR.Close()
		facts <- readPreparationFacts(factsR)
	}()
	p := &Process{cmd: cmd, liveness: liveW, root: root, output: output, waitDone: make(chan struct{}), facts: facts}
	_ = configR.Close()
	_ = readyW.Close()
	_ = liveR.Close()
	go func() { _, _ = configW.Write(request); _ = configW.Close() }()
	ready := make(chan namespaceReadyResult, 1)
	go func() { ready <- readNamespaceReady(readyR) }()
	stage = "helper_ready"
	select {
	case result := <-ready:
		if result.err != nil {
			contextErr := ctx.Err()
			_ = p.Stop()
			_, _ = p.Wait()
			if probe {
				return nil, namespaceReadyFailure(result, contextErr)
			}
			return nil, result.err
		}
		p.namespaceID = result.response.NamespaceID
	case <-ctx.Done():
		_ = p.Stop()
		_, _ = p.Wait()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		_ = p.Stop()
		_, _ = p.Wait()
		if probe {
			return nil, context.DeadlineExceeded
		}
		return nil, ErrUnsupported
	}
	ok = true
	return p, nil
}

func (c *NamespaceController) prepareRoot() (string, error) {
	if !filepath.IsAbs(c.Executable) || !filepath.IsAbs(c.Directory) {
		return "", ErrUnsupported
	}
	f, err := openStaticExecutable(c.Executable)
	if err != nil {
		return "", err
	}
	defer f.Close()
	binaries := []struct {
		name string
		file *os.File
	}{{"agent", f}}
	if c.CLIExecutable != "" {
		command, err := openStaticExecutable(c.CLIExecutable)
		if err != nil {
			return "", err
		}
		defer command.Close()
		binaries = append(binaries, struct {
			name string
			file *os.File
		}{"multica", command})
	}
	if err := privateDirectory(c.Directory); err != nil {
		return "", err
	}
	root, err := os.MkdirTemp(c.Directory, "run-*")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(root)
		}
	}()
	for _, sub := range []string{"proc", "work", "tmp", "dev", ".oldroot"} {
		if err := os.Mkdir(filepath.Join(root, sub), 0700); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(root, "dev", "null"), nil, 0600); err != nil {
		return "", err
	}
	for _, binary := range binaries {
		out, err := os.OpenFile(filepath.Join(root, binary.name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0500)
		if err != nil {
			return "", err
		}
		_, seekErr := binary.file.Seek(0, io.SeekStart)
		var copyErr error
		if seekErr == nil {
			_, copyErr = io.Copy(out, binary.file)
		}
		if err := errors.Join(seekErr, copyErr, out.Close()); err != nil {
			return "", err
		}
	}
	ok = true
	return root, nil
}

func openStaticExecutable(path string) (*os.File, error) {
	// Scripts and dynamically linked providers need a separate, verified adapter;
	// silently exposing the host's loader/libraries/home would break isolation.
	if !filepath.IsAbs(path) {
		return nil, ErrUnsupported
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrUnsupported
	}
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
		}
	}()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil, ErrUnsupported
	}
	binary, err := elf.NewFile(f)
	if err != nil {
		return nil, ErrUnsupported
	}
	if binary.Type != elf.ET_EXEC && binary.Type != elf.ET_DYN {
		return nil, ErrUnsupported
	}
	if (runtime.GOARCH == "amd64" && binary.Machine != elf.EM_X86_64) || (runtime.GOARCH == "arm64" && binary.Machine != elf.EM_AARCH64) || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return nil, ErrUnsupported
	}
	for _, p := range binary.Progs {
		if p.Type == elf.PT_INTERP {
			return nil, ErrUnsupported
		}
	}
	ok = true
	return f, nil
}

func (p *Process) NamespaceID() string { return p.namespaceID }

func (p *Process) Stop() error {
	p.stopOnce.Do(func() {
		_ = p.liveness.Close()
		p.stopErr = p.cmd.Process.Kill()
		if errors.Is(p.stopErr, os.ErrProcessDone) {
			p.stopErr = nil
		}
	})
	return p.stopErr
}

func (p *Process) Wait() (ContainmentResult, error) {
	p.waitOnce.Do(func() {
		err := p.cmd.Wait()
		_ = p.liveness.Close()
		if p.namespaceID == "" || p.cmd.ProcessState == nil {
			p.waitErr = ErrUnconfirmed
		} else {
			var exit *exec.ExitError
			if err != nil && !errors.As(err, &exit) {
				p.waitErr = ErrUnconfirmed
			}
			p.result = ContainmentResult{ExitCode: p.cmd.ProcessState.ExitCode(), Output: append([]byte(nil), p.output.b.Bytes()...), OutputTruncated: p.output.truncated}
		}
		if p.facts != nil {
			p.result.PreparationFacts = <-p.facts
		}
		// A private per-run copy is removed only after the namespace is reaped.
		if p.waitErr == nil {
			if err := os.RemoveAll(p.root); err != nil {
				p.waitErr = errors.Join(p.waitErr, err)
			}
		}
		close(p.waitDone)
	})
	<-p.waitDone
	return p.result, p.waitErr
}

func readPreparationFacts(r io.Reader) []preparationFact {
	scanner := bufio.NewScanner(io.LimitReader(r, 32<<10))
	scanner.Buffer(make([]byte, 4096), 4096)
	var facts []preparationFact
	seen := make(map[string]bool)
	for scanner.Scan() {
		var fact preparationFact
		if decodeStrict(scanner.Bytes(), &fact) != nil || len(facts) >= 8 || !validPreparationFact(fact) || seen[fact.Stage+"/"+fact.State] {
			return nil
		}
		seen[fact.Stage+"/"+fact.State] = true
		facts = append(facts, fact)
	}
	if scanner.Err() != nil {
		return nil
	}
	return facts
}

func validPreparationFact(f preparationFact) bool {
	return (f.Stage == "attachments" || f.Stage == "bundle" || f.Stage == "git" || f.Stage == "workspace") &&
		(f.State == "prepared" && f.Code == "" || f.State == "failed" && (f.Code == "attachment_unavailable" || f.Code == "input_limit_exceeded" ||
			f.Code == "bundle_unavailable" || f.Code == "git_unavailable" || f.Code == "configuration_changed" || f.Code == "cancelled" || f.Code == "preparation_unavailable"))
}
