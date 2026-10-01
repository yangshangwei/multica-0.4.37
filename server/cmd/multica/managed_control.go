package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/daemon"
)

func requestDaemonShutdownForProfile(port int, profile string) (bool, error) {
	config, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return true, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return true, err
	}
	controlPath := filepath.Join(daemonDirForProfile(profile), "management-control.json")
	_, controlErr := os.Lstat(controlPath)
	managed := config.ManagementDeploymentID != "" || controlErr == nil
	if !managed {
		return false, requestDaemonShutdown(port)
	}
	if config.ManagementDeploymentID == "" {
		return true, errors.New("managed profile marker is missing; refusing unauthenticated shutdown")
	}
	token, err := daemon.ReadManagementControlToken(home, profile)
	if err != nil {
		return true, errors.New("managed control credential unavailable; daemon left running")
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/management/session", port), nil)
	if err != nil {
		return true, err
	}
	headers, err := daemon.ManagementRequestHeaders(token, http.MethodGet, "/management/session", nil)
	if err != nil {
		return true, err
	}
	req.Header = headers
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return true, err
	}
	defer response.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 16385))
	if readErr != nil || len(raw) > 16384 {
		return true, errors.New("invalid managed session response")
	}
	if err = daemon.VerifyManagementResponse(token, http.MethodGet, "/management/session", req.Header, response.StatusCode, raw, response.Header); err != nil {
		return true, err
	}
	if response.StatusCode != http.StatusOK {
		return true, fmt.Errorf("managed session rejected control (%d); daemon left running", response.StatusCode)
	}
	var scope struct {
		DrainIntentID   *string `json:"drain_intent_id"`
		DeploymentID    string  `json:"deployment_id"`
		Profile         string  `json:"profile"`
		UserID          string  `json:"user_id"`
		ManagedDaemonID string  `json:"managed_daemon_id"`
		InstallationID  string  `json:"installation_id"`
	}
	if err = json.Unmarshal(raw, &scope); err != nil {
		return true, errors.New("invalid managed control scope")
	}
	identity, err := daemon.LoadManagedInstallation(home, config.ManagementDeploymentID)
	if err != nil {
		return true, err
	}
	if scope.DeploymentID != config.ManagementDeploymentID || scope.Profile != profile || scope.InstallationID != identity.InstallationID() || scope.ManagedDaemonID != identity.ManagedDaemonID(scope.UserID) {
		return true, errors.New("managed control scope mismatch; daemon left running")
	}
	return true, postDaemonShutdown(port, token, scope.DrainIntentID)
}

func managedProcessProfile(profile string, handoff bool) (bool, string, error) {
	config, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return false, "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", err
	}
	_, controlErr := os.Lstat(filepath.Join(daemonDirForProfile(profile), "management-control.json"))
	if controlErr != nil && !errors.Is(controlErr, os.ErrNotExist) {
		return false, "", controlErr
	}
	return handoff || config.ManagementDeploymentID != "" || controlErr == nil, home, nil
}
