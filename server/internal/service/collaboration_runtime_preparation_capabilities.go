package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"reflect"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func decodeCollaborationPreparationJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if iteration.ValidateObjectJSON(raw) != nil || decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		return collaborationError(http.StatusServiceUnavailable, "runtime_preparation_unavailable")
	}
	return nil
}

func collaborationRegisteredCapabilities(runtime db.AgentRuntime) (*protocol.CollaborationRuntimeCapabilityRegistration, error) {
	if runtime.CollaborationCapabilities == nil {
		return nil, nil
	}
	var saved protocol.CollaborationRuntimeCapabilityRegistration
	if err := decodeCollaborationPreparationJSON(runtime.CollaborationCapabilities, &saved); err != nil {
		return nil, err
	}
	if protocol.ValidateCollaborationRuntimeCapabilities(saved.Capabilities) != nil {
		return nil, collaborationError(http.StatusServiceUnavailable, "runtime_preparation_unavailable")
	}
	return &saved, nil
}

func (s *CollaborationService) lockPreparationRuntime(ctx context.Context, q *db.Queries, runtimeID pgtype.UUID) (db.AgentRuntime, db.InstallationDaemonBinding, error) {
	runtime, binding, err := s.authorizeCollaborationDaemon(ctx, q, runtimeID)
	if err != nil {
		return runtime, binding, err
	}
	if _, err = LockManagedRuntime(ctx, s.collaborationRuntimeLookup(q), runtime); err != nil {
		return runtime, binding, err
	}
	locked, err := q.LockAgentRuntime(ctx, runtimeID)
	if err != nil {
		return runtime, binding, err
	}
	if locked.OwnerID != runtime.OwnerID || locked.WorkspaceID != runtime.WorkspaceID || locked.DaemonID != runtime.DaemonID {
		return runtime, binding, collaborationError(http.StatusConflict, "stale_runtime_binding")
	}
	return locked, binding, nil
}

func preparationCapabilityResponse(runtime db.AgentRuntime) (protocol.CollaborationRuntimeCapabilitiesResponse, error) {
	registration, err := collaborationRegisteredCapabilities(runtime)
	return protocol.CollaborationRuntimeCapabilitiesResponse{RuntimeID: util.UUIDToString(runtime.ID),
		RuntimeConfigEpoch: strconv.FormatInt(runtime.CollaborationConfigEpoch, 10), Registration: registration}, err
}

