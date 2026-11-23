package protocol

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestKnowledgeAutomationPublicVersionsPreserveLegacy(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			proposal := knowledgeProposal(t)
			proposal.ProtocolVersion = version
			raw, err := json.Marshal(proposal)
			if err != nil {
				t.Fatal(err)
			}
			var decoded CollaborationKnowledgeProposal
			if err = json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("supported public knowledge version %d rejected: %v", version, err)
			}
			if !reflect.DeepEqual(decoded, proposal) {
				t.Fatal("public request changed during strict decode")
			}
		})
	}
	proposal := knowledgeProposal(t)
	proposal.ProtocolVersion = 3
	if ValidateCollaborationKnowledgeProposal(proposal, knowledgeTestID) == nil {
		t.Fatal("unknown public knowledge version accepted")
	}
}

func automationDeliveryProposalJSON(t *testing.T) []byte {
	t.Helper()
	proposal := knowledgeProposal(t)
	proposal.ProtocolVersion = 2
	raw, err := json.Marshal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["sources"] = json.RawMessage(`[{"kind":"delivery","issue_id":"` + knowledgeTestID + `","submission_id":"22222222-2222-4222-8222-222222222222","delivery_event_seq":"11","accepted_event_seq":"15","sha256":"` + strings.Repeat("a", 64) + `"}]`)
	raw, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestKnowledgeAutomationDeliverySourcePreservesExactIdentity(t *testing.T) {
	raw := automationDeliveryProposalJSON(t)
	var proposal CollaborationKnowledgeProposal
	if err := json.Unmarshal(raw, &proposal); err != nil {
		t.Fatalf("v2 accepted delivery source rejected: %v", err)
	}
	encoded, err := json.Marshal(proposal.Sources)
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]json.RawMessage
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	var before, after any
	if json.Unmarshal(expected["sources"], &before) != nil || json.Unmarshal(encoded, &after) != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("delivery submission, event or complete-body hash was lost")
	}
}

func TestKnowledgeAutomationDeliverySourceRejectsAmbiguity(t *testing.T) {
	raw := string(automationDeliveryProposalJSON(t))
	for name, change := range map[string]func(string) string{
		"legacy envelope": func(s string) string {
			return strings.Replace(s, `"protocol_version":2`, `"protocol_version":1`, 1)
		},
		"missing submission": func(s string) string {
			return strings.Replace(s, `"submission_id":"22222222-2222-4222-8222-222222222222",`, "", 1)
		},
		"noncanonical acceptance": func(s string) string {
			return strings.Replace(s, `"accepted_event_seq":"15"`, `"accepted_event_seq":"015"`, 1)
		},
		"acceptance before delivery": func(s string) string {
			return strings.Replace(s, `"accepted_event_seq":"15"`, `"accepted_event_seq":"10"`, 1)
		},
		"duplicate acceptance": func(s string) string {
			return strings.Replace(s, `"accepted_event_seq":"15"`, `"accepted_event_seq":"15","accepted_event_seq":"16"`, 1)
		},
		"mixed comment identity": func(s string) string {
			return strings.Replace(s, `"kind":"delivery"`, `"kind":"delivery","comment_id":"`+knowledgeTestID+`"`, 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var proposal CollaborationKnowledgeProposal
			if json.Unmarshal([]byte(change(raw)), &proposal) == nil {
				t.Fatal("ambiguous or v1-inexpressible delivery accepted")
			}
		})
	}
}

func TestKnowledgeAutomationPublicRequestCannotChooseSystemIdentity(t *testing.T) {
	for _, version := range []int{1, 2} {
		proposal := knowledgeProposal(t)
		proposal.ProtocolVersion = version
		raw, err := json.Marshal(proposal)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{`"proposal_origin":"automation"`, `"decision_origin":"automation"`, `"automation":{"job_id":"` + knowledgeTestID + `","policy_revision":"0","rule_version":"v1"}`} {
			var decoded CollaborationKnowledgeProposal
			forged := strings.Replace(string(raw), "{", "{"+field+",", 1)
			if json.Unmarshal([]byte(forged), &decoded) == nil {
				t.Fatalf("public version %d allowed the caller to choose system authority", version)
			}
		}
	}
}

