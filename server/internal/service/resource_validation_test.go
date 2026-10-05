package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
)

func resourceSkillText() []byte {
	return []byte("---\nname: example\ndescription: Example skill\n---\nDo useful work.\n")
}

func resourceZip(t *testing.T, names []string, contents [][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for i, name := range names {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		w, err := z.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(contents[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestResourceValidationSkillArchives(t *testing.T) {
	for _, prefix := range []string{"", "wrapper/"} {
		t.Run(prefix, func(t *testing.T) {
			data := resourceZip(t, []string{prefix + "SKILL.md", prefix + "references/info.md"}, [][]byte{resourceSkillText(), []byte("info")})
			got, err := validateResource("skill", "example", "example.skill", data, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Files) != 2 || got.Files[0].Path != "SKILL.md" || got.Name != "example" {
				t.Fatalf("unexpected bundle: %+v", got)
			}
		})
	}
	for _, names := range [][]string{{"SKILL.md", "../escape"}, {"SKILL.md", "skill.md"}, {"SKILL.md", "a", "a/b"}, {"wrapper/SKILL.md", "other.txt"}, {"SKILL.md", "CON.txt"}, {"SKILL.md", "A/x", "a/y"}, {"SKILL.md", "a\\b"}, {"SKILL.md", "SKILL.md"}} {
		t.Run(names[len(names)-1], func(t *testing.T) {
			contents := make([][]byte, len(names))
			for i := range contents {
				contents[i] = resourceSkillText()
			}
			_, err := validateResource("skill", "example", "x.zip", resourceZip(t, names, contents), false)
			if err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestResourceValidationRejectsMalformedContent(t *testing.T) {
	for _, data := range [][]byte{[]byte("no header"), []byte("---\nname: []\n---\nbody"), []byte("---\nname: example\nname: second\n---\nbody"), append(resourceSkillText(), 0), append(resourceSkillText(), 0xff), bytes.Repeat([]byte("x"), (1<<20)+1)} {
		if _, err := validateResource("skill", "example", "SKILL.md", data, false); err == nil {
			t.Fatal("invalid skill accepted")
		}
	}
	_, err := validateResource("mcp", "example", "mcp.json", []byte(`{"schema_version":1,"titles":{"en":"Example"},"config":{"command":"node"}}`), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateResource("mcp", "example", "mcp.json", []byte(`{"schema_version":1,"schema_version":1}`), false); err == nil {
		t.Fatal("duplicate MCP keys accepted")
	}
}

func TestResourceValidationRejectsSymlinks(t *testing.T) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	h := &zip.FileHeader{Name: "SKILL.md"}
	h.SetMode(os.ModeSymlink | 0600)
	w, _ := z.CreateHeader(h)
	_, _ = w.Write(resourceSkillText())
	_ = z.Close()
	if _, err := validateResource("skill", "example", "x.zip", out.Bytes(), false); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestResourceValidationArchiveLimits(t *testing.T) {
	tests := []struct {
		name      string
		names     []string
		contents  [][]byte
		wantValid bool
	}{
		{name: "per-file boundary", names: []string{"SKILL.md", "reference.txt"}, contents: [][]byte{resourceSkillText(), bytes.Repeat([]byte("x"), 1<<20)}, wantValid: true},
		{name: "per-file overflow", names: []string{"SKILL.md", "reference.txt"}, contents: [][]byte{resourceSkillText(), bytes.Repeat([]byte("x"), (1<<20)+1)}},
	}
	for _, count := range []int{256, 257} {
		test := struct {
			name      string
			names     []string
			contents  [][]byte
			wantValid bool
		}{name: fmt.Sprintf("supporting-files-%d", count), names: []string{"SKILL.md"}, contents: [][]byte{resourceSkillText()}, wantValid: count == 256}
		for i := range count {
			test.names = append(test.names, fmt.Sprintf("references/%03d.txt", i))
			test.contents = append(test.contents, []byte("x"))
		}
		tests = append(tests, test)
	}
	for _, extra := range []int{0, 1} {
		test := struct {
			name      string
			names     []string
			contents  [][]byte
			wantValid bool
		}{name: fmt.Sprintf("supporting-byte-limit-extra-%d", extra), names: []string{"SKILL.md"}, contents: [][]byte{resourceSkillText()}, wantValid: extra == 0}
		for i := range 8 {
			test.names = append(test.names, fmt.Sprintf("references/%d.txt", i))
			test.contents = append(test.contents, bytes.Repeat([]byte("x"), 1<<20))
		}
		if extra > 0 {
			test.names = append(test.names, "extra.txt")
			test.contents = append(test.contents, []byte("x"))
		}
		tests = append(tests, test)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateResource("skill", "example", "test.zip", resourceZip(t, test.names, test.contents), false)
			if (err == nil) != test.wantValid {
				t.Fatalf("valid=%t error=%v", test.wantValid, err)
			}
		})
	}
}

func TestResourceValidationArchiveEntryLimitIncludesDirectories(t *testing.T) {
	for _, count := range []int{511, 512} {
		var out bytes.Buffer
		z := zip.NewWriter(&out)
		w, err := z.Create("SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(resourceSkillText()); err != nil {
			t.Fatal(err)
		}
		for i := range count {
			h := &zip.FileHeader{Name: fmt.Sprintf("directory-%d/", i)}
			h.SetMode(os.ModeDir | 0700)
			if _, err = z.CreateHeader(h); err != nil {
				t.Fatal(err)
			}
		}
		if err = z.Close(); err != nil {
			t.Fatal(err)
		}
		_, err = validateResource("skill", "example", "test.zip", out.Bytes(), false)
		if (err == nil) != (count == 511) {
			t.Fatalf("directory count %d: %v", count, err)
		}
	}
}

func resourceErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	var e *ResourceError
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("error=%v, want %s", err, want)
	}
}
func resourceTestGuard(_ context.Context, apply func() (ResourceMutationResult, error)) (ResourceMutationResult, error) {
	return apply()
}
