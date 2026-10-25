package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	CollaborationTokenEnv   = "MULTICA_COLLABORATION_TOKEN"
	CollaborationServerEnv  = "MULTICA_COLLABORATION_SERVER"
	CollaborationContextEnv = "MULTICA_COLLABORATION_CONTEXT"
	CollaborationIssueEnv   = "MULTICA_COLLABORATION_ISSUE_ID"
	CollaborationRunEnv     = "MULTICA_COLLABORATION_RUN_ID"
)

type CollaborationContext struct {
	ProtocolVersion int
	Containment     string
	Identity        protocol.CollaborationRunIdentity
	Server          string
	Token           string
}

// Host runner configuration is not a provider identity. Only these five
// injected values select the narrow command surface, including partial input.
func HasCollaborationContext() bool {
	for _, name := range []string{CollaborationTokenEnv, CollaborationServerEnv, CollaborationContextEnv, CollaborationIssueEnv, CollaborationRunEnv} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}

func ReadCollaborationContext() (*CollaborationContext, error) {
	if !HasCollaborationContext() {
		return nil, nil
	}
	invalid := errors.New("incomplete or invalid collaboration run identity; use the identity supplied by the admitted runtime")
	value := &CollaborationContext{Server: os.Getenv(CollaborationServerEnv), Token: os.Getenv(CollaborationTokenEnv)}
	raw := os.Getenv(CollaborationContextEnv)
	if len(raw) > 4096 || decodeCollaborationContext(raw, value) != nil ||
		!strings.HasPrefix(value.Token, "mcb_") || len(value.Token) != 68 {
		return nil, invalid
	}
	for _, c := range value.Token[4:] {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return nil, invalid
		}
	}
	id := value.Identity
	for _, s := range []string{id.DeploymentID, id.InstallationID, id.IssueID, id.SessionID, id.TargetID, id.ChannelID, id.BatchID, id.GroupID, id.RunID, id.LaunchNonce} {
		parsed, err := uuid.Parse(s)
		if err != nil || parsed == uuid.Nil || parsed.String() != s {
			return nil, invalid
		}
	}
	for _, s := range []string{id.Fence, id.ClaimGeneration} {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 1 || strconv.FormatInt(n, 10) != s {
			return nil, invalid
		}
	}
	if os.Getenv(CollaborationIssueEnv) != id.IssueID || os.Getenv(CollaborationRunEnv) != id.RunID {
		return nil, invalid
	}
	server, err := url.Parse(value.Server)
	if err != nil || (server.Scheme != "http" && server.Scheme != "https") || server.Host == "" || server.User != nil || server.RawQuery != "" || server.ForceQuery || server.Fragment != "" {
		return nil, invalid
	}
	value.Server = strings.TrimRight(value.Server, "/")
	return value, nil
}

func collaborationObjectFields(raw string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(strings.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errors.New("invalid collaboration identity object")
	}
	fields := make(map[string]json.RawMessage)
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if _, exists := fields[name]; !ok || exists {
			return nil, errors.New("duplicate collaboration identity field")
		}
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return nil, err
		}
		fields[name] = value
	}
	if end, err := d.Token(); err != nil || end != json.Delim('}') || d.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid collaboration identity object")
	}
	return fields, nil
}

