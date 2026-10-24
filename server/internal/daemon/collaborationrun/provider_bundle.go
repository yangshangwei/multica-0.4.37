package collaborationrun

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	maxProviderBundleFiles = 1024
	maxProviderBundleBytes = 512 << 20
	maxProviderFileBytes   = 256 << 20
)

// ProviderBundleConfig is local configuration. Sources and account variable
// names never leave the machine; capabilities contain only its opaque digest.
// No glob, directory discovery, installer, host loader or ambient account is
// an alternative to an explicitly listed regular file.
type ProviderBundleConfig struct {
	Version        int                                     `json:"version"`
	Provider       string                                  `json:"provider"`
	ProtocolFamily string                                  `json:"protocol_family"`
	AdapterID      string                                  `json:"adapter_id"`
	Entrypoint     string                                  `json:"entrypoint"`
	Arguments      []string                                `json:"arguments,omitempty"`
	Files          []ProviderBundleFile                    `json:"files"`
	Devices        []string                                `json:"devices,omitempty"`
	Account        *ProviderAccountConfig                  `json:"account,omitempty"`
	Git            *ProviderGitConfig                      `json:"git,omitempty"`
	Limits         protocol.CollaborationPreparationLimits `json:"limits"`
}

type ProviderBundleFile struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes"`
	Mode        string `json:"mode"`
}

type ProviderCredentialEnvironment struct {
	Name      string `json:"name"`
	SourceEnv string `json:"source_env"`
}

// The first account adapter is an explicitly selected Claude API key. OAuth
// homes, refresh files, session directories and other account layouts remain
// unsupported until they have their own narrowly validated adapter.
type ProviderAccountConfig struct {
	Environment []ProviderCredentialEnvironment `json:"environment"`
}

type ProviderGitConfig struct {
	BundleFile   string               `json:"bundle_file"`
	BundleSHA256 string               `json:"bundle_sha256"`
	SizeBytes    int64                `json:"size_bytes"`
	ObjectFormat string               `json:"object_format"`
	CommitOID    string               `json:"commit_oid"`
	Executable   string               `json:"executable"`
	Files        []ProviderBundleFile `json:"files"`
	Devices      []string             `json:"devices,omitempty"`
}

func (b *ProviderBundleConfig) gitSummary() *protocol.CollaborationPreparationGit {
	if b.Git == nil {
		return nil
	}
	return &protocol.CollaborationPreparationGit{ObjectFormat: b.Git.ObjectFormat, CommitOID: b.Git.CommitOID, BundleDigest: b.Git.BundleSHA256}
}

func (b *ProviderBundleConfig) capabilityShape(digest string) protocol.CollaborationRuntimeCapabilities {
	return protocol.CollaborationRuntimeCapabilities{Version: b.Version, PreparationVersion: protocol.CollaborationPreparationVersion,
		OS: "linux", Arch: runtime.GOARCH, Containment: protocol.CollaborationContainmentLinuxPIDNamespace, AttachmentFormat: protocol.CollaborationAttachmentFormat,
		Provider: protocol.CollaborationPreparationProvider{Provider: b.Provider, ProtocolFamily: b.ProtocolFamily, AdapterID: b.AdapterID, AdapterVersion: 1,
			BundleDigest: digest, AccountSource: "runtime_configuration", Resume: "fresh", Fallback: false}, Git: b.gitSummary(), Limits: b.Limits}
}

func (b *ProviderBundleConfig) Validate() error {
	if b == nil {
		return ErrUnsupported
	}
	family := b.Provider
	if builtin, ok := agent.BuiltinRuntimeByID(b.Provider); ok {
		family = builtin.ProtocolFamily
	}
	if !agent.IsSupportedType(family) || family != b.ProtocolFamily ||
		protocol.ValidateCollaborationRuntimeCapabilities(b.capabilityShape(strings.Repeat("0", 64))) != nil ||
		validateBundleFiles(b.Files, b.Entrypoint) != nil || !validBundleDevices(b.Devices) || len(b.Arguments) > 64 {
		return ErrUnsupported
	}
	for _, arg := range b.Arguments {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return ErrUnsupported
		}
	}
	if b.AdapterID == "claude_stream_json_v1" && (len(b.Arguments) != 0 || b.Account == nil) {
		return ErrUnsupported
	}
	if b.Account != nil {
		if b.Provider != "claude" || len(b.Account.Environment) != 1 {
			return ErrUnsupported
		}
		item := b.Account.Environment[0]
		if item.Name != "ANTHROPIC_API_KEY" || !explicitCredentialVariable(item.SourceEnv) {
			return ErrUnsupported
		}
	}
	if b.Git != nil && (!filepath.IsAbs(b.Git.BundleFile) || filepath.Clean(b.Git.BundleFile) != b.Git.BundleFile ||
		b.Git.SizeBytes <= 0 || b.Git.SizeBytes > maxProviderFileBytes || !validDigest(b.Git.BundleSHA256) ||
		validateBundleFiles(b.Git.Files, b.Git.Executable) != nil || !validBundleDevices(b.Git.Devices)) {
		return ErrUnsupported
	}
	return nil
}