func TestKnowledgeAutomationRuntimeVersionsPreserveLegacy(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			if err := ValidateTaskKnowledgeRequest(version, knowledgeTestID, "1", "2026-11-14T00:00:00Z"); err != nil {
				t.Errorf("supported ordinary runtime version %d rejected: %v", version, err)
			}
			request := runKnowledgeBundleFixture().CollaborationRunKnowledgePrepare
			request.ProtocolVersion = version
			if err := ValidateCollaborationRunKnowledgePrepare(request); err != nil {
				t.Errorf("supported collaboration runtime version %d rejected: %v", version, err)
			}
		})
	}
	if ValidateTaskKnowledgeRequest(3, knowledgeTestID, "1", "2026-11-14T00:00:00Z") == nil {
		t.Fatal("unknown ordinary runtime version accepted")
	}
	request := runKnowledgeBundleFixture().CollaborationRunKnowledgePrepare
	request.ProtocolVersion = 3
	if ValidateCollaborationRunKnowledgePrepare(request) == nil {
		t.Fatal("unknown collaboration runtime version accepted")
	}
}

func automationManifestJSON() string {
	return `{"protocol_version":2,"entries":[{"candidate_id":"` + knowledgeTestID + `","version":"1","content_sha256":"` + strings.Repeat("a", 64) + `","source_issue_id":"` + knowledgeTestID + `","target":{"kind":"project","workspace_id":"` + knowledgeTestID + `","project_id":"22222222-2222-4222-8222-222222222222"},"validity":{"revision":"1","accepted_event_seq":"15","submission_id":"33333333-3333-4333-8333-333333333333","validated_close_epoch":"3"}}]}`
}

