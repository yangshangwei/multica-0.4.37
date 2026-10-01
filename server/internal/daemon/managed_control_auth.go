package daemon

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const managementNonceHeader = "X-Multica-Management-Nonce"
const managementTimeHeader = "X-Multica-Management-Time"
const managementAuthHeader = "X-Multica-Management-Auth"
const managementResponseHeader = "X-Multica-Management-Response"

func managementMAC(secret, domain, method, path, nonce, timestamp string, status int, body []byte) (string, error) {
	key, err := managedDecode(secret, 32)
	if err != nil || len(key) != 32 {
		return "", errors.New("invalid management control key")
	}
	digest := sha256.Sum256(body)
	message := domain + "\n" + method + "\n" + path + "\n" + nonce + "\n" + timestamp + "\n"
	if status != 0 {
		message += strconv.Itoa(status) + "\n"
	}
	message += hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func ManagementRequestHeaders(secret, method, path string, body []byte) (http.Header, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(nonce)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac, err := managementMAC(secret, "multica-management-request-v1", method, path, encoded, timestamp, 0, body)
	if err != nil {
		return nil, err
	}
	headers := http.Header{}
	headers.Set(managementNonceHeader, encoded)
	headers.Set(managementTimeHeader, timestamp)
	headers.Set(managementAuthHeader, mac)
	return headers, nil
}

func VerifyManagementResponse(secret, method, path string, requestHeaders http.Header, status int, body []byte, responseHeaders http.Header) error {
	expected, err := managementMAC(secret, "multica-management-response-v1", method, path, requestHeaders.Get(managementNonceHeader), requestHeaders.Get(managementTimeHeader), status, body)
	if err != nil {
		return err
	}
	actual := responseHeaders.Get(managementResponseHeader)
	if len(actual) != 64 || !hmac.Equal([]byte(expected), []byte(actual)) {
		return errors.New("local daemon did not prove management ownership")
	}
	return nil
}

func (d *Daemon) authenticateManagementRequest(r *http.Request, body []byte) bool {
	state := d.managed
	if state == nil || state.controlToken == "" {
		return false
	}
	nonce, err := managedDecode(r.Header.Get(managementNonceHeader), 32)
	if err != nil || len(nonce) != 32 {
		return false
	}
	timestamp, ok := managedNumber(r.Header.Get(managementTimeHeader))
	if !ok || timestamp < time.Now().Unix()-30 || timestamp > time.Now().Unix()+30 {
		return false
	}
	expected, err := managementMAC(state.controlToken, "multica-management-request-v1", r.Method, r.URL.Path, r.Header.Get(managementNonceHeader), r.Header.Get(managementTimeHeader), 0, body)
	actual := r.Header.Get(managementAuthHeader)
	if err != nil || len(actual) != 64 || !hmac.Equal([]byte(expected), []byte(actual)) {
		return false
	}
	if r.Method == http.MethodPost {
		state.controlMu.Lock()
		defer state.controlMu.Unlock()
		if state.controlNonces == nil {
			state.controlNonces = make(map[string]int64)
		}
		for key, seenAt := range state.controlNonces {
			if seenAt < time.Now().Unix()-60 {
				delete(state.controlNonces, key)
			}
		}
		key := r.Header.Get(managementNonceHeader)
		if _, seen := state.controlNonces[key]; seen || len(state.controlNonces) >= 512 {
			return false
		}
		state.controlNonces[key] = timestamp
	}
	return true
}

func (d *Daemon) writeManagementResponse(w http.ResponseWriter, r *http.Request, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "Management response unavailable", 500)
		return
	}
	proof, err := managementMAC(d.managed.controlToken, "multica-management-response-v1", r.Method, r.URL.Path, r.Header.Get(managementNonceHeader), r.Header.Get(managementTimeHeader), status, body)
	if err != nil {
		http.Error(w, "Management response unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(managementResponseHeader, proof)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func managementStatusError(status int) error {
	if status == http.StatusConflict {
		return errors.New("managed daemon is busy or its scope changed; it was left running")
	}
	return fmt.Errorf("authenticated management request failed (%d)", status)
}
