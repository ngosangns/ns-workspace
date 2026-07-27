package harness

import (
	"bufio"
	"strings"
	"testing"
)

func TestExtractJSONBlockFenced(t *testing.T) {
	raw := "Some explanation.\n```json\n{\"subtasks\":[],\"hypotheses\":[]}\n```\nmore text"
	block, ok := ExtractJSONBlock(raw)
	if !ok {
		t.Fatal("expected to find fenced block")
	}
	if block != `{"subtasks":[],"hypotheses":[]}` {
		t.Fatalf("unexpected block: %s", block)
	}
}

func TestExtractJSONBlockFallback(t *testing.T) {
	raw := "prefix {\"subtasks\":[],\"hypotheses\":[]} suffix"
	block, ok := ExtractJSONBlock(raw)
	if !ok {
		t.Fatal("expected fallback extraction")
	}
	if block != `{"subtasks":[],"hypotheses":[]}` {
		t.Fatalf("unexpected block: %s", block)
	}
}

func TestParsePlanOutputMerges(t *testing.T) {
	state := NewState("t")
	raw := "```json\n{\"subtasks\":[{\"id\":\"s1\",\"description\":\"do x\",\"done\":false}],\"hypotheses\":[{\"id\":\"h1\",\"description\":\"try y\",\"tried\":false}]}\n```"
	ParsePlanOutput(raw, state)
	if len(state.Subtasks) != 1 || state.Subtasks[0].ID != "s1" {
		t.Fatalf("expected one subtask, got %+v", state.Subtasks)
	}
	if len(state.Hypotheses) != 1 || state.Hypotheses[0].ID != "h1" {
		t.Fatalf("expected one hypothesis, got %+v", state.Hypotheses)
	}
}

func TestParsePlanOutputFailOpen(t *testing.T) {
	state := NewState("t")
	state.LastError = "compile error"
	ParsePlanOutput("not json", state)
	if len(state.Hypotheses) != 1 {
		t.Fatalf("expected default hypothesis on parse failure, got %+v", state.Hypotheses)
	}
	if state.Hypotheses[0].Description != "compile error" {
		t.Fatalf("expected default description from last error, got %q", state.Hypotheses[0].Description)
	}
}

func TestParseExecuteOutput(t *testing.T) {
	state := NewState("t")
	state.Hypotheses = []Hypothesis{{ID: "h1", Description: "try"}, {ID: "h2", Description: "fix"}}
	raw := "```json\n{\"completed_hypotheses\":[\"h1\"],\"completed_subtasks\":[\"s1\"]}\n```"
	state.Subtasks = []Subtask{{ID: "s1", Description: "step"}}
	ParseExecuteOutput(raw, state)
	if !state.Hypotheses[0].Tried || state.Hypotheses[1].Tried {
		t.Fatalf("expected only h1 tried: %+v", state.Hypotheses)
	}
	if !state.Subtasks[0].Done {
		t.Fatal("expected s1 done")
	}
}

func TestReadGuidance(t *testing.T) {
	input := "line one\nline two\n\n"
	r := bufio.NewReader(strings.NewReader(input))
	got := readGuidance(r)
	want := "line one\nline two"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
