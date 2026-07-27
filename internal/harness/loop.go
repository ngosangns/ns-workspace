package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type LoopController struct {
	Engine     *Engine
	Evaluator  *Evaluator
	Dispatcher *DriverRegistry
	Reporter   Reporter
}

type LoopResult struct {
	State      *State
	Finalized  bool
	Reason     string
	Iterations int
}

func (lc *LoopController) Run(ctx context.Context, task *Task, state *State) (*LoopResult, error) {
	phases := task.DefaultPhases()
	maxFail := task.DefaultMaxConsecutiveFailures()
	store := NewStore(lc.Engine.ProjectRoot, task)
	for {
		if state.Paused {
			if lc.Engine.Interactive {
				lc.Reporter.Line("task %s is paused: %s", task.ID, state.PausedReason)
				lc.Reporter.Line("waiting for user input...")
				fmt.Println("Harness paused. Options:")
				fmt.Println("  [r] resume")
				fmt.Println("  [a] answer with guidance")
				fmt.Println("  [s] stop")
				fmt.Println("  [q] quit without stopping")
				var choice string
				fmt.Print("Choice: ")
				if _, err := fmt.Scanln(&choice); err != nil {
					choice = "q"
				}
				switch choice {
				case "r", "resume":
					state.Paused = false
					state.PausedReason = ""
					_ = store.Save(state)
				case "a", "answer":
					fmt.Println("Enter guidance (blank line to finish):")
					guidance := readGuidance(bufio.NewReader(os.Stdin))
					state.ContextNotes["user_guidance"] = guidance
					state.Paused = false
					state.PausedReason = ""
					_ = store.Save(state)
				case "s", "stop":
					_ = store.Save(state)
					return &LoopResult{State: state, Finalized: false, Reason: "user stopped", Iterations: state.Iteration}, nil
				default:
					_ = store.Save(state)
					return &LoopResult{State: state, Finalized: false, Reason: state.PausedReason, Iterations: state.Iteration}, nil
				}
			} else {
				if lc.Engine.DecisionWriter != nil {
					if err := lc.Engine.DecisionWriter.Write(lc.Engine.ProjectRoot, state, task); err != nil {
						lc.Reporter.Line("warning: failed to write decision request: %v", err)
					}
				}
				_ = store.Save(state)
				return &LoopResult{State: state, Finalized: false, Reason: state.PausedReason, Iterations: state.Iteration}, nil
			}
		}
		if state.HasRepeatedState() {
			state.Paused = true
			state.PausedReason = "detected repeated state"
			if lc.Engine.DecisionWriter != nil {
				_ = lc.Engine.DecisionWriter.Write(lc.Engine.ProjectRoot, state, task)
			}
			_ = store.Save(state)
			return &LoopResult{State: state, Finalized: false, Reason: state.PausedReason, Iterations: state.Iteration}, nil
		}
		phase := lc.pickPhase(phases, state)
		state.Phase = phase
		state.Iteration++
		lc.Reporter.Line("loop: iteration=%d phase=%s", state.Iteration, phase)
		switch phase {
		case "plan":
			if err := lc.runPlan(ctx, task, state); err != nil {
				state.LastError = err.Error()
				state.ConsecutiveFailures++
			}
		case "execute":
			if err := lc.runExecute(ctx, task, state); err != nil {
				state.LastError = err.Error()
				state.ConsecutiveFailures++
			}
		case "verify":
			finalized, reason := lc.runVerify(ctx, task, state)
			if finalized {
				state.Phase = "finalized"
				_ = store.Save(state)
				return &LoopResult{State: state, Finalized: true, Reason: reason, Iterations: state.Iteration}, nil
			}
			// After a verify failure, run diagnose to produce new hypotheses before
			// the next execute attempt. Diagnose errors are recorded but do not
			// count toward consecutive failures (the verify failure already does).
			if state.ConsecutiveFailures >= 1 && state.ConsecutiveFailures < maxFail {
				if err := lc.runDiagnose(ctx, task, state); err != nil {
					state.LastError = err.Error()
				}
				state.Phase = "diagnose"
			}
		}
		state.RecordSnapshot()
		if state.ConsecutiveFailures >= maxFail {
			state.Paused = true
			state.PausedReason = fmt.Sprintf("%d consecutive failures", state.ConsecutiveFailures)
			if lc.Engine.DecisionWriter != nil {
				_ = lc.Engine.DecisionWriter.Write(lc.Engine.ProjectRoot, state, task)
			}
			_ = store.Save(state)
			return &LoopResult{State: state, Finalized: false, Reason: state.PausedReason, Iterations: state.Iteration}, nil
		}
		if task.Stopping.RequireHumanOnAmbiguity {
			if ok, reason := lc.detectAmbiguity(state); ok {
				state.Paused = true
				state.PausedReason = reason
				if lc.Engine.DecisionWriter != nil {
					_ = lc.Engine.DecisionWriter.Write(lc.Engine.ProjectRoot, state, task)
				}
				_ = store.Save(state)
				return &LoopResult{State: state, Finalized: false, Reason: state.PausedReason, Iterations: state.Iteration}, nil
			}
		}
		if err := store.Save(state); err != nil {
			return nil, err
		}
	}
}

