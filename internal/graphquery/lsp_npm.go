package graphquery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func npmCacheDirs(spec InstallSpec) []string {
	return []string{filepath.Join(CacheRoot(), spec.ID, "node_modules", ".bin")}
}

func npmInstallCommand(spec InstallSpec, packages []string) string {
	return "npm install --prefix " + shellQuote(filepath.Join(CacheRoot(), spec.ID)) + " " + strings.Join(packages, " ")
}

func installNPMLSP(ctx context.Context, spec InstallSpec, packages []string) (string, error) {
	prefix := filepath.Join(CacheRoot(), spec.ID)
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return "", err
	}
	args := append([]string{"install", "--prefix", prefix}, packages...)
	cmd := exec.CommandContext(ctx, "npm", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("npm install failed: %w: %s", err, trimCommandOutput(out))
	}
	for _, name := range executableNames(spec.Command) {
		path := filepath.Join(prefix, "node_modules", ".bin", name)
		if executableFile(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s installed but not found in %s", spec.Command, filepath.Join(prefix, "node_modules", ".bin"))
}
