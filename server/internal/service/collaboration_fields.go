package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationFieldLabel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CollaborationPropertyDefinition struct {
	ID       string                   `json:"id"`
	Name     string                   `json:"name"`
	Type     string                   `json:"type"`
	Icon     string                   `json:"icon"`
	Config   util.IssuePropertyConfig `json:"config"`
	Archived bool                     `json:"archived"`
}

type CollaborationFieldOptions struct {
	IssueID               string                            `json:"issue_id"`
	SourceWorkspaceID     string                            `json:"source_workspace_id"`
	CollaborationRevision int64                             `json:"collaboration_revision,string"`
	Labels                []CollaborationFieldLabel         `json:"labels"`
	Properties            []CollaborationPropertyDefinition `json:"properties"`
}

type CollaborationIssueFields struct {
	Labels              []CollaborationFieldLabel         `json:"labels"`
	Properties          map[string]json.RawMessage        `json:"properties"`
	PropertyDefinitions []CollaborationPropertyDefinition `json:"property_definitions"`
}

// FieldOptions is an editing projection authorized by the source issue, not a
// workspace directory. Authority and the catalog share a repeatable snapshot;
// the edit itself rechecks selected definitions under its write locks.
func (s *CollaborationService) FieldOptions(ctx context.Context, issueID pgtype.UUID) (CollaborationFieldOptions, error) {
	var out CollaborationFieldOptions
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return out, collaborationError(http.StatusNotFound, "not_found")
	}
	if !issueID.Valid {
		return out, collaborationError(http.StatusBadRequest, "invalid_issue_id")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return out, err
	}
	q := s.Queries.WithTx(tx)
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	if err != nil {
		return out, err
	}
	if !access.Capabilities.Edit {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	labels, err := q.ListCollaborationFieldLabels(ctx, access.Issue.WorkspaceID)
	if err != nil {
		return out, err
	}
	definitions, err := q.ListCollaborationFieldProperties(ctx, db.ListCollaborationFieldPropertiesParams{WorkspaceID: access.Issue.WorkspaceID})
	if err != nil {
		return out, err
	}
	out = CollaborationFieldOptions{IssueID: util.UUIDToString(issueID), SourceWorkspaceID: util.UUIDToString(access.Issue.WorkspaceID),
		CollaborationRevision: access.Revision, Labels: make([]CollaborationFieldLabel, len(labels)), Properties: make([]CollaborationPropertyDefinition, 0, len(definitions))}
	for i, label := range labels {
		out.Labels[i] = CollaborationFieldLabel{ID: util.UUIDToString(label.ID), Name: label.Name, Color: label.Color}
	}
	for _, definition := range definitions {
		projected, err := collaborationPropertyDefinition(definition)
		if err != nil {
			return CollaborationFieldOptions{}, err
		}
		out.Properties = append(out.Properties, projected)
	}
	if err = tx.Commit(ctx); err != nil {
		return CollaborationFieldOptions{}, err
	}
	return out, nil
}

// IssueFields consumes the already-authorized issue in the caller's read
// snapshot. Historical values keep archived definitions and departed actor
// references; they do not grant fresh access to a source member directory.
func (s *CollaborationService) IssueFields(ctx context.Context, access CollaborationIssueAccess) (CollaborationIssueFields, error) {
	var out CollaborationIssueFields
	if !access.Capabilities.Read {
		return out, collaborationError(http.StatusNotFound, "not_found")
	}
	var err error
	out.Properties, err = collaborationPropertyBag(access.Issue.Properties)
	if err != nil {
		return out, err
	}
	labels, err := s.Queries.ListLabelsByIssue(ctx, db.ListLabelsByIssueParams{IssueID: access.Issue.ID, WorkspaceID: access.Issue.WorkspaceID})
	if err != nil {
		return out, err
	}
	out.Labels = collaborationFieldLabels(labels)
	out.PropertyDefinitions = make([]CollaborationPropertyDefinition, 0, len(out.Properties))
	if len(out.Properties) == 0 {
		return out, nil
	}
	definitions, err := s.Queries.ListCollaborationFieldProperties(ctx, db.ListCollaborationFieldPropertiesParams{WorkspaceID: access.Issue.WorkspaceID, IncludeArchived: true})
	if err != nil {
		return out, err
	}
	for _, definition := range definitions {
		if _, referenced := out.Properties[util.UUIDToString(definition.ID)]; !referenced {
			continue
		}
		projected, err := collaborationPropertyDefinition(definition)
		if err != nil {
			return CollaborationIssueFields{}, err
		}
		out.PropertyDefinitions = append(out.PropertyDefinitions, projected)
	}
	if len(out.PropertyDefinitions) != len(out.Properties) {
		return CollaborationIssueFields{}, errors.New("collaboration property definition unavailable")
	}
	return out, nil
}