func (lc *LoopController) pickPhase(phases []string, state *State) string {
	if state.Phase == "verify" || state.Phase == "diagnose" {
		return "execute"
	}
	nextIdx := 0
	for i, p := range phases {
		if p == state.Phase {
			nextIdx = i + 1
			break
		}
	}
	for i := nextIdx; i < len(phases); i++ {
		if phases[i] == "verify" && state.Iteration == 0 {
			continue
		}
		return phases[i]
	}
	for _, p := range phases {
		if p == "verify" && state.Iteration == 0 {
			return p
		}
	}
	return "execute"
}

func (lc *LoopController) runPlan(ctx context.Context, task *Task, state *State) error {
	// Enrich-docs tasks run the full plan→fetch→execute→write flow in a single
	// call (runEnrich). Drive it from the plan phase so it executes exactly once;
	// the execute phase is a no-op for these tasks (see runExecute). The verify
	// phase still runs afterwards to evaluate acceptance commands (Req 5.1, 6.4).
	if task.Type == "enrich-docs" {
		return lc.runEnrich(ctx, task, state)
	}
	agent := task.SelectAgent("plan")
	prompt := buildPlanPrompt(task, state)
	res, err := lc.Dispatcher.Resolve(agent).Dispatch(WithProjectRoot(ctx, lc.Engine.ProjectRoot), agent, prompt)
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("plan subagent failed: %s", res.Stderr)
	}
	state.ContextNotes["last_plan"] = res.Stdout
	ParsePlanOutput(res.Stdout, state)
	consumeUserGuidance(state)
	return nil
}

func (lc *LoopController) runExecute(ctx context.Context, task *Task, state *State) error {
	// Enrich-docs tasks complete their work during the plan phase via runEnrich,
	// so the execute phase is intentionally a no-op to avoid re-running the LLM
	// enrichment (Req 5.1).
	if task.Type == "enrich-docs" {
		return nil
	}
	agent := task.SelectAgent("execute")
	prompt := buildExecutePrompt(task, state)
	res, err := lc.Dispatcher.Resolve(agent).Dispatch(WithProjectRoot(ctx, lc.Engine.ProjectRoot), agent, prompt)
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("execute subagent failed: %s", res.Stderr)
	}
	state.ContextNotes["last_execute"] = res.Stdout
	ParseExecuteOutput(res.Stdout, state)
	consumeUserGuidance(state)
	return nil
}

