package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Keep the global inventory unchanged: unrelated missing registrations remain
// visible there. This inventory proves every S2 concurrent build has its retry hook.
func TestStageConcurrentIndexCleanupInventory(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".up.sql")
		version, _ := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if version < 805 || (!strings.Contains(name, "collaboration_stage_") && !strings.Contains(name, "project_stage_policy_") && !strings.Contains(name, "collaboration_test_subject")) {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		matches := concurrentIndexNamePattern.FindAllSubmatch(stripSQLLineComments(body), -1)
		if len(matches) == 0 {
			continue
		}
		count++
		if len(matches) != 1 {
			t.Errorf("%s: expected one concurrent build", name)
			continue
		}
		if got := concurrentIndexCleanups[name]; got != string(matches[0][1]) || hooksForDirection("up")[name] == nil {
			t.Errorf("%s: missing or incorrect cleanup for %s", name, matches[0][1])
		}
	}
	if count < 16 {
		t.Fatalf("S2 inventory omitted builds: %d", count)
	}
}