func collaborationPropertyDefinition(def db.IssueProperty) (CollaborationPropertyDefinition, error) {
	var config *util.IssuePropertyConfig
	if err := json.Unmarshal(def.Config, &config); err != nil || config == nil {
		return CollaborationPropertyDefinition{}, errors.New("invalid collaboration property configuration")
	}
	switch def.Type {
	case "select", "multi_select":
		seen := make(map[string]bool, len(config.Options))
		if len(config.Options) == 0 {
			return CollaborationPropertyDefinition{}, errors.New("collaboration property options unavailable")
		}
		for _, option := range config.Options {
			if option.ID == "" || seen[option.ID] {
				return CollaborationPropertyDefinition{}, errors.New("invalid collaboration property options")
			}
			seen[option.ID] = true
		}
	case "text", "number", "date", "checkbox", "url", "actor", "multi_actor":
		if len(config.Options) != 0 {
			return CollaborationPropertyDefinition{}, errors.New("unexpected collaboration property options")
		}
	default:
		return CollaborationPropertyDefinition{}, errors.New("unknown collaboration property type")
	}
	return CollaborationPropertyDefinition{ID: util.UUIDToString(def.ID), Name: def.Name, Type: def.Type,
		Icon: def.Icon, Config: *config, Archived: def.ArchivedAt.Valid}, nil
}

func collaborationFieldLabels(labels []db.IssueLabel) []CollaborationFieldLabel {
	out := make([]CollaborationFieldLabel, len(labels))
	for i, label := range labels {
		out[i] = CollaborationFieldLabel{ID: util.UUIDToString(label.ID), Name: label.Name, Color: label.Color}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func collaborationPropertyBag(raw []byte) (map[string]json.RawMessage, error) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, errors.New("invalid collaboration property values")
	}
	return values, nil
}

type collaborationFieldEdit struct {
	beforeLabels, afterLabels        []db.IssueLabel
	labelsChanged, propertiesChanged bool
}

func (t *collaborationWorkflowTx) prepareFieldEdit(ctx context.Context, p CollaborationEditParams, after *db.Issue) (collaborationFieldEdit, string, error) {
	var fields collaborationFieldEdit
	if p.Priority != nil {
		switch *p.Priority {
		case "none", "urgent", "high", "medium", "low":
			after.Priority = *p.Priority
		default:
			return fields, "invalid_priority", nil
		}
	}
	var err error
	fields.beforeLabels, err = t.q.ListLabelsByIssue(ctx, db.ListLabelsByIssueParams{IssueID: after.ID, WorkspaceID: after.WorkspaceID})
	if err != nil {
		return fields, "", err
	}
	fields.afterLabels = fields.beforeLabels
	if p.LabelIDs != nil {
		ids := make(map[pgtype.UUID]bool, len(*p.LabelIDs))
		for _, id := range *p.LabelIDs {
			if !id.Valid {
				return fields, "invalid_labels", nil
			}
			ids[id] = true
		}
		fields.afterLabels, err = t.q.LockCollaborationFieldLabels(ctx, db.LockCollaborationFieldLabelsParams{WorkspaceID: after.WorkspaceID, LabelIds: *p.LabelIDs})
		if err != nil {
			return fields, "", err
		}
		if len(fields.afterLabels) != len(ids) {
			return fields, "invalid_labels", nil
		}
		fields.labelsChanged = len(fields.beforeLabels) != len(ids)
		for _, label := range fields.beforeLabels {
			fields.labelsChanged = fields.labelsChanged || !ids[label.ID]
		}
	}
	if len(p.Properties) != 0 {
		changed, code, err := t.mergeFieldProperties(ctx, p.Properties, after)
		if err != nil || code != "" {
			return fields, code, err
		}
		fields.propertiesChanged = changed
	}
	return fields, "", nil
}