func validBundleDevices(devices []string) bool {
	return len(devices) == 0 || len(devices) == 1 && devices[0] == "/dev/urandom"
}

func (b *ProviderBundleConfig) preparationDevices() []string {
	devices := append([]string(nil), b.Devices...)
	if b.Git != nil {
		for _, device := range b.Git.Devices {
			if !slices.Contains(devices, device) {
				devices = append(devices, device)
			}
		}
	}
	return devices
}

func validBundleDestination(value string) bool {
	if !path.IsAbs(value) || path.Clean(value) != value || value == "/" || len(value) > 4096 || strings.ContainsAny(value, "\x00\\") {
		return false
	}
	for _, reserved := range []string{"/proc", "/work", "/tmp", "/home", "/dev", "/inputs", "/.oldroot", "/.preparation"} {
		if value == reserved || strings.HasPrefix(value, reserved+"/") {
			return false
		}
	}
	// Neither file can be used to expand the declared loader dependency closure.
	return value != "/etc/ld.so.preload" && value != "/etc/ld.so.cache"
}

func validateBundleFiles(files []ProviderBundleFile, entrypoint string) error {
	if len(files) == 0 || len(files) > maxProviderBundleFiles || !validBundleDestination(entrypoint) {
		return ErrUnsupported
	}
	seen, total, executable := make(map[string]bool, len(files)), int64(0), false
	for _, file := range files {
		if !filepath.IsAbs(file.Source) || filepath.Clean(file.Source) != file.Source || strings.ContainsRune(file.Source, 0) ||
			!validBundleDestination(file.Destination) || !validDigest(file.SHA256) || file.SizeBytes < 0 || file.SizeBytes > maxProviderFileBytes ||
			file.Mode != "executable" && file.Mode != "readonly" || seen[file.Destination] {
			return ErrUnsupported
		}
		seen[file.Destination] = true
		total += file.SizeBytes
		executable = executable || file.Destination == entrypoint && file.Mode == "executable"
	}
	for destination := range seen {
		for parent := path.Dir(destination); parent != "/"; parent = path.Dir(parent) {
			if seen[parent] {
				return ErrUnsupported
			}
		}
	}
	if !executable || total > maxProviderBundleBytes {
		return ErrUnsupported
	}
	return nil
}

