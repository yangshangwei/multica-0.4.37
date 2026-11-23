package collaborationrun

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func preparationTestCapabilities() protocol.CollaborationRuntimeCapabilities {
	return protocol.CollaborationRuntimeCapabilities{
		Version: 1, PreparationVersion: 1, OS: "linux", Arch: "arm64",
		Containment: protocol.CollaborationContainmentLinuxPIDNamespace, AttachmentFormat: protocol.CollaborationAttachmentFormat,
		Provider: protocol.CollaborationPreparationProvider{Provider: "claude", ProtocolFamily: "claude", AdapterID: "static_json_v1", AdapterVersion: 1,
			BundleDigest: strings.Repeat("a", 64), AccountSource: "runtime_configuration", Resume: "fresh"},
		Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20,
			WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 65536, TimeoutSeconds: 600},
	}
}

func preparationTestClaim(t *testing.T) (protocol.CollaborationRunClaimRequest, protocol.CollaborationRunClaimResponse) {
	t.Helper()
	request, claim := testClaimRequest(), testClaim()
	caps := preparationTestCapabilities()
	request.PreparationVersion, request.BundleDigest = 1, caps.Provider.BundleDigest
	file := protocol.CollaborationPreparationAttachment{ID: claim.Identity.IssueID, Version: "9007199254740993", Name: "../../display only",
		ContentType: "text/plain", SizeBytes: 4, Destination: protocol.CollaborationPreparationAttachmentDestination(claim.Identity.IssueID, "9007199254740993")}
	claim.Inputs[0].AttachmentIDs = []string{file.ID}
	claim.Preparation = &protocol.CollaborationPreparation{Version: 1, Identity: claim.Identity,
		CommandIDs: []string{claim.Inputs[0].CommandID, claim.Inputs[1].CommandID},
		Resource: protocol.CollaborationPreparationResource{AgentID: claim.AgentID, RuntimeID: request.RuntimeID, OwnerUserID: claim.ResourceOwnerID,
			ExecutionWorkspaceID: claim.ExecutionWorkspaceID, TargetEpoch: "1", ConfigVersion: "2", RuntimeConfigEpoch: "5"},
		Provider: caps.Provider, Limits: caps.Limits, Attachments: []protocol.CollaborationPreparationAttachment{file},
	}
	if err := protocol.SealCollaborationPreparation(claim.Preparation); err != nil {
		t.Fatal(err)
	}
	return request, claim
}

