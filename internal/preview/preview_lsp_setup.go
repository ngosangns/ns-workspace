package preview

import (
	"context"
	"io"
	"os"

	"github.com/ngosangns/ns-workspace/internal/graphquery"
)

const lspCacheEnv = graphquery.CacheEnv

type lspSymbolMode = graphquery.SymbolMode

const (
	lspSymbolModeCallable = graphquery.SymbolModeCallable
	lspSymbolModeDocument = graphquery.SymbolModeDocument
	lspSymbolModeSelector = graphquery.SymbolModeSelector
)

type lspInstallSpec = graphquery.InstallSpec
type lspLanguageSpec = graphquery.LanguageSpec
type lspStatusRow = graphquery.StatusRow
type lspInstallResult = graphquery.InstallResult
type lspEnsureOptions = graphquery.EnsureOptions

type GraphQueryLSPDetector struct{}

func RunLSP(args []string) error {
	return graphquery.RunLSP(args, GraphQueryLSPDetector{})
}

func runLSPList(args []string, out io.Writer) error {
	return graphquery.RunLSPList(args, out, GraphQueryLSPDetector{})
}

func runLSPInstall(args []string, out io.Writer) error {
	return graphquery.RunLSPInstall(args, out, GraphQueryLSPDetector{})
}

func printLSPUsage() {
	graphquery.PrintLSPUsage(os.Stdout)
}

func lspLanguageSpecs() []lspLanguageSpec {
	return graphquery.LanguageSpecs()
}

func lspInstallSpecByID(id string) (lspInstallSpec, bool) {
	return graphquery.InstallSpecByID(id)
}

func lspInstallSpecByServerID(serverID string) (lspInstallSpec, bool) {
	return graphquery.InstallSpecByServerID(serverID)
}

func lspSupportedIDs() []string {
	return graphquery.SupportedIDs()
}

func lspListStatus(projectRoot, docsDir string) []lspStatusRow {
	return graphquery.ListStatus(projectRoot, docsDir, GraphQueryLSPDetector{})
}

func (GraphQueryLSPDetector) DetectedLanguages(projectRoot, docsDir string) map[string]bool {
	_, files, _ := lspSourceFiles(projectRoot, docsRoot(projectRoot, docsDir))
	detected := map[string]bool{}
	for _, file := range files {
		detected[file.Language.ID] = true
	}
	return detected
}

func (GraphQueryLSPDetector) DetectedInstallIDs(projectRoot, docsDir string) map[string]bool {
	_, files, _ := lspSourceFiles(projectRoot, docsRoot(projectRoot, docsDir))
	detected := map[string]bool{}
	for _, file := range files {
		if spec, ok := graphquery.InstallSpecByServerID(file.Language.ServerID); ok {
			detected[spec.ID] = true
		}
	}
	return detected
}

func installProjectLSPs(ctx context.Context, projectRoot, docsDir string, opts lspEnsureOptions) []lspInstallResult {
	return graphquery.InstallProjectLSPs(ctx, projectRoot, docsDir, opts, GraphQueryLSPDetector{})
}

func ensureProjectLSP(ctx context.Context, projectRoot, docsDir string, opts lspEnsureOptions) []string {
	return graphquery.EnsureProjectLSP(ctx, projectRoot, docsDir, opts, GraphQueryLSPDetector{})
}

func lspCacheCommandDirs(command string) []string {
	return graphquery.CacheCommandDirs(command)
}

func lspCommandSource(path, projectRoot string) string {
	return graphquery.CommandSource(path, projectRoot)
}

func lspUnavailableWarning(lang lspLanguage, detail string) string {
	return graphquery.UnavailableWarning(lang.ServerID, lang.Name, detail)
}
