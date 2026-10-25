package execenv

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

// PrepareCollaborationCodexHome uses the ordinary account/configuration policy
// while retaining only this collaboration identity's local sessions. It never
// imports a resume pointer or rollout from an ordinary/private task.
func PrepareCollaborationCodexHome(root, version string, logger *slog.Logger) (string, error) {
	home := filepath.Join(root, "codex-home")
	if err := prepareCodexHomeWithOpts(home, CodexHomeOptions{CodexVersion: version}, logger); err != nil {
		return "", err
	}
	return home, nil
}

// InjectCollaborationBrief reuses the normal provider file mapping and marker
// preservation without constructing an ordinary TaskContext or its workflow.
func InjectCollaborationBrief(workDir, provider, brief string) error {
	path := runtimeConfigPath(workDir, provider)
	if path == "" {
		return errors.New("unsupported collaboration context provider")
	}
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return errors.New("collaboration runtime context is not a regular file")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeRuntimeConfigFile(path, brief)
}
