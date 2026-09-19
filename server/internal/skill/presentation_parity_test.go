package skill

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

// The TS module is the source of truth for the category enum and icon
// whitelist. This test reads its literals so the two lists cannot drift apart
// silently: a category the picker offers must be one the server accepts.
const presentationTSPath = "../../../packages/core/skills/presentation.ts"

func readPresentationTS(t *testing.T) string {
	t.Helper()
	path := filepath.FromSlash(presentationTSPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("skipping parity check: %s not found (server-only checkout?)", presentationTSPath)
		}
		t.Fatalf("read %s: %v", presentationTSPath, err)
	}
	return string(data)
}

var (
	tsQuotedString = regexp.MustCompile(`"([^"]+)"`)
	tsMapEntry     = regexp.MustCompile(`(\w+):\s*"([^"]+)"`)
)

// tsStringArray extracts the quoted entries of `export const NAME = [ ... ] as const;`.
func tsStringArray(t *testing.T, src, name string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?s)export const ` + name + `\s*=\s*\[(.*?)\]\s*as const;`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s literal not found in %s", name, presentationTSPath)
	}
	var out []string
	for _, q := range tsQuotedString.FindAllStringSubmatch(m[1], -1) {
		out = append(out, q[1])
	}
	return out
}

func tsStringMap(t *testing.T, src, name string) map[string]string {
	t.Helper()
	re := regexp.MustCompile(`(?s)export const ` + name + `[^{]*\{(.*?)\};`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s literal not found in %s", name, presentationTSPath)
	}
	out := map[string]string{}
	for _, e := range tsMapEntry.FindAllStringSubmatch(m[1], -1) {
		out[e[1]] = e[2]
	}
	return out
}

func TestPresentationParityWithTypeScript(t *testing.T) {
	src := readPresentationTS(t)

	if got := tsStringArray(t, src, "SKILL_CATEGORIES"); !reflect.DeepEqual(got, Categories) {
		t.Errorf("SKILL_CATEGORIES = %v, Go Categories = %v", got, Categories)
	}
	if got := tsStringArray(t, src, "SKILL_ICON_NAMES"); !reflect.DeepEqual(got, IconNames) {
		t.Errorf("SKILL_ICON_NAMES = %v, Go IconNames = %v", got, IconNames)
	}
	if got := tsStringMap(t, src, "SKILL_CATEGORY_DEFAULT_ICON"); !reflect.DeepEqual(got, CategoryDefaultIcon) {
		t.Errorf("SKILL_CATEGORY_DEFAULT_ICON = %v, Go CategoryDefaultIcon = %v", got, CategoryDefaultIcon)
	}
	re := regexp.MustCompile(`export const DEFAULT_SKILL_CATEGORY: SkillCategory = "([^"]+)";`)
	if m := re.FindStringSubmatch(src); m == nil || m[1] != DefaultCategory {
		t.Errorf("DEFAULT_SKILL_CATEGORY = %v, Go DefaultCategory = %q", m, DefaultCategory)
	}
}