// Full-summary replacement is conditional on the caller's observed epoch.
// Current identical summaries are declarative no-ops, including lost responses;
// no read or repeated report may renew stale capabilities behind new consent.
func (s *CollaborationService) RegisterRuntimeCapabilities(ctx context.Context, p protocol.CollaborationRuntimeCapabilitiesRequest) (protocol.CollaborationRuntimeCapabilitiesResponse, error) {
	var empty protocol.CollaborationRuntimeCapabilitiesResponse
	expected, valid := collaborationRuntimeVersion(p.ExpectedRuntimeConfigEpoch, false)
	runtimeID, idErr := util.ParseUUID(p.RuntimeID)
	if p.ProtocolVersion != protocol.CollaborationRuntimeProtocolVersion || !valid || expected == math.MaxInt64 || idErr != nil ||
		protocol.ValidateCollaborationRuntimeCapabilities(p.Capabilities) != nil {
		return empty, collaborationError(http.StatusBadRequest, "invalid_runtime_capabilities")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRuntimeCapabilitiesResponse, error) {
		q := s.Queries.WithTx(tx)
		runtime, binding, err := s.lockPreparationRuntime(ctx, q, runtimeID)
		if err != nil {
			return empty, err
		}
		installation, err := q.GetManagedInstallation(ctx, binding.InstallationID)
		if err != nil {
			return empty, err
		}
		if p.DeploymentID != auth.ManagedDeploymentID() || p.DeploymentID != util.UUIDToString(installation.DeploymentID) ||
			p.InstallationID != util.UUIDToString(binding.InstallationID) || p.BindingID != util.UUIDToString(binding.ID) ||
			p.BindingEpoch != strconv.FormatInt(binding.BindingEpoch, 10) {
			return empty, collaborationError(http.StatusConflict, "stale_runtime_binding")
		}
		family := runtime.Provider
		if builtin, ok := agent.BuiltinRuntimeByID(runtime.Provider); ok {
			family = builtin.ProtocolFamily
		}
		if !agent.IsSupportedType(family) || runtime.ProfileID.Valid || p.Capabilities.Provider.Provider != runtime.Provider ||
			p.Capabilities.Provider.ProtocolFamily != family || installation.Lifecycle != "active" || installation.Os.String != p.Capabilities.OS {
			return empty, collaborationError(http.StatusConflict, "unsupported_runtime_configuration")
		}
		saved, err := collaborationRegisteredCapabilities(runtime)
		if err != nil {
			return empty, err
		}
		if saved != nil && saved.RuntimeConfigEpoch == strconv.FormatInt(runtime.CollaborationConfigEpoch, 10) &&
			saved.DeploymentID == p.DeploymentID && saved.InstallationID == p.InstallationID && saved.BindingID == p.BindingID &&
			saved.BindingEpoch == p.BindingEpoch && reflect.DeepEqual(saved.Capabilities, p.Capabilities) {
			return preparationCapabilityResponse(runtime)
		}
		if runtime.CollaborationConfigEpoch != expected {
			return empty, collaborationError(http.StatusConflict, "stale_runtime_configuration")
		}
		record := protocol.CollaborationRuntimeCapabilityRegistration{DeploymentID: p.DeploymentID, InstallationID: p.InstallationID,
			BindingID: p.BindingID, BindingEpoch: p.BindingEpoch, RuntimeConfigEpoch: strconv.FormatInt(expected+1, 10), Capabilities: p.Capabilities}
		encoded, err := json.Marshal(record)
		if err != nil {
			return empty, err
		}
		runtime, err = q.RegisterCollaborationRuntimeCapabilities(ctx, db.RegisterCollaborationRuntimeCapabilitiesParams{RuntimeID: runtimeID,
			ExpectedEpoch: expected, Capabilities: encoded})
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, collaborationError(http.StatusConflict, "stale_runtime_configuration")
		}
		if err != nil {
			return empty, err
		}
		return preparationCapabilityResponse(runtime)
	})
}

func (s *CollaborationService) RuntimeCapabilities(ctx context.Context, runtimeID pgtype.UUID) (protocol.CollaborationRuntimeCapabilitiesResponse, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRuntimeCapabilitiesResponse, error) {
		runtime, _, err := s.lockPreparationRuntime(ctx, s.Queries.WithTx(tx), runtimeID)
		if err != nil {
			return protocol.CollaborationRuntimeCapabilitiesResponse{}, err
		}
		return preparationCapabilityResponse(runtime)
	})
}

func currentCollaborationPreparationCapabilities(runtime db.AgentRuntime, installationID, bindingID pgtype.UUID, bindingEpoch int64) (*protocol.CollaborationRuntimeCapabilityRegistration, error) {
	saved, err := collaborationRegisteredCapabilities(runtime)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return nil, collaborationError(http.StatusConflict, "runtime_preparation_unsupported")
	}
	if saved.RuntimeConfigEpoch != strconv.FormatInt(runtime.CollaborationConfigEpoch, 10) || saved.DeploymentID != auth.ManagedDeploymentID() ||
		saved.InstallationID != util.UUIDToString(installationID) || saved.BindingID != util.UUIDToString(bindingID) ||
		saved.BindingEpoch != strconv.FormatInt(bindingEpoch, 10) || saved.Capabilities.Provider.Provider != runtime.Provider {
		return nil, collaborationError(http.StatusConflict, "runtime_capability_changed")
	}
	return saved, nil
}
