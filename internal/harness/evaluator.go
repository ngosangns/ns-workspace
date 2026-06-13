package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type EvalResult struct {
	Name       string
	Command    string
	Passed     bool
	ExitCode   int
	Stdout     string
	Stderr     string
	Error      string
	MustPass   bool
	DurationMs int64
}

type Evaluator struct {
	ProjectRoot string
	Reporter    Reporter
}

func NewEvaluator(projectRoot string, reporter Reporter) *Evaluator {
	if reporter == nil {
		reporter = noopReporter{}
	}
	return &Evaluator{ProjectRoot: projectRoot, Reporter: reporter}
}

func (e *Evaluator) EvaluateAll(task *Task, prior map[string]bool) ([]EvalResult, bool) {
	commands := e.buildCommands(task)
	results := make([]EvalResult, 0, len(commands))
	allPassed := true
	status := map[string]bool{}
	for name, cmd := range commands {
		res := e.run(name, cmd)
		results = append(results, res)
		status[name] = res.Passed
		if !res.Passed && res.MustPass {
			allPassed = false
		}
	}
	for k, v := range status {
		prior[k] = v
	}
	return results, allPassed
}

func (e *Evaluator) buildCommands(task *Task) map[string]string {
	commands := map[string]string{}
	for i, acc := range task.Acceptance {
		name := fmt.Sprintf("accept-%d", i)
		if acc.Command != "" {
			commands[name] = acc.Command
		} else if acc.Script != "" {
			commands[name] = "sh " + acc.Script
		}
	}
	e.discoverPackageScripts(commands)
	e.discoverGoTests(commands)
	return commands
}

func (e *Evaluator) discoverGoTests(into map[string]string) {
	if _, ok := into["go-test"]; ok {
		return
	}
	if _, err := os.Stat(filepath.Join(e.ProjectRoot, "go.mod")); err != nil {
		return
	}
	into["go-test"] = "go test ./..."
}

func (e *Evaluator) discoverPackageScripts(into map[string]string) {
	pkgPath := filepath.Join(e.ProjectRoot, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}
	for _, key := range []string{"test", "lint", "typecheck", "build"} {
		if _, ok := pkg.Scripts[key]; ok {
			name := "pkg-" + key
			if _, exists := into[name]; !exists {
				into[name] = "npm run " + key
			}
		}
	}
}

func (e *Evaluator) run(name, command string) EvalResult {
	res := EvalResult{Name: name, Command: command, Passed: false}
	if command == "" {
		res.Error = "empty command"
		return res
	}
	e.Reporter.Line("eval: %s: %s", name, command)
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = e.ProjectRoot
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	res.Stdout = string(out)
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		res.Passed = false
	} else if err != nil {
		res.Error = err.Error()
		res.Passed = false
	} else {
		res.ExitCode = 0
		res.Passed = true
	}
	if !res.Passed {
		res.Stderr = strings.TrimSpace(res.Stdout)
	}
	e.Reporter.Line("eval result: %s: passed=%v exit=%d", name, res.Passed, res.ExitCode)
	return res
}

func (e *Evaluator) EvaluateScript(name, scriptPath string) EvalResult {
	return e.run(name, "sh "+scriptPath)
}
