package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Task struct {
	ID           string         `json:"id" yaml:"id"`
	Description  string         `json:"description" yaml:"description"`
	Domain       string         `json:"domain" yaml:"domain"`
	Type         string         `json:"type" yaml:"type"`
	Requirements []Requirement  `json:"requirements" yaml:"requirements"`
	Scope        Scope          `json:"scope" yaml:"scope"`
	Acceptance   []Acceptance   `json:"acceptance" yaml:"acceptance"`
	Phases       []string       `json:"phases" yaml:"phases"`
	Routing      Routing        `json:"routing" yaml:"routing"`
	Memory       MemoryConfig   `json:"memory" yaml:"memory"`
	Stopping     StoppingConfig `json:"stopping" yaml:"stopping"`
	Enrich       EnrichConfig   `json:"enrich" yaml:"enrich"`
}

// EnrichConfig cấu hình task type enrich-docs (Feature 3).
type EnrichConfig struct {
	Seeds  []EnrichSeed `json:"seeds" yaml:"seeds"`
	Caps   EnrichCaps   `json:"caps" yaml:"caps"`
	Target EnrichTarget `json:"target" yaml:"target"`
}

type EnrichSeed struct {
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
	File string `json:"file,omitempty" yaml:"file,omitempty"`
}

type EnrichCaps struct {
	MaxPages            int      `json:"max_pages" yaml:"max_pages"`
	MaxDepth            int      `json:"max_depth" yaml:"max_depth"`
	AllowedHosts        []string `json:"allowed_hosts" yaml:"allowed_hosts"`
	FetchTimeoutSeconds int      `json:"fetch_timeout_seconds" yaml:"fetch_timeout_seconds"`
}

type EnrichTarget struct {
	Mode          string `json:"mode" yaml:"mode"`                     // references | enrich
	ReferencesDir string `json:"references_dir" yaml:"references_dir"`
}

type Requirement struct {
	ID   string `json:"id" yaml:"id"`
	Text string `json:"text" yaml:"text"`
}

type Scope struct {
	Include []string `json:"include" yaml:"include"`
	Exclude []string `json:"exclude" yaml:"exclude"`
}

type Acceptance struct {
	Command  string `json:"command" yaml:"command"`
	Script   string `json:"script" yaml:"script"`
	MustPass bool   `json:"must_pass" yaml:"must_pass"`
}

type Routing struct {
	Default string      `json:"default" yaml:"default"`
	Plan    RoutingRule `json:"plan" yaml:"plan"`
	Execute RoutingRule `json:"execute" yaml:"execute"`
	Verify  RoutingRule `json:"verify" yaml:"verify"`
}

type RoutingRule struct {
	Domain string `json:"domain" yaml:"domain"`
	Agent  string `json:"agent" yaml:"agent"`
}

type MemoryConfig struct {
	ProjectPath string `json:"project_path" yaml:"project_path"`
	SharedPath  string `json:"shared_path" yaml:"shared_path"`
}

type StoppingConfig struct {
	MaxConsecutiveFailures  int  `json:"max_consecutive_failures" yaml:"max_consecutive_failures"`
	RequireHumanOnAmbiguity bool `json:"require_human_on_ambiguity" yaml:"require_human_on_ambiguity"`
}

func (t *Task) DefaultPhases() []string {
	if len(t.Phases) > 0 {
		return t.Phases
	}
	return []string{"plan", "execute", "verify"}
}

func (t *Task) DefaultMaxConsecutiveFailures() int {
	if t.Stopping.MaxConsecutiveFailures > 0 {
		return t.Stopping.MaxConsecutiveFailures
	}
	return 3
}

func (t *Task) SelectAgent(phase string) string {
	var rule RoutingRule
	switch phase {
	case "plan":
		rule = t.Routing.Plan
	case "execute":
		rule = t.Routing.Execute
	case "verify":
		rule = t.Routing.Verify
	}
	if rule.Agent != "" {
		return rule.Agent
	}
	if t.Routing.Default != "" {
		return t.Routing.Default
	}
	return "opencode"
}

func LoadTask(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	var task Task
	if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, &task); err != nil {
			return nil, fmt.Errorf("parse yaml %s: %w", path, err)
		}
	} else {
		if err := json.Unmarshal(data, &task); err != nil {
			return nil, fmt.Errorf("parse json %s: %w", path, err)
		}
	}
	if task.ID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	return &task, nil
}

func LoadTasksFromDir(dir string) ([]*Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var tasks []*Task
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			continue
		}
		task, err := LoadTask(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}