func (t *collaborationWorkflowTx) mergeFieldProperties(ctx context.Context, patch map[string]json.RawMessage, after *db.Issue) (bool, string, error) {
	values, err := collaborationPropertyBag(after.Properties)
	if err != nil {
		return false, "", err
	}
	canonical := make(map[string]json.RawMessage, len(patch))
	keys := make([]string, 0, len(patch))
	for key, value := range patch {
		id, err := util.ParseUUID(key)
		if err != nil {
			return false, "invalid_properties", nil
		}
		key = util.UUIDToString(id)
		if _, duplicate := canonical[key]; duplicate {
			return false, "invalid_properties", nil
		}
		keys = append(keys, key)
		canonical[key] = value
	}
	sort.Strings(keys)
	ids := make([]pgtype.UUID, 0, len(keys))
	for _, key := range keys {
		id, err := util.ParseUUID(key)
		if err != nil {
			return false, "invalid_properties", nil
		}
		locked, err := t.q.TryLockCollaborationFieldProperty(ctx, id)
		if err != nil {
			return false, "", err
		}
		if !locked {
			return false, "", &pgconn.PgError{Code: "55P03", Message: "collaboration property busy"}
		}
		ids = append(ids, id)
	}
	definitions, err := t.q.LockCollaborationFieldProperties(ctx, db.LockCollaborationFieldPropertiesParams{WorkspaceID: after.WorkspaceID, PropertyIds: ids})
	if err != nil {
		return false, "", err
	}
	if len(definitions) != len(keys) {
		return false, "invalid_properties", nil
	}
	actors := make(map[pgtype.UUID]bool)
	for _, definition := range definitions {
		key := util.UUIDToString(definition.ID)
		raw := canonical[key]
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			delete(values, key)
			continue
		}
		if definition.ArchivedAt.Valid {
			return false, "property_archived", nil
		}
		if _, err := collaborationPropertyDefinition(definition); err != nil {
			return false, "", err
		}
		value, err := util.ValidateIssuePropertyValue(definition, raw)
		if err != nil {
			// Ordinary errors contain private values/config hints. Collaboration
			// refusals retain only a bounded code in the response and receipt.
			return false, "invalid_property_value", nil
		}
		values[key] = value
		if util.IssuePropertyTypeIsActor(definition.Type) {
			refs, err := util.IssuePropertyActorRefsInValue(definition.Type, value)
			if err != nil {
				return false, "", err
			}
			for _, ref := range refs {
				id, err := util.ParseUUID(ref.ID)
				if err != nil {
					return false, "invalid_property_value", nil
				}
				actors[id] = true
			}
		}
	}
	if len(actors) > 0 {
		ids := make([]pgtype.UUID, 0, len(actors))
		for id := range actors {
			ids = append(ids, id)
		}
		members, err := t.q.LockCollaborationFieldMembers(ctx, db.LockCollaborationFieldMembersParams{WorkspaceID: after.WorkspaceID, UserIds: ids})
		if err != nil {
			return false, "", err
		}
		if len(members) != len(actors) {
			return false, "invalid_property_value", nil
		}
	}
	merged, err := json.Marshal(values)
	if err != nil {
		return false, "", err
	}
	check, err := t.q.CheckCollaborationProperties(ctx, db.CheckCollaborationPropertiesParams{Properties: merged, CurrentProperties: after.Properties})
	if err != nil {
		return false, "", err
	}
	if !check.Fits {
		return false, "properties_too_large", nil
	}
	if !check.Unchanged {
		after.Properties = merged
	}
	return !check.Unchanged, "", nil
}

func (t *collaborationWorkflowTx) replaceFieldLabels(ctx context.Context, labels []db.IssueLabel) error {
	i := t.access.Issue
	if err := t.q.ClearCollaborationIssueLabels(ctx, db.ClearCollaborationIssueLabelsParams{IssueID: i.ID, WorkspaceID: i.WorkspaceID}); err != nil {
		return err
	}
	if len(labels) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, len(labels))
	for n, label := range labels {
		ids[n] = label.ID
	}
	inserted, err := t.q.InsertCollaborationIssueLabels(ctx, db.InsertCollaborationIssueLabelsParams{IssueID: i.ID, WorkspaceID: i.WorkspaceID, LabelIds: ids})
	if err != nil {
		return err
	}
	if inserted != int64(len(ids)) {
		return errors.New("collaboration label set changed during edit")
	}
	return nil
}
