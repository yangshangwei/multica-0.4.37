package daemon

import (
	"context"
	"io"
	"net/http"
	"time"
)

type managementDrainRequest struct {
	IntentID         string  `json:"intent_id"`
	ExpectedIntentID *string `json:"expected_intent_id"`
}

// Drain belongs to this process and never resumes claims on a caller timeout.
// The CAS prevents a delayed request from replacing a newer shutdown intent.
func (d *Daemon) requestManagedDrain(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	var request managementDrainRequest
	if err != nil || managedCanonicalJSON(raw, &request) != nil || !managedUUID.MatchString(request.IntentID) || (request.ExpectedIntentID != nil && !managedUUID.MatchString(*request.ExpectedIntentID)) {
		d.writeManagementResponse(w, r, http.StatusBadRequest, map[string]string{"error": "Invalid drain intent"})
		return
	}
	expected := ""
	if request.ExpectedIntentID != nil {
		expected = *request.ExpectedIntentID
	}
	d.claimMu.Lock()
	if d.cancelFunc == nil || (d.managedDrainIntent == "" && d.pauseClaims) || (d.managedDrainIntent != request.IntentID && d.managedDrainIntent != expected) {
		d.claimMu.Unlock()
		d.writeManagementResponse(w, r, http.StatusConflict, map[string]string{"error": "Management drain intent changed or another operation owns the claim gate"})
		return
	}
	changed := d.managedDrainIntent != request.IntentID
	d.managedDrainIntent = request.IntentID
	d.pauseClaims = true
	d.claimMu.Unlock()
	d.writeManagementResponse(w, r, http.StatusAccepted, map[string]any{"accepted": true, "status": "draining", "intent_id": request.IntentID})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	if changed {
		go d.finishManagedDrain(request.IntentID)
	}
}

func (d *Daemon) finishManagedDrain(intent string) {
	ctx := d.rootCtx
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		d.claimMu.Lock()
		if d.managedDrainIntent != intent {
			d.claimMu.Unlock()
			return
		}
		idle := d.claimsInFlight == 0 && d.activeTasks.Load() == 0
		if idle {
			d.cancelFunc()
		}
		d.claimMu.Unlock()
		if idle {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
