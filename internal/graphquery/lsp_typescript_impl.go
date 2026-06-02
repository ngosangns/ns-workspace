package graphquery

import "context"

type typeScriptImplementation struct{}

func (typeScriptImplementation) installSpec() InstallSpec {
	return InstallSpec{
		ID:         "typescript",
		Name:       "TypeScript/JavaScript",
		Aliases:    []string{"ts", "javascript", "js"},
		ServerID:   "typescript",
		Extensions: []string{".ts", ".tsx", ".js", ".jsx", ".cjs", ".mjs"},
		Command:    "typescript-language-server",
		Args:       []string{"--stdio"},
		CheckArgs:  []string{"--version"},
		Prerequisites: []Prerequisite{
			{Name: "Node.js 18+", Command: "node", Args: []string{"--version"}, InstallHint: "Install Node.js 18+ from https://nodejs.org/", MinMajor: 18},
			{Name: "npm", Command: "npm", Args: []string{"--version"}, InstallHint: "Install npm with Node.js."},
		},
	}
}

func (typeScriptImplementation) languageSpecs() []LanguageSpec {
	return []LanguageSpec{
		{ID: "javascript", Name: "JavaScript", Aliases: []string{"js"}, Extensions: []string{".js", ".cjs", ".mjs"}, ServerID: "typescript", LanguageID: "javascript", SymbolMode: SymbolModeCallable},
		{ID: "javascript", Name: "JavaScript", Aliases: []string{"jsx"}, Extensions: []string{".jsx"}, ServerID: "typescript", LanguageID: "javascriptreact", SymbolMode: SymbolModeCallable},
		{ID: "typescript", Name: "TypeScript", Aliases: []string{"ts"}, Extensions: []string{".ts"}, ServerID: "typescript", LanguageID: "typescript", SymbolMode: SymbolModeCallable},
		{ID: "typescript", Name: "TypeScript", Aliases: []string{"tsx"}, Extensions: []string{".tsx"}, ServerID: "typescript", LanguageID: "typescriptreact", SymbolMode: SymbolModeCallable},
	}
}

func (impl typeScriptImplementation) cacheCommandDirs() []string {
	return npmCacheDirs(impl.installSpec())
}

func (impl typeScriptImplementation) installCommand() string {
	return npmInstallCommand(impl.installSpec(), []string{"typescript-language-server", "typescript"})
}

func (impl typeScriptImplementation) install(ctx context.Context) (string, error) {
	return installNPMLSP(ctx, impl.installSpec(), []string{"typescript-language-server", "typescript"})
}
