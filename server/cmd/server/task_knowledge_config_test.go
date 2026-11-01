package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
)

func TestKnowledgeContextRequiresExplicitStartupOptIn(t *testing.T) {
	t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", "false")
	t.Setenv("MULTICA_VCS_INTEGRATION_ENABLED", "false")
	for _, tc := range []struct {
		name, value string
		enabled     bool
	}{{"default", "", false}, {"enabled", "true", true}, {"disabled", "false", false}, {"noncanonical", "1", false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MULTICA_KNOWLEDGE_CONTEXT_ENABLED", tc.value)
			_, h := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
			if h.TaskService == nil || h.TaskService.KnowledgeContextEnabled != tc.enabled {
				t.Fatalf("startup knowledge gate = %v, want %v", h.TaskService != nil && h.TaskService.KnowledgeContextEnabled, tc.enabled)
			}
		})
	}
}
