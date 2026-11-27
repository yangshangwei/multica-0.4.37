package materialrepo

import (
	"bytes"
	"io"
	"reflect"

	"gopkg.in/yaml.v3"
)

type Layout struct {
	Modules      string `yaml:"modules"`
	Requirements string `yaml:"requirements"`
	Issues       string `yaml:"issues"`
	Releases     string `yaml:"releases"`
	Contracts    string `yaml:"contracts"`
	Specs        string `yaml:"specs"`
	Decisions    string `yaml:"decisions"`
	Knowledge    string `yaml:"knowledge"`
}
type Manifest struct {
	SchemaVersion int    `yaml:"schema_version"`
	BundleVersion string `yaml:"bundle_version"`
	SystemID      string `yaml:"system_id"`
	Layout        Layout `yaml:"layout"`
}

func expectedLayout() Layout {
	return Layout{"modules", "requirements", "issues", "releases", "contracts", "specs", "decisions", "knowledge"}
}

// ParseManifest accepts formatting differences, not unknown fields, YAML aliases,
// duplicate keys, alternate layouts or unrecognized format/bundle versions.
func ParseManifest(body string) (Manifest, error) {
	var m Manifest
	if !textValid(body) || len(body) > 16384 {
		return m, invalid("invalid_manifest", "")
	}
	dec := yaml.NewDecoder(bytes.NewBufferString(body))
	var node yaml.Node
	if dec.Decode(&node) != nil || len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return m, invalid("invalid_manifest", "")
	}
	var trailing yaml.Node
	if dec.Decode(&trailing) != io.EOF {
		return m, invalid("invalid_manifest", "")
	}
	count := 0
	if !safeYAML(&node, 0, &count) || !manifestNodeTypes(node.Content[0]) {
		return m, invalid("invalid_manifest", "")
	}
	typed := yaml.NewDecoder(bytes.NewBufferString(body))
	typed.KnownFields(true)
	if typed.Decode(&m) != nil {
		return Manifest{}, invalid("invalid_manifest", "")
	}
	if m.SchemaVersion != SchemaVersion || (m.BundleVersion != "1.0.0" && m.BundleVersion != "1.0.1") {
		return Manifest{}, invalid("unsupported_format", "")
	}
	if !idPattern.MatchString(m.SystemID) || !reflect.DeepEqual(m.Layout, expectedLayout()) {
		return Manifest{}, invalid("manifest_conflict", "")
	}
	return m, nil
}
func safeYAML(n *yaml.Node, depth int, count *int) bool {
	*count = *count + 1
	if depth > 8 || *count > 128 || n.Kind == yaml.AliasNode || n.Anchor != "" {
		return false
	}
	switch n.Kind {
	case yaml.DocumentNode:
	case yaml.MappingNode:
		if n.Tag != "!!map" {
			return false
		}
	case yaml.SequenceNode:
		if n.Tag != "!!seq" {
			return false
		}
	case yaml.ScalarNode:
		if n.Tag != "!!str" && n.Tag != "!!int" {
			return false
		}
	default:
		return false
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		if len(n.Content)%2 != 0 {
			return false
		}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || seen[k.Value] {
				return false
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if !safeYAML(c, depth+1, count) {
			return false
		}
	}
	return true
}

func manifestNodeTypes(root *yaml.Node) bool {
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i].Value, root.Content[i+1]
		switch key {
		case "schema_version":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!int" {
				return false
			}
		case "system_id", "bundle_version":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return false
			}
		case "layout":
			if value.Kind != yaml.MappingNode {
				return false
			}
			for j := 1; j < len(value.Content); j += 2 {
				if value.Content[j].Kind != yaml.ScalarNode || value.Content[j].Tag != "!!str" {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}
