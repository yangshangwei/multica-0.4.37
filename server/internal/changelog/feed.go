// Package changelog serves deployment-owned release history without public
// network access. A configured file is revalidated on every read.
package changelog

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// MaxFileSize bounds both the embedded baseline and deployment file to 2 MiB.
// Oversized histories are rejected, never silently truncated.
const MaxFileSize = 2 << 20

// Warning codes are safe to return to clients; detailed errors stay server-side.
const (
	WarningFileUnavailable = "changelog_file_unavailable"
	WarningFileInvalid     = "changelog_file_invalid"
)

//go:embed content/changelog.json
var embeddedContent []byte

var (
	commitPattern     = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	timestampPattern  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?(?:Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])$`)
)

type Feed struct {
	SchemaVersion int       `json:"schema_version"`
	GeneratedAt   string    `json:"generated_at"`
	Releases      []Release `json:"releases"`
}

type Release struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	Title       string    `json:"title"`
	PublishedAt *string   `json:"published_at"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
	Commit      *string   `json:"commit"`
	BaseCommit  *string   `json:"base_commit"`
	Sections    []Section `json:"sections"`
}

// UnmarshalJSON distinguishes explicitly unknown historical metadata (null)
// from required fields that are absent altogether.
func (r *Release) UnmarshalJSON(data []byte) error {
	type releaseJSON Release
	_, err := decodeObject(data, (*releaseJSON)(r), "id", "version", "title", "published_at", "status", "source", "commit", "base_commit", "sections")
	return err
}

type Section struct {
	Category string `json:"category"`
	Items    []Item `json:"items"`
}

func (s *Section) UnmarshalJSON(data []byte) error {
	type sectionJSON Section
	_, err := decodeObject(data, (*sectionJSON)(s), "category", "items")
	return err
}

type Item struct {
	Text   string  `json:"text"`
	Commit *string `json:"commit,omitempty"`
}

func (i *Item) UnmarshalJSON(data []byte) error {
	type itemJSON Item
	fields, err := decodeObject(data, (*itemJSON)(i), "text")
	if err != nil {
		return err
	}
	if _, present := fields["commit"]; present && i.Commit == nil {
		return fmt.Errorf("item commit must be a full commit ID when present")
	}
	return nil
}

type Response struct {
	Feed
	ServerVersion *string `json:"server_version"`
	FeedSource    string  `json:"feed_source"`
	IsStale       bool    `json:"is_stale"`
	Warning       *string `json:"warning"`
}

type Resource struct {
	content []byte
	etag    string
}

// Content returns an isolated copy so callers cannot mutate a response resource.
func (r Resource) Content() []byte { return bytes.Clone(r.content) }

// ETag is a strong validator of the complete response, including stale state.
func (r Resource) ETag() string { return r.etag }

// Reader owns one deployment's source configuration and last-valid snapshot.
// There is no shared mutable cache between server/handler instances.
type Reader struct {
	mu            sync.Mutex
	filePath      string
	serverVersion *string
	embedded      Feed
	lastValid     *Feed
}

// New captures source configuration once. Invalid build-owned content is a
// build defect; a missing or invalid deployment file instead degrades on Read.
func New(filePath, serverVersion string) *Reader {
	baseline, err := Validate(embeddedContent)
	if err != nil {
		panic(fmt.Sprintf("changelog: invalid embedded baseline: %v", err))
	}
	reader := &Reader{filePath: filePath, embedded: baseline}
	if serverVersion = strings.TrimSpace(serverVersion); serverVersion != "" {
		reader.serverVersion = &serverVersion
	}
	return reader
}

// Read always returns validated content. A non-nil error reports why a
// configured source could not refresh; the response itself carries only the
// public warning code and retains the last valid file or embedded baseline.
func (r *Reader) Read() (Resource, error) {
	// Keep the read and validation inside the lock, not just the assignment:
	// an older slow read must never overwrite a more recently read snapshot.
	r.mu.Lock()
	defer r.mu.Unlock()

	response := Response{Feed: r.embedded, ServerVersion: r.serverVersion, FeedSource: "embedded"}
	var refreshErr error
	if r.filePath != "" {
		warning := WarningFileUnavailable
		data, err := readFile(r.filePath)
		if err == nil {
			var feed Feed
			feed, err = Validate(data)
			if err == nil {
				r.lastValid = &feed
			} else {
				warning = WarningFileInvalid
			}
		}
		if r.lastValid != nil {
			response.Feed = *r.lastValid
			response.FeedSource = "file"
		}
		if err != nil {
			refreshErr = fmt.Errorf("refresh changelog file %q: %w", r.filePath, err)
			response.IsStale = true
			response.Warning = &warning
		}
	}
	data, err := json.Marshal(response)
	if err != nil {
		panic(fmt.Sprintf("changelog: marshal validated response: %v", err))
	}
	return Resource{content: data, etag: fmt.Sprintf(`"%x"`, sha256.Sum256(data))}, refreshErr
}

func readFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("changelog source must be a regular file")
	}
	return io.ReadAll(io.LimitReader(file, MaxFileSize+1))
}

// Validate accepts one complete v1 artifact. Unknown additive properties are
// ignored, but publication identity and all essential fields remain strict.
func Validate(data []byte) (Feed, error) {
	var feed Feed
	if len(data) > MaxFileSize {
		return feed, fmt.Errorf("changelog exceeds the %d-byte limit", MaxFileSize)
	}
	if !utf8.Valid(data) {
		return feed, fmt.Errorf("changelog must be valid UTF-8")
	}
	if _, err := decodeObject(data, &feed, "schema_version", "generated_at", "releases"); err != nil {
		return Feed{}, fmt.Errorf("decode changelog: %w", err)
	}
	if feed.SchemaVersion != 1 {
		return Feed{}, fmt.Errorf("unsupported changelog schema_version %d", feed.SchemaVersion)
	}
	if !validTimestamp(feed.GeneratedAt) {
		return Feed{}, fmt.Errorf("generated_at must be an RFC 3339 timestamp")
	}
	if feed.Releases == nil {
		return Feed{}, fmt.Errorf("releases must be an array")
	}
	identities := make(map[string]bool, len(feed.Releases))
	forkRepository := ""
	for index, release := range feed.Releases {
		repository, err := validateRelease(release)
		if err != nil {
			return Feed{}, fmt.Errorf("releases[%d]: %w", index, err)
		}
		if identities[release.ID] {
			return Feed{}, fmt.Errorf("duplicate release identity %q", release.ID)
		}
		identities[release.ID] = true
		if release.Source == "fork" {
			if forkRepository != "" && forkRepository != repository {
				return Feed{}, fmt.Errorf("changelog cannot mix fork repositories")
			}
			forkRepository = repository
		}
	}
	return feed, nil
}

func validateRelease(release Release) (string, error) {
	if strings.TrimSpace(release.Version) == "" || strings.TrimSpace(release.Title) == "" {
		return "", fmt.Errorf("version and title must be nonempty text")
	}
	if release.Source != "fork" && release.Source != "upstream" {
		return "", fmt.Errorf("unknown release source %q", release.Source)
	}
	identityVersion := release.Version
	switch release.Status {
	case "published", "prerelease":
		if release.PublishedAt == nil || !validTimestamp(*release.PublishedAt) {
			return "", fmt.Errorf("published releases require an RFC 3339 published_at")
		}
	case "unreleased":
		if release.PublishedAt != nil {
			return "", fmt.Errorf("unreleased published_at must be null")
		}
		identityVersion = "unreleased"
	default:
		return "", fmt.Errorf("unknown release status %q", release.Status)
	}
	identity := strings.Split(release.ID, ":")
	if len(identity) != 3 || identity[0] != release.Source || !repositoryPattern.MatchString(identity[1]) || identity[2] != identityVersion {
		return "", fmt.Errorf("release id must match source, repository, and version")
	}
	for _, commit := range []*string{release.Commit, release.BaseCommit} {
		if commit == nil {
			if release.Source == "fork" {
				return "", fmt.Errorf("fork releases require commit and base_commit")
			}
		} else if !commitPattern.MatchString(*commit) {
			return "", fmt.Errorf("commit and base_commit must be full lowercase commit IDs")
		}
	}
	if release.Sections == nil {
		return "", fmt.Errorf("sections must be an array")
	}
	for _, section := range release.Sections {
		switch section.Category {
		case "features", "improvements", "fixes", "other":
		default:
			return "", fmt.Errorf("unknown section category %q", section.Category)
		}
		if section.Items == nil {
			return "", fmt.Errorf("section items must be an array")
		}
		for _, item := range section.Items {
			if strings.TrimSpace(item.Text) == "" {
				return "", fmt.Errorf("item text must be nonempty")
			}
			if item.Commit != nil && !commitPattern.MatchString(*item.Commit) {
				return "", fmt.Errorf("item commit must be a full lowercase commit ID")
			}
		}
	}
	return identity[1], nil
}

func validTimestamp(value string) bool {
	if !timestampPattern.MatchString(value) {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func decodeObject(data []byte, dest any, required ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	for _, field := range required {
		if _, present := fields[field]; !present {
			return nil, fmt.Errorf("%s is required", field)
		}
	}
	return fields, json.Unmarshal(data, dest)
}
