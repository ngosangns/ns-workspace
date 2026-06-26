package graphquery

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
)

type kotlinImplementation struct{}

var resolveArchiveSource = defaultKotlinArchiveSource

// runtimeOS / runtimeArch là seam test: cho phép thay thế runtime.GOOS/GOARCH
// để test các nhánh phụ thuộc OS/Arch.
var runtimeOS = func() string { return runtime.GOOS }
var runtimeArch = func() string { return runtime.GOARCH }

func (kotlinImplementation) installSpec() InstallSpec {
	return InstallSpec{
		ID:         "kotlin",
		Name:       "Kotlin",
		Aliases:    []string{"kt"},
		ServerID:   "kotlin",
		Extensions: []string{".kt", ".kts"},
		Command:    "kotlin-lsp",
		Args:       []string{"--stdio"},
		CheckArgs:  []string{"--help"},
	}
}

func (kotlinImplementation) languageSpecs() []LanguageSpec {
	return []LanguageSpec{
		{ID: "kotlin", Name: "Kotlin", Aliases: []string{"kt"}, Extensions: []string{".kt", ".kts"}, ServerID: "kotlin", LanguageID: "kotlin", SymbolMode: SymbolModeCallable},
	}
}

func (impl kotlinImplementation) cacheCommandDirs() []string {
	spec := impl.installSpec()
	return []string{filepath.Join(CacheRoot(), spec.ID, "bin")}
}

func (impl kotlinImplementation) installCommand() string {
	spec := impl.installSpec()
	source, err := resolveArchiveSource(spec)
	target := filepath.Join(CacheRoot(), spec.ID, "bin", spec.Command)
	if err != nil {
		return err.Error()
	}
	return "download " + source.URL + " -> " + shellQuote(target) + " (sha256 " + source.SHA256 + ")"
}

func (impl kotlinImplementation) install(ctx context.Context) (string, error) {
	spec := impl.installSpec()
	source, err := resolveArchiveSource(spec)
	if err != nil {
		return "", err
	}
	return installArchiveLSP(ctx, spec, source, []string{"bin/intellij-server", "kotlin-lsp.sh", "bin/kotlin-lsp", "kotlin-lsp"})
}

func SetArchiveSourceForTest(fn func(InstallSpec) (ArchiveSource, error)) func() {
	previous := resolveArchiveSource
	resolveArchiveSource = fn
	return func() {
		resolveArchiveSource = previous
	}
}

func defaultKotlinArchiveSource(spec InstallSpec) (ArchiveSource, error) {
	if spec.ID != "kotlin" {
		return ArchiveSource{}, fmt.Errorf("archive install is not configured for %s", spec.ID)
	}
	const version = "262.4739.0"
	baseURL := "https://download-cdn.jetbrains.com/kotlin-lsp/" + version
	suffix := ""
	extension := ""
	sha := ""
	switch runtimeOS() {
	case "darwin":
		extension = "sit"
		switch runtimeArch() {
		case "arm64":
			suffix = "-aarch64"
			sha = "1b745743ce22ad92681a1bc3b1046803e942a6e1f36e04fb85ae9a40334a2f1e"
		case "amd64":
			sha = "6f06efe7a10f94b9c8a028c4efeb6c7e1769f47a01edfb74450acf30ab5665e4"
		default:
			return ArchiveSource{}, fmt.Errorf("Kotlin LSP archive install does not support darwin/%s", runtimeArch())
		}
	case "linux":
		extension = "tar.gz"
		switch runtimeArch() {
		case "arm64":
			suffix = "-aarch64"
			sha = "625870ae091c6d0dee25514d545c708a6ea50d7cbb5154aaf1aa9123ccff338b"
		case "amd64":
			sha = "46971110c9b8a3360ce3fdf5437467f4c447dad37ad73dbf81d64af6779e4105"
		default:
			return ArchiveSource{}, fmt.Errorf("Kotlin LSP archive install does not support linux/%s", runtimeArch())
		}
	default:
		return ArchiveSource{}, fmt.Errorf("Kotlin LSP archive install does not support %s/%s", runtimeOS(), runtimeArch())
	}
	fileName := fmt.Sprintf("kotlin-server-%s%s.%s", version, suffix, extension)
	format := extension
	if extension == "sit" {
		// JetBrains publishes macOS .sit artifacts as ZIP-compatible archives.
		format = "zip"
	}
	return ArchiveSource{
		Version:  version,
		FileName: fileName,
		URL:      baseURL + "/" + fileName,
		SHA256:   sha,
		Format:   format,
	}, nil
}