func (lc *LoopController) runVerify(ctx context.Context, task *Task, state *State) (bool, string) {
	results, allPassed := lc.Evaluator.EvaluateAll(task, state.AcceptanceStatus)
	// Run LLM judge for text-only acceptance criteria when a verify agent is
	// configured. Command/script criteria are handled by EvaluateAll.
	for i, acc := range task.Acceptance {
		if acc.Command == "" && acc.Script == "" && acc.Text != "" {
			res := lc.runJudge(ctx, task, state, acc, i)
			results = append(results, res)
			state.AcceptanceStatus[res.Name] = res.Passed
			if !res.Passed && res.MustPass {
				allPassed = false
			}
		}
	}
	if allPassed {
		return true, "all acceptance criteria passed"
	}
	var failures []string
	for _, r := range results {
		if !r.Passed && r.MustPass {
			failures = append(failures, r.Name)
		}
	}
	if len(failures) > 0 {
		state.LastError = strings.Join(failures, ", ")
		state.ConsecutiveFailures++
	}
	if state.AllSubtasksDone() {
		return true, "all subtasks completed"
	}
	return false, "verify incomplete"
}

func (lc *LoopController) runJudge(ctx context.Context, task *Task, state *State, acc Acceptance, idx int) EvalResult {
	name := fmt.Sprintf("judge-%d", idx)
	// Only run an LLM judge if a verify agent or default agent is explicitly
	// configured. Otherwise skip the criterion to avoid unexpected subprocess
	// calls.
	if task.Routing.Verify.Agent == "" && task.Routing.Default == "" {
		return EvalResult{
			Name:     name,
			Passed:   !acc.MustPass,
			MustPass: acc.MustPass,
			Error:    "no verify agent configured; skipped",
		}
	}
	agent := task.SelectAgent("verify")
	prompt := buildJudgePrompt(task, state, acc.Text)
	res, err := lc.Dispatcher.Resolve(agent).Dispatch(WithProjectRoot(ctx, lc.Engine.ProjectRoot), agent, prompt)
	if err != nil {
		return EvalResult{Name: name, Passed: false, MustPass: acc.MustPass, Error: err.Error()}
	}
	if !res.Success {
		return EvalResult{Name: name, Passed: false, MustPass: acc.MustPass, Stderr: res.Stderr, Error: res.Error}
	}
	passed := parseJudgeVerdict(res.Stdout)
	return EvalResult{Name: name, Passed: passed, MustPass: acc.MustPass, Stdout: res.Stdout}
}

func parseJudgeVerdict(raw string) bool {
	if block, ok := ExtractJSONBlock(raw); ok {
		var v struct {
			Verdict string `json:"verdict"`
		}
		if err := json.Unmarshal([]byte(block), &v); err == nil {
			return strings.EqualFold(strings.TrimSpace(v.Verdict), "PASS")
		}
	}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "PASS") {
			return true
		}
		if strings.HasPrefix(strings.ToUpper(line), "FAIL") {
			return false
		}
	}
	return false
}

func buildJudgePrompt(task *Task, state *State, criterion string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Description: %s\n", task.Description))
	b.WriteString(fmt.Sprintf("Criterion: %s\n", criterion))
	if state.LastError != "" {
		b.WriteString(fmt.Sprintf("Last error: %s\n", state.LastError))
	}
	if execOut, ok := state.ContextNotes["last_execute"]; ok {
		b.WriteString(fmt.Sprintf("Last execution output:\n%s\n", execOut))
	}
	b.WriteString("Return a verdict as a fenced JSON block (```json ... ```): {\"verdict\":\"PASS\"} or {\"verdict\":\"FAIL\"}.\n")
	return b.String()
}

func (lc *LoopController) runDiagnose(ctx context.Context, task *Task, state *State) error {
	agent := task.SelectAgent("diagnose")
	prompt := buildDiagnosePrompt(task, state)
	res, err := lc.Dispatcher.Resolve(agent).Dispatch(WithProjectRoot(ctx, lc.Engine.ProjectRoot), agent, prompt)
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("diagnose subagent failed: %s", res.Stderr)
	}
	state.ContextNotes["last_diagnose"] = res.Stdout
	ParsePlanOutput(res.Stdout, state)
	consumeUserGuidance(state)
	return nil
}

func (lc *LoopController) detectAmbiguity(state *State) (bool, string) {
	for _, key := range []string{"last_plan", "last_diagnose"} {
		if note, ok := state.ContextNotes[key]; ok {
			if q, found := extractAmbiguityMarker(note); found {
				return true, q
			}
		}
	}
	return false, ""
}

