package collaborationrun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type hostPreparedRun struct {
	Root      string
	Work      string
	Temp      string
	CodexHome string
	Facts     []preparationFact
}

func (h *HostExecutor) prepare(ctx context.Context, claim protocol.CollaborationRunClaimResponse, key string, token func() string, client *Client, lifecycle *agent.LaunchLifecycle) (*hostPreparedRun, error) {
	manifest := claim.Preparation
	if protocol.ValidateCollaborationPreparation(manifest, claim.Identity, claim.Inputs) != nil || manifest.Version != protocol.CollaborationHostPreparationVersion {
		return nil, preparationFailure("workspace", "configuration_changed", ErrProtocol)
	}
	caps, err := h.Config.Capabilities()
	if err != nil {
		return nil, err
	}
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil || digest != manifest.CapabilityDigest || !reflect.DeepEqual(caps.Provider, manifest.Provider) || !reflect.DeepEqual(caps.Host, manifest.Host) ||
		!reflect.DeepEqual(caps.Git, manifest.Git) || caps.Limits != manifest.Limits {
		return nil, preparationFailure("workspace", "configuration_changed", ErrUnsupported)
	}
	prepared := &hostPreparedRun{Root: filepath.Join(h.Directory, key)}
	prepared.Work = filepath.Join(prepared.Root, "work")
	for _, directory := range []string{prepared.Root, filepath.Join(prepared.Root, "runs")} {
		if err := privateDirectory(directory); err != nil {
			return nil, err
		}
	}
	prepared.Temp = filepath.Join(prepared.Root, "runs", claim.Identity.RunID)
	if err := privateDirectory(prepared.Temp); err != nil {
		return nil, err
	}
	if err := client.CheckPreparation(ctx, *manifest, claim.Inputs, token()); err != nil {
		return nil, preparationFailure("workspace", "configuration_changed", err)
	}
	if err := h.prepareCLI(ctx, prepared.Temp); err != nil {
		return nil, preparationFailure("workspace", "preparation_unavailable", err)
	}
	markerPath := filepath.Join(prepared.Root, ".workspace.state")
	marker, markerErr := os.ReadFile(markerPath)
	if errors.Is(markerErr, os.ErrNotExist) {
		if _, err := os.Lstat(prepared.Work); err == nil || !errors.Is(err, os.ErrNotExist) {
			return nil, preparationFailure("workspace", "preparation_unavailable", ErrUnconfirmed)
		}
		staged := filepath.Join(prepared.Temp, "initial-work")
		if err := privateDirectory(staged); err != nil {
			return nil, err
		}
		if h.Config.Git != nil {
			if err := h.prepareGit(ctx, prepared.Temp, staged, lifecycle); err != nil {
				return nil, preparationFailure("git", "git_unavailable", err)
			}
		}
		if err := os.Rename(staged, prepared.Work); err != nil {
			return nil, err
		}
		if err := atomicPrivateWrite(markerPath, []byte(manifest.Host.ConfigurationDigest)); err != nil {
			return nil, err
		}
	} else if markerErr != nil || string(marker) != manifest.Host.ConfigurationDigest {
		return nil, preparationFailure("workspace", "configuration_changed", ErrUnconfirmed)
	}
	if err := privateDirectory(prepared.Work); err != nil {
		return nil, err
	}
	if h.Config.Git != nil {
		prepared.Facts = append(prepared.Facts, preparationFact{Stage: "git", State: "prepared"})
	}
	root, err := os.OpenRoot(prepared.Work)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for _, file := range manifest.Attachments {
		if err := root.MkdirAll(filepath.Dir(file.Destination), 0700); err != nil {
			return nil, preparationFailure("attachments", "attachment_unavailable", err)
		}
		// Replace only the exact delivered version. OpenRoot prevents a prior
		// agent-created link from turning preparation into a write elsewhere.
		if err := root.Remove(file.Destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, preparationFailure("attachments", "attachment_unavailable", err)
		}
		f, err := root.OpenFile(file.Destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, preparationFailure("attachments", "attachment_unavailable", err)
		}
		err = client.DownloadAttachment(ctx, *manifest, file, token(), f)
		err = errors.Join(err, f.Sync(), f.Close())
		if err != nil {
			return nil, preparationFailure("attachments", "attachment_unavailable", err)
		}
	}
	if len(manifest.Attachments) > 0 {
		prepared.Facts = append(prepared.Facts, preparationFact{Stage: "attachments", State: "prepared"})
	}
	if h.Config.Provider == "codex" {
		prepared.CodexHome, err = execenv.PrepareCollaborationCodexHome(prepared.Root, h.Config.ProviderVersion, h.logger())
		if err != nil {
			return nil, preparationFailure("workspace", "preparation_unavailable", err)
		}
	}
	prepared.Facts = append(prepared.Facts, preparationFact{Stage: "workspace", State: "prepared"})
	return prepared, nil
}

// The brief names `multica`, so bind that name to the selected executable.
// Adding only the source directory to PATH could select an ambient binary
// when this executable has a development or versioned filename.
func (h *HostExecutor) prepareCLI(ctx context.Context, temp string) error {
	bin := filepath.Join(temp, "bin")
	if err := privateDirectory(bin); err != nil {
		return err
	}
	source, err := os.Open(h.CLIExecutable)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxProviderFileBytes ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return ErrUnsupported
	}
	file, err := os.CreateTemp(bin, ".multica-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := protectNewPrivateFile(file); err != nil {
		_ = file.Close()
		return err
	}
	n, copyErr := io.Copy(file, io.LimitReader(contextReader{ctx, source}, info.Size()+1))
	err = errors.Join(copyErr, file.Chmod(0700), file.Sync(), file.Close())
	if err != nil || n != info.Size() {
		return errors.Join(ErrProtocol, err)
	}
	name := "multica"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := replacePrivateFile(file.Name(), filepath.Join(bin, name)); err != nil {
		return err
	}
	return syncDirectory(bin)
}

