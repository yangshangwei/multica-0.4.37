package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

func stageBoundedText(value string, limit int) bool {
	return len(value) > 0 && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
func stageIntegrationValid(source StageIntegrationIdentity) bool {
	return stageBoundedText(source.Adapter, 64) && stageBoundedText(source.AdapterVersion, 64) && stageUUID(source.IntegrationID) && stagePositiveVersion(source.TrustRevision) && stagePositiveVersion(source.ConfigRevision)
}
func stageExecutionDigest(domain string, value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(domain+":"), raw...))
	return hex.EncodeToString(sum[:]), nil
}

// StageExecutionTargetDigest identifies reviewed configuration. Conditional
// state is retained separately at signing and compared at actual admission.
func StageExecutionTargetDigest(target StageExecutionTarget) (string, error) {
	if !stageUUID(target.WorkspaceID) || !stageIntegrationValid(target.Source) || !stageBoundedText(target.Environment, 128) || !stageBoundedText(target.ConcurrencyDomain, 128) || !stageBoundedText(target.RemoteInstance, 256) || !stageBoundedText(target.RemoteTenant, 256) || !stageBoundedText(target.RemoteTarget, 256) || !stageBoundedText(target.ConditionalVersion, 256) || !target.SupportsConditional || !target.SupportsOriginalQuery || target.CurrentArtifactDigest != nil && !stageDigest(*target.CurrentArtifactDigest) {
		return "", collaborationError(409, "stage_execution_unavailable")
	}
	target.ConditionalVersion = ""
	target.CurrentArtifactDigest = nil
	return stageExecutionDigest("collaboration-stage-target-v1", target)
}
func stageExecutionPhysicalKey(target StageExecutionTarget) string {
	value := struct{ Domain, Instance, Tenant, Target string }{target.ConcurrencyDomain, target.RemoteInstance, target.RemoteTenant, target.RemoteTarget}
	digest, _ := stageExecutionDigest("collaboration-stage-physical-target-v1", value)
	return digest
}
func stageExecutionRequestDigest(request StageExecutionRequest) (string, error) {
	return stageExecutionDigest("collaboration-stage-execution-request-v1", request)
}
func stageMetricStream(metric StageRollbackMetric) string {
	value := struct{ Deployment, Target, Source, Query, Rule string }{metric.DeploymentID, metric.TargetDigest, metric.StreamIdentity, metric.QueryID, metric.RuleID}
	digest, _ := stageExecutionDigest("collaboration-stage-metric-stream-v1", value)
	return digest
}
