package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	mcpManifestMaxBytes      = 64 * 1024
	mcpCatalogMaxBytes       = 4 * 1024 * 1024
	mcpCatalogMaxEntries     = 256
	mcpManifestMaxDepth      = 16
	mcpManifestVersionDomain = "multica-mcp-catalog:v1\n"
)

var (
	mcpTemplateName       = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	mcpInputName          = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	mcpEnvironmentName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	mcpArgumentFlag       = regexp.MustCompile(`^--?[A-Za-z0-9][A-Za-z0-9_-]*$`)
	mcpHeaderName         = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
	errMcpManifestInvalid = errors.New("invalid manifest")
	errMcpManifestFile    = errors.New("unsafe or unreadable manifest")
)

type mcpManifest struct {
	SchemaVersion    int                 `json:"schema_version"`
	Titles           map[string]string   `json:"titles"`
	Descriptions     map[string]string   `json:"descriptions,omitempty"`
	Category         string              `json:"category"`
	DocumentationURL string              `json:"documentation_url,omitempty"`
	Requirements     map[string][]string `json:"requirements,omitempty"`
	Config           mcpManifestConfig   `json:"config"`
	Inputs           []mcpManifestInput  `json:"inputs,omitempty"`
}

type mcpManifestConfig struct {
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
}

type mcpManifestInput struct {
	Key          string            `json:"key"`
	Labels       map[string]string `json:"labels,omitempty"`
	Descriptions map[string]string `json:"descriptions,omitempty"`
	Required     bool              `json:"required"`
	Secret       *bool             `json:"secret"`
	Validator    string            `json:"validator"`
	Target       mcpManifestTarget `json:"target"`
}

