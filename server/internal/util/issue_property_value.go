package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	maxPropertyTextValueLen = 2000
	maxPropertyURLValueLen  = 2048
	// MaxIssuePropertyActorValues bounds one field within the shared 16 KiB value bag.
	MaxIssuePropertyActorValues = 20
)

type IssuePropertyOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type IssuePropertyConfig struct {
	Options []IssuePropertyOption `json:"options,omitempty"`
}

func ParseIssuePropertyConfig(raw []byte) IssuePropertyConfig {
	var cfg IssuePropertyConfig
	if len(raw) == 0 {
		return cfg
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return IssuePropertyConfig{}
	}
	return cfg
}

func IssuePropertyOptionIDs(cfg IssuePropertyConfig) map[string]int {
	ids := make(map[string]int, len(cfg.Options))
	for i, opt := range cfg.Options {
		ids[opt.ID] = i
	}
	return ids
}

func selectOptionsHint(cfg IssuePropertyConfig) string {
	parts := make([]string, len(cfg.Options))
	for i, opt := range cfg.Options {
		parts[i] = fmt.Sprintf("%s (%s)", opt.ID, opt.Name)
	}
	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// Actor values (MUL-6286)
// ---------------------------------------------------------------------------

// actorPropertyKinds is the V1 value range for actor properties: workspace
// members only. The issue assignee also accepts "agent" and "squad", but
// neither belongs in a passive reference yet — an agent reference drags in the
// whole agent-visibility question (private / non-allow-listed agents must not
// become discoverable by id) for no demonstrated use case, and a squad is a
// routing target rather than a person.
//
// The stored form is "<kind>:<uuid>", so widening this list is a one-line
// change: no migration, no new property type, and existing definitions gain
// the new kind in place. Anything added here that is NOT universally visible
// to every workspace member (an agent, for one) must also restore a visibility
// gate on both the write path and the table-facet read path.
var actorPropertyKinds = []string{"member"}

// IssuePropertyActorRef is a parsed "<kind>:<uuid>" property value.
type IssuePropertyActorRef struct {
	Kind string
	ID   string
}

func (a IssuePropertyActorRef) String() string { return a.Kind + ":" + a.ID }

func IssuePropertyTypeIsActor(t string) bool {
	return t == "actor" || t == "multi_actor"
}

func actorKindsHint() string {
	return strings.Join(actorPropertyKinds, " / ")
}

// ParseIssuePropertyActorRef splits a stored actor value. Members are referenced by
// user_id — the same id the assignee pair uses — so "who is this" resolves
// identically everywhere in the product.
func ParseIssuePropertyActorRef(s string) (IssuePropertyActorRef, error) {
	kind, id, found := strings.Cut(s, ":")
	if !found {
		return IssuePropertyActorRef{}, fmt.Errorf("value must look like \"<kind>:<uuid>\" where kind is one of: %s", actorKindsHint())
	}
	valid := false
	for _, k := range actorPropertyKinds {
		if kind == k {
			valid = true
			break
		}
	}
	if !valid {
		return IssuePropertyActorRef{}, fmt.Errorf("unknown actor kind %q; valid kinds: %s", kind, actorKindsHint())
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return IssuePropertyActorRef{}, fmt.Errorf("actor id in %q must be a UUID", s)
	}
	// Store the canonical lowercase-hyphenated form. uuid.Parse also accepts
	// uppercase, braces and the urn: prefix; every consumer downstream (the
	// member directory lookup in the client, the "= me" filter, @> containment)
	// compares reference strings exactly, so an unnormalized id would store
	// fine and then render as Unknown and never match a filter.
	return IssuePropertyActorRef{Kind: kind, ID: parsed.String()}, nil
}

// ParseIssuePropertyActorRefList validates a multi_actor array: every element must parse,
// duplicates are dropped, and the caller's order is preserved. Unlike
// multi_select there is no config order to canonicalize against, and sorting
// by id would make the avatar row reshuffle on every edit. @> containment is
// order-insensitive, so filtering is unaffected either way.
func ParseIssuePropertyActorRefList(items []any) ([]IssuePropertyActorRef, error) {
	if len(items) == 0 {
		return nil, errors.New("value must be a non-empty array of actor references")
	}
	if len(items) > MaxIssuePropertyActorValues {
		return nil, fmt.Errorf("value cannot list more than %d actors", MaxIssuePropertyActorValues)
	}
	seen := make(map[string]struct{}, len(items))
	refs := make([]IssuePropertyActorRef, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, errors.New("value must be an array of actor reference strings")
		}
		ref, err := ParseIssuePropertyActorRef(s)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[ref.String()]; dup {
			continue
		}
		seen[ref.String()] = struct{}{}
		refs = append(refs, ref)
	}
	return refs, nil
}

