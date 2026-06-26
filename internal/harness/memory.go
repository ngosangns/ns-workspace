package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type State struct {
	TaskID              string            `json:"task_id"`
	Phase               string            `json:"phase"`
	Iteration           int               `json:"iteration"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	History             []StateSnapshot   `json:"history"`
	Hypotheses          []Hypothesis      `json:"hypotheses"`
	Subtasks            []Subtask         `json:"subtasks"`
	AcceptanceStatus    map[string]bool   `json:"acceptance_status"`
	RequirementStatus   map[string]bool   `json:"requirement_status"`
	LastError           string            `json:"last_error"`
	Paused              bool              `json:"paused"`
	PausedReason        string            `json:"paused_reason"`
	ContextNotes        map[string]string `json:"context_notes"`
	Warnings            []string          `json:"warnings,omitempty"`
}

// AddWarning ghi một warning vào state (fail-open traceability).
// Dùng bởi enrichment loop khi fetch lỗi / skip URL ngoài allowlist
// để tiếp tục loop mà vẫn giữ lại dấu vết (Requirements 5.5).
func (s *State) AddWarning(msg string) {
	s.Warnings = append(s.Warnings, msg)
}

type StateSnapshot struct {
	Phase     string `json:"phase"`
	Iteration int    `json:"iteration"`
	Hash      string `json:"hash"`
}

type Hypothesis struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Tried       bool   `json:"tried"`
}

type Subtask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

func NewState(taskID string) *State {
	return &State{
		TaskID:            taskID,
		AcceptanceStatus:  map[string]bool{},
		RequirementStatus: map[string]bool{},
		ContextNotes:      map[string]string{},
		History:           []StateSnapshot{},
		Hypotheses:        []Hypothesis{},
		Subtasks:          []Subtask{},
	}
}

func (s *State) Hash() string {
	clone := *s
	clone.History = nil
	data, _ := json.Marshal(clone)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:16]
}

func (s *State) RecordSnapshot() {
	s.History = append(s.History, StateSnapshot{
		Phase:     s.Phase,
		Iteration: s.Iteration,
		Hash:      s.Hash(),
	})
}

func (s *State) HasRepeatedState() bool {
	current := s.Hash()
	for i, snap := range s.History {
		if i == len(s.History)-1 {
			continue
		}
		if snap.Hash == current {
			return true
		}
	}
	return false
}

func (s *State) UntriedHypotheses() []Hypothesis {
	var out []Hypothesis
	for _, h := range s.Hypotheses {
		if !h.Tried {
			out = append(out, h)
		}
	}
	return out
}

func (s *State) MarkHypothesisTried(id string) {
	for i := range s.Hypotheses {
		if s.Hypotheses[i].ID == id {
			s.Hypotheses[i].Tried = true
			return
		}
	}
}

func (s *State) AllSubtasksDone() bool {
	if len(s.Subtasks) == 0 {
		return false
	}
	for _, st := range s.Subtasks {
		if !st.Done {
			return false
		}
	}
	return true
}

func (s *State) AllAcceptancePassed() bool {
	if len(s.AcceptanceStatus) == 0 {
		return false
	}
	for _, passed := range s.AcceptanceStatus {
		if !passed {
			return false
		}
	}
	return true
}

func (s *State) AllRequirementsSatisfied() bool {
	if len(s.RequirementStatus) == 0 {
		return false
	}
	for _, ok := range s.RequirementStatus {
		if !ok {
			return false
		}
	}
	return true
}

type Store struct {
	ProjectRoot string
	Task        *Task
}

func NewStore(projectRoot string, task *Task) *Store {
	return &Store{ProjectRoot: projectRoot, Task: task}
}

func (st *Store) projectPath() string {
	path := st.Task.Memory.ProjectPath
	if path == "" {
		path = filepath.Join(".harness", "state", st.Task.ID+".json")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(st.ProjectRoot, path)
	}
	return path
}

func (st *Store) sharedPath() string {
	path := st.Task.Memory.SharedPath
	if path == "" {
		path = filepath.Join("~/.agents", "harness", projectSlug(st.ProjectRoot), st.Task.ID+".json")
	}
	path = expandTilde(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(st.ProjectRoot, path)
	}
	return path
}

func projectSlug(root string) string {
	base := filepath.Base(root)
	if base == "" || base == "." {
		base = "unknown"
	}
	h := sha256.Sum256([]byte(root))
	return fmt.Sprintf("%s-%s", base, hex.EncodeToString(h[:])[:8])
}

func expandTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func (st *Store) Load() (*State, error) {
	for _, path := range []string{st.projectPath(), st.sharedPath()} {
		data, err := os.ReadFile(path)
		if err == nil {
			var state State
			if err := json.Unmarshal(data, &state); err != nil {
				return nil, fmt.Errorf("load state %s: %w", path, err)
			}
			return &state, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return NewState(st.Task.ID), nil
}

func (st *Store) Save(state *State) error {
	data, _ := json.MarshalIndent(state, "", "  ")
	data = append(data, '\n')
	project := st.projectPath()
	if err := os.MkdirAll(filepath.Dir(project), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(project, data, 0o644); err != nil {
		return err
	}
	shared := st.sharedPath()
	if err := os.MkdirAll(filepath.Dir(shared), 0o755); err != nil {
		return err
	}
	return os.WriteFile(shared, data, 0o644)
}

func (st *Store) Remove() error {
	var firstErr error
	if err := os.Remove(st.projectPath()); err != nil && !os.IsNotExist(err) {
		firstErr = err
	}
	if err := os.Remove(st.sharedPath()); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
