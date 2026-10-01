package main

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
)

type adminWorkerObservation struct {
	mu        sync.RWMutex
	checkedAt time.Time
	failed    bool
}

func (s *adminWorkerObservation) record(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkedAt = time.Now().UTC()
	s.failed = err != nil
}

func (s *adminWorkerObservation) snapshot(name string, now time.Time, deadline time.Duration) handler.AdminHealthSource {
	s.mu.RLock()
	checked, failed := s.checkedAt, s.failed
	s.mu.RUnlock()
	result := handler.AdminHealthSource{Name: name, State: "unknown", Code: "not_observed"}
	if checked.IsZero() {
		return result
	}
	result.CheckedAt = &checked
	switch {
	case now.Sub(checked) > deadline:
		result.State, result.Code = "stale", "worker_stale"
	case failed:
		result.State, result.Code = "unavailable", "cycle_failed"
	default:
		result.State, result.Code = "healthy", "cycle_succeeded"
	}
	return result
}

func configuredAdminRetention(name string) *int {
	raw := os.Getenv(name)
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 36500 {
		return nil
	}
	return &value
}

func adminDetectorSource(ctx context.Context, alerts *service.AdminAlertService) handler.AdminHealthSource {
	result := handler.AdminHealthSource{Name: "alert_detector", State: "unknown", Code: "not_observed"}
	org, err := alerts.Queries.GetInternalOrganization(ctx)
	if err != nil {
		result.State, result.Code = "unavailable", "detector_unavailable"
		return result
	}
	rows, err := alerts.DetectorHealth(ctx, org.ID)
	if err != nil {
		result.State, result.Code = "unavailable", "detector_unavailable"
		return result
	}
	now := time.Now().UTC()
	priorities := map[string]int{"healthy": 0, "unknown": 1, "stale": 2, "unavailable": 3}
	priority := -1
	for _, row := range rows {
		state, code := "unknown", "not_observed"
		var successful time.Time
		if row.LastSuccessfulAt != nil {
			parsed, parseErr := time.Parse(time.RFC3339Nano, *row.LastSuccessfulAt)
			if parseErr == nil && !parsed.After(now.Add(time.Minute)) {
				successful = parsed
			}
		}
		checked := successful
		if row.LastStartedAt != nil {
			parsed, parseErr := time.Parse(time.RFC3339Nano, *row.LastStartedAt)
			if parseErr == nil && !parsed.After(now.Add(time.Minute)) {
				checked = parsed
			}
		}
		if !checked.IsZero() && (result.CheckedAt == nil || checked.Before(*result.CheckedAt)) {
			copy := checked
			result.CheckedAt = &copy
		}
		switch {
		case row.SourceState == "unavailable":
			state, code = "unavailable", "source_unavailable"
		case successful.IsZero():
		case now.Sub(successful) > time.Minute:
			state, code = "stale", "detector_stale"
		case row.SourceState != "healthy" || !row.ScanComplete || row.UnknownCount > 0:
			state, code = "unknown", "evidence_incomplete"
		default:
			state, code = "healthy", "cycle_succeeded"
		}
		if priorities[state] > priority {
			result.State, result.Code, priority = state, code, priorities[state]
		}
	}
	return result
}
