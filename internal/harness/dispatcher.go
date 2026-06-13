package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type DispatchResult struct {
	Agent   string
	Prompt  string
	Stdout  string
	Stderr  string
	Success bool
	Error   string
}

type SubagentDriver interface {
	Name() string
	Dispatch(ctx context.Context, agent, prompt string) (DispatchResult, error)
	Available() bool
}

type OpenCodeDriver struct{}

func (d OpenCodeDriver) Name() string { return "opencode" }

func (d OpenCodeDriver) Available() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

func (d OpenCodeDriver) Dispatch(ctx context.Context, agent, prompt string) (DispatchResult, error) {
	res := DispatchResult{Agent: agent, Prompt: prompt}
	if !d.Available() {
		res.Error = "opencode not found on PATH"
		return res, fmt.Errorf("opencode not found")
	}
	args := []string{"run", "--dangerously-skip-permissions"}
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = projectRootFromContext(ctx)
	cmd.Stdin = os.Stdin
	out, err := cmd.CombinedOutput()
	res.Stdout = string(out)
	res.Success = err == nil
	if err != nil {
		res.Error = err.Error()
		res.Stderr = strings.TrimSpace(res.Stdout)
	}
	return res, nil
}

type MockDriver struct {
	Responses map[string]DispatchResult
}

func (d MockDriver) Name() string { return "mock" }

func (d MockDriver) Available() bool { return true }

func (d MockDriver) Dispatch(ctx context.Context, agent, prompt string) (DispatchResult, error) {
	if res, ok := d.Responses[prompt]; ok {
		res.Agent = agent
		res.Prompt = prompt
		return res, nil
	}
	return DispatchResult{Agent: agent, Prompt: prompt, Success: true, Stdout: "mock ok"}, nil
}

type DriverRegistry struct {
	drivers map[string]SubagentDriver
}

func NewDriverRegistry(extra ...SubagentDriver) *DriverRegistry {
	r := &DriverRegistry{drivers: map[string]SubagentDriver{}}
	r.Register(OpenCodeDriver{})
	for _, d := range extra {
		r.Register(d)
	}
	return r
}

func (r *DriverRegistry) Register(d SubagentDriver) {
	r.drivers[strings.ToLower(d.Name())] = d
}

func (r *DriverRegistry) Resolve(name string) SubagentDriver {
	if name == "" {
		name = "opencode"
	}
	if d, ok := r.drivers[strings.ToLower(name)]; ok {
		return d
	}
	return OpenCodeDriver{}
}

type contextKey string

const projectRootKey contextKey = "projectRoot"

func WithProjectRoot(ctx context.Context, root string) context.Context {
	return context.WithValue(ctx, projectRootKey, root)
}

func projectRootFromContext(ctx context.Context) string {
	if root, ok := ctx.Value(projectRootKey).(string); ok {
		return root
	}
	return ""
}
