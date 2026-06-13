package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Reporter interface {
	Line(format string, args ...any)
}

type stdoutReporter struct{}

func (stdoutReporter) Line(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

type noopReporter struct{}

func (noopReporter) Line(format string, args ...any) {}

type Engine struct {
	ProjectRoot    string
	TaskDir        string
	Reporter       Reporter
	Dispatcher     *DriverRegistry
	Interactive    bool
	DecisionWriter DecisionWriter
}

type DecisionWriter interface {
	Write(projectRoot string, state *State, task *Task) error
}

type fileDecisionWriter struct{}

func (fileDecisionWriter) Write(projectRoot string, state *State, task *Task) error {
	path := filepath.Join(projectRoot, ".harness", "decision-request.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Decision Request\n\n")
	b.WriteString(fmt.Sprintf("**Task:** %s\n\n", task.ID))
	b.WriteString(fmt.Sprintf("**Phase:** %s\n\n", state.Phase))
	b.WriteString(fmt.Sprintf("**Reason:** %s\n\n", state.PausedReason))
	b.WriteString(fmt.Sprintf("**Iteration:** %d\n\n", state.Iteration))
	b.WriteString("## Context\n\n")
	b.WriteString(fmt.Sprintf("```\n%s\n```\n\n", state.LastError))
	b.WriteString("## Required Decision\n\n")
	b.WriteString("Please provide guidance or update the task/state, then run:\n\n")
	b.WriteString(fmt.Sprintf("```bash\ngo run . harness resume --task %s --project %s\n```\n", task.ID, projectRoot))
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func NewEngine(projectRoot string, reporter Reporter) *Engine {
	if reporter == nil {
		reporter = stdoutReporter{}
	}
	return &Engine{
		ProjectRoot:    projectRoot,
		TaskDir:        filepath.Join(projectRoot, ".harness", "tasks"),
		Reporter:       reporter,
		Dispatcher:     NewDriverRegistry(),
		Interactive:    isInteractive(),
		DecisionWriter: fileDecisionWriter{},
	}
}

func isInteractive() bool {
	if os.Getenv("CI") != "" {
		return false
	}
	if os.Getenv("NONINTERACTIVE") != "" {
		return false
	}
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	return true
}

func (e *Engine) ListTasks() ([]*Task, error) {
	return LoadTasksFromDir(e.TaskDir)
}

func (e *Engine) LoadTask(id string) (*Task, error) {
	candidates := []string{
		filepath.Join(e.TaskDir, id+".yaml"),
		filepath.Join(e.TaskDir, id+".yml"),
		filepath.Join(e.TaskDir, id+".json"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return LoadTask(path)
		}
	}
	return nil, fmt.Errorf("task %s not found in %s", id, e.TaskDir)
}

func (e *Engine) Run(ctx context.Context, id string, dryRun bool) (*LoopResult, error) {
	task, err := e.LoadTask(id)
	if err != nil {
		return nil, err
	}
	store := NewStore(e.ProjectRoot, task)
	state, err := store.Load()
	if err != nil {
		return nil, err
	}
	if state.Paused {
		e.Reporter.Line("task %s is paused: %s", id, state.PausedReason)
	}
	if dryRun {
		e.Reporter.Line("dry-run: would run harness %s", id)
		return &LoopResult{State: state, Finalized: false, Reason: "dry-run", Iterations: state.Iteration}, nil
	}
	evaluator := NewEvaluator(e.ProjectRoot, e.Reporter)
	lc := &LoopController{
		Engine:     e,
		Evaluator:  evaluator,
		Dispatcher: e.Dispatcher,
		Reporter:   e.Reporter,
	}
	return lc.Run(ctx, task, state)
}

func (e *Engine) Eval(id string) ([]EvalResult, error) {
	task, err := e.LoadTask(id)
	if err != nil {
		return nil, err
	}
	evaluator := NewEvaluator(e.ProjectRoot, e.Reporter)
	results, _ := evaluator.EvaluateAll(task, map[string]bool{})
	return results, nil
}

func (e *Engine) Status(id string) (*State, error) {
	task, err := e.LoadTask(id)
	if err != nil {
		return nil, err
	}
	store := NewStore(e.ProjectRoot, task)
	return store.Load()
}

func (e *Engine) Resume(ctx context.Context, id string) (*LoopResult, error) {
	state, err := e.Status(id)
	if err != nil {
		return nil, err
	}
	if !state.Paused {
		return nil, fmt.Errorf("task %s is not paused", id)
	}
	state.Paused = false
	state.PausedReason = ""
	task, err := e.LoadTask(id)
	if err != nil {
		return nil, err
	}
	store := NewStore(e.ProjectRoot, task)
	if err := store.Save(state); err != nil {
		return nil, err
	}
	return e.Run(ctx, id, false)
}

func (e *Engine) Stop(id string) error {
	task, err := e.LoadTask(id)
	if err != nil {
		return err
	}
	store := NewStore(e.ProjectRoot, task)
	return store.Remove()
}
