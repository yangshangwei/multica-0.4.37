//go:build linux || darwin

package artifact

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const executionOne = "99999999-9999-4999-8999-999999999991"
const executionTwo = "99999999-9999-4999-8999-999999999992"

type fixture struct {
	p                         *Packager
	r                         Request
	b                         Binding
	spec                      Spec
	stage, output, generation string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{r: testRequest(t), b: testBinding(), stage: filepath.Join(base, "staging"), output: filepath.Join(base, "output")}
	f.generation = filepath.Join(f.stage, f.r.StagingID)
	if err := os.MkdirAll(filepath.Join(f.generation, "files/nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.output, 0700); err != nil {
		t.Fatal(err)
	}
	content := map[string][]byte{"z.txt": []byte("zebra\n"), "nested/a.txt": []byte("alpha\n")}
	f.r.Entries = nil
	for _, name := range []string{"z.txt", "nested/a.txt"} {
		data := content[name]
		sum := sha256.Sum256(data)
		f.r.Entries = append(f.r.Entries, Entry{Path: name, SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])})
		if err := os.WriteFile(filepath.Join(f.generation, "files", name), data, 0400); err != nil {
			t.Fatal(err)
		}
	}
	f.r.InputManifestDigest, err = InputDigest(f.r.Entries)
	if err != nil {
		t.Fatal(err)
	}
	manifest := StagingManifest{WorkspaceID: f.b.WorkspaceID, AgentID: f.r.AgentID, TaskID: f.r.TaskID, TaskGeneration: f.b.TaskGeneration, ResourceID: f.r.ResourceID, SourceRevision: f.r.SourceRevision, InputManifestDigest: f.r.InputManifestDigest}
	f.r.StagingManifestDigest, err = StagingDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	data, err := canonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.generation, "manifest.json"), data, 0400); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{f.stage, f.generation, filepath.Join(f.generation, "files"), filepath.Join(f.generation, "files/nested")} {
		if err := os.Chmod(dir, 0500); err != nil {
			t.Fatal(err)
		}
	}
	// Test cleanup may remove deliberately sealed operator directories.
	t.Cleanup(func() {
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(path, 0700)
			}
			return nil
		})
	})
	f.p, err = New(Config{StagingRoot: f.stage, OutputRoot: f.output, Identity: "test-operator-generation-1"})
	if err != nil {
		t.Fatal(err)
	}
	f.spec, err = f.p.Prepare(context.Background(), f.r, f.b, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestArtifactPackageDeterministic(t *testing.T) {
	f := newFixture(t)
	first := f.p.Pack(context.Background(), executionOne, f.spec)
	if first.Status != "succeeded" {
		t.Fatal(first)
	}
	reordered := f.r
	reordered.Entries = []Entry{f.r.Entries[1], f.r.Entries[0]}
	secondSpec, err := f.p.Prepare(context.Background(), reordered, f.b, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second := f.p.Pack(context.Background(), executionTwo, secondSpec)
	if second.Status != "succeeded" || first.OutputDigest != second.OutputDigest {
		t.Fatal(first, second)
	}
	one, err := os.ReadFile(filepath.Join(f.output, executionOne, "artifact.tar"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := os.ReadFile(filepath.Join(f.output, executionTwo, "artifact.tar"))
	if err != nil || !bytes.Equal(one, two) {
		t.Fatal("not byte deterministic", err)
	}
	sum := sha256.Sum256(one)
	if first.OutputDigest != hex.EncodeToString(sum[:]) || first.OutputSize != int64(len(one)) {
		t.Fatal("wrong output digest/size")
	}
	reader := tar.NewReader(bytes.NewReader(one))
	var names []string
	for {
		h, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
		if h.Format != tar.FormatUSTAR || h.Mode != 0644 || h.Uid != 0 || h.Gid != 0 || h.Uname != "" || h.Gname != "" || !h.ModTime.Equal(time.Unix(0, 0)) || len(h.PAXRecords) != 0 || h.Typeflag != tar.TypeReg {
			t.Fatal("header policy", h)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if h.Name == "z.txt" && string(data) != "zebra\n" {
			t.Fatal("wrong archive bytes")
		}
	}
	if strings.Join(names, ",") != "nested/a.txt,z.txt" {
		t.Fatal(names)
	}
	replay := f.p.Pack(context.Background(), executionOne, f.spec)
	if replay.Status != "failed" || replay.Code != "output_collision" {
		t.Fatal(replay)
	}
	after, err := os.ReadFile(filepath.Join(f.output, executionOne, "artifact.tar"))
	if err != nil || !bytes.Equal(one, after) {
		t.Fatal("collision overwritten", err)
	}
}

func TestArtifactPackageInputBoundaries(t *testing.T) {
	for _, kind := range []string{"leaf_symlink", "parent_symlink", "hardlink", "fifo", "input_changed", "oversize", "writable_file", "writable_directory", "manifest_changed", "manifest_unknown", "manifest_missing_generation"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			files := filepath.Join(f.generation, "files")
			leaf := filepath.Join(files, "z.txt")
			if err := os.Chmod(files, 0700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "leaf_symlink":
				if err := os.Remove(leaf); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("nested/a.txt", leaf); err != nil {
					t.Fatal(err)
				}
			case "parent_symlink":
				nested := filepath.Join(files, "nested")
				if err := os.Chmod(nested, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(nested, nested+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("nested-original", nested); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(leaf, filepath.Join(files, "extra")); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := os.Remove(leaf); err != nil {
					t.Fatal(err)
				}
				if err := unix.Mkfifo(leaf, 0400); err != nil {
					t.Fatal(err)
				}
			case "input_changed", "oversize":
				if err := os.Chmod(leaf, 0600); err != nil {
					t.Fatal(err)
				}
				data := []byte("tamper")
				if kind == "oversize" {
					data = bytes.Repeat([]byte("x"), 100)
				}
				if err := os.WriteFile(leaf, data, 0400); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(leaf, 0400); err != nil {
					t.Fatal(err)
				}
			case "writable_file":
				if err := os.Chmod(leaf, 0600); err != nil {
					t.Fatal(err)
				}
			case "writable_directory":
			case "manifest_changed", "manifest_unknown", "manifest_missing_generation":
				path := filepath.Join(f.generation, "manifest.json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "manifest_changed" {
					data = bytes.Replace(data, []byte(f.b.WorkspaceID), []byte(f.b.InitiatorID), 1)
				}
				if kind == "manifest_unknown" {
					data = bytes.Replace(data, []byte(`{"workspace_id"`), []byte(`{"unknown":true,"workspace_id"`), 1)
				}
				if kind == "manifest_missing_generation" {
					data = bytes.Replace(data, []byte(`"task_generation":1,`), nil, 1)
				}
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0400); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0400); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "writable_directory" {
				if err := os.Chmod(files, 0500); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.p.ValidateStaging(context.Background(), f.r, f.b); !errors.Is(err, ErrInvalid) {
				t.Fatal("validation accepted", err)
			}
			outcome := f.p.Pack(context.Background(), executionOne, f.spec)
			if outcome.Status != "failed" {
				t.Fatal(outcome)
			}
			entries, err := os.ReadDir(f.output)
			if err != nil || len(entries) != 0 {
				t.Fatal("refusal created output", err)
			}
		})
	}
}

func TestArtifactPackageMutationDuringCapture(t *testing.T) {
	f := newFixture(t)
	f.p.fault = func(step string) error {
		if strings.HasSuffix(step, "/z.txt") {
			path := filepath.Join(f.generation, "files/z.txt")
			if err := os.Chmod(path, 0600); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte("mutate"), 0400); err != nil {
				return err
			}
			return os.Chmod(path, 0400)
		}
		return nil
	}
	result := f.p.Pack(context.Background(), executionOne, f.spec)
	if result.Status != "failed" || result.Code != "input_invalid" {
		t.Fatal(result)
	}
}

func TestArtifactPackageRootAndOutputBoundary(t *testing.T) {
	for _, kind := range []string{"stage_replaced", "output_replaced", "output_symlink", "output_collision", "stage_symlink", "nested_output", "missing_config"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			switch kind {
			case "stage_replaced", "output_replaced":
				path, mode := f.stage, os.FileMode(0500)
				if kind == "output_replaced" {
					path = f.output
					mode = 0700
				}
				if err := os.Chmod(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path, path+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, mode); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path+"-old", 0700) })
			case "output_symlink":
				if err := os.Symlink(f.stage, filepath.Join(f.output, executionOne)); err != nil {
					t.Fatal(err)
				}
			case "output_collision":
				if err := os.Mkdir(filepath.Join(f.output, executionOne), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(f.output, executionOne, "keep"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "stage_symlink":
				if err := os.Symlink(f.stage, f.stage+"-link"); err != nil {
					t.Fatal(err)
				}
				if _, err := New(Config{StagingRoot: f.stage + "-link", OutputRoot: f.output, Identity: "test"}); !errors.Is(err, ErrDisabled) {
					t.Fatal(err)
				}
				return
			case "nested_output":
				if _, err := New(Config{StagingRoot: f.stage, OutputRoot: filepath.Join(f.stage, "output"), Identity: "test"}); !errors.Is(err, ErrDisabled) {
					t.Fatal(err)
				}
				return
			case "missing_config":
				if _, err := New(Config{}); !errors.Is(err, ErrDisabled) {
					t.Fatal(err)
				}
				return
			}
			result := f.p.Pack(context.Background(), executionOne, f.spec)
			if result.Status != "failed" {
				t.Fatal(result)
			}
			if kind == "output_collision" {
				data, err := os.ReadFile(filepath.Join(f.output, executionOne, "keep"))
				if err != nil || string(data) != "keep" {
					t.Fatal("collision erased", err)
				}
			}
		})
	}
}

func TestArtifactPackageIOUncertainty(t *testing.T) {
	for _, step := range []string{"write", "cleanup", "sync_partial", "publish", "sync_final", "panic", "deadline"} {
		t.Run(step, func(t *testing.T) {
			f := newFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.p.fault = func(at string) error {
				if step == "panic" && at == "write" {
					panic("private detail")
				}
				if step == "deadline" && at == "snapshot" {
					cancel()
					return nil
				}
				if at == step || (step == "cleanup" && at == "write") {
					return errors.New("private detail")
				}
				return nil
			}
			result := f.p.Pack(ctx, executionOne, f.spec)
			want := "unknown"
			if step == "write" {
				want = "failed"
			}
			if result.Status != want {
				t.Fatal(result)
			}
			if strings.Contains(result.Code, "private") {
				t.Fatal("raw error leak")
			}
			if step == "write" {
				entries, err := os.ReadDir(f.output)
				if err != nil || len(entries) != 0 {
					t.Fatal("clean refusal left output", err)
				}
			}
			if step == "sync_final" {
				if _, err := os.Stat(filepath.Join(f.output, executionOne, "artifact.tar")); err != nil {
					t.Fatal("uncertain published output not retained", err)
				}
			}
		})
	}
}

func TestArtifactPackageFrozenBinding(t *testing.T) {
	f := newFixture(t)
	changed := f.b
	changed.TaskGeneration++
	if _, err := f.p.Prepare(context.Background(), f.r, changed, time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal("staging generation not bound", err)
	}
	spec := f.spec
	spec.RootIdentity = strings.Repeat("0", 64)
	if out := f.p.Pack(context.Background(), executionOne, spec); out.Status != "failed" || out.Code != "root_changed" {
		t.Fatal(out)
	}
	spec = f.spec
	spec.Request.TargetEnvironment = "production"
	if out := f.p.Pack(context.Background(), executionOne, spec); out.Status != "failed" || out.Code != "invalid_spec" {
		t.Fatal(out)
	}
}

func TestArtifactPackageRejectsReplacedOutputWithSameBytes(t *testing.T) {
	f := newFixture(t)
	f.p.fault = func(step string) error {
		if step != "publish" {
			return nil
		}
		partial := filepath.Join(f.output, executionOne, "artifact.partial")
		data, err := os.ReadFile(partial)
		if err != nil {
			return err
		}
		if err = os.Rename(partial, partial+"-original"); err != nil {
			return err
		}
		return os.WriteFile(partial, data, 0600)
	}
	if out := f.p.Pack(context.Background(), executionOne, f.spec); out.Status != "unknown" {
		t.Fatalf("replaced inode accepted: %+v", out)
	}
}

func TestArtifactPackageConcurrentOutputCreation(t *testing.T) {
	f := newFixture(t)
	const n = 8
	start := make(chan struct{})
	outcomes := make(chan Outcome, n)
	for i := 0; i < n; i++ {
		go func() { <-start; outcomes <- f.p.Pack(context.Background(), executionOne, f.spec) }()
	}
	close(start)
	success := 0
	for i := 0; i < n; i++ {
		out := <-outcomes
		if out.Status == "succeeded" {
			success++
		} else if out.Status != "failed" || out.Code != "output_collision" {
			t.Fatal(out)
		}
	}
	if success != 1 {
		t.Fatalf("published %d artifacts", success)
	}
	entries, err := os.ReadDir(filepath.Join(f.output, executionOne))
	if err != nil || len(entries) != 1 || entries[0].Name() != "artifact.tar" {
		t.Fatal("unexpected output files", entries, err)
	}
}

func TestArtifactPackageParentReplacedWhileReading(t *testing.T) {
	f := newFixture(t)
	f.p.fault = func(step string) error {
		if !strings.HasSuffix(step, "/nested/a.txt") {
			return nil
		}
		parent := filepath.Join(f.generation, "files")
		dir := filepath.Join(parent, "nested")
		if err := os.Chmod(parent, 0700); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return err
		}
		if err := os.Rename(dir, dir+"-old"); err != nil {
			return err
		}
		if err := os.Mkdir(dir, 0500); err != nil {
			return err
		}
		return os.Chmod(parent, 0500)
	}
	if out := f.p.Pack(context.Background(), executionOne, f.spec); out.Status != "failed" || out.Code != "input_invalid" {
		t.Fatal(out)
	}
}

func TestArtifactPackageCleanupDoesNotDeleteReplacedPartial(t *testing.T) {
	f := newFixture(t)
	f.p.fault = func(step string) error {
		if step != "write" {
			return nil
		}
		path := filepath.Join(f.output, executionOne, "artifact.partial")
		if err := os.Rename(path, path+"-original"); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("retain replacement evidence"), 0600); err != nil {
			return err
		}
		return errors.New("fail before writing")
	}
	out := f.p.Pack(context.Background(), executionOne, f.spec)
	if out.Status != "unknown" {
		t.Fatal(out)
	}
	data, err := os.ReadFile(filepath.Join(f.output, executionOne, "artifact.partial"))
	if err != nil || string(data) != "retain replacement evidence" {
		t.Fatal("cleanup deleted replacement", err)
	}
}
