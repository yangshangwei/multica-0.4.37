package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
)

type issueBudgetEnableBody struct {
	RequestKey          string        `json:"request_key"`
	Reason              string        `json:"reason"`
	LimitTicks          *budget.Ticks `json:"limit_ticks"`
	AttemptReserveTicks *budget.Ticks `json:"attempt_reserve_ticks"`
}

type issueBudgetOperationBody struct {
	Kind                   string       `json:"kind"`
	RequestKey             string       `json:"request_key"`
	Reason                 string       `json:"reason"`
	ExpectedPolicyRevision budget.Count `json:"expected_policy_revision"`
}

type issueBudgetSetLimitBody struct {
	issueBudgetOperationBody
	LimitTicks          *budget.Ticks `json:"limit_ticks"`
	AttemptReserveTicks *budget.Ticks `json:"attempt_reserve_ticks"`
}

type issueBudgetSetModeBody struct {
	issueBudgetOperationBody
	Mode string `json:"mode"`
}

type issueBudgetErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// decodeIssueBudgetBody checks exact field spelling and presence before typed
// decoding. Nullable amounts must be explicitly supplied; null on any other
// field is a malformed request, rather than the zero value of a Go field.
func decodeIssueBudgetBody(raw []byte, target any, fields ...string) error {
	invalid := &service.IssueBudgetError{Code: service.IssueBudgetInvalidRequest}
	if !utf8.Valid(raw) || iteration.ValidateObjectJSON(raw) != nil {
		return invalid
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || len(values) != len(fields) {
		return invalid
	}
	for _, field := range fields {
		value, exists := values[field]
		if !exists {
			return invalid
		}
		if field != "limit_ticks" && field != "attempt_reserve_ticks" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return invalid
		}
	}
	if json.Unmarshal(raw, target) != nil {
		return invalid
	}
	return nil
}

func readIssueBudgetBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, &service.IssueBudgetError{Code: service.IssueBudgetInvalidRequest}
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
	if err != nil {
		return nil, &service.IssueBudgetError{Code: service.IssueBudgetInvalidRequest}
	}
	return raw, nil
}

func parseIssueBudgetOperation(raw []byte) (issueBudgetSetLimitBody, string, error) {
	invalid := &service.IssueBudgetError{Code: service.IssueBudgetInvalidRequest}
	var kind struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(raw, &kind) != nil {
		return issueBudgetSetLimitBody{}, "", invalid
	}
	fields := []string{"kind", "request_key", "reason", "expected_policy_revision"}
	var out issueBudgetSetLimitBody
	var mode string
	var err error
	switch kind.Kind {
	case service.IssueBudgetOperationSetLimit:
		err = decodeIssueBudgetBody(raw, &out, append(fields, "limit_ticks", "attempt_reserve_ticks")...)
	case service.IssueBudgetOperationSetMode:
		var body issueBudgetSetModeBody
		err = decodeIssueBudgetBody(raw, &body, append(fields, "mode")...)
		out.issueBudgetOperationBody, mode = body.issueBudgetOperationBody, body.Mode
	case service.IssueBudgetOperationClose, service.IssueBudgetOperationReopen:
		err = decodeIssueBudgetBody(raw, &out.issueBudgetOperationBody, fields...)
	default:
		err = invalid
	}
	if err == nil {
		_, err = delegationKey(out.RequestKey)
	}
	if err != nil {
		return issueBudgetSetLimitBody{}, "", invalid
	}
	return out, mode, nil
}

func parseIssueBudgetPage(rawQuery string) (service.IssueBudgetPageParams, error) {
	out := service.IssueBudgetPageParams{Limit: 50}
	invalid := &service.IssueBudgetError{Code: service.IssueBudgetInvalidRequest}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return out, invalid
	}
	for name, values := range q {
		if len(values) != 1 {
			return out, invalid
		}
		switch name {
		case "limit":
			n, err := strconv.Atoi(values[0])
			if err != nil || strconv.Itoa(n) != values[0] {
				return out, invalid
			}
			out.Limit = n
		case "cursor":
			if values[0] == "" {
				return out, invalid
			}
			out.Cursor = values[0]
		}
	}
	return out, out.Validate()
}
