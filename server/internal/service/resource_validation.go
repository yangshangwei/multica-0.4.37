package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/skill"
	"gopkg.in/yaml.v3"
)

type resourceContentFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type resourceBundle struct {
	Files                             []resourceContentFile `json:"files"`
	Name, Description, Category, Icon string                `json:"-"`
	Digest                            string                `json:"-"`
	Bytes                             int64                 `json:"-"`
}

var (
	resourcePortableComponent = regexp.MustCompile(`^[A-Za-z0-9_. -]+$`)
	resourceReservedDevice    = regexp.MustCompile(`^(COM|LPT)[1-9]$`)
)

func validResourceKey(key string) bool {
	return len(key) > 0 && len(key) <= 128 && mcpTemplateName.MatchString(key) && portableResourcePath(key)
}
func portableResourcePath(name string) bool {
	if len(name) == 0 || len(name) > 1024 || !utf8.ValidString(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || len(part) > 255 || !resourcePortableComponent.MatchString(part) || strings.TrimRight(part, " .") != part {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || resourceReservedDevice.MatchString(base) {
			return false
		}
	}
	return true
}

func validateResource(kind, key, filename string, data []byte, allowHTTP bool) (resourceBundle, error) {
	invalid := func() (resourceBundle, error) { return resourceBundle{}, resourceError("resource_invalid") }
	if !validResourceKey(key) || len(data) == 0 || len(data) > 16<<20 {
		return invalid()
	}
	bundle := resourceBundle{}
	switch kind {
	case "mcp":
		if path.Ext(filename) != ".json" || len(data) > mcpManifestMaxBytes || bytes.IndexByte(data, 0) >= 0 {
			return invalid()
		}
		template, err := parseMcpManifest(key, data, allowHTTP)
		if err != nil {
			return invalid()
		}
		bundle.Files = []resourceContentFile{{"mcp.json", string(data)}}
		bundle.Name = template.Titles["en"]
		bundle.Description = template.Descriptions["en"]
	case "skill":
		if filename == "SKILL.md" {
			bundle.Files = []resourceContentFile{{"SKILL.md", string(data)}}
		} else {
			ext := strings.ToLower(path.Ext(filename))
			if ext != ".zip" && ext != ".skill" {
				return invalid()
			}
			files, err := readResourceArchive(data)
			if err != nil {
				return invalid()
			}
			bundle.Files = files
		}
		var primary string
		for _, f := range bundle.Files {
			if f.Path == "SKILL.md" {
				primary = f.Content
				break
			}
		}
		meta, err := strictResourceFrontmatter(primary)
		if err != nil {
			return invalid()
		}
		bundle.Name = meta.Name
		bundle.Description = meta.Description
		bundle.Category = meta.Category
		bundle.Icon = meta.Icon
	default:
		return invalid()
	}
	supporting := int64(0)
	for _, f := range bundle.Files {
		size := int64(len(f.Content))
		if size > 1<<20 || !utf8.ValidString(f.Content) || strings.IndexByte(f.Content, 0) >= 0 {
			return invalid()
		}
		bundle.Bytes += size
		if f.Path != "SKILL.md" {
			supporting += size
		}
	}
	if len(bundle.Files) > 257 || supporting > 8<<20 || bundle.Bytes > 9<<20 {
		return invalid()
	}
	canonical, _ := json.Marshal(bundle.Files)
	sum := sha256.Sum256(append([]byte("multica-resource-content:v1\n"+kind+"\n"), canonical...))
	bundle.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return bundle, nil
}

func readResourceArchive(data []byte) ([]resourceContentFile, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(z.File) > 512 {
		return nil, resourceError("resource_invalid")
	}
	// Validate the entire archive before stripping its optional single wrapper.
	aliases := map[string]string{}
	filesByPath := map[string]bool{}
	regular := map[string]bool{}
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		mode := f.Mode()
		if !portableResourcePath(name) || (!mode.IsRegular() && !mode.IsDir()) || mode.IsDir() != strings.HasSuffix(f.Name, "/") {
			return nil, resourceError("resource_invalid")
		}
		if filesByPath[name] {
			return nil, resourceError("resource_invalid")
		}
		filesByPath[name] = true
		parts := strings.Split(name, "/")
		for i := range parts {
			p := strings.Join(parts[:i+1], "/")
			lower := strings.ToLower(p)
			if prior, ok := aliases[lower]; ok && prior != p {
				return nil, resourceError("resource_invalid")
			}
			aliases[lower] = p
		}
		if mode.IsRegular() {
			regular[name] = true
		}
	}
	for name := range filesByPath {
		for p := path.Dir(name); p != "."; p = path.Dir(p) {
			if regular[p] {
				return nil, resourceError("resource_invalid")
			}
		}
	}
	wrapper := ""
	if !regular["SKILL.md"] {
		for name := range regular {
			parts := strings.Split(name, "/")
			if len(parts) == 2 && parts[1] == "SKILL.md" {
				if wrapper != "" {
					return nil, resourceError("resource_invalid")
				}
				wrapper = parts[0] + "/"
			}
		}
		if wrapper == "" {
			return nil, resourceError("resource_invalid")
		}
	}
	result := []resourceContentFile{}
	total := int64(0)
	supporting := int64(0)
	for _, f := range z.File {
		name := f.Name
		if wrapper != "" {
			if name == wrapper && f.Mode().IsDir() {
				continue
			}
			if !strings.HasPrefix(name, wrapper) {
				return nil, resourceError("resource_invalid")
			}
			name = strings.TrimPrefix(name, wrapper)
		}
		if f.Mode().IsDir() {
			continue
		}
		if strings.EqualFold(name, "SKILL.md") && name != "SKILL.md" {
			return nil, resourceError("resource_invalid")
		}
		if len(result) >= 257 || f.UncompressedSize64 > 1<<20 {
			return nil, resourceError("resource_invalid")
		}
		r, err := f.Open()
		if err != nil {
			return nil, resourceError("resource_invalid")
		}
		content, readErr := io.ReadAll(io.LimitReader(r, (1<<20)+1))
		closeErr := r.Close()
		if readErr != nil || closeErr != nil || len(content) > 1<<20 {
			return nil, resourceError("resource_invalid")
		}
		total += int64(len(content))
		if name != "SKILL.md" {
			supporting += int64(len(content))
		}
		if total > 9<<20 || supporting > 8<<20 {
			return nil, resourceError("resource_invalid")
		}
		result = append(result, resourceContentFile{name, string(content)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

func strictResourceFrontmatter(content string) (skill.Frontmatter, error) {
	invalid := func() (skill.Frontmatter, error) { return skill.Frontmatter{}, resourceError("resource_invalid") }
	if len(content) > 1<<20 || !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return invalid()
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return invalid()
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return invalid()
	}
	header := normalized[4 : 4+end]
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(header), &node); err != nil || len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return invalid()
	}
	var check func(*yaml.Node, int) bool
	check = func(n *yaml.Node, depth int) bool {
		if depth > 16 || n.Kind == yaml.AliasNode || n.Anchor != "" {
			return false
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || seen[k.Value] {
					return false
				}
				seen[k.Value] = true
			}
		}
		for _, child := range n.Content {
			if !check(child, depth+1) {
				return false
			}
		}
		return true
	}
	if !check(&node, 0) {
		return invalid()
	}
	var fields map[string]any
	if err := node.Decode(&fields); err != nil {
		return invalid()
	}
	name, nameOK := fields["name"].(string)
	description, descriptionOK := fields["description"].(string)
	if !nameOK || !validResourceKey(strings.TrimSpace(name)) || !descriptionOK || strings.TrimSpace(description) == "" || len(description) > 8192 || strings.ContainsRune(description, 0) {
		return invalid()
	}
	meta := skill.ParseSkillFrontmatterMeta(content)
	if meta.Name == "" {
		return invalid()
	}
	return meta, nil
}
