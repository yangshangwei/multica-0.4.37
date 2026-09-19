package skill

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidatePresentation(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr string // substring; empty means no error
	}{
		{name: "nil config", config: nil},
		{name: "no presentation key", config: map[string]any{"origin": map[string]any{"type": "github"}}},
		{name: "nil presentation", config: map[string]any{"presentation": nil}},
		{name: "empty object", config: map[string]any{"presentation": map[string]any{}}},
		{name: "valid full", config: map[string]any{"presentation": map[string]any{
			"category": "engineering", "icon": "shield-check",
		}}},
		{name: "legacy tags key is not validated", config: map[string]any{"presentation": map[string]any{
			"category": "engineering", "tags": "anything",
		}}},
		{name: "presentation not object", config: map[string]any{"presentation": "engineering"},
			wantErr: "config.presentation must be an object"},
		{name: "presentation array", config: map[string]any{"presentation": []any{}},
			wantErr: "config.presentation must be an object"},
		{name: "category wrong type", config: map[string]any{"presentation": map[string]any{"category": 1}},
			wantErr: "config.presentation.category must be a string"},
		{name: "category unknown", config: map[string]any{"presentation": map[string]any{"category": "x"}},
			wantErr: `config.presentation.category: unknown value "x"`},
		{name: "icon wrong type", config: map[string]any{"presentation": map[string]any{"icon": true}},
			wantErr: "config.presentation.icon must be a string"},
		{name: "icon unknown", config: map[string]any{"presentation": map[string]any{"icon": "PenLine"}},
			wantErr: `config.presentation.icon: unknown value "PenLine"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePresentation(tt.config)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizePresentation(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   map[string]any
	}{
		{name: "nil", config: nil, want: nil},
		{name: "no presentation", config: map[string]any{"origin": "x"}, want: map[string]any{"origin": "x"}},
		{name: "empty presentation dropped", config: map[string]any{"origin": "x", "presentation": map[string]any{}},
			want: map[string]any{"origin": "x"}},
		{name: "non-object presentation dropped", config: map[string]any{"presentation": "junk"},
			want: map[string]any{}},
		{name: "default category alone dropped", config: map[string]any{"presentation": map[string]any{"category": "other"}},
			want: map[string]any{}},
		{name: "category kept", config: map[string]any{"presentation": map[string]any{"category": "data"}},
			want: map[string]any{"presentation": map[string]any{"category": "data"}}},
		{name: "unknown category dropped", config: map[string]any{"presentation": map[string]any{"category": "nope", "icon": "bell"}},
			want: map[string]any{"presentation": map[string]any{"category": "other", "icon": "bell"}}},
		{name: "icon equal to category default dropped",
			config: map[string]any{"presentation": map[string]any{"category": "engineering", "icon": "code"}},
			want:   map[string]any{"presentation": map[string]any{"category": "engineering"}}},
		{name: "icon equal to other default dropped entirely",
			config: map[string]any{"presentation": map[string]any{"icon": "book-open-text"}},
			want:   map[string]any{}},
		{name: "icon override kept and category written for default bucket",
			config: map[string]any{"presentation": map[string]any{"icon": "bell"}},
			want:   map[string]any{"presentation": map[string]any{"category": "other", "icon": "bell"}}},
		{name: "unknown icon dropped", config: map[string]any{"presentation": map[string]any{"category": "data", "icon": "nope"}},
			want: map[string]any{"presentation": map[string]any{"category": "data"}}},
		{name: "legacy tags key dropped", config: map[string]any{"presentation": map[string]any{
			"category": "writing", "tags": []any{"docs"},
		}}, want: map[string]any{"presentation": map[string]any{"category": "writing"}}},
		{name: "legacy tags alone drop presentation", config: map[string]any{"presentation": map[string]any{"tags": []any{"docs"}}},
			want: map[string]any{}},
		{name: "siblings preserved", config: map[string]any{
			"origin":       map[string]any{"type": "github"},
			"presentation": map[string]any{"category": "writing", "icon": "book-open"},
		}, want: map[string]any{
			"origin":       map[string]any{"type": "github"},
			"presentation": map[string]any{"category": "writing", "icon": "book-open"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePresentation(tt.config)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizePresentation() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizePresentation_DoesNotMutateInput(t *testing.T) {
	inner := map[string]any{"category": "engineering", "icon": "code", "tags": []any{"legacy"}}
	config := map[string]any{"origin": "o", "presentation": inner}
	_ = NormalizePresentation(config)
	if inner["icon"] != "code" || len(inner["tags"].([]any)) != 1 {
		t.Fatalf("input presentation mutated: %#v", inner)
	}
	if _, ok := config["presentation"]; !ok {
		t.Fatal("input config lost its presentation key")
	}
}

func TestPresentationFromFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		fm   Frontmatter
		want map[string]any
	}{
		{name: "empty", fm: Frontmatter{}, want: nil},
		{name: "all invalid ignored", fm: Frontmatter{Category: "nope", Icon: "Nope"}, want: nil},
		{name: "valid", fm: Frontmatter{Category: "operations", Icon: "bell"},
			want: map[string]any{"category": "operations", "icon": "bell"}},
		{name: "invalid icon with valid category", fm: Frontmatter{Category: "data", Icon: "junk"},
			want: map[string]any{"category": "data"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PresentationFromFrontmatter(tt.fm)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCategoryDefaultIconsAreWhitelisted(t *testing.T) {
	for _, category := range Categories {
		icon, ok := CategoryDefaultIcon[category]
		if !ok {
			t.Errorf("category %q has no default icon", category)
			continue
		}
		if !IsIconName(icon) {
			t.Errorf("category %q default icon %q is not on the whitelist", category, icon)
		}
	}
	if len(CategoryDefaultIcon) != len(Categories) {
		t.Errorf("CategoryDefaultIcon has %d entries, want %d", len(CategoryDefaultIcon), len(Categories))
	}
}