func TestClientPreparationClaimRejectsMissingOrMismatchedSnapshot(t *testing.T) {
	for _, kind := range []string{"valid", "missing", "unrequested", "digest", "command", "resource", "bundle", "legacy_files", "missing_invocation", "null_invocation"} {
		t.Run(kind, func(t *testing.T) {
			request, claim := preparationTestClaim(t)
			switch kind {
			case "missing":
				claim.Preparation = nil
			case "unrequested":
				request.PreparationVersion, request.BundleDigest = 0, ""
			case "digest":
				claim.Preparation.ManifestDigest = strings.Repeat("b", 64)
			case "command":
				claim.Preparation.CommandIDs[0], claim.Preparation.CommandIDs[1] = claim.Preparation.CommandIDs[1], claim.Preparation.CommandIDs[0]
				_ = protocol.SealCollaborationPreparation(claim.Preparation)
			case "resource":
				claim.Preparation.Resource.AgentID = claim.Identity.LaunchNonce
				_ = protocol.SealCollaborationPreparation(claim.Preparation)
			case "bundle":
				request.BundleDigest = strings.Repeat("b", 64)
			case "legacy_files":
				request.PreparationVersion, request.BundleDigest, claim.Preparation = 0, "", nil
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if kind == "missing_invocation" || kind == "null_invocation" {
					raw, _ := json.Marshal(claim)
					var wire map[string]json.RawMessage
					_ = json.Unmarshal(raw, &wire)
					var prepared map[string]json.RawMessage
					_ = json.Unmarshal(wire["preparation"], &prepared)
					if kind == "missing_invocation" {
						delete(prepared, "invocation")
					} else {
						prepared["invocation"] = json.RawMessage("null")
					}
					wire["preparation"], _ = json.Marshal(prepared)
					_ = json.NewEncoder(w).Encode(wire)
					return
				}
				_ = json.NewEncoder(w).Encode(claim)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Claim(t.Context(), claim.Identity.RunID, request)
			if (err == nil) != (kind == "valid") {
				t.Fatalf("claim correlation kind=%s err=%v", kind, err)
			}
		})
	}
}

func TestClientPreparationCapabilitiesRecoverOnlyExactCurrentRegistration(t *testing.T) {
	for _, kind := range []string{"created", "lost", "different_limits", "changed_epoch", "wrong_binding", "already_current"} {
		t.Run(kind, func(t *testing.T) {
			claim := testClaimRequest()
			caps := preparationTestCapabilities()
			registration := protocol.CollaborationRuntimeCapabilityRegistration{DeploymentID: claim.DeploymentID, InstallationID: claim.InstallationID,
				BindingID: claim.BindingID, BindingEpoch: claim.BindingEpoch, RuntimeConfigEpoch: "5", Capabilities: caps}
			response := protocol.CollaborationRuntimeCapabilitiesResponse{RuntimeID: claim.RuntimeID, RuntimeConfigEpoch: "4"}
			if kind == "already_current" {
				response.RuntimeConfigEpoch, response.Registration = "5", &registration
			}
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/collaboration/runtime/capabilities" || r.Header.Get("Authorization") != "Bearer mdt_fixture" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Workspace-ID") != "" {
					t.Error("capability registration escaped managed scope")
				}
				if r.Method == http.MethodPost {
					posts++
					var in protocol.CollaborationRuntimeCapabilitiesRequest
					if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ExpectedRuntimeConfigEpoch != "4" || !reflect.DeepEqual(in.Capabilities, caps) {
						t.Error("registration changed the observed epoch or full summary")
					}
					response.RuntimeConfigEpoch, response.Registration = "5", &registration
					switch kind {
					case "different_limits":
						response.Registration.Capabilities.Limits.FileCount--
					case "changed_epoch":
						response.RuntimeConfigEpoch = "6"
					case "wrong_binding":
						response.Registration.BindingID = claim.RuntimeID
					}
					if kind != "created" {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
						} else {
							_ = conn.Close()
						}
						return
					}
				} else if r.Method != http.MethodGet || r.URL.Query().Get("runtime_id") != claim.RuntimeID {
					t.Error("invalid capability lookup")
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.EnsureCapabilities(t.Context(), claim, "4", caps)
			wantSuccess := kind == "created" || kind == "lost" || kind == "already_current"
			if (err == nil) != wantSuccess || posts > 1 || kind == "already_current" && posts != 0 {
				t.Fatalf("registration kind=%s posts=%d err=%v", kind, posts, err)
			}
		})
	}
}

func TestClientPreparationAttachmentUsesExactVersionAndBounds(t *testing.T) {
	for _, kind := range []string{"valid", "short", "oversized", "wrong_run", "wrong_version", "redirect", "partial", "encoded", "term_token"} {
		t.Run(kind, func(t *testing.T) {
			_, claim := preparationTestClaim(t)
			manifest, calls, redirected := *claim.Preparation, 0, 0
			file := manifest.Attachments[0]
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected++ }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != runPath(claim.Identity.RunID, "attachments/"+file.ID+"/content") || r.URL.RawQuery != "version="+file.Version || r.Header.Get("Authorization") != "Bearer mcb_fixture" || r.Header.Get("X-Workspace-ID") != "" {
					t.Error("download widened its scope or rounded the version")
				}
				w.Header().Set("X-Multica-Run-ID", manifest.Identity.RunID)
				w.Header().Set("X-Multica-Attachment-ID", file.ID)
				w.Header().Set("X-Multica-Attachment-Version", file.Version)
				body := "data"
				switch kind {
				case "short":
					body = "bad"
				case "oversized":
					body = "extra"
				case "wrong_run":
					w.Header().Set("X-Multica-Run-ID", file.ID)
				case "wrong_version":
					w.Header().Set("X-Multica-Attachment-Version", "9007199254740992")
				case "redirect":
					http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
					return
				case "partial":
					w.WriteHeader(http.StatusPartialContent)
				case "encoded":
					w.Header().Set("Content-Encoding", "gzip")
				}
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			token := "mcb_fixture"
			if kind == "term_token" {
				token = "mct_fixture"
			}
			var out bytes.Buffer
			err = client.DownloadAttachment(t.Context(), manifest, file, token, &out)
			if (err == nil) != (kind == "valid") || redirected != 0 || kind == "term_token" && calls != 0 || kind == "valid" && out.String() != "data" {
				t.Fatalf("download kind=%s calls=%d redirected=%d bytes=%d err=%v", kind, calls, redirected, out.Len(), err)
			}
		})
	}
}
