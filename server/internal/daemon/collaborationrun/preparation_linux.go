//go:build linux

package collaborationrun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"golang.org/x/sys/unix"
)

func executePreparedNamespace(request namespaceRequest, input ProviderInput, facts io.Writer) int {
	execution := request.Execution
	manifest := input.Preparation
	report := func(stage, state, code string) {
		_ = json.NewEncoder(facts).Encode(preparationFact{Stage: stage, State: state, Code: code})
	}
	fail := func(stage, code string) int { report(stage, "failed", code); return 125 }
	if execution == nil || manifest == nil || !businessPreparationToken(request.BusinessToken) ||
		execution.AdapterID != manifest.Provider.AdapterID || !validBundleDestination(execution.Entrypoint) ||
		len(execution.Checksums) != len(manifest.Attachments) || execution.Deadline.IsZero() ||
		time.Until(execution.Deadline) > time.Duration(manifest.Limits.TimeoutSeconds)*time.Second {
		return fail("workspace", "configuration_changed")
	}
	ctx, cancel := context.WithDeadline(context.Background(), execution.Deadline)
	defer cancel()
	// /work is the new tmpfs now. A copy into the host-side root/work before
	// isolateRoot would have been masked by that mount.
	for _, file := range manifest.Attachments {
		if err := materializePreparedAttachment(ctx, file, execution.Checksums[file.Destination]); err != nil {
			return fail("attachments", "attachment_unavailable")
		}
	}
	report("attachments", "prepared", "")
	if manifest.Git != nil {
		gitRequest := namespaceRequest{Root: "/.preparation/git-root", Devices: execution.GitDevices, Git: &gitPreparationRequest{Executable: execution.GitExecutable,
			Git: *manifest.Git, Limits: manifest.Limits, Deadline: execution.Deadline}}
		if err := runNestedGit(ctx, gitRequest, "/work"); err != nil {
			return fail("git", "git_unavailable")
		}
		report("git", "prepared", "")
	}
	// A device requested only by the Git bundle must disappear with preparation.
	// Its sole source descriptor was already closed by the bind operation.
	for _, device := range execution.GitDevices {
		if !slices.Contains(execution.Devices, device) {
			if unix.Unmount(device, 0) != nil || os.Remove(device) != nil {
				return fail("workspace", "preparation_unavailable")
			}
		}
	}
	// The nested PID namespace has exited, so no helper descendants or object
	// descriptors survive this cleanup. No object store is visible to provider.
	if err := os.RemoveAll("/.preparation"); err != nil {
		return fail("workspace", "preparation_unavailable")
	}
	if err := os.MkdirAll("/home/provider", 0700); err != nil {
		return fail("workspace", "preparation_unavailable")
	}
	providerEnv := []string{"LANG=C", "HOME=/home/provider", "TMPDIR=/tmp", "PATH=/bin:/usr/bin:/"}
	for _, item := range execution.AccountEnvironment {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key != "ANTHROPIC_API_KEY" || value == "" || len(value) > 16<<10 || strings.ContainsAny(value, "\x00\r\n") || len(execution.AccountEnvironment) != 1 {
			return fail("bundle", "configuration_changed")
		}
		providerEnv = append(providerEnv, item)
	}
	client, err := NewClient(request.ServerURL, "", nil)
	if err != nil || client.CheckPreparation(ctx, *manifest, input.Inputs, request.BusinessToken) != nil || ctx.Err() != nil {
		return fail("workspace", "configuration_changed")
	}
	runEnvironment, err := cli.CollaborationEnvironment(input.Identity, request.ServerURL, request.BusinessToken)
	if err != nil || makeNamespaceRootReadOnly() != nil || dropNamespacePrivileges() != nil {
		return fail("workspace", "preparation_unavailable")
	}
	providerEnv = append(providerEnv, runEnvironment...)
	report("bundle", "prepared", "")
	report("workspace", "prepared", "")
	switch execution.AdapterID {
	case "static_json_v1":
		cmd := exec.Command(execution.Entrypoint, execution.Arguments...)
		cmd.Env, cmd.Dir = providerEnv, "/work"
		cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(request.Input), os.Stdout, os.Stderr
		return waitProviderCommand(cmd)
	case "claude_stream_json_v1":
		if len(execution.Arguments) != 0 || len(execution.AccountEnvironment) != 1 {
			return fail("bundle", "configuration_changed")
		}
		// This marker is set only after the real namespace and privilege drop.
		providerEnv = append(providerEnv, "IS_SANDBOX=1", "CLAUDE_CONFIG_DIR=/home/provider/.claude")
		return executeContainedClaude(execution.Entrypoint, providerEnv, input)
	default:
		return fail("bundle", "configuration_changed")
	}
}

