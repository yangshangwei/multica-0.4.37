package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
)

const IssueBudgetUsageMaxBytes = 256 * 1024

var (
	ErrIssueBudgetUsageTooLarge = errors.New("budget usage request too large")
	issueBudgetBatchKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
	issueBudgetAdapterPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)
	issueBudgetVersionPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

type issueBudgetUsageBatch struct {
	ProtocolVersion   int32                        `json:"protocol_version"`
	BatchKey          string                       `json:"batch_key"`
	ProducerEpoch     string                       `json:"producer_epoch"`
	Adapter           string                       `json:"adapter"`
	AdapterVersion    string                       `json:"adapter_version,omitempty"`
	NormalizerVersion string                       `json:"normalizer_version"`
	ReportedAt        string                       `json:"reported_at"`
	Invocations       []issueBudgetUsageInvocation `json:"invocations"`
	PayloadDigest     string                       `json:"-"`
}

type issueBudgetUsageInvocation struct {
	InvocationID string                 `json:"invocation_id"`
	Resumed      bool                   `json:"resumed"`
	Ended        string                 `json:"ended"`
	ObservedAt   string                 `json:"observed_at"`
	Usage        []issueBudgetUsageItem `json:"usage"`
}

type issueBudgetUsageItem struct {
	Model     string             `json:"model"`
	Tokens    budget.TokenCounts `json:"tokens"`
	CostTicks *budget.Ticks      `json:"cost_ticks,omitempty"`
}

