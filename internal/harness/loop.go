package harness

import (
	"context"
	"fmt"
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
	if maxFail <= 0 {
		maxFail = 3
	}
	store := NewStore(lc.Engine.ProjectRoot, task)
	for {
		if state.Paused {
			if lc.Engine.Interactive {
				lc.Reporter.Line("task %s is paused: %s", task.ID, state.PausedReason)
				lc.Reporter.Line("waiting for user input...")
				fmt.Println("Harness paused. Options:")
				fmt.Println("  [r] resume")
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
			finalized, reason, err := lc.runVerify(task, state)
			if err != nil {
				state.LastError = err.Error()
				state.ConsecutiveFailures++
				break
			}
			if finalized {
				state.Phase = "finalized"
				_ = store.Save(state)
				return &LoopResult{State: state, Finalized: true, Reason: reason, Iterations: state.Iteration}, nil
			}
			if state.ConsecutiveFailures >= 1 {
				state.Phase = "diagnose"
			}
		case "diagnose":
			if err := lc.runDiagnose(ctx, task, state); err != nil {
				state.LastError = err.Error()
				state.ConsecutiveFailures++
			} else {
				state.ConsecutiveFailures = 0
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
		if task.Stopping.RequireHumanOnAmbiguity && lc.detectAmbiguity(state) {
			state.Paused = true
			state.PausedReason = "ambiguity detected"
			if lc.Engine.DecisionWriter != nil {
				_ = lc.Engine.DecisionWriter.Write(lc.Engine.ProjectRoot, state, task)
			}
			_ = store.Save(state)
			return &LoopResult{State: state, Finalized: false, Reason: state.PausedReason, Iterations: state.Iteration}, nil
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
	return nil
}

func (lc *LoopController) runExecute(ctx context.Context, task *Task, state *State) error {
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
	return nil
}

func (lc *LoopController) runVerify(task *Task, state *State) (bool, string, error) {
	results, allPassed := lc.Evaluator.EvaluateAll(task, state.AcceptanceStatus)
	if allPassed {
		return true, "all acceptance criteria passed", nil
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
	if state.AllAcceptancePassed() {
		return true, "all acceptance criteria passed", nil
	}
	if state.AllSubtasksDone() {
		return true, "all subtasks completed", nil
	}
	if len(state.UntriedHypotheses()) == 0 && state.ConsecutiveFailures > 0 {
		state.Paused = true
		state.PausedReason = "verify failed and no untried hypotheses"
		return false, state.PausedReason, nil
	}
	return false, "verify incomplete", nil
}

func (lc *LoopController) runDiagnose(ctx context.Context, task *Task, state *State) error {
	agent := task.SelectAgent("execute")
	prompt := buildDiagnosePrompt(task, state)
	res, err := lc.Dispatcher.Resolve(agent).Dispatch(WithProjectRoot(ctx, lc.Engine.ProjectRoot), agent, prompt)
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("diagnose subagent failed: %s", res.Stderr)
	}
	state.ContextNotes["last_diagnose"] = res.Stdout
	return nil
}

func (lc *LoopController) detectAmbiguity(state *State) bool {
	if note, ok := state.ContextNotes["last_plan"]; ok && strings.Contains(strings.ToLower(note), "ambiguous") {
		return true
	}
	if note, ok := state.ContextNotes["last_diagnose"]; ok && strings.Contains(strings.ToLower(note), "ambiguous") {
		return true
	}
	return false
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
	b.WriteString("Produce a plan with concrete subtasks and hypotheses. Return as markdown list.")
	return b.String()
}

func buildExecutePrompt(task *Task, state *State) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Phase: execute\n"))
	if plan, ok := state.ContextNotes["last_plan"]; ok {
		b.WriteString(fmt.Sprintf("Plan:\n%s\n", plan))
	}
	if hyps := state.UntriedHypotheses(); len(hyps) > 0 {
		b.WriteString("Try the first untried hypothesis and mark it done.\n")
	} else {
		b.WriteString("Implement the next unfinished subtask.\n")
	}
	b.WriteString("Return a summary of files changed and tests run.")
	return b.String()
}

func buildDiagnosePrompt(task *Task, state *State) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Last error: %s\n", state.LastError))
	if execOut, ok := state.ContextNotes["last_execute"]; ok {
		b.WriteString(fmt.Sprintf("Last execution output:\n%s\n", execOut))
	}
	b.WriteString("Diagnose the failure, propose new hypotheses, and mark which existing hypotheses are exhausted.")
	return b.String()
}