func materializePreparedAttachment(ctx context.Context, file protocol.CollaborationPreparationAttachment, checksum string) error {
	if !validDigest(checksum) || file.Destination != protocol.CollaborationPreparationAttachmentDestination(file.ID, file.Version) {
		return ErrProtocol
	}
	in, err := os.Open(filepath.Join("/inputs", file.Destination))
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.SizeBytes {
		return ErrProtocol
	}
	destination := filepath.Join("/work", file.Destination)
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(contextReader{ctx, in}, file.SizeBytes+1))
	if err := errors.Join(copyErr, out.Close()); err != nil || n != file.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != checksum {
		return ErrProtocol
	}
	return nil
}

func waitProviderCommand(cmd *exec.Cmd) int {
	if err := cmd.Start(); err != nil {
		return 125
	}
	if err := cmd.Wait(); err != nil {
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() < 0 {
			return 137
		}
		return cmd.ProcessState.ExitCode()
	}
	return 0
}

// runNestedGit creates a separate PID/mount/network scope inside the retained
// outer run. Waiting for this init closes every descendant and object FD before
// the caller can expose credentials or start its one provider process.
func runNestedGit(ctx context.Context, request namespaceRequest, work string) error {
	raw, err := json.Marshal(request)
	if err != nil || len(raw) > 64<<10 {
		return ErrProtocol
	}
	configR, configW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer configR.Close()
	defer configW.Close()
	readyR, readyW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer readyR.Close()
	defer readyW.Close()
	liveR, liveW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer liveR.Close()
	defer liveW.Close()
	outR, outW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer outR.Close()
	defer outW.Close()
	null, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	cmd := exec.CommandContext(ctx, "/.preparation/helper", NamespaceHelperArg)
	cmd.Env = []string{"LANG=C"}
	cmd.ExtraFiles = []*os.File{configR, readyW, liveR, null}
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = outW, null, time.Second
	configureNamespace(cmd)
	cmd.SysProcAttr.Cloneflags |= unix.CLONE_NEWNET
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = configR.Close()
	_ = readyW.Close()
	_ = liveR.Close()
	_ = outW.Close()
	go func() { _, _ = configW.Write(raw); _ = configW.Close() }()
	ready := make(chan error, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(readyR, 4097))
		var response namespaceReady
		if err == nil && (len(data) > 4096 || decodeStrict(data, &response) != nil || response.NamespaceID == "") {
			err = ErrUnsupported
		}
		ready <- err
	}()
	select {
	case err = <-ready:
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err == nil && !request.Probe {
		var size uint32
		if err = binary.Read(outR, binary.BigEndian, &size); err == nil {
			if size > maxGitIndexBytes {
				err = ErrProtocol
			} else {
				index := make([]byte, size)
				_, err = io.ReadFull(outR, index)
				var entries []gitTreeEntry
				if err == nil {
					err = decodeStrict(index, &entries)
				}
				if err == nil {
					err = extractGitArchive(ctx, outR, work, entries, request.Git.Git.ObjectFormat, request.Git.Git.CommitOID, request.Git.Limits)
				}
			}
		}
	}
	if err != nil {
		_ = cmd.Process.Kill()
	}
	// No group/PID absence heuristic is used here; this is the nested init we
	// actually spawned, and Linux destroys its descendants before Wait returns.
	waitErr := cmd.Wait()
	return errors.Join(err, waitErr)
}

type boundedGitOutput struct {
	bytes.Buffer
	truncated bool
}

func (b *boundedGitOutput) Write(value []byte) (int, error) {
	n := len(value)
	remaining := maxGitIndexBytes - b.Len()
	if len(value) > remaining {
		b.truncated = true
		value = value[:remaining]
	}
	_, _ = b.Buffer.Write(value)
	return n, nil
}