func extractAmbiguityMarker(s string) (string, bool) {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "AMBIGUITY:"
		if strings.HasPrefix(strings.ToUpper(line), prefix) {
			return strings.TrimSpace(line[len(prefix):]), true
		}
	}
	// Fallback heuristic for outputs that do not use the structured marker.
	if strings.Contains(strings.ToLower(s), "ambiguous") {
		return "ambiguity detected", true
	}
	return "", false
}

func buildPlanPrompt(task *Task, state *State) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Description: %s\n", task.Description))
	b.WriteString("Requirements:\n")
	for _, r := range task.Requirements {
		b.WriteString(fmt.Sprintf("- %s: %s\n", r.ID, r.Text))
	}
	b.WriteString("Scope:\n")
	for _, inc := range task.Scope.Include {
		b.WriteString(fmt.Sprintf("- include: %s\n", inc))
	}
	for _, exc := range task.Scope.Exclude {
		b.WriteString(fmt.Sprintf("- exclude: %s\n", exc))
	}
	if state.LastError != "" {
		b.WriteString(fmt.Sprintf("Last error: %s\n", state.LastError))
	}
	b.WriteString("\n")
	b.WriteString("Produce a plan with concrete subtasks and hypotheses. " +
		"Return a fenced JSON block (```json ... ```) with this exact schema:\n")
	b.WriteString(`{"subtasks":[{"id":"...","description":"...","done":false}],`)
	b.WriteString(`"hypotheses":[{"id":"...","description":"...","tried":false}]}`)
	b.WriteString("\n")
	b.WriteString("If the task is ambiguous, start a line with AMBIGUITY: <question>.\n")
	appendUserGuidance(&b, state)
	return b.String()
}

func buildExecutePrompt(task *Task, state *State) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", task.ID))
	b.WriteString("Phase: execute\n")
	if plan, ok := state.ContextNotes["last_plan"]; ok {
		b.WriteString(fmt.Sprintf("Plan:\n%s\n", plan))
	}
	if hyps := state.UntriedHypotheses(); len(hyps) > 0 {
		b.WriteString("Try the first untried hypothesis and mark it done.\n")
	} else {
		b.WriteString("Implement the next unfinished subtask.\n")
	}
	b.WriteString("Return a summary, followed by a fenced JSON block with this schema:\n")
	b.WriteString(`{"completed_hypotheses":["id1"],"completed_subtasks":["id1"]}`)
	b.WriteString("\n")
	appendUserGuidance(&b, state)
	return b.String()
}

func buildDiagnosePrompt(task *Task, state *State) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Last error: %s\n", state.LastError))
	if execOut, ok := state.ContextNotes["last_execute"]; ok {
		b.WriteString(fmt.Sprintf("Last execution output:\n%s\n", execOut))
	}
	if hyps := state.Hypotheses; len(hyps) > 0 {
		b.WriteString("Existing hypotheses:\n")
		for _, h := range hyps {
			status := "untried"
			if h.Tried {
				status = "tried"
			}
			b.WriteString(fmt.Sprintf("- %s (%s): %s\n", h.ID, status, h.Description))
		}
	}
	b.WriteString("Diagnose the failure, propose new hypotheses, and mark exhausted ones. " +
		"Return a fenced JSON block (```json ... ```) with this exact schema:\n")
	b.WriteString(`{"subtasks":[{"id":"...","description":"...","done":false}],`)
	b.WriteString(`"hypotheses":[{"id":"...","description":"...","tried":false}]}`)
	b.WriteString("\n")
	b.WriteString("If the situation is ambiguous, start a line with AMBIGUITY: <question>.\n")
	appendUserGuidance(&b, state)
	return b.String()
}

func appendUserGuidance(b *strings.Builder, state *State) {
	if g, ok := state.ContextNotes["user_guidance"]; ok && g != "" {
		b.WriteString("\nUser guidance:\n")
		b.WriteString(g)
		b.WriteString("\n")
	}
}

func consumeUserGuidance(state *State) {
	delete(state.ContextNotes, "user_guidance")
}

func readGuidance(r *bufio.Reader) string {
	var lines []string
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimRight(line, "\n")
		if line == "" || err != nil {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
