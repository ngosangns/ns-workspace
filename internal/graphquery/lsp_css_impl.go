package graphquery

import "context"

type cssImplementation struct{}

func (cssImplementation) installSpec() InstallSpec {
	return InstallSpec{
		ID:         "css",
		Name:       "CSS/SCSS/Sass",
		Aliases:    []string{"scss", "sass"},
		ServerID:   "css",
		Extensions: []string{".css", ".scss", ".sass"},
		Command:    "vscode-css-language-server",
		Args:       []string{"--stdio"},
		CheckArgs:  []string{"--version"},
		Prerequisites: []Prerequisite{
			{Name: "Node.js 18+", Command: "node", Args: []string{"--version"}, InstallHint: "Install Node.js 18+ from https://nodejs.org/", MinMajor: 18},
			{Name: "npm", Command: "npm", Args: []string{"--version"}, InstallHint: "Install npm with Node.js."},
		},
	}
}

func (cssImplementation) languageSpecs() []LanguageSpec {
	return []LanguageSpec{
		{ID: "css", Name: "CSS", Extensions: []string{".css"}, ServerID: "css", LanguageID: "css", SymbolMode: SymbolModeSelector},
		{ID: "scss", Name: "SCSS", Aliases: []string{"sass"}, Extensions: []string{".scss", ".sass"}, ServerID: "css", LanguageID: "scss", SymbolMode: SymbolModeSelector},
	}
}

func (impl cssImplementation) cacheCommandDirs() []string {
	return npmCacheDirs(impl.installSpec())
}

func (impl cssImplementation) installCommand() string {
	return npmInstallCommand(impl.installSpec(), []string{"vscode-langservers-extracted"})
}

func (impl cssImplementation) install(ctx context.Context) (string, error) {
	return installNPMLSP(ctx, impl.installSpec(), []string{"vscode-langservers-extracted"})
}
