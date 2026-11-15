package main

import (
	"os"
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
)

func TestKnowledgeContextDefaultEnabled(t *testing.T) {
	t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", "false")
	t.Setenv("MULTICA_VCS_INTEGRATION_ENABLED", "false")
	for _, tc := range []struct {
		name, value string
		unset       bool
		enabled     bool
	}{
		{name: "unset", unset: true, enabled: true},
		{name: "empty", value: "", enabled: true},
		{name: "enabled", value: "true", enabled: true},
		{name: "disabled", value: "false", enabled: false},
		{name: "noncanonical", value: "1", enabled: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MULTICA_KNOWLEDGE_CONTEXT_ENABLED", tc.value)
			if tc.unset {
				if err := os.Unsetenv("MULTICA_KNOWLEDGE_CONTEXT_ENABLED"); err != nil {
					t.Fatal(err)
				}
			}
			_, h := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
			if h.TaskService == nil || h.TaskService.KnowledgeContextEnabled != tc.enabled {
				t.Fatalf("startup knowledge gate = %v, want %v", h.TaskService != nil && h.TaskService.KnowledgeContextEnabled, tc.enabled)
			}
		})
	}
}