// DecodeIssueBudgetUsage checks the entire v1 wire contract before a transaction
// can start. Error paths use zero-based invocation and usage item indexes.
func DecodeIssueBudgetUsage(raw []byte) (issueBudgetUsageBatch, error) {
	fail := func(err error) (issueBudgetUsageBatch, error) { return issueBudgetUsageBatch{}, err }
	if len(raw) > IssueBudgetUsageMaxBytes {
		return fail(ErrIssueBudgetUsageTooLarge)
	}
	if !utf8.Valid(raw) {
		return fail(errors.New("request: invalid UTF-8"))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := checkIssueBudgetUsageJSON(decoder, reflect.TypeFor[issueBudgetUsageBatch](), ""); err != nil {
		return fail(err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fail(errors.New("request: trailing data"))
	}
	var batch issueBudgetUsageBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return fail(fmt.Errorf("request: %w", err))
	}
	if batch.ProtocolVersion != 1 {
		return fail(errors.New("protocol_version: must be 1"))
	}
	if !ValidIssueBudgetUsageBatchKey(batch.BatchKey) {
		return fail(errors.New("batch_key: invalid format"))
	}
	if !issueBudgetAdapterPattern.MatchString(batch.Adapter) {
		return fail(errors.New("adapter: invalid format"))
	}
	if !issueBudgetVersionPattern.MatchString(batch.ProducerEpoch) {
		return fail(errors.New("producer_epoch: invalid format"))
	}
	if !issueBudgetVersionPattern.MatchString(batch.NormalizerVersion) {
		return fail(errors.New("normalizer_version: invalid format"))
	}
	if len(batch.AdapterVersion) > 120 {
		return fail(errors.New("adapter_version: too long"))
	}
	if _, err := time.Parse(time.RFC3339, batch.ReportedAt); err != nil {
		return fail(errors.New("reported_at: invalid RFC3339 time"))
	}
	if len(batch.Invocations) > 32 {
		return fail(errors.New("invocations: exceeds 32 invocations"))
	}
	ids := make(map[string]bool, len(batch.Invocations))
	items := 0
	for i, inv := range batch.Invocations {
		path := fmt.Sprintf("invocations[%d]", i)
		id, err := util.ParseUUID(inv.InvocationID)
		if err != nil || util.UUIDToString(id) != inv.InvocationID {
			return fail(fmt.Errorf("%s.invocation_id: expected canonical lowercase UUID", path))
		}
		if ids[inv.InvocationID] {
			return fail(fmt.Errorf("%s.invocation_id: duplicate invocation", path))
		}
		ids[inv.InvocationID] = true
		if inv.Ended != "returned" && inv.Ended != "interrupted" && inv.Ended != "unknown" {
			return fail(fmt.Errorf("%s.ended: invalid outcome", path))
		}
		if _, err := time.Parse(time.RFC3339, inv.ObservedAt); err != nil {
			return fail(fmt.Errorf("%s.observed_at: invalid RFC3339 time", path))
		}
		if inv.Ended == "unknown" && len(inv.Usage) > 0 {
			return fail(fmt.Errorf("%s.usage: unknown outcome requires empty usage", path))
		}
		items += len(inv.Usage)
		if items > 256 {
			return fail(fmt.Errorf("%s.usage: exceeds 256 items", path))
		}
		models := make(map[string]bool, len(inv.Usage))
		for j, item := range inv.Usage {
			itemPath := fmt.Sprintf("%s.usage[%d]", path, j)
			if len(item.Model) == 0 || len(item.Model) > 120 {
				return fail(fmt.Errorf("%s.model: must contain 1 to 120 bytes", itemPath))
			}
			if models[item.Model] {
				return fail(fmt.Errorf("%s.model: duplicate model", itemPath))
			}
			models[item.Model] = true
			if item.CostTicks != nil && *item.CostTicks <= 0 {
				return fail(fmt.Errorf("%s.cost_ticks: must be positive", itemPath))
			}
			if item.CostTicks == nil && item.Tokens == (budget.TokenCounts{}) {
				return fail(fmt.Errorf("%s: empty usage", itemPath))
			}
		}
	}
	sum := sha256.Sum256(raw)
	batch.PayloadDigest = hex.EncodeToString(sum[:])
	return batch, nil
}

func ValidIssueBudgetUsageBatchKey(key string) bool {
	return issueBudgetBatchKeyPattern.MatchString(key)
}

// Walk the typed schema before unmarshalling: encoding/json alone accepts
// duplicate keys, case-folded keys and nulls for required scalar fields.
func checkIssueBudgetUsageJSON(d *json.Decoder, typ reflect.Type, path string) error {
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("%s: invalid JSON", path)
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	bad := func() error { return fmt.Errorf("%s: invalid type", path) }
	if typ == reflect.TypeFor[budget.Count]() || typ == reflect.TypeFor[budget.Ticks]() {
		text, ok := token.(string)
		if !ok {
			return bad()
		}
		if _, err := budget.ParseDecimal(text); err != nil {
			return fmt.Errorf("%s: invalid decimal", path)
		}
		return nil
	}
	switch typ.Kind() {
	case reflect.Struct:
		if token != json.Delim('{') {
			return bad()
		}
		fields := map[string]reflect.StructField{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name != "-" {
				fields[name] = f
			}
		}
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return fmt.Errorf("%s: invalid object key", path)
			}
			name, ok := key.(string)
			if !ok {
				return bad()
			}
			child := name
			if path != "" {
				child = path + "." + name
			}
			f, ok := fields[name]
			if !ok {
				return fmt.Errorf("%s: unknown field", child)
			}
			if seen[name] {
				return fmt.Errorf("%s: duplicate field", child)
			}
			seen[name] = true
			if err := checkIssueBudgetUsageJSON(d, f.Type, child); err != nil {
				return err
			}
		}
		if end, err := d.Token(); err != nil || end != json.Delim('}') {
			return bad()
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag := f.Tag.Get("json")
			name := strings.Split(tag, ",")[0]
			if name != "-" && !strings.Contains(tag, ",omitempty") && !seen[name] {
				child := name
				if path != "" {
					child = path + "." + name
				}
				return fmt.Errorf("%s: required field", child)
			}
		}
	case reflect.Slice:
		if token != json.Delim('[') {
			return bad()
		}
		for i := 0; d.More(); i++ {
			if err := checkIssueBudgetUsageJSON(d, typ.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		if end, err := d.Token(); err != nil || end != json.Delim(']') {
			return bad()
		}
	case reflect.String:
		text, ok := token.(string)
		if !ok {
			return bad()
		}
		if path == "adapter_version" && len(text) == 0 {
			return fmt.Errorf("%s: empty version", path)
		}
		if strings.IndexFunc(text, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s: control character", path)
		}
	case reflect.Bool:
		if _, ok := token.(bool); !ok {
			return bad()
		}
	case reflect.Int32:
		n, ok := token.(json.Number)
		if !ok {
			return bad()
		}
		if _, err := n.Int64(); err != nil {
			return bad()
		}
	default:
		return bad()
	}
	return nil
}
