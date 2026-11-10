// Package artifact implements the fixed local-synthetic artifact packaging policy.
// It does not authorize execution; callers must durably claim an approved spec first.
package artifact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	Action                  = "package_release_candidate"
	Environment             = "local-synthetic"
	Policy                  = "artifact-tar-v1"
	MaxRequestBytes         = 32 * 1024
	MaxSpecBytes            = 24 * 1024
	MaxFiles                = 64
	MaxFileBytes      int64 = 8 * 1024 * 1024
	MaxTotalBytes     int64 = 32 * 1024 * 1024
	AdmissionLifetime       = 15 * time.Minute
	ExecutionTimeout        = 120 * time.Second
	timestampLayout         = "2006-01-02T15:04:05.000000Z"
)

var (
	ErrInvalid       = errors.New("invalid artifact contract")
	ErrDisabled      = errors.New("artifact adapter unavailable")
	uuidPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	digestPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	revisionPattern  = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	componentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

type Entry struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type Request struct {
	AgentID               string  `json:"agent_id"`
	TaskID                string  `json:"task_id"`
	ResourceKind          string  `json:"resource_kind"`
	ResourceID            string  `json:"resource_id"`
	Action                string  `json:"action"`
	StagingID             string  `json:"staging_id"`
	SourceRevision        string  `json:"source_revision"`
	StagingManifestDigest string  `json:"staging_manifest_digest"`
	InputManifestDigest   string  `json:"input_manifest_digest"`
	Entries               []Entry `json:"entries"`
	TargetEnvironment     string  `json:"target_environment"`
	PolicyVersion         string  `json:"policy_version"`
}

// Binding contains trusted, transaction-resolved identities, never HTTP hints.
type Binding struct {
	WorkspaceID      string `json:"workspace_id"`
	InitiatorKind    string `json:"initiator_kind"`
	InitiatorID      string `json:"initiator_id"`
	OriginatorUserID string `json:"originator_user_id"`
	RuntimeID        string `json:"runtime_id"`
	TaskGeneration   int64  `json:"task_generation"`
}

type Spec struct {
	Request          Request `json:"request"`
	Binding          Binding `json:"binding"`
	RootIdentity     string  `json:"root_identity"`
	InvocationDigest string  `json:"invocation_digest"`
	CreatedAt        string  `json:"created_at"`
	ExpiresAt        string  `json:"expires_at"`
}

// StagingManifest is operator-written at <staging>/<staging_id>/manifest.json.
// Selected regular files live below that generation's files/ directory.
type StagingManifest struct {
	WorkspaceID         string `json:"workspace_id"`
	AgentID             string `json:"agent_id"`
	TaskID              string `json:"task_id"`
	TaskGeneration      int64  `json:"task_generation"`
	ResourceID          string `json:"resource_id"`
	SourceRevision      string `json:"source_revision"`
	InputManifestDigest string `json:"input_manifest_digest"`
}

func validUUID(s string) bool {
	return uuidPattern.MatchString(s) && s != "00000000-0000-0000-0000-000000000000"
}

func validPath(path string) bool {
	if len(path) == 0 || len(path) > 96 {
		return false
	}
	parts := strings.Split(path, "/")
	if len(parts) > 8 {
		return false
	}
	for _, part := range parts {
		if !componentPattern.MatchString(part) || strings.HasSuffix(part, ".") {
			return false
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
			return false
		}
	}
	return true
}

func normalizeEntries(entries []Entry) ([]Entry, error) {
	if len(entries) < 1 || len(entries) > MaxFiles {
		return nil, ErrInvalid
	}
	normalized := append([]Entry(nil), entries...)
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Path < normalized[j].Path })
	seen := make(map[string]bool, len(entries))
	var total int64
	for _, e := range normalized {
		if !validPath(e.Path) || e.SizeBytes < 0 || e.SizeBytes > MaxFileBytes || !digestPattern.MatchString(e.SHA256) {
			return nil, ErrInvalid
		}
		lower := strings.ToLower(e.Path)
		if seen[lower] {
			return nil, ErrInvalid
		}
		seen[lower] = true
		total += e.SizeBytes
		if total > MaxTotalBytes {
			return nil, ErrInvalid
		}
	}
	for path := range seen {
		parts := strings.Split(path, "/")
		for i := 1; i < len(parts); i++ {
			if seen[strings.Join(parts[:i], "/")] {
				return nil, ErrInvalid
			}
		}
	}
	return normalized, nil
}

func canonicalJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, ErrInvalid
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func domainDigest(domain string, v any) (string, error) {
	b, err := canonicalJSON(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(domain+"\n"), b...))
	return hex.EncodeToString(sum[:]), nil
}

func InputDigest(entries []Entry) (string, error) {
	normalized, err := normalizeEntries(entries)
	if err != nil {
		return "", err
	}
	return domainDigest("multica-artifact-input-v1", normalized)
}

func NormalizeRequest(r Request) (Request, error) {
	if !validUUID(r.AgentID) || !validUUID(r.TaskID) || !validUUID(r.ResourceID) || !validUUID(r.StagingID) || r.ResourceKind != "issue" || r.Action != Action || r.TargetEnvironment != Environment || r.PolicyVersion != Policy || !revisionPattern.MatchString(r.SourceRevision) || !digestPattern.MatchString(r.StagingManifestDigest) || !digestPattern.MatchString(r.InputManifestDigest) {
		return Request{}, ErrInvalid
	}
	entries, err := normalizeEntries(r.Entries)
	if err != nil {
		return Request{}, err
	}
	r.Entries = entries
	digest, err := InputDigest(entries)
	if err != nil || digest != r.InputManifestDigest {
		return Request{}, ErrInvalid
	}
	return r, nil
}

func validateBinding(b Binding) error {
	if !validUUID(b.WorkspaceID) || !validUUID(b.InitiatorID) || !validUUID(b.OriginatorUserID) || !validUUID(b.RuntimeID) || (b.InitiatorKind != "human" && b.InitiatorKind != "agent") || b.TaskGeneration < 0 {
		return ErrInvalid
	}
	return nil
}

func RequestDigest(r Request, b Binding) (string, error) {
	normalized, err := NormalizeRequest(r)
	if err != nil {
		return "", err
	}
	if err = validateBinding(b); err != nil {
		return "", err
	}
	if b.InitiatorKind == "agent" && b.InitiatorID != r.AgentID {
		return "", ErrInvalid
	}
	return domainDigest("multica-artifact-request-v1", struct {
		Request     Request `json:"request"`
		WorkspaceID string  `json:"workspace_id"`
		Kind        string  `json:"initiator_kind"`
		ID          string  `json:"initiator_id"`
	}{normalized, b.WorkspaceID, b.InitiatorKind, b.InitiatorID})
}

func StagingDigest(m StagingManifest) (string, error) {
	if !validUUID(m.WorkspaceID) || !validUUID(m.AgentID) || !validUUID(m.TaskID) || !validUUID(m.ResourceID) || m.TaskGeneration < 0 || !revisionPattern.MatchString(m.SourceRevision) || !digestPattern.MatchString(m.InputManifestDigest) {
		return "", ErrInvalid
	}
	return domainDigest("multica-artifact-staging-v1", m)
}

func invocationDigest(r Request) (string, error) {
	return domainDigest("multica-artifact-invocation-v1", struct {
		Adapter       string `json:"adapter"`
		Policy        string `json:"policy"`
		StagingID     string `json:"staging_id"`
		StagingDigest string `json:"staging_digest"`
		InputDigest   string `json:"input_digest"`
		Environment   string `json:"environment"`
		Format        string `json:"format"`
		Headers       string `json:"headers"`
	}{"in-process-tar", Policy, r.StagingID, r.StagingManifestDigest, r.InputManifestDigest, Environment, "USTAR", "regular-0644-epoch-uid0-gid0-v1"})
}

func makeSpec(r Request, b Binding, root string, created time.Time) (Spec, error) {
	r, err := NormalizeRequest(r)
	if err != nil {
		return Spec{}, err
	}
	if _, err = RequestDigest(r, b); err != nil {
		return Spec{}, err
	}
	if !digestPattern.MatchString(root) || created.IsZero() {
		return Spec{}, ErrInvalid
	}
	invocation, err := invocationDigest(r)
	if err != nil {
		return Spec{}, err
	}
	created = created.UTC().Truncate(time.Microsecond)
	s := Spec{Request: r, Binding: b, RootIdentity: root, InvocationDigest: invocation, CreatedAt: created.Format(timestampLayout), ExpiresAt: created.Add(AdmissionLifetime).Format(timestampLayout)}
	if _, _, err := CanonicalSpec(s); err != nil {
		return Spec{}, err
	}
	return s, nil
}