func decodeCollaborationIdentity(raw string, out *protocol.CollaborationRunIdentity) error {
	fields, err := collaborationObjectFields(raw)
	if err != nil {
		return err
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

type hostCollaborationContextWire struct {
	ProtocolVersion int             `json:"protocol_version"`
	Containment     string          `json:"containment"`
	Identity        json.RawMessage `json:"identity"`
}

func decodeCollaborationContext(raw string, out *CollaborationContext) error {
	fields, err := collaborationObjectFields(raw)
	if err != nil {
		return err
	}
	if _, versioned := fields["protocol_version"]; !versioned {
		out.ProtocolVersion, out.Containment = protocol.CollaborationRuntimeProtocolVersion, protocol.CollaborationContainmentLinuxPIDNamespace
		return decodeCollaborationIdentity(raw, &out.Identity)
	}
	var wire hostCollaborationContextWire
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if len(fields) != 3 || d.Decode(&wire) != nil || wire.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion ||
		wire.Containment != protocol.CollaborationContainmentHostProcessSupervision || len(wire.Identity) == 0 {
		return errors.New("unsupported collaboration execution context")
	}
	if err := decodeCollaborationIdentity(string(wire.Identity), &out.Identity); err != nil {
		return err
	}
	out.ProtocolVersion, out.Containment = wire.ProtocolVersion, wire.Containment
	return nil
}

type CollaborationRuntimeComment struct {
	ID         string  `json:"id"`
	AuthorType string  `json:"author_type"`
	AuthorID   string  `json:"author_id"`
	Content    string  `json:"content"`
	ParentID   *string `json:"parent_id"`
	CreatedAt  string  `json:"created_at"`
}

type CollaborationRuntimeContext struct {
	Identity    protocol.CollaborationRunIdentity `json:"identity"`
	IssueID     string                            `json:"issue_id"`
	Title       string                            `json:"title"`
	Description string                            `json:"description"`
	AgentID     string                            `json:"agent_id"`
	SquadID     *string                           `json:"squad_id"`
	StepKind    string                            `json:"step_kind"`
	Roster      []struct {
		AgentID string `json:"agent_id"`
		Name    string `json:"name"`
		Role    string `json:"role"`
	} `json:"roster"`
	Inputs          []protocol.CollaborationRunInput `json:"inputs"`
	Comments        []CollaborationRuntimeComment    `json:"comments"`
	DispatchComment string                           `json:"dispatch_comment"`
}

func (c *CollaborationContext) Path(suffix string) string {
	return "/api/collaboration/runtime/runs/" + c.Identity.RunID + suffix
}

func (c *CollaborationContext) Client() *APIClient {
	client := NewAPIClient(c.Server, "", c.Token)
	client.HTTPClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("collaboration capability cannot follow redirects")
	}
	return client
}

func (c *CollaborationContext) Read(ctx context.Context, client *APIClient) (CollaborationRuntimeContext, error) {
	var out CollaborationRuntimeContext
	if err := client.GetJSON(ctx, c.Path("/context"), &out); err != nil {
		return out, CollaborationError(err, "")
	}
	if out.Identity != c.Identity || out.IssueID != c.Identity.IssueID {
		return out, errors.New("collaboration response does not match the admitted run and issue")
	}
	return out, nil
}

func CollaborationError(err error, requestID string) error {
	if err == nil {
		return nil
	}
	var response *HTTPError
	if errors.As(err, &response) {
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return WithUserMessage("Collaboration run capability rejected. Stop this turn; only its admitted runtime can provide authority.", err)
		}
		if response.StatusCode < 500 {
			return err
		}
	}
	if requestID != "" {
		return WithUserMessage("Collaboration write result is unknown. Retry the unchanged command with --request-id "+requestID+".", err)
	}
	return err
}

// Encode the admitted identity once for the isolated provider's environment.
// It contains no managed or termination credential.
func CollaborationEnvironment(identity protocol.CollaborationRunIdentity, server, token string) ([]string, error) {
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(identity); err != nil {
		return nil, err
	}
	return []string{CollaborationTokenEnv + "=" + token, CollaborationServerEnv + "=" + server,
		CollaborationContextEnv + "=" + strings.TrimSpace(data.String()), CollaborationIssueEnv + "=" + identity.IssueID, CollaborationRunEnv + "=" + identity.RunID}, nil
}

func HostCollaborationEnvironment(identity protocol.CollaborationRunIdentity, server, token string) ([]string, error) {
	raw, err := json.Marshal(identity)
	if err != nil {
		return nil, err
	}
	context, err := json.Marshal(hostCollaborationContextWire{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion,
		Containment: protocol.CollaborationContainmentHostProcessSupervision, Identity: raw})
	if err != nil {
		return nil, err
	}
	return []string{CollaborationTokenEnv + "=" + token, CollaborationServerEnv + "=" + server, CollaborationContextEnv + "=" + string(context),
		CollaborationIssueEnv + "=" + identity.IssueID, CollaborationRunEnv + "=" + identity.RunID}, nil
}
