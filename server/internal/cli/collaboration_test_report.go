package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (c *CollaborationContext) ReadTestSubject(ctx context.Context, client *APIClient) (protocol.CollaborationTestSubjectResponse, error) {
	var raw json.RawMessage
	if err := client.GetJSON(ctx, c.Path("/test-subject"), &raw); err != nil {
		return protocol.CollaborationTestSubjectResponse{}, CollaborationError(err, "")
	}
	out, err := protocol.DecodeCollaborationTestSubjectResponse(raw)
	if err != nil || out.IssueID != c.Identity.IssueID || out.RunID != c.Identity.RunID {
		return protocol.CollaborationTestSubjectResponse{}, errors.New("test subject does not match the admitted run and issue")
	}
	return out, nil
}

func (c *CollaborationContext) SubmitTestReport(ctx context.Context, client *APIClient, request string, report protocol.CollaborationTestReport) (protocol.CollaborationTestReportReceipt, error) {
	p := protocol.CollaborationTestReportRequest{Version: 1, Identity: c.Identity, RequestID: request, Report: report}
	if protocol.ValidateCollaborationTestReportRequest(p) != nil {
		return protocol.CollaborationTestReportReceipt{}, errors.New("invalid test report or request UUID")
	}
	var raw json.RawMessage
	if err := client.PostJSON(ctx, c.Path("/test-reports"), p, &raw); err != nil {
		return protocol.CollaborationTestReportReceipt{}, CollaborationError(err, request)
	}
	out, err := protocol.DecodeCollaborationTestReportReceipt(raw)
	if err != nil || out.IssueID != c.Identity.IssueID || out.RunID != c.Identity.RunID || out.RequestID != request {
		return protocol.CollaborationTestReportReceipt{}, WithUserMessage(fmt.Sprintf("Test report result is unconfirmed. Keep --request-id %s and the original report. Run `multica collaboration test-report receipt %s --request-id %s`; a 404 remains unconfirmed.", request, c.Identity.IssueID, request), errors.New("invalid test report receipt"))
	}
	return out, nil
}

func (c *CollaborationContext) ReadTestReportReceipt(ctx context.Context, client *APIClient, request string) (protocol.CollaborationTestReportReceipt, error) {
	id, err := uuid.Parse(request)
	if err != nil || id == uuid.Nil || id.String() != request {
		return protocol.CollaborationTestReportReceipt{}, errors.New("receipt requires the original --request-id UUID")
	}
	var raw json.RawMessage
	if err := client.GetJSON(ctx, c.Path("/test-reports/by-request/"+request), &raw); err != nil {
		return protocol.CollaborationTestReportReceipt{}, CollaborationError(err, request)
	}
	out, err := protocol.DecodeCollaborationTestReportReceipt(raw)
	if err != nil || out.IssueID != c.Identity.IssueID || out.RunID != c.Identity.RunID || out.RequestID != request {
		return protocol.CollaborationTestReportReceipt{}, CollaborationError(errors.New("invalid test report receipt"), request)
	}
	return out, nil
}
