package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugifyGoal(t *testing.T) {
	cases := map[string]string{
		"refactor auth module":            "refactor-auth-module",
		"Add health endpoint to the API":  "add-health-endpoint-api",
		"hãy viết test cho module parser": "vit-test-module-parser",
		"!!!":                             "goal-task",
		"the a an to of and":              "goal-task",
	}
	for in, want := range cases {
		if got := SlugifyGoal(in); got != want {
			t.Errorf("SlugifyGoal(%q) = %q, want %q", in, got, want)
		}
	}
	long := "one two three four five six seven eight nine ten"
	if got := SlugifyGoal(long); got != "one-two-three-four-five-six" {
		t.Errorf("expected max 6 words, got %q", got)
	}
}

func TestMaterializeGoalTask(t *testing.T) {
	dir := t.TempDir()
	task, path, err := MaterializeGoalTask(context.Background(), dir, "add health endpoint", GoalOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "add-health-endpoint" {
		t.Fatalf("unexpected id: %s", task.ID)
	}
	if task.Description != "add health endpoint" {
		t.Fatalf("unexpected description: %q", task.Description)
	}
	if len(task.Requirements) != 1 || task.Requirements[0].Text != "add health endpoint" {
		t.Fatalf("unexpected requirements: %+v", task.Requirements)
	}
	if len(task.Scope.Include) != 1 || task.Scope.Include[0] != "**" {
		t.Fatalf("unexpected scope: %+v", task.Scope)
	}
	if task.Routing.Diagnose.Agent != "opencode-fixer" {
		t.Fatalf("unexpected routing: %+v", task.Routing)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("task file not written: %v", err)
	}
}

func TestMaterializeGoalTaskIdempotent(t *testing.T) {
	dir := t.TempDir()
	_, path1, err := MaterializeGoalTask(context.Background(), dir, "add health endpoint", GoalOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	task2, path2, err := MaterializeGoalTask(context.Background(), dir, "add health endpoint", GoalOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if path1 != path2 {
		t.Fatalf("expected reuse of task file, got %s vs %s", path1, path2)
	}
	if task2.ID != "add-health-endpoint" {
		t.Fatalf("expected same id, got %s", task2.ID)
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".harness", "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one task file, got %d", len(entries))
	}
}

func TestMaterializeGoalTaskConflict(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := MaterializeGoalTask(context.Background(), dir, "add health endpoint", GoalOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	// Cùng slug ("please" là stopword) nhưng description khác → suffix -2,
	// không ghi đè task cũ.
	task, path, err := MaterializeGoalTask(context.Background(), dir, "add health endpoint please", GoalOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "add-health-endpoint-2" {
		t.Fatalf("expected suffixed id, got %s", task.ID)
	}
	if filepath.Base(path) == "add-health-endpoint.yaml" {
		t.Fatal("must not overwrite existing different task")
	}
}

func TestMaterializeGoalTaskEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := MaterializeGoalTask(context.Background(), dir, "   ", GoalOptions{}, nil); err == nil {
		t.Fatal("expected error for empty goal")
	}
}

func TestMaterializeGoalTaskRefine(t *testing.T) {
	dir := t.TempDir()
	refineJSON := "```json\n" +
		`{"requirements":[{"id":"REQ-1","text":"split handler"},{"id":"REQ-2","text":"add tests"}],` +
		`"scope":{"include":["internal/auth/**"],"exclude":[]}}` + "\n```"
	dispatcher := NewDriverRegistry(MockDriver{
		Responses: map[string]DispatchResult{
			buildGoalRefinePrompt("refactor auth"): {Success: true, Stdout: refineJSON},
		},
	})
	task, _, err := MaterializeGoalTask(context.Background(), dir, "refactor auth", GoalOptions{Refine: true, Agent: "mock"}, dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Requirements) != 2 {
		t.Fatalf("expected refined requirements, got %+v", task.Requirements)
	}
	if len(task.Scope.Include) != 1 || task.Scope.Include[0] != "internal/auth/**" {
		t.Fatalf("expected refined scope, got %+v", task.Scope)
	}
}

func TestMaterializeGoalTaskRefineFailOpen(t *testing.T) {
	dir := t.TempDir()
	dispatcher := NewDriverRegistry(MockDriver{
		Responses: map[string]DispatchResult{},
	})
	task, _, err := MaterializeGoalTask(context.Background(), dir, "refactor auth", GoalOptions{Refine: true}, dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Requirements) != 1 || task.Requirements[0].Text != "refactor auth" {
		t.Fatalf("expected fallback requirements, got %+v", task.Requirements)
	}
}

func TestMaterializedTaskParsesAsYAML(t *testing.T) {
	dir := t.TempDir()
	goal := "goal with \"quotes\" and: colon"
	_, path, err := MaterializeGoalTask(context.Background(), dir, goal, GoalOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadTask(path)
	if err != nil {
		t.Fatalf("materialized file must parse: %v", err)
	}
	if !strings.Contains(loaded.Description, "quotes") {
		t.Fatalf("description lost content: %q", loaded.Description)
	}
}
