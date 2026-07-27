package harness

import (
	"encoding/json"
	"fmt"
	"strings"
)

// planOutput is the structured data a plan/diagnose subagent should return
// inside a fenced JSON block. Markdown explanation is allowed outside the
// block.
type planOutput struct {
	Subtasks   []subtaskJSON   `json:"subtasks"`
	Hypotheses []hypothesisJSON `json:"hypotheses"`
}

type subtaskJSON struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

type hypothesisJSON struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Tried       bool   `json:"tried"`
}

// executeOutput lets the execute subagent report which hypothesis/subtask it
// actually completed, so the loop can update state without guessing.
type executeOutput struct {
	CompletedHypotheses []string `json:"completed_hypotheses"`
	CompletedSubtasks   []string `json:"completed_subtasks"`
}

// ExtractJSONBlock finds the last fenced ```json block in s and returns its
// content. If no fenced block exists, it falls back to the substring from the
// first '{' to the last '}'.
func ExtractJSONBlock(s string) (string, bool) {
	lower := strings.ToLower(s)
	startFence := strings.LastIndex(lower, "```json")
	if startFence == -1 {
		startBrace := strings.Index(s, "{")
		endBrace := strings.LastIndex(s, "}")
		if startBrace == -1 || endBrace == -1 || endBrace <= startBrace {
			return "", false
		}
		return s[startBrace : endBrace+1], true
	}
	contentStart := startFence + len("```json")
	endFence := strings.Index(lower[contentStart:], "```")
	if endFence == -1 {
		return "", false
	}
	return strings.TrimSpace(s[contentStart : contentStart+endFence]), true
}

// ParsePlanOutput parses a plan or diagnose response and merges the resulting
// subtasks/hypotheses into state. If the response cannot be parsed, it creates
// a single fail-open hypothesis so the self-correct loop does not pause
// immediately.
func ParsePlanOutput(raw string, state *State) {
	block, ok := ExtractJSONBlock(raw)
	if !ok {
		addDefaultHypothesis(state)
		return
	}
	var out planOutput
	if err := json.Unmarshal([]byte(block), &out); err != nil {
		addDefaultHypothesis(state)
		return
	}
	mergeSubtasks(state, out.Subtasks)
	mergeHypotheses(state, out.Hypotheses)
	if len(state.UntriedHypotheses()) == 0 && len(state.Hypotheses) == 0 {
		addDefaultHypothesis(state)
	}
}

// ParseExecuteOutput parses an execute response and updates which hypotheses
// and subtasks were completed. If no completions are reported, the first
// untried hypothesis is marked tried so the loop makes progress.
func ParseExecuteOutput(raw string, state *State) {
	block, ok := ExtractJSONBlock(raw)
	if ok {
		var out executeOutput
		if err := json.Unmarshal([]byte(block), &out); err == nil {
			for _, id := range out.CompletedHypotheses {
				state.MarkHypothesisTried(id)
			}
			for _, id := range out.CompletedSubtasks {
				state.MarkSubtaskDone(id)
			}
			return
		}
	}
	// Fallback: mark the first untried hypothesis as tried so the loop does not
	// retry the same idea forever.
	if hyps := state.UntriedHypotheses(); len(hyps) > 0 {
		state.MarkHypothesisTried(hyps[0].ID)
	}
}

func mergeSubtasks(state *State, incoming []subtaskJSON) {
	for _, st := range incoming {
		if st.ID == "" {
			continue
		}
		found := false
		for i, existing := range state.Subtasks {
			if existing.ID == st.ID {
				state.Subtasks[i].Description = firstNonEmpty(st.Description, existing.Description)
				if st.Done {
					state.Subtasks[i].Done = true
				}
				found = true
				break
			}
		}
		if !found {
			state.Subtasks = append(state.Subtasks, Subtask{
				ID:          st.ID,
				Description: st.Description,
				Done:        st.Done,
			})
		}
	}
}

func mergeHypotheses(state *State, incoming []hypothesisJSON) {
	for _, h := range incoming {
		if h.ID == "" {
			continue
		}
		found := false
		for i, existing := range state.Hypotheses {
			if existing.ID == h.ID {
				state.Hypotheses[i].Description = firstNonEmpty(h.Description, existing.Description)
				if h.Tried {
					state.Hypotheses[i].Tried = true
				}
				found = true
				break
			}
		}
		if !found {
			state.Hypotheses = append(state.Hypotheses, Hypothesis{
				ID:          h.ID,
				Description: h.Description,
				Tried:       h.Tried,
			})
		}
	}
}

func addDefaultHypothesis(state *State) {
	desc := state.LastError
	if desc == "" {
		desc = "continue from previous step"
	}
	id := fmt.Sprintf("auto-%d", state.Iteration)
	for _, h := range state.Hypotheses {
		if h.ID == id {
			id = fmt.Sprintf("auto-%d-%d", state.Iteration, len(state.Hypotheses))
		}
	}
	state.Hypotheses = append(state.Hypotheses, Hypothesis{
		ID:          id,
		Description: desc,
		Tried:       false,
	})
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
