package portal

// Skill is a preset skill exposed through the portal API.
type Skill struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"` // "embedded" or overlay path
	Overridden bool   `json:"overridden"`
	Enabled    bool   `json:"enabled"`
	Content    string `json:"content,omitempty"`
}

// SkillUpdate is the request body for updating a skill.
type SkillUpdate struct {
	Content string `json:"content"`
}

// EnableRequest is the body for enable/disable endpoints.
type EnableRequest struct {
	Enabled bool `json:"enabled"`
}

// MCPServers is the shared MCP manifest exposed through the portal API.
type MCPServers struct {
	MCPServers map[string]any `json:"mcpServers"`
}

// MCPServerItem is one MCP server with enable state for the portal list UI.
type MCPServerItem struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Config  any    `json:"config"`
}

// MCPManifest combines the effective MCP servers with provenance metadata.
type MCPManifest struct {
	MCPServers `json:",inline"`
	// DisabledServers are portal-disabled entries preserved as // comments
	// in the JSONC overlay (not deleted from the file).
	DisabledServers map[string]any `json:"disabledServers,omitempty"`
	// Items is the flat list used by the UI (enabled + disabled; never drops entries).
	Items []MCPServerItem `json:"items,omitempty"`
	// Content is the full JSONC file text (live keys + // commented disabled
	// servers). Portal file editor should show this so disable is visible as
	// comment-out, not as deletion.
	Content    string `json:"content,omitempty"`
	Overridden bool   `json:"overridden"`
	Source     string `json:"source"`
}

// Servers returns the effective (enabled) MCP server map.
func (m *MCPManifest) Servers() map[string]any {
	return m.MCPServers.MCPServers
}

// RegistrySkills is the registry skills manifest exposed through the portal API.
type RegistrySkills struct {
	Skills []RegistrySkill `json:"skills"`
}

// RegistrySkill is one entry inside RegistrySkills.
type RegistrySkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Skill  string `json:"skill"`
}

// Adapter is the public description of an agent adapter.
type Adapter struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Tier      string   `json:"tier"`
	Enabled   bool     `json:"enabled"`
	Docs      []string `json:"docs,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
	Notes     string   `json:"notes,omitempty"`
}

// AdapterStatus contains the resolved status paths for an adapter.
type AdapterStatus struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Paths  []PathStatus      `json:"paths"`
}

// PathStatus describes the state of one file or directory.
type PathStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	IsDir  bool   `json:"isDir"`
}

// SyncRequest is the body for starting a sync command.
type SyncRequest struct {
	Command string `json:"command"` // init, update, registry, doctor, status
	Tools   string `json:"tools"`   // comma-separated provider filter, empty means all
}

// SyncJob describes a running or completed sync job.
type SyncJob struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// SyncLogLine is one line emitted by the sync reporter.
type SyncLogLine struct {
	JobID string `json:"jobId"`
	Line  string `json:"line"`
}

// UserOverlay lists the current user-config overlay entries.
type UserOverlay struct {
	Origin  string            `json:"origin"`
	Entries map[string]string `json:"entries"`
}

// StatusSummary gives a quick overview of the shared agents home.
type StatusSummary struct {
	AgentsDir string       `json:"agentsDir"`
	Paths     []PathStatus `json:"paths"`
}

// ErrorResponse is the standard error shape returned by the API.
type ErrorResponse struct {
	Error string `json:"error"`
}
