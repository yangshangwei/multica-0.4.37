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
	"sync"
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

func (c *NamespaceController) Probe(ctx context.Context) error {
	if !namespaceSupported() {
		return ErrUnsupported
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, err := c.start(probeCtx, ProviderInput{}, true)
	if err != nil {
		return err
	}
	result, err := p.Wait()
	if err != nil || result.ExitCode != 0 {
		return ErrUnsupported
	}
	return nil
}

func (c *NamespaceController) Start(ctx context.Context, input ProviderInput) (*Process, error) {
	if !namespaceSupported() {
		return nil, ErrUnsupported
	}
	return c.start(ctx, input, false)
}

func (c *NamespaceController) start(ctx context.Context, input ProviderInput, probe bool) (*Process, error) {
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
	if err := cmd.Start(); err != nil {
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
	type readyResult struct {
		response namespaceReady
		err      error
	}
	ready := make(chan readyResult, 1)
	go func() {
		var response namespaceReady
		raw, err := io.ReadAll(io.LimitReader(readyR, 4097))
		if err == nil && len(raw) <= 4096 {
			err = decodeStrict(raw, &response)
		}
		if err == nil && response.NamespaceID == "" {
			err = fmt.Errorf("%w: %s", ErrUnsupported, response.ErrorCode)
		}
		ready <- readyResult{response: response, err: err}
	}()
	select {
	case result := <-ready:
		if result.err != nil {
			_ = p.Stop()
			_, _ = p.Wait()
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
