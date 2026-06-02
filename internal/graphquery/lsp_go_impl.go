package graphquery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type goImplementation struct{}

func (goImplementation) installSpec() InstallSpec {
	return InstallSpec{
		ID:         "go",
		Name:       "Go/Golang",
		Aliases:    []string{"golang"},
		ServerID:   "go",
		Extensions: []string{".go"},
		Command:    "gopls",
		Args:       []string{"serve"},
		CheckArgs:  []string{"version"},
		Prerequisites: []Prerequisite{{
			Name:        "Go",
			Command:     "go",
			Args:        []string{"version"},
			InstallHint: "Install Go from https://go.dev/dl/",
		}},
	}
}

func (goImplementation) languageSpecs() []LanguageSpec {
	return []LanguageSpec{
		{ID: "go", Name: "Go/Golang", Aliases: []string{"golang"}, Extensions: []string{".go"}, ServerID: "go", LanguageID: "go", SymbolMode: SymbolModeCallable},
	}
}

func (impl goImplementation) cacheCommandDirs() []string {
	spec := impl.installSpec()
	return []string{filepath.Join(CacheRoot(), spec.ID, "bin")}
}

func (impl goImplementation) installCommand() string {
	spec := impl.installSpec()
	return "GOBIN=" + shellQuote(filepath.Join(CacheRoot(), spec.ID, "bin")) + " go install golang.org/x/tools/gopls@latest"
}

func (impl goImplementation) install(ctx context.Context) (string, error) {
	spec := impl.installSpec()
	binDir := filepath.Join(CacheRoot(), spec.ID, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", "install", "golang.org/x/tools/gopls@latest")
	cmd.Env = append(os.Environ(), "GOBIN="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go install failed: %w: %s", err, trimCommandOutput(out))
	}
	path := filepath.Join(binDir, executableNames(spec.Command)[0])
	if !executableFile(path) {
		return "", fmt.Errorf("%s installed but not found at %s", spec.Command, path)
	}
	return path, nil
}