type mcpManifestTarget struct {
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Flag   string `json:"flag,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

func (c McpCatalog) readDeploymentTemplates() ([]McpServerTemplate, error) {
	if c.Directory == "" {
		return nil, nil
	}
	// Remove a trailing separator before Lstat: on Unix it otherwise follows
	// a final symlink instead of returning information about the link itself.
	directoryPath := filepath.Clean(c.Directory)
	info, err := os.Lstat(directoryPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.IsDir() {
		return nil, unavailableMcpCatalog("root")
	}
	root, err := openMcpTemplateDirectory(nil, directoryPath)
	if err != nil {
		return nil, unavailableMcpCatalog("root")
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return nil, unavailableMcpCatalog("root")
	}
	defer directory.Close()
	opened, err := directory.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, unavailableMcpCatalog("root_changed")
	}
	entries, err := directory.ReadDir(mcpCatalogMaxEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, unavailableMcpCatalog("directory_read")
	}
	if len(entries) > mcpCatalogMaxEntries {
		return nil, unavailableMcpCatalog("entry_limit")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var result []McpServerTemplate
	remaining := mcpCatalogMaxBytes
	for _, entry := range entries {
		key := entry.Name()
		if !mcpTemplateName.MatchString(key) || len(key) > 128 {
			// Invalid names are untrusted content and are intentionally not logged.
			continue
		}
		if !entry.IsDir() {
			slog.Warn("MCP deployment template skipped", "key", key, "category", "entry")
			continue
		}
		data, readErr := readMcpManifestFile(root, key, &remaining)
		if errors.Is(readErr, ErrMcpCatalogUnavailable) {
			return nil, readErr
		}
		if readErr != nil {
			slog.Warn("MCP deployment template skipped", "key", key, "category", "file")
			continue
		}
		template, parseErr := parseMcpManifest(key, data, c.AllowHTTP)
		if parseErr != nil {
			slog.Warn("MCP deployment template skipped", "key", key, "category", "manifest")
			continue
		}
		result = append(result, template)
	}
	return result, nil
}

func unavailableMcpCatalog(category string) error {
	slog.Warn("MCP deployment catalog unavailable", "category", category)
	return ErrMcpCatalogUnavailable
}

func openMcpTemplateDirectory(parent *os.Root, name string) (*os.Root, error) {
	// Go 1.26's Unix OpenRoot checks the type only after opening, which can
	// block if a directory was replaced by a FIFO after Lstat. Keep a literal
	// final dot: the kernel (or Root's O_DIRECTORY traversal) must open name as
	// a directory before reaching ".". filepath.Join would erase this guard;
	// a trailing separator alone still leaves Root's check/open race intact.
	name += string(os.PathSeparator) + "."
	if parent == nil {
		return os.OpenRoot(name)
	}
	return parent.OpenRoot(name)
}

func readMcpManifestFile(root *os.Root, key string, remaining *int) ([]byte, error) {
	before, err := root.Lstat(key)
	if err != nil || !before.IsDir() {
		return nil, errMcpManifestFile
	}
	entry, err := openMcpTemplateDirectory(root, key)
	if err != nil {
		return nil, errMcpManifestFile
	}
	defer entry.Close()
	current, err := entry.Stat(".")
	if err != nil || !os.SameFile(before, current) {
		return nil, errMcpManifestFile
	}
	after, err := root.Lstat(key)
	if err != nil || !after.IsDir() || !os.SameFile(before, after) {
		return nil, errMcpManifestFile
	}
	fileInfo, err := entry.Lstat("mcp.json")
	if err != nil || !fileInfo.Mode().IsRegular() {
		return nil, errMcpManifestFile
	}
	if fileInfo.Size() > mcpManifestMaxBytes {
		return nil, errMcpManifestFile
	}
	if fileInfo.Size() > int64(*remaining) {
		return nil, unavailableMcpCatalog("byte_limit")
	}
	file, err := entry.OpenFile("mcp.json", mcpManifestOpenFlags(), 0)
	if err != nil {
		return nil, errMcpManifestFile
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(fileInfo, opened) {
		return nil, errMcpManifestFile
	}
	limit := min(mcpManifestMaxBytes, *remaining)
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	*remaining -= len(data)
	if *remaining < 0 {
		return nil, unavailableMcpCatalog("byte_limit")
	}
	if err != nil || len(data) > mcpManifestMaxBytes {
		return nil, errMcpManifestFile
	}
	return data, nil
}

// Parse tokens before decoding the schema: encoding/json otherwise accepts
// duplicate keys and matches struct fields case-insensitively.
func parseMcpManifest(key string, data []byte, allowHTTP bool) (McpServerTemplate, error) {
	invalid := func() (McpServerTemplate, error) { return McpServerTemplate{}, errMcpManifestInvalid }
	if len(data) > mcpManifestMaxBytes || !utf8.Valid(data) {
		return invalid()
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := readMcpJSONValue(decoder, 0)
	if err != nil {
		return invalid()
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return invalid()
	}
	object, ok := value.(map[string]any)
	if !ok || !mcpObjectFields(object, "schema_version", "titles", "descriptions", "category", "documentation_url", "requirements", "config", "inputs") {
		return invalid()
	}
	config, ok := object["config"].(map[string]any)
	if !ok || !mcpObjectFields(config, "type", "command", "args", "url") {
		return invalid()
	}
	if inputs, exists := object["inputs"]; exists {
		items, ok := inputs.([]any)
		if !ok || len(items) > 64 {
			return invalid()
		}
		for _, item := range items {
			input, ok := item.(map[string]any)
			if !ok || !mcpObjectFields(input, "key", "labels", "descriptions", "required", "secret", "validator", "target") {
				return invalid()
			}
			target, ok := input["target"].(map[string]any)
			if !ok {
				return invalid()
			}
			switch target["kind"] {
			case "env":
				if !mcpObjectFields(target, "kind", "name") {
					return invalid()
				}
			case "arg":
				if !mcpObjectFields(target, "kind", "flag") {
					return invalid()
				}
			case "header":
				if !mcpObjectFields(target, "kind", "name", "prefix") {
					return invalid()
				}
			default:
				return invalid()
			}
		}
	}
	var manifest mcpManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return invalid()
	}
	if manifest.SchemaVersion != 1 || !normalizeMcpCopy(manifest.Titles, true) || !normalizeMcpCopy(manifest.Descriptions, false) {
		return invalid()
	}
	if manifest.Category == "" {
		manifest.Category = "other"
	}
	switch manifest.Category {
	case "browser", "reasoning", "documentation", "coding", "database", "other":
	default:
		return invalid()
	}
	if manifest.DocumentationURL != "" && !validMcpManifestURL(manifest.DocumentationURL, false, false) {
		return invalid()
	}
	if !normalizeMcpRequirements(manifest.Requirements) {
		return invalid()
	}
	if manifest.Config.Type == "" {
		manifest.Config.Type = "stdio"
	}
	var runtimeConfig map[string]any
	switch manifest.Config.Type {
	case "stdio":
		if _, exists := config["url"]; exists {
			return invalid()
		}
		if strings.TrimSpace(manifest.Config.Command) == "" || !mcpSingleLine(manifest.Config.Command, 8192) || len(manifest.Config.Args) > 128 {
			return invalid()
		}
		args := make([]any, 0, len(manifest.Config.Args))
		for _, arg := range manifest.Config.Args {
			if !mcpSingleLine(arg, 8192) {
				return invalid()
			}
			args = append(args, arg)
		}
		runtimeConfig = map[string]any{"command": manifest.Config.Command, "args": args}
	case "http":
		if _, exists := config["command"]; exists {
			return invalid()
		}
		if _, exists := config["args"]; exists {
			return invalid()
		}
		if !validMcpManifestURL(manifest.Config.URL, allowHTTP, true) {
			return invalid()
		}
		runtimeConfig = map[string]any{"type": "http", "url": manifest.Config.URL}
		if strings.HasPrefix(manifest.Config.URL, "http://") {
			if manifest.Requirements == nil {
				manifest.Requirements = map[string][]string{}
			}
			for language, requirement := range map[string]string{
				"en": "This endpoint uses an unencrypted HTTP connection",
				"zh": "此端点使用未加密的 HTTP 连接",
			} {
				if !slices.Contains(manifest.Requirements[language], requirement) {
					manifest.Requirements[language] = append(manifest.Requirements[language], requirement)
				}
			}
		}
	default:
		return invalid()
	}
	inputs := make([]McpTemplateInput, 0, len(manifest.Inputs))
	keys, targets := map[string]bool{}, map[string]bool{}
	for i := range manifest.Inputs {
		input := &manifest.Inputs[i]
		if !mcpInputName.MatchString(input.Key) || len(input.Key) > 128 || keys[input.Key] || !normalizeMcpCopy(input.Labels, false) || !normalizeMcpCopy(input.Descriptions, false) {
			return invalid()
		}
		keys[input.Key] = true
		if input.Validator == "" {
			input.Validator = "string"
		}
		switch input.Validator {
		case "string", "absolute_path", "database_url":
		default:
			return invalid()
		}
		if input.Secret == nil {
			secret := input.Target.Kind == "header"
			input.Secret = &secret
		}
		resolved := McpTemplateInput{Key: input.Key, Required: input.Required, Secret: *input.Secret, Labels: input.Labels, Descriptions: input.Descriptions, validator: input.Validator}
		target := input.Target.Kind + ":" + input.Target.Name
		switch input.Target.Kind {
		case "env":
			if manifest.Config.Type != "stdio" || !mcpEnvironmentName.MatchString(input.Target.Name) || len(input.Target.Name) > 128 {
				return invalid()
			}
			resolved.environment = input.Target.Name
		case "arg":
			if manifest.Config.Type != "stdio" || *input.Secret || (input.Target.Flag != "" && (!mcpArgumentFlag.MatchString(input.Target.Flag) || len(input.Target.Flag) > 128)) {
				return invalid()
			}
			resolved.positional = input.Target.Flag == ""
			resolved.argument = input.Target.Flag
			target += input.Target.Flag
		case "header":
			if manifest.Config.Type != "http" || !mcpHeaderName.MatchString(input.Target.Name) || len(input.Target.Name) > 128 || !mcpSingleLine(input.Target.Prefix, 128) {
				return invalid()
			}
			resolved.header, resolved.prefix = input.Target.Name, input.Target.Prefix
			target = "header:" + strings.ToLower(input.Target.Name)
		default:
			return invalid()
		}
		if targets[target] {
			return invalid()
		}
		targets[target] = true
		inputs = append(inputs, resolved)
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return invalid()
	}
	digest := sha256.Sum256(append([]byte(mcpManifestVersionDomain), canonical...))
	return McpServerTemplate{Key: key, Source: "deployment", Transport: manifest.Config.Type, Version: "sha256:" + hex.EncodeToString(digest[:]), Titles: manifest.Titles, Descriptions: manifest.Descriptions, Category: manifest.Category, DocumentationURL: manifest.DocumentationURL, Requirements: manifest.Requirements, Inputs: inputs, Config: runtimeConfig}, nil
}

func readMcpJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > mcpManifestMaxDepth {
		return nil, errMcpManifestInvalid
	}
	token, err := decoder.Token()
	if err != nil || token == nil {
		return nil, errMcpManifestInvalid
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, errMcpManifestInvalid
			}
			name, ok := key.(string)
			if !ok {
				return nil, errMcpManifestInvalid
			}
			if _, exists := object[name]; exists {
				return nil, errMcpManifestInvalid
			}
			value, err := readMcpJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errMcpManifestInvalid
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := readMcpJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, errMcpManifestInvalid
		}
		return array, nil
	default:
		return nil, errMcpManifestInvalid
	}
}

func mcpObjectFields(object map[string]any, fields ...string) bool {
	for key := range object {
		found := false
		for _, field := range fields {
			if key == field {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func normalizeMcpCopy(copy map[string]string, required bool) bool {
	for language, value := range copy {
		if (language != "en" && language != "zh") || !mcpSingleLine(value, 8192) {
			return false
		}
		copy[language] = strings.TrimSpace(value)
		if copy[language] == "" {
			delete(copy, language)
		}
	}
	if len(copy) == 0 {
		return !required
	}
	if copy["en"] == "" {
		copy["en"] = copy["zh"]
	}
	if copy["zh"] == "" {
		copy["zh"] = copy["en"]
	}
	return true
}

func normalizeMcpRequirements(requirements map[string][]string) bool {
	for language, items := range requirements {
		if (language != "en" && language != "zh") || len(items) > 32 {
			return false
		}
		for i, value := range items {
			if strings.TrimSpace(value) == "" || !mcpSingleLine(value, 8192) {
				return false
			}
			items[i] = strings.TrimSpace(value)
		}
		if len(items) == 0 {
			delete(requirements, language)
		}
	}
	if len(requirements) == 0 {
		return true
	}
	if len(requirements["en"]) == 0 {
		requirements["en"] = append([]string(nil), requirements["zh"]...)
	}
	if len(requirements["zh"]) == 0 {
		requirements["zh"] = append([]string(nil), requirements["en"]...)
	}
	return true
}

func mcpSingleLine(value string, maxBytes int) bool {
	return len(value) <= maxBytes && !strings.ContainsAny(value, "\x00\r\n")
}

func validMcpManifestURL(value string, allowHTTP, endpoint bool) bool {
	parsed, err := url.Parse(value)
	if err != nil || !mcpSingleLine(value, 8192) || parsed.Opaque != "" || parsed.Hostname() == "" || parsed.User != nil || strings.Contains(value, "#") {
		return false
	}
	if parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http") {
		return false
	}
	return !endpoint || (parsed.RawQuery == "" && !parsed.ForceQuery)
}