// IssuePropertyActorRefsInValue re-reads the canonical stored JSON for an actor property.
// SetIssueProperty uses it to resolve references against the workspace after
// the pure shape validation has run.
func IssuePropertyActorRefsInValue(propType string, stored []byte) ([]IssuePropertyActorRef, error) {
	if propType == "actor" {
		var s string
		if err := json.Unmarshal(stored, &s); err != nil {
			return nil, err
		}
		ref, err := ParseIssuePropertyActorRef(s)
		if err != nil {
			return nil, err
		}
		return []IssuePropertyActorRef{ref}, nil
	}
	var list []string
	if err := json.Unmarshal(stored, &list); err != nil {
		return nil, err
	}
	refs := make([]IssuePropertyActorRef, 0, len(list))
	for _, s := range list {
		ref, err := ParseIssuePropertyActorRef(s)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// ValidateIssuePropertyValue checks a raw JSON value against the definition's type
// and returns the canonical JSON to store. Error strings enumerate the legal
// values where possible — agents consume these directly to self-correct.
func ValidateIssuePropertyValue(def db.IssueProperty, raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("value is required")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("value must be valid JSON: %w", err)
	}
	if v == nil {
		return nil, errors.New("value cannot be null (use DELETE to unset a property)")
	}

	cfg := ParseIssuePropertyConfig(def.Config)
	switch def.Type {
	case "text":
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("value must be a string")
		}
		if strings.TrimSpace(s) == "" {
			return nil, errors.New("value cannot be empty (use DELETE to unset a property)")
		}
		if utf8.RuneCountInString(s) > maxPropertyTextValueLen {
			return nil, fmt.Errorf("value must be %d characters or fewer", maxPropertyTextValueLen)
		}
		return json.Marshal(SanitizeTextForPostgres(s))
	case "url":
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("value must be a URL string")
		}
		s = strings.TrimSpace(s)
		if len(s) > maxPropertyURLValueLen {
			return nil, fmt.Errorf("value must be %d characters or fewer", maxPropertyURLValueLen)
		}
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, errors.New("value must be an http(s) URL")
		}
		return json.Marshal(s)
	case "number":
		if _, ok := v.(float64); !ok {
			return nil, errors.New("value must be a number")
		}
		return json.Marshal(v)
	case "checkbox":
		if _, ok := v.(bool); !ok {
			return nil, errors.New("value must be true or false")
		}
		return json.Marshal(v)
	case "date":
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("value must be a date string in YYYY-MM-DD format")
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, errors.New("value must be a date string in YYYY-MM-DD format")
		}
		return json.Marshal(s)
	case "select":
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("value must be one of the option ids: %s", selectOptionsHint(cfg))
		}
		if _, exists := IssuePropertyOptionIDs(cfg)[s]; !exists {
			return nil, fmt.Errorf("value must be one of the option ids: %s", selectOptionsHint(cfg))
		}
		return json.Marshal(s)
	case "multi_select":
		items, ok := v.([]any)
		if !ok || len(items) == 0 {
			return nil, fmt.Errorf("value must be a non-empty array of option ids: %s", selectOptionsHint(cfg))
		}
		order := IssuePropertyOptionIDs(cfg)
		seen := make(map[string]struct{}, len(items))
		ids := make([]string, 0, len(items))
		for _, item := range items {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("value must be a non-empty array of option ids: %s", selectOptionsHint(cfg))
			}
			if _, exists := order[s]; !exists {
				return nil, fmt.Errorf("unknown option id %q; valid option ids: %s", s, selectOptionsHint(cfg))
			}
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			ids = append(ids, s)
		}
		// Canonicalize to config order so equal selections serialize equally
		// (stable @> containment filtering and change detection).
		sort.SliceStable(ids, func(a, b int) bool { return order[ids[a]] < order[ids[b]] })
		return json.Marshal(ids)
	case "actor":
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("value must be an actor reference string like \"member:<uuid>\" (kinds: %s)", actorKindsHint())
		}
		ref, err := ParseIssuePropertyActorRef(s)
		if err != nil {
			return nil, err
		}
		return json.Marshal(ref.String())
	case "multi_actor":
		items, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("value must be an array of actor reference strings like \"member:<uuid>\" (kinds: %s)", actorKindsHint())
		}
		refs, err := ParseIssuePropertyActorRefList(items)
		if err != nil {
			return nil, err
		}
		out := make([]string, len(refs))
		for i, ref := range refs {
			out[i] = ref.String()
		}
		return json.Marshal(out)
	default:
		return nil, fmt.Errorf("unsupported property type %q", def.Type)
	}
}
