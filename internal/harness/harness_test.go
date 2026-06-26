package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.yaml")
	content := `
id: sample
description: sample task
domain: backend
type: refactor
requirements:
  - id: REQ-1
    text: do something
scope:
  include:
    - internal/**
acceptance:
  - command: go test ./...
    must_pass: true
stopping:
  max_consecutive_failures: 2
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	task, err := LoadTask(path)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "sample" {
		t.Fatalf("expected id sample, got %s", task.ID)
	}
	if task.DefaultMaxConsecutiveFailures() != 2 {
		t.Fatalf("expected max failures 2, got %d", task.DefaultMaxConsecutiveFailures())
	}
}

func TestStateHashAndRepeat(t *testing.T) {
	state := NewState("sample")
	state.RecordSnapshot()
	h1 := state.Hash()
	state.Iteration = 1
	h2 := state.Hash()
	if h1 == h2 {
		t.Fatal("expected different hashes")
	}
	state.Iteration = 0
	state.RecordSnapshot()
	if !state.HasRepeatedState() {
		t.Fatal("expected repeated state")
	}
}

func TestMemoryStore(t *testing.T) {
	dir := t.TempDir()
	task := &Task{
		ID: "sample",
		Memory: MemoryConfig{
			ProjectPath: filepath.Join(dir, "state.json"),
			SharedPath:  filepath.Join(dir, "shared.json"),
		},
	}
	store := NewStore(dir, task)
	state := NewState("sample")
	state.Iteration = 5
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Iteration != 5 {
		t.Fatalf("expected iteration 5, got %d", loaded.Iteration)
	}
}

func TestEvaluatorCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"exit 0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	eval := NewEvaluator(dir, noopReporter{})
	res := eval.run("pkg-test", "exit 0")
	if !res.Passed {
		t.Fatalf("expected pass, got fail: %s", res.Stderr)
	}
}

func TestLoopDetectRepeatedState(t *testing.T) {
	dir := t.TempDir()
	task := &Task{
		ID: "loop-test",
		Acceptance: []Acceptance{
			{Command: "exit 1", MustPass: true},
		},
		Stopping: StoppingConfig{MaxConsecutiveFailures: 1},
		Routing:  Routing{Default: "mock"},
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(dir, noopReporter{})
	engine.Dispatcher = NewDriverRegistry(MockDriver{
		Responses: map[string]DispatchResult{
			buildPlanPrompt(task, NewState("loop-test")):    {Success: true, Stdout: "plan"},
			buildExecutePrompt(task, NewState("loop-test")): {Success: true, Stdout: "execute"},
		},
	})
	state := NewState("loop-test")
	lc := &LoopController{
		Engine:     engine,
		Evaluator:  NewEvaluator(dir, noopReporter{}),
		Dispatcher: engine.Dispatcher,
		Reporter:   noopReporter{},
	}
	res, err := lc.Run(context.Background(), task, state)
	if err != nil {
		t.Fatal(err)
	}
	if res.Finalized {
		t.Fatal("expected not finalized")
	}
	if res.State.ConsecutiveFailures == 0 {
		t.Fatal("expected some failures")
	}
}

func TestEngineListAndLoad(t *testing.T) {
	dir := t.TempDir()
	engine := NewEngine(dir, noopReporter{})
	if err := os.MkdirAll(engine.TaskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(engine.TaskDir, "one.yaml"), []byte("id: one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tasks, err := engine.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	loaded, err := engine.LoadTask("one")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "one" {
		t.Fatalf("expected one, got %s", loaded.ID)
	}
}

func TestFullLoopFinalize(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".harness", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `
id: full
description: full loop
requirements:
  - id: REQ-1
    text: should pass
scope:
  include:
    - "**"
acceptance:
  - command: echo ok
    must_pass: true
phases:
  - plan
  - execute
  - verify
routing:
  default: mock
stopping:
  max_consecutive_failures: 0
`
	if err := os.WriteFile(filepath.Join(dir, ".harness", "tasks", "full.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(dir, noopReporter{})
	engine.Dispatcher = NewDriverRegistry(MockDriver{
		Responses: map[string]DispatchResult{},
	})
	res, err := engine.Run(context.Background(), "full", false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Finalized {
		t.Fatalf("expected finalized, got %s", res.Reason)
	}
	if res.Iterations != 3 {
		t.Fatalf("expected 3 iterations (plan, execute, verify), got %d", res.Iterations)
	}
}

func TestDecisionRequestWrittenInCIMode(t *testing.T) {
	dir := t.TempDir()
	task := &Task{
		ID: "ci",
		Acceptance: []Acceptance{
			{Command: "exit 1", MustPass: true},
		},
		Stopping: StoppingConfig{MaxConsecutiveFailures: 1},
		Routing:  Routing{Default: "mock"},
	}
	engine := NewEngine(dir, noopReporter{})
	engine.Interactive = false
	engine.DecisionWriter = fileDecisionWriter{}
	engine.Dispatcher = NewDriverRegistry(MockDriver{
		Responses: map[string]DispatchResult{},
	})
	state := NewState(task.ID)
	lc := &LoopController{
		Engine:     engine,
		Evaluator:  NewEvaluator(dir, noopReporter{}),
		Dispatcher: engine.Dispatcher,
		Reporter:   noopReporter{},
	}
	res, err := lc.Run(context.Background(), task, state)
	if err != nil {
		t.Fatal(err)
	}
	if res.Finalized {
		t.Fatal("expected not finalized")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".harness", "decision-request.md"))
	if err != nil {
		t.Fatalf("expected decision request file: %v", err)
	}
	if !strings.Contains(string(data), "ci") {
		t.Fatal("decision request should mention task id")
	}
}

func TestGoTestDiscoveryRequiresGoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"exit 0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	eval := NewEvaluator(dir, noopReporter{})
	commands := eval.buildCommands(&Task{ID: "noop"})
	if _, ok := commands["go-test"]; ok {
		t.Fatal("expected no go-test command without go.mod")
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commands = eval.buildCommands(&Task{ID: "noop"})
	if _, ok := commands["go-test"]; !ok {
		t.Fatal("expected go-test command with go.mod")
	}
}
