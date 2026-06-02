package graphquery

import "context"

type htmlImplementation struct{}

func (htmlImplementation) installSpec() InstallSpec {
	return InstallSpec{
		ID:         "html",
		Name:       "HTML",
		ServerID:   "html",
		Extensions: []string{".html", ".htm"},
		Command:    "vscode-html-language-server",
		Args:       []string{"--stdio"},
		CheckArgs:  []string{"--version"},
		Prerequisites: []Prerequisite{
			{Name: "Node.js 18+", Command: "node", Args: []string{"--version"}, InstallHint: "Install Node.js 18+ from https://nodejs.org/", MinMajor: 18},
			{Name: "npm", Command: "npm", Args: []string{"--version"}, InstallHint: "Install npm with Node.js."},
		},
	}
}

func (htmlImplementation) languageSpecs() []LanguageSpec {
	return []LanguageSpec{
		{ID: "html", Name: "HTML", Extensions: []string{".html", ".htm"}, ServerID: "html", LanguageID: "html", SymbolMode: SymbolModeDocument},
	}
}

func (impl htmlImplementation) cacheCommandDirs() []string {
	return npmCacheDirs(impl.installSpec())
}

func (impl htmlImplementation) installCommand() string {
	return npmInstallCommand(impl.installSpec(), []string{"vscode-langservers-extracted"})
}

func (impl htmlImplementation) install(ctx context.Context) (string, error) {
	return installNPMLSP(ctx, impl.installSpec(), []string{"vscode-langservers-extracted"})
}