func TestKnowledgeAutomationManifestBindsReacceptedValidity(t *testing.T) {
	raw := automationManifestJSON()
	var first, reaccepted TaskKnowledgeManifest
	if err := json.Unmarshal([]byte(raw), &first); err != nil {
		t.Fatalf("v2 automatic validity rejected: %v", err)
	}
	changed := strings.Replace(raw, `"revision":"1"`, `"revision":"2"`, 1)
	changed = strings.Replace(changed, `"accepted_event_seq":"15"`, `"accepted_event_seq":"21"`, 1)
	changed = strings.Replace(changed, `"validated_close_epoch":"3"`, `"validated_close_epoch":"5"`, 1)
	if err := json.Unmarshal([]byte(changed), &reaccepted); err != nil {
		t.Fatalf("new exact acceptance validity rejected: %v", err)
	}
	firstHash, err := TaskKnowledgeRequestHash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := TaskKnowledgeRequestHash(reaccepted)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == secondHash {
		t.Fatal("reacceptance changed the validity proof without changing the retained manifest hash")
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"validity":{"revision":"1","accepted_event_seq":"15","submission_id":"33333333-3333-4333-8333-333333333333","validated_close_epoch":"3"}`) {
		t.Fatal("exact automatic validity was not retained on round trip")
	}
}

func TestKnowledgeAutomationManifestRejectsAmbiguousOrLegacyValidity(t *testing.T) {
	raw := automationManifestJSON()
	for name, change := range map[string]func(string) string{
		"legacy envelope": func(s string) string {
			return strings.Replace(s, `"protocol_version":2`, `"protocol_version":1`, 1)
		},
		"missing acceptance": func(s string) string {
			return strings.Replace(s, `"accepted_event_seq":"15",`, "", 1)
		},
		"duplicate proof revision": func(s string) string {
			return strings.Replace(s, `"revision":"1"`, `"revision":"1","revision":"2"`, 1)
		},
		"unknown proof field": func(s string) string {
			return strings.Replace(s, `"validity":{`, `"validity":{"current":true,`, 1)
		},
		"zero close epoch": func(s string) string {
			return strings.Replace(s, `"validated_close_epoch":"3"`, `"validated_close_epoch":"0"`, 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var manifest TaskKnowledgeManifest
			if json.Unmarshal([]byte(change(raw)), &manifest) == nil {
				t.Fatal("invalid or v1-inexpressible validity silently accepted or discarded")
			}
		})
	}
}

func TestKnowledgeAutomationLegacyManifestAndReceiptBytesStayExact(t *testing.T) {
	manifest := `{"protocol_version":1,"entries":[{"candidate_id":"` + knowledgeTestID + `","version":"1","content_sha256":"` + strings.Repeat("a", 64) + `","source_issue_id":"` + knowledgeTestID + `","target":{"kind":"workspace","workspace_id":"` + knowledgeTestID + `"}}]}`
	var decoded TaskKnowledgeManifest
	if err := json.Unmarshal([]byte(manifest), &decoded); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(decoded)
	if err != nil || string(raw) != manifest {
		t.Fatal("v1 manifest gained fields or changed canonical order:", err)
	}
	for _, version := range []int{1, 2} {
		receipt := `{"protocol_version":` + strconv.Itoa(version) + `,"request_id":"` + knowledgeTestID + `","action":"approve","code":"approved","candidate_id":"` + knowledgeTestID + `","version":"1","state_revision":"2","state":"approved","target_knowledge_revision":"1"}`
		var result CollaborationKnowledgeReceipt
		if err := json.Unmarshal([]byte(receipt), &result); err != nil {
			t.Fatalf("original v%d receipt rejected: %v", version, err)
		}
		raw, err := json.Marshal(result)
		if err != nil || string(raw) != receipt {
			t.Fatalf("original v%d receipt was rewritten: %v", version, err)
		}
	}
}

func TestKnowledgeAutomationValidityEqualityAndHashBindEachField(t *testing.T) {
	var first TaskKnowledgeManifest
	if err := json.Unmarshal([]byte(automationManifestJSON()), &first); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*TaskKnowledgeValidity){
		"revision":   func(v *TaskKnowledgeValidity) { v.Revision = "2" },
		"acceptance": func(v *TaskKnowledgeValidity) { v.AcceptedEventSeq = "21" },
		"submission": func(v *TaskKnowledgeValidity) { v.SubmissionID = knowledgeTestID },
		"epoch":      func(v *TaskKnowledgeValidity) { v.ValidatedCloseEpoch = "5" },
	} {
		t.Run(name, func(t *testing.T) {
			var second TaskKnowledgeManifest
			if err := json.Unmarshal([]byte(automationManifestJSON()), &second); err != nil {
				t.Fatal(err)
			}
			if !EqualTaskKnowledgeManifestEntry(first.Entries[0], second.Entries[0]) {
				t.Fatal("independently decoded equal proofs compared by pointer")
			}
			change(second.Entries[0].Validity)
			firstHash, err := TaskKnowledgeRequestHash(first)
			if err != nil {
				t.Fatal(err)
			}
			secondHash, err := TaskKnowledgeRequestHash(second)
			if err != nil {
				t.Fatal(err)
			}
			if firstHash == secondHash || EqualTaskKnowledgeManifestEntry(first.Entries[0], second.Entries[0]) {
				t.Fatal("changed proof field was not part of equality and manifest hash")
			}
		})
	}
}

func TestKnowledgeAutomationRuntimeEnvelopeRejectsDowngrade(t *testing.T) {
	base := runKnowledgeBundleFixture()
	base.ProtocolVersion, base.Manifest.ProtocolVersion = 2, 2
	item := &base.Contributions[0]
	item.Content = "The accepted finding remains supported."
	item.ContentSHA256 = CollaborationKnowledgeHash(item.Content)
	item.Target.Kind, item.Target.ProjectID = "project", knowledgeTestID
	item.Sources = []CollaborationKnowledgeSource{{Kind: "delivery", IssueID: item.SourceIssueID, SHA256: strings.Repeat("a", 64), SubmissionID: knowledgeTestID, DeliveryEventSeq: "11", AcceptedEventSeq: "15"}}
	base.Manifest.Entries = []TaskKnowledgeManifestEntry{{CandidateID: item.CandidateID, Version: item.Version, ContentSHA256: item.ContentSHA256, SourceIssueID: item.SourceIssueID, Target: item.Target, Validity: &TaskKnowledgeValidity{Revision: "1", AcceptedEventSeq: "15", SubmissionID: knowledgeTestID, ValidatedCloseEpoch: "3"}}}
	for name, change := range map[string]func(*CollaborationRunKnowledgeBundle){
		"supported v2": func(*CollaborationRunKnowledgeBundle) {},
		"outer v1":     func(p *CollaborationRunKnowledgeBundle) { p.ProtocolVersion = 1 },
		"inner v1":     func(p *CollaborationRunKnowledgeBundle) { p.Manifest.ProtocolVersion = 1 },
		"v1 delivery without proof": func(p *CollaborationRunKnowledgeBundle) {
			p.ProtocolVersion, p.Manifest.ProtocolVersion = 1, 1
			p.Manifest.Entries[0].Validity = nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			value := base
			value.Manifest.Entries = append([]TaskKnowledgeManifestEntry{}, base.Manifest.Entries...)
			change(&value)
			manifest, err := json.Marshal(value.Manifest)
			if err != nil {
				t.Fatal(err)
			}
			value.ManifestSHA256 = CollaborationKnowledgeHash(string(manifest))
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var decoded CollaborationRunKnowledgeBundle
			err = json.Unmarshal(raw, &decoded)
			if name == "supported v2" {
				if err != nil || !reflect.DeepEqual(decoded, value) {
					t.Fatalf("supported v2 full bundle changed: %v", err)
				}
			} else if err == nil {
				t.Fatal("downgraded envelope accepted automatic material")
			}
		})
	}
}