func CanonicalSpec(s Spec) ([]byte, string, error) {
	r, err := NormalizeRequest(s.Request)
	if err != nil {
		return nil, "", err
	}
	s.Request = r
	if _, err = RequestDigest(r, s.Binding); err != nil {
		return nil, "", err
	}
	created, e1 := time.Parse(timestampLayout, s.CreatedAt)
	expires, e2 := time.Parse(timestampLayout, s.ExpiresAt)
	if e1 != nil || e2 != nil || created.IsZero() || created.Format(timestampLayout) != s.CreatedAt || expires.Format(timestampLayout) != s.ExpiresAt || !expires.Equal(created.Add(AdmissionLifetime)) || !digestPattern.MatchString(s.RootIdentity) {
		return nil, "", ErrInvalid
	}
	invocation, err := invocationDigest(r)
	if err != nil || invocation != s.InvocationDigest {
		return nil, "", ErrInvalid
	}
	data, err := canonicalJSON(s)
	if err != nil || len(data) > MaxSpecBytes {
		return nil, "", ErrInvalid
	}
	digest, err := domainDigest("multica-artifact-spec-v1", s)
	return data, digest, err
}

// rejectAmbiguity rejects duplicate keys and nulls at every nesting level.
func rejectAmbiguity(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil || token == nil {
		return ErrInvalid
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return ErrInvalid
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return ErrInvalid
			}
			seen[key] = true
			if err = rejectAmbiguity(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := rejectAmbiguity(dec); err != nil {
				return err
			}
		}
	default:
		return ErrInvalid
	}
	end, err := dec.Token()
	if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
		return ErrInvalid
	}
	return nil
}

func decodeStrict(data []byte, v any) error {
	// encoding/json otherwise accepts case-insensitive aliases of declared keys.
	typ := reflect.TypeOf(v)
	if typ == nil || typ.Kind() != reflect.Pointer || typ.Elem().Kind() != reflect.Struct {
		return ErrInvalid
	}
	typ = typ.Elem()
	allowed := make(map[string]bool, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.PkgPath == "" && name != "-" {
			if name == "" {
				name = field.Name
			}
			allowed[name] = true
		}
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return ErrInvalid
	}
	for key := range object {
		if !allowed[key] {
			return ErrInvalid
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := rejectAmbiguity(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return ErrInvalid
	}
	dec = json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return ErrInvalid
	}
	return nil
}

func requireKeys(data []byte, keys ...string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil || len(obj) != len(keys) {
		return ErrInvalid
	}
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			return ErrInvalid
		}
	}
	return nil
}

func ParseRequest(reader io.Reader) (Request, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if err != nil || len(data) > MaxRequestBytes {
		return Request{}, ErrInvalid
	}
	var r Request
	if err := decodeStrict(data, &r); err != nil {
		return Request{}, err
	}
	if err := requireKeys(data, "agent_id", "task_id", "resource_kind", "resource_id", "action", "staging_id", "source_revision", "staging_manifest_digest", "input_manifest_digest", "entries", "target_environment", "policy_version"); err != nil {
		return Request{}, err
	}
	var raw struct {
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Request{}, ErrInvalid
	}
	for _, entry := range raw.Entries {
		if err := requireKeys(entry, "path", "size_bytes", "sha256"); err != nil {
			return Request{}, err
		}
	}
	return NormalizeRequest(r)
}

func ParseSpec(data []byte) (Spec, error) {
	if len(data) > MaxSpecBytes {
		return Spec{}, ErrInvalid
	}
	var s Spec
	if err := decodeStrict(data, &s); err != nil {
		return Spec{}, err
	}
	canonical, _, err := CanonicalSpec(s)
	if err != nil || !bytes.Equal(data, canonical) {
		return Spec{}, ErrInvalid
	}
	return s, nil
}

// DecodeStrict is the bounded decoder for the small decision/execution bodies.
// Callers must separately validate required field values.
func DecodeStrict(reader io.Reader, dst any) error {
	data, err := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if err != nil || len(data) > MaxRequestBytes {
		return ErrInvalid
	}
	return decodeStrict(data, dst)
}