func explicitCredentialVariable(value string) bool {
	if len(value) == 0 || len(value) > 128 || strings.HasPrefix(value, "MULTICA_") || strings.HasPrefix(value, "LD_") || strings.HasPrefix(value, "DYLD_") {
		return false
	}
	for i, c := range value {
		if !(c >= 'A' && c <= 'Z' || c == '_' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return !slices.Contains([]string{"HOME", "PATH", "SHELL", "SSH_AUTH_SOCK", "NODE_OPTIONS", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"}, value)
}

func (b *ProviderBundleConfig) Capabilities() (protocol.CollaborationRuntimeCapabilities, error) {
	if err := b.Validate(); err != nil {
		return protocol.CollaborationRuntimeCapabilities{}, err
	}
	// Source locations are part of the owner-selected local configuration, but
	// only this digest leaves the host. Credentials and Git contents are not read.
	raw, err := json.Marshal(b)
	if err != nil {
		return protocol.CollaborationRuntimeCapabilities{}, ErrUnsupported
	}
	digest := sha256.Sum256(raw)
	return b.capabilityShape(hex.EncodeToString(digest[:])), nil
}

func (b *ProviderBundleConfig) accountEnvironment() ([]string, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	if b.Account == nil {
		return nil, nil
	}
	item := b.Account.Environment[0]
	value, present := os.LookupEnv(item.SourceEnv)
	if !present || value == "" || len(value) > 16<<10 || strings.ContainsAny(value, "\x00\r\n") {
		return nil, ErrUnsupported
	}
	for _, prefix := range []string{"mdt_", "mat_", "mul_", "mcn_", "mcb_", "mct_"} {
		if strings.HasPrefix(value, prefix) {
			return nil, ErrUnsupported
		}
	}
	return []string{item.Name + "=" + value}, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func snapshotBundleFile(ctx context.Context, file ProviderBundleFile, destination string) error {
	if file.SizeBytes < 0 || file.SizeBytes > maxProviderFileBytes || !validDigest(file.SHA256) {
		return ErrUnsupported
	}
	source, err := openBundleSource(file.Source)
	if err != nil {
		return ErrUnsupported
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || info.Size() != file.SizeBytes {
		return ErrUnsupported
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	mode := os.FileMode(0400)
	if file.Mode == "executable" {
		mode = 0500
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(contextReader{ctx, source}, file.SizeBytes+1))
	err = errors.Join(copyErr, out.Sync(), out.Close())
	if err != nil || n != file.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != file.SHA256 {
		_ = os.Remove(destination)
		return ErrUnsupported
	}
	return nil
}

// validateBundleELFs inspects only the private snapshots. No ldd, executable
// probe or implicit host-library lookup may be used to resolve dependencies.
func validateBundleELFs(root string, files []ProviderBundleFile) error {
	declared := make(map[string]ProviderBundleFile, len(files))
	for _, file := range files {
		declared[file.Destination] = file
	}
	for _, file := range files {
		f, err := os.Open(filepath.Join(root, file.Destination))
		if err != nil {
			return ErrUnsupported
		}
		err = validateBundleELF(f, file, declared)
		_ = f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func validateBundleELF(f *os.File, file ProviderBundleFile, declared map[string]ProviderBundleFile) error {
	var magic [4]byte
	n, _ := f.ReadAt(magic[:], 0)
	if n != 4 || string(magic[:]) != "\x7fELF" {
		if file.Mode == "executable" {
			return ErrUnsupported
		}
		return validateBundleSystemFile(f, file)
	}
	binary, err := elf.NewFile(f)
	if err != nil || binary.Type != elf.ET_EXEC && binary.Type != elf.ET_DYN || binary.Class != elf.ELFCLASS64 ||
		binary.Data != elf.ELFDATA2LSB || runtime.GOARCH == "amd64" && binary.Machine != elf.EM_X86_64 ||
		runtime.GOARCH == "arm64" && binary.Machine != elf.EM_AARCH64 || runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return ErrUnsupported
	}
	for _, prog := range binary.Progs {
		if prog.Type != elf.PT_INTERP {
			continue
		}
		value, err := io.ReadAll(io.LimitReader(prog.Open(), 4097))
		if err != nil || len(value) < 2 || len(value) > 4096 || value[len(value)-1] != 0 {
			return ErrUnsupported
		}
		loader := string(value[:len(value)-1])
		provided, ok := declared[loader]
		if !ok || provided.Mode != "executable" || !validBundleDestination(loader) {
			return ErrUnsupported
		}
	}
	libraries, err := binary.ImportedLibraries()
	if err != nil {
		return ErrUnsupported
	}
	search := []string{}
	for _, tag := range []elf.DynTag{elf.DT_RPATH, elf.DT_RUNPATH} {
		paths, err := binary.DynString(tag)
		if err != nil {
			return ErrUnsupported
		}
		for _, entries := range paths {
			for _, directory := range strings.Split(entries, ":") {
				directory = strings.ReplaceAll(strings.ReplaceAll(directory, "${ORIGIN}", path.Dir(file.Destination)), "$ORIGIN", path.Dir(file.Destination))
				if !path.IsAbs(directory) || strings.ContainsRune(directory, '$') {
					return ErrUnsupported
				}
				search = append(search, path.Clean(directory))
			}
		}
	}
	triplet := "aarch64-linux-gnu"
	if runtime.GOARCH == "amd64" {
		triplet = "x86_64-linux-gnu"
	}
	search = append(search, "/lib/"+triplet, "/usr/lib/"+triplet, "/lib64", "/usr/lib64", "/lib", "/usr/lib")
	for _, library := range libraries {
		if path.Base(library) != library || strings.ContainsAny(library, "\x00\\") {
			return ErrUnsupported
		}
		found := false
		for _, directory := range search {
			if _, ok := declared[path.Join(directory, library)]; ok {
				found = true
				break
			}
		}
		if !found {
			return ErrUnsupported
		}
	}
	return nil
}

// Non-executable files are limited to the network configuration required by
// the supported native adapter. Arbitrary account/config/session trees cannot
// be disguised as readonly bundle data to bypass the account allowlist.
func validateBundleSystemFile(f *os.File, file ProviderBundleFile) error {
	certificate := file.Destination == "/etc/ssl/cert.pem" || file.Destination == "/etc/ssl/certs/ca-certificates.crt"
	if !certificate && !slices.Contains([]string{"/etc/hosts", "/etc/resolv.conf", "/etc/nsswitch.conf"}, file.Destination) {
		return ErrUnsupported
	}
	limit := int64(64 << 10)
	if certificate {
		limit = 2 << 20
	}
	if file.SizeBytes > limit {
		return ErrUnsupported
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) != file.SizeBytes || !utf8.Valid(raw) || strings.ContainsRune(string(raw), 0) {
		return ErrUnsupported
	}
	if !certificate {
		return nil
	}
	count := 0
	for len(strings.TrimSpace(string(raw))) != 0 {
		block, rest := pem.Decode(raw)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return ErrUnsupported
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return ErrUnsupported
		}
		count++
		raw = rest
	}
	if count == 0 {
		return ErrUnsupported
	}
	return nil
}
