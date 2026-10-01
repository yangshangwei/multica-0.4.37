package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestManagedControlSharedHMACVector(t *testing.T) {
	raw, err := os.ReadFile("testdata/managed-control.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Secret, Method, Path, Nonce, Timestamp string
		RequestBody                            string `json:"request_body"`
		RequestMAC                             string `json:"request_mac"`
		Status                                 int
		ResponseBody                           string `json:"response_body"`
		ResponseMAC                            string `json:"response_mac"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	actual, err := managementMAC(v.Secret, "multica-management-request-v1", v.Method, v.Path, v.Nonce, v.Timestamp, 0, []byte(v.RequestBody))
	if err != nil || actual != v.RequestMAC {
		t.Fatalf("request vector mismatch: %s %v", actual, err)
	}
	actual, err = managementMAC(v.Secret, "multica-management-response-v1", v.Method, v.Path, v.Nonce, v.Timestamp, v.Status, []byte(v.ResponseBody))
	if err != nil || actual != v.ResponseMAC {
		t.Fatalf("response vector mismatch: %s %v", actual, err)
	}
}

func TestManagedControlRequestBindsMethodPathBodyTimeAndRejectsReplay(t *testing.T) {
	secret := strings.Repeat("A", 43)
	body := []byte(`{"intent_id":"test"}`)
	for _, mutation := range []string{"method", "path", "body", "time", "secret", "replay"} {
		t.Run(mutation, func(t *testing.T) {
			d := &Daemon{managed: &managedDaemonLifecycle{controlToken: secret}}
			request := httptest.NewRequest("POST", "/shutdown", nil)
			request.Header, _ = ManagementRequestHeaders(secret, request.Method, request.URL.Path, body)
			input := body
			switch mutation {
			case "method":
				request.Method = "GET"
			case "path":
				request.URL.Path = "/management/handoff"
			case "body":
				input = []byte(`{}`)
			case "time":
				request.Header.Set(managementTimeHeader, "1")
			case "secret":
				d.managed.controlToken = "B" + secret[1:]
			case "replay":
				if !d.authenticateManagementRequest(request, body) {
					t.Fatal("initial valid request rejected")
				}
			}
			if d.authenticateManagementRequest(request, input) {
				t.Fatal("mutated or replayed request accepted")
			}
		})
	}
}

func TestManagedControlResponseRejectsTamperAndForgedPeer(t *testing.T) {
	secret := strings.Repeat("A", 43)
	headers, _ := ManagementRequestHeaders(secret, "POST", "/shutdown", nil)
	body := []byte(`{"accepted":true}`)
	proof, _ := managementMAC(secret, "multica-management-response-v1", "POST", "/shutdown", headers.Get(managementNonceHeader), headers.Get(managementTimeHeader), 202, body)
	response := http.Header{}
	response.Set(managementResponseHeader, proof)
	if err := VerifyManagementResponse(secret, "POST", "/shutdown", headers, 202, body, response); err != nil {
		t.Fatal(err)
	}
	if VerifyManagementResponse(secret, "POST", "/shutdown", headers, 200, body, response) == nil {
		t.Fatal("status tamper accepted")
	}
	if VerifyManagementResponse(secret, "POST", "/shutdown", headers, 202, []byte(`{}`), response) == nil {
		t.Fatal("body tamper accepted")
	}
	if VerifyManagementResponse(secret, "POST", "/shutdown", headers, 202, body, http.Header{}) == nil {
		t.Fatal("unproven peer accepted")
	}
}