func executePreparedGit(request gitPreparationRequest) int {
	if !validBundleDestination(request.Executable) || !gitObjectID(request.Git.CommitOID, request.Git.ObjectFormat) || request.Deadline.IsZero() {
		return 125
	}
	ctx, cancel := context.WithDeadline(context.Background(), request.Deadline)
	defer cancel()
	env := []string{"LANG=C", "HOME=/home/provider", "TMPDIR=/tmp", "PATH=/bin:/usr/bin:/", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_ATTR_NOSYSTEM=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0", "GIT_PROTOCOL_FROM_USER=0", "GIT_LFS_SKIP_SMUDGE=1"}
	env = append(env, "GIT_EXEC_PATH="+filepath.Dir(request.Executable))
	command := func(args ...string) *exec.Cmd {
		fixed := []string{"-c", "protocol.allow=never", "-c", "protocol.file.allow=always", "-c", "core.hooksPath=/dev/null",
			"-c", "core.attributesfile=/dev/null", "-c", "core.fsmonitor=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "transfer.fsckObjects=true"}
		cmd := exec.CommandContext(ctx, request.Executable, append(fixed, args...)...)
		cmd.Dir, cmd.Env, cmd.Stderr, cmd.WaitDelay = "/work", env, os.Stderr, time.Second
		return cmd
	}
	run := func(args ...string) ([]byte, error) {
		var output boundedGitOutput
		cmd := command(args...)
		cmd.Stdout = &output
		err := cmd.Run()
		if output.truncated {
			return nil, ErrProtocol
		}
		return output.Bytes(), err
	}
	if _, err := run("init", "--bare", "--template=", "--object-format="+request.Git.ObjectFormat, "/work/repository"); err != nil {
		return 125
	}
	if _, err := run("-C", "/work/repository", "bundle", "verify", "/inputs/source.bundle"); err != nil {
		return 125
	}
	if _, err := run("-C", "/work/repository", "bundle", "unbundle", "/inputs/source.bundle"); err != nil {
		return 125
	}
	typeName, err := run("-C", "/work/repository", "cat-file", "-t", request.Git.CommitOID)
	if err != nil || string(typeName) != "commit\n" {
		return 125
	}
	tree, err := run("-C", "/work/repository", "ls-tree", "--full-tree", "-r", "-z", "--long", request.Git.CommitOID)
	if err != nil {
		return 125
	}
	entries, err := parseGitTree(tree, request.Git.ObjectFormat, request.Limits)
	if err != nil {
		return 125
	}
	index, err := json.Marshal(entries)
	if err != nil || len(index) > maxGitIndexBytes {
		return 125
	}
	if binary.Write(os.Stdout, binary.BigEndian, uint32(len(index))) != nil {
		return 125
	}
	if _, err := os.Stdout.Write(index); err != nil {
		return 125
	}
	cmd := command("-C", "/work/repository", "archive", "--format=tar", request.Git.CommitOID)
	cmd.Stdout = os.Stdout
	if cmd.Run() != nil {
		return 125
	}
	return 0
}

type serializedProviderInput struct {
	mu sync.Mutex
	f  *os.File
}

func (w *serializedProviderInput) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Write(value)
}
func (w *serializedProviderInput) Close() error { return w.f.Close() }

func executeContainedClaude(executable string, env []string, input ProviderInput) int {
	invocation := input.Preparation.Invocation
	args, err := agent.ClaudeFreshStreamArguments(invocation.Instructions, invocation.Model, invocation.ThinkingLevel)
	if err != nil {
		return 125
	}
	// The complete immutable inputs and admitted file/commit metadata are the
	// user frame. Agent instructions are kept in the separate system argument.
	prompt, err := json.Marshal(input)
	if err != nil {
		return 125
	}
	frame, err := agent.ClaudeStreamInput(string(prompt))
	if err != nil {
		return 125
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return 125
	}
	defer outR.Close()
	defer outW.Close()
	inR, inW, err := os.Pipe()
	if err != nil {
		return 125
	}
	defer inR.Close()
	defer inW.Close()
	cmd := exec.Command(executable, args...)
	cmd.Env, cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = env, "/work", inR, outW, os.Stderr
	if err := cmd.Start(); err != nil {
		return 125
	}
	_ = inR.Close()
	_ = outW.Close()
	writer := &serializedProviderInput{f: inW}
	written := make(chan error, 1)
	go func() { _, err := writer.Write(frame); written <- err }()
	var codec agent.ClaudeStreamCodec
	read := make(chan error, 1)
	go func() { read <- codec.Read(outR, writer) }()
	exitErr := cmd.Wait()
	_ = writer.Close()
	var readErr error
	select {
	case readErr = <-read:
	case <-time.After(time.Second):
		// A descendant holding stdio cannot prevent the namespace init from
		// ending. Incomplete output remains absent; its exit proof is separate.
		_ = outR.Close()
		readErr = <-read
		if readErr == nil {
			readErr = ErrProtocol
		}
	}
	writeErr := <-written
	if readErr == nil && writeErr == nil {
		if result, err := codec.Result(exitErr); err == nil {
			_ = json.NewEncoder(os.Stdout).Encode(result)
		}
	}
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() < 0 {
		return 137
	}
	return cmd.ProcessState.ExitCode()
}
