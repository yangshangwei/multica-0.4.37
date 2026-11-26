package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/spf13/cobra"
)

const materialCLIInputLimit = 600 * 1024
const materialCLIResponseLimit = 16 * 1024 * 1024

func init() { projectCmd.AddCommand(newProjectMaterialCommand()) }

func newProjectMaterialCommand() *cobra.Command {
	group := &cobra.Command{Use: "material", Short: "Read and author versioned Git engineering materials"}
	for _, action := range []string{"list", "get", "create", "revise"} {
		use, args := action+" <project-uuid>", 1
		if action == "get" || action == "revise" {
			use, args = use+" <material-uuid>", 2
		}
		command := &cobra.Command{Use: use, Short: map[string]string{
			"list": "List project materials as JSON", "get": "Read material revisions and their verified baseline as JSON",
			"create": "Create a requirement draft with an explicit request ID",
			"revise": "Save one exact revision request from a JSON file or stdin",
		}[action], Args: exactArgs(args)}
		command.Long = command.Short + ". Output is always JSON. Writes are sent once; retain the original request ID and unchanged input after an uncertain result. Human approval and workflow configuration remain in the app."
		switch action {
		case "create":
			command.Flags().String("title", "", "Requirement title (required)")
			command.Flags().String("request-id", "", "Original canonical request UUID (required; never generated automatically)")
		case "revise":
			command.Flags().String("file", "", "RevisionInput JSON file, or - for stdin (required; at most 600 KiB)")
		}
		command.RunE = func(cmd *cobra.Command, args []string) error { return runProjectMaterial(cmd, action, args) }
		group.AddCommand(command)
	}
	return group
}

func materialCLIUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func runProjectMaterial(cmd *cobra.Command, action string, args []string) error {
	for _, id := range args {
		if !materialCLIUUID(id) {
			return errors.New("project and material IDs must be canonical nonzero UUIDs; names and prefixes are not resolved")
		}
	}
	var input any
	var revision material.RevisionInput
	var create material.CreateInput
	var key string
	switch action {
	case "create":
		create.Title, _ = cmd.Flags().GetString("title")
		create.RequestID, _ = cmd.Flags().GetString("request-id")
		if !materialCLIUUID(create.RequestID) || strings.TrimSpace(create.Title) == "" || len(create.Title) > 500 {
			return errors.New("create requires --title (1–500 bytes) and an explicit canonical nonzero --request-id UUID")
		}
		input, key = create, create.RequestID
	case "revise":
		name, _ := cmd.Flags().GetString("file")
		if name == "" {
			return errors.New("--file is required; use - to read the revision JSON from stdin")
		}
		reader := cmd.InOrStdin()
		if name != "-" {
			file, err := os.Open(name)
			if err != nil {
				return fmt.Errorf("open material revision file: %w", err)
			}
			defer file.Close()
			reader = file
		}
		var err error
		revision, err = readMaterialCLIRevision(reader)
		if err != nil {
			return err
		}
		input, key = revision, revision.RequestID
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if !materialCLIUUID(client.WorkspaceID) {
		return errors.New("an explicit canonical workspace UUID is required for material commands")
	}
	path := "/api/projects/" + args[0] + "/materials"
	if action == "get" || action == "revise" {
		path += "/" + args[1]
	}
	if action == "revise" {
		path += "/revisions"
	}
	method := http.MethodGet
	if input != nil {
		method = http.MethodPost
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	raw, err := materialCLIRequest(ctx, client, method, path, key, input)
	if err != nil {
		return fmt.Errorf("material %s: %w", action, err)
	}
	if action == "list" {
		var result material.List
		if err = materialCLIDecodeResponse(raw, &result, "workspace_id", "project_id", "materials"); err != nil ||
			result.WorkspaceID != client.WorkspaceID || result.ProjectID != args[0] || result.Materials == nil {
			return errors.New("invalid material list response; no result was printed")
		}
		seen := map[string]bool{}
		for _, item := range result.Materials {
			if !validMaterialCLIItem(item, client.WorkspaceID, args[0]) || seen[item.ID] {
				return errors.New("invalid or foreign material in list response; no result was printed")
			}
			seen[item.ID] = true
		}
		return cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	var result material.Detail
	if err = materialCLIDecodeResponse(raw, &result, "material", "revisions", "decisions", "comments", "baseline", "plan", "capabilities"); err != nil ||
		!validMaterialCLIDetail(result, client.WorkspaceID, args[0]) ||
		(action != "create" && result.Material.ID != args[1]) ||
		(action == "create" && result.Material.Title != strings.TrimSpace(create.Title)) ||
		(action == "revise" && !materialCLIRevisionConfirmed(result, revision)) {
		return errors.New("material response is unconfirmed; retain the original request ID and input before retrying; no result was printed")
	}
	return cli.PrintJSON(cmd.OutOrStdout(), result)
}

func readMaterialCLIRevision(reader io.Reader) (material.RevisionInput, error) {
	var input material.RevisionInput
	raw, err := io.ReadAll(io.LimitReader(reader, materialCLIInputLimit+1))
	if err != nil {
		return input, fmt.Errorf("read material revision: %w", err)
	}
	if len(raw) > materialCLIInputLimit {
		return input, errors.New("material revision exceeds the 600 KiB input limit")
	}
	if !utf8.Valid(raw) {
		return input, errors.New("material revision JSON must be valid UTF-8")
	}
	fields, err := materialCLIObject(raw)
	if err != nil {
		return input, err
	}
	for _, key := range []string{"request_id", "expected_version", "kind", "content", "sources", "acceptance_criteria", "blocking_questions", "ready"} {
		if fields[key] == nil {
			return input, fmt.Errorf("material revision is missing %s", key)
		}
	}
	for key, value := range fields {
		switch key {
		case "request_id", "expected_version", "kind", "content", "sources", "acceptance_criteria", "blocking_questions", "ready", "plan_content", "source_baseline_id":
		default:
			return input, errors.New("material revision contains an unknown field")
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return input, errors.New("material revision fields cannot be null; omit unused optional fields")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || !materialCLIUUID(input.RequestID) || input.ExpectedVersion < 1 || input.ExpectedVersion > 9007199254740991 ||
		input.Sources == nil || input.AcceptanceCriteria == nil || input.BlockingQuestions == nil {
		return input, errors.New("invalid material revision JSON; use the exact RevisionInput fields and explicit arrays")
	}
	var sources []json.RawMessage
	if json.Unmarshal(fields["sources"], &sources) != nil {
		return input, errors.New("invalid material sources array")
	}
	for i, source := range sources {
		object, err := materialCLIObject(source)
		if err != nil || len(object) != 3 || object["resource_id"] == nil || object["commit"] == nil || object["path"] == nil ||
			!materialCLIUUID(input.Sources[i].ResourceID) || !material.IsSHA(input.Sources[i].Commit) || !material.ValidPath(input.Sources[i].Path, false) {
			return input, errors.New("each source requires a resource UUID, full commit SHA and relative file path")
		}
	}
	for _, name := range []string{"acceptance_criteria", "blocking_questions"} {
		var items []*string
		if json.Unmarshal(fields[name], &items) != nil {
			return input, errors.New("material criteria and questions must be arrays of strings")
		}
		for _, item := range items {
			if item == nil {
				return input, errors.New("material criteria and questions cannot contain null")
			}
		}
	}
	if input.SourceBaselineID != nil && !materialCLIUUID(*input.SourceBaselineID) {
		return input, errors.New("source_baseline_id must be a canonical nonzero UUID")
	}
	if err = material.ValidateRevision(input); err != nil {
		return input, fmt.Errorf("invalid material revision: %w", err)
	}
	return input, nil
}

// Reject duplicate keys and trailing JSON before decoding the typed boundary.
func materialCLIObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errors.New("material JSON must contain one object")
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || fields[name] != nil {
			return nil, errors.New("material JSON contains an invalid or duplicate field")
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, errors.New("invalid material JSON field")
		}
		fields[name] = value
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return nil, errors.New("invalid material JSON object")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, errors.New("material JSON contains trailing data")
	}
	return fields, nil
}

func materialCLIDecodeResponse(raw []byte, out any, required ...string) error {
	fields, err := materialCLIObject(raw)
	if err != nil {
		return err
	}
	for _, key := range required {
		if fields[key] == nil || (key != "baseline" && key != "plan" && bytes.Equal(bytes.TrimSpace(fields[key]), []byte("null"))) {
			return errors.New("material response is missing a required field")
		}
	}
	if capabilities := fields["capabilities"]; capabilities != nil {
		object, err := materialCLIObject(capabilities)
		if err != nil {
			return err
		}
		for _, key := range []string{"can_edit", "can_review", "can_start_plan", "can_retry"} {
			var value *bool
			if json.Unmarshal(object[key], &value) != nil || value == nil {
				return errors.New("invalid material response capabilities")
			}
		}
	}
	return json.Unmarshal(raw, out)
}

func validMaterialCLIItem(item material.Material, workspaceID, projectID string) bool {
	return materialCLIUUID(item.ID) && item.WorkspaceID == workspaceID && item.ProjectID == projectID && item.Version > 0 &&
		item.Status != "" && item.Title != "" && material.ValidPath(item.Path, false) && !item.CreatedAt.IsZero() && !item.UpdatedAt.IsZero()
}

func validMaterialCLIDetail(detail material.Detail, workspaceID, projectID string) bool {
	if !validMaterialCLIItem(detail.Material, workspaceID, projectID) || detail.Revisions == nil || detail.Decisions == nil || detail.Comments == nil {
		return false
	}
	revisions := map[string]material.Revision{}
	for _, revision := range detail.Revisions {
		if !materialCLIUUID(revision.ID) || revisions[revision.ID].ID != "" || revision.Kind == "" || revision.PublicationStatus == "" ||
			revision.ContentSHA256 != material.Digest(revision.Content) || revision.CreatedAt.IsZero() || revision.Sources == nil ||
			revision.AcceptanceCriteria == nil || revision.BlockingQuestions == nil || (revision.HeadSHA != nil && !material.IsSHA(*revision.HeadSHA)) {
			return false
		}
		revisions[revision.ID] = revision
	}
	for _, decision := range detail.Decisions {
		if !materialCLIUUID(decision.ID) || revisions[decision.RevisionID].ID == "" || !materialCLIUUID(decision.ActorID) || !material.IsSHA(decision.HeadSHA) || decision.Decision == "" || decision.CreatedAt.IsZero() {
			return false
		}
	}
	for _, comment := range detail.Comments {
		if !materialCLIUUID(comment.ID) || revisions[comment.RevisionID].ID == "" || !materialCLIUUID(comment.ActorID) || comment.CreatedAt.IsZero() {
			return false
		}
	}
	if baseline := detail.Baseline; baseline != nil {
		if !materialCLIUUID(baseline.ID) || !material.IsSHA(baseline.Commit) || revisions[baseline.RevisionID].ID == "" || baseline.ContentSHA256 != revisions[baseline.RevisionID].ContentSHA256 || baseline.VerifiedAt.IsZero() || !material.ValidPath(baseline.Path, false) {
			return false
		}
	}
	if plan := detail.Plan; plan != nil {
		if !materialCLIUUID(plan.BaselineID) || plan.Status == "" || (plan.IssueID != nil && !materialCLIUUID(*plan.IssueID)) || (plan.RevisionID != nil && revisions[*plan.RevisionID].ID == "") {
			return false
		}
	}
	return true
}

func materialCLIRevisionConfirmed(detail material.Detail, input material.RevisionInput) bool {
	if detail.Material.Version <= input.ExpectedVersion {
		return false
	}
	for _, revision := range detail.Revisions {
		if revision.Kind == input.Kind && revision.Content == input.Content && reflect.DeepEqual(revision.PlanContent, input.PlanContent) &&
			reflect.DeepEqual(revision.Sources, input.Sources) && reflect.DeepEqual(revision.AcceptanceCriteria, input.AcceptanceCriteria) &&
			reflect.DeepEqual(revision.BlockingQuestions, input.BlockingQuestions) && reflect.DeepEqual(revision.SourceBaselineID, input.SourceBaselineID) {
			return true
		}
	}
	return false
}

// The ordinary client owns context and transport. This boundary additionally
// needs whole-body limits and a single request even across 307/308 redirects.
func materialCLIRequest(ctx context.Context, client *cli.APIClient, method, path, key string, input any) ([]byte, error) {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, client.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.GetBody = nil
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Capabilities", "stable_attachment_urls")
	for name, value := range map[string]string{"X-Workspace-ID": client.WorkspaceID, "X-Agent-ID": client.AgentID, "X-Task-ID": client.TaskID,
		"X-Client-Platform": cli.ClientPlatform, "X-Client-Version": cli.ClientVersion, "X-Client-OS": cli.ClientOS} {
		if value != "" {
			req.Header.Set(name, value)
		}
	}
	if client.Token != "" {
		req.Header.Set("Authorization", "Bearer "+client.Token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	transport := *client.HTTPClient
	transport.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := transport.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && !(method == http.MethodPost && response.StatusCode == http.StatusCreated) {
		message := "material request failed"
		if method == http.MethodPost {
			message = "material request unconfirmed; retain the original request ID and input before retrying"
		}
		return nil, &cli.HTTPError{Method: method, Path: path, StatusCode: response.StatusCode, Body: message, TaskScoped: strings.HasPrefix(client.Token, cli.TaskTokenPrefix)}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, materialCLIResponseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > materialCLIResponseLimit {
		return nil, errors.New("material response exceeds the 16 MiB limit; no result was printed")
	}
	return raw, nil
}