type hostGitOutput struct {
	bytes.Buffer
	truncated bool
}

type hostArchiveOutput struct {
	writer    io.Writer
	remaining int64
	truncated bool
}

func (w *hostArchiveOutput) Write(value []byte) (int, error) {
	n := len(value)
	if int64(len(value)) > w.remaining {
		w.truncated = true
		value = value[:int(w.remaining)]
	}
	written, err := w.writer.Write(value)
	w.remaining -= int64(written)
	if err != nil {
		return written, err
	}
	return n, nil
}

func (b *hostGitOutput) Write(value []byte) (int, error) {
	n := len(value)
	remaining := maxGitIndexBytes - b.Len()
	if len(value) > remaining {
		b.truncated = true
		value = value[:remaining]
	}
	_, _ = b.Buffer.Write(value)
	return n, nil
}

func (h *HostExecutor) prepareGit(ctx context.Context, temp, work string, lifecycle *agent.LaunchLifecycle) error {
	g := h.Config.Git
	input, err := os.Open(g.BundleFile)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != g.SizeBytes {
		return ErrProtocol
	}
	bundle := filepath.Join(temp, "source.bundle")
	f, err := os.OpenFile(bundle, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(contextReader{ctx, input}, g.SizeBytes+1))
	err = errors.Join(copyErr, f.Sync(), f.Close())
	if err != nil || n != g.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != g.BundleDigest {
		return errors.Join(ErrProtocol, err)
	}
	repository := filepath.Join(temp, "repository")
	env := []string{"LANG=C", "PATH=" + os.Getenv("PATH"), "HOME=" + temp, "TMPDIR=" + temp, "TEMP=" + temp, "TMP=" + temp,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_ATTR_NOSYSTEM=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0", "GIT_PROTOCOL_FROM_USER=0", "GIT_LFS_SKIP_SMUDGE=1"}
	for _, key := range []string{"SYSTEMROOT", "SystemRoot", "WINDIR"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	command := func(args ...string) *exec.Cmd {
		fixed := []string{"-c", "protocol.allow=never", "-c", "protocol.file.allow=always", "-c", "core.hooksPath=" + os.DevNull,
			"-c", "core.attributesfile=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "transfer.fsckObjects=true"}
		cmd := lifecycle.CommandContext(ctx, agent.LaunchInfo{Provider: "git", Kind: agent.LaunchKindPreparation}, g.Executable, append(fixed, args...)...)
		cmd.Dir, cmd.Env, cmd.Stderr, cmd.WaitDelay = temp, env, io.Discard, time.Second
		return cmd
	}
	run := func(args ...string) ([]byte, error) {
		var output hostGitOutput
		cmd := command(args...)
		cmd.Stdout = &output
		err := lifecycle.Run(cmd)
		if output.truncated {
			return nil, ErrProtocol
		}
		return output.Bytes(), err
	}
	for _, args := range [][]string{
		{"init", "--bare", "--template=", "--object-format=" + g.ObjectFormat, repository},
		{"-C", repository, "bundle", "verify", bundle},
		{"-C", repository, "bundle", "unbundle", bundle},
	} {
		if _, err := run(args...); err != nil {
			return err
		}
	}
	typeName, err := run("-C", repository, "cat-file", "-t", g.CommitOID)
	if err != nil || strings.TrimSpace(string(typeName)) != "commit" {
		return errors.Join(ErrProtocol, err)
	}
	tree, err := run("-C", repository, "ls-tree", "--full-tree", "-r", "-z", "--long", g.CommitOID)
	if err != nil {
		return err
	}
	entries, err := parseGitTree(tree, g.ObjectFormat, h.Config.Limits)
	if err != nil {
		return err
	}
	archive, err := os.OpenFile(filepath.Join(temp, "source.tar"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer archive.Close()
	cmd := command("-C", repository, "archive", "--format=tar", g.CommitOID)
	output := &hostArchiveOutput{writer: archive, remaining: h.Config.Limits.GitExpandedBytes + int64(h.Config.Limits.EntryCount)*2048 + 4096}
	cmd.Stdout = output
	if err := lifecycle.Run(cmd); err != nil {
		return err
	}
	if output.truncated {
		return ErrProtocol
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return extractGitArchive(ctx, archive, work, entries, g.ObjectFormat, g.CommitOID, h.Config.Limits)
}

func (h *HostExecutor) environment(claim protocol.CollaborationRunClaimResponse, prepared *hostPreparedRun, server, token string) (map[string]string, error) {
	identityEnv, err := cli.HostCollaborationEnvironment(claim.Identity, server, token)
	if err != nil {
		return nil, err
	}
	env := map[string]string{"PATH": filepath.Join(prepared.Temp, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TMPDIR": prepared.Temp, "TMP": prepared.Temp, "TEMP": prepared.Temp}
	for _, entry := range identityEnv {
		key, value, _ := strings.Cut(entry, "=")
		env[key] = value
	}
	if prepared.CodexHome != "" {
		env["CODEX_HOME"] = prepared.CodexHome
	}
	return env, nil
}
