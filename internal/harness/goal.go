package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// GoalOptions điều khiển cách MaterializeGoalTask sinh task file từ goal text.
type GoalOptions struct {
	// Scope giới hạn scope.include; rỗng nghĩa là "**".
	Scope []string
	// Refine bật bước gọi plan subagent để đề xuất requirements/scope chi tiết.
	Refine bool
	// Agent dùng cho bước refine; rỗng nghĩa là "opencode-planner".
	Agent string
}

// goalTaskYAML là shape YAML tối giản ghi ra .harness/tasks/<slug>.yaml.
// Dùng struct riêng (thay vì Task) để output gọn, không lộ field rỗng.
type goalTaskYAML struct {
	ID           string         `yaml:"id"`
	Description  string         `yaml:"description"`
	Requirements []Requirement  `yaml:"requirements"`
	Scope        Scope          `yaml:"scope,omitempty"`
	Routing      Routing        `yaml:"routing"`
	Stopping     StoppingConfig `yaml:"stopping"`
}

// goalStopwords là các từ phổ biến bị loại khi tạo slug.
var goalStopwords = map[string]bool{
	"hãy": true, "cho": true, "tôi": true, "một": true, "các": true,
	"của": true, "và": true, "với": true, "trong": true, "là": true,
	"the": true, "a": true, "an": true, "to": true, "please": true,
	"for": true, "of": true, "and": true, "in": true, "on": true,
}

// SlugifyGoal tạo task id kebab-case từ goal text: giữ tối đa 6 từ có nghĩa,
// bỏ stopword, chỉ giữ ký tự [a-z0-9]. Fallback "goal-task" khi không còn gì.
func SlugifyGoal(goal string) string {
	words := strings.FieldsFunc(strings.ToLower(goal), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var kept []string
	for _, w := range words {
		if goalStopwords[w] {
			continue
		}
		ascii := stripNonASCII(w)
		if ascii == "" {
			continue
		}
		kept = append(kept, ascii)
		if len(kept) == 6 {
			break
		}
	}
	if len(kept) == 0 {
		return "goal-task"
	}
	return strings.Join(kept, "-")
}

func stripNonASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// MaterializeGoalTask sinh (hoặc reuse) task file từ goal text rồi trả về
// task đã load. Idempotent: nội dung giống hệt thì reuse; trùng id nhưng khác
// nội dung thì tạo slug mới với suffix -2, -3, ...
//
// Acceptance cố tình để trống: evaluator auto-discover `go test ./...` và
// package.json scripts nên goal task chạy được ngay trên repo Go/Node.
func MaterializeGoalTask(ctx context.Context, projectRoot, goalText string, opts GoalOptions, dispatcher *DriverRegistry) (*Task, string, error) {
	goalText = strings.TrimSpace(goalText)
	if goalText == "" {
		return nil, "", fmt.Errorf("goal text is required")
	}
	taskDir := filepath.Join(projectRoot, ".harness", "tasks")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		return nil, "", err
	}

	requirements := []Requirement{{ID: "REQ-1", Text: goalText}}
	scope := Scope{Include: opts.Scope}
	if len(scope.Include) == 0 {
		scope.Include = []string{"**"}
	}
	if opts.Refine {
		if r, s, ok := refineGoalWithAgent(ctx, projectRoot, goalText, opts.Agent, dispatcher); ok {
			if len(r) > 0 {
				requirements = r
			}
			if len(s.Include) > 0 {
				scope = s
			}
		}
	}

	task := goalTaskYAML{
		ID:           SlugifyGoal(goalText),
		Description:  goalText,
		Requirements: requirements,
		Scope:        scope,
		Routing: Routing{
			Default:  "opencode",
			Plan:     RoutingRule{Agent: "opencode-planner"},
			Execute:  RoutingRule{Agent: "opencode-executor"},
			Verify:   RoutingRule{Agent: "eval-judge"},
			Diagnose: RoutingRule{Agent: "opencode-fixer"},
		},
		Stopping: StoppingConfig{
			MaxConsecutiveFailures:  3,
			RequireHumanOnAmbiguity: true,
		},
	}
	data, err := yaml.Marshal(task)
	if err != nil {
		return nil, "", err
	}

	base := task.ID
	for i := 1; ; i++ {
		id := base
		if i > 1 {
			id = fmt.Sprintf("%s-%d", base, i)
		}
		path := filepath.Join(taskDir, id+".yaml")
		existing, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			task.ID = id
			data, err = yaml.Marshal(task)
			if err != nil {
				return nil, "", err
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return nil, "", err
			}
			loaded, err := LoadTask(path)
			if err != nil {
				return nil, "", fmt.Errorf("materialized task %s does not parse: %w", path, err)
			}
			return loaded, path, nil
		}
		if err != nil {
			return nil, "", err
		}
		// Cùng ID: nếu nội dung giống hệt (trừ id) thì reuse; khác thì slug mới.
		if sameGoalTask(existing, data) {
			loaded, err := LoadTask(path)
			if err != nil {
				return nil, "", err
			}
			return loaded, path, nil
		}
	}
}

// sameGoalTask so sánh 2 YAML task, bỏ qua field id (có thể khác do suffix).
func sameGoalTask(existing, candidate []byte) bool {
	var a, b goalTaskYAML
	if err := yaml.Unmarshal(existing, &a); err != nil {
		return false
	}
	if err := yaml.Unmarshal(candidate, &b); err != nil {
		return false
	}
	a.ID = ""
	b.ID = ""
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// goalRefineOutput là JSON mà plan subagent trả về khi refine goal.
type goalRefineOutput struct {
	Requirements []Requirement `json:"requirements"`
	Scope        Scope         `json:"scope"`
}

func refineGoalWithAgent(ctx context.Context, projectRoot, goalText, agent string, dispatcher *DriverRegistry) ([]Requirement, Scope, bool) {
	if dispatcher == nil {
		return nil, Scope{}, false
	}
	if agent == "" {
		agent = "opencode-planner"
	}
	prompt := buildGoalRefinePrompt(goalText)
	res, err := dispatcher.Resolve(agent).Dispatch(WithProjectRoot(ctx, projectRoot), agent, prompt)
	if err != nil || !res.Success {
		return nil, Scope{}, false
	}
	block, ok := ExtractJSONBlock(res.Stdout)
	if !ok {
		return nil, Scope{}, false
	}
	var out goalRefineOutput
	if err := json.Unmarshal([]byte(block), &out); err != nil {
		return nil, Scope{}, false
	}
	return out.Requirements, out.Scope, true
}

func buildGoalRefinePrompt(goal string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Goal: %s\n", goal))
	b.WriteString("Propose concrete requirements and code scope for this goal. " +
		"Return a fenced JSON block (```json ... ```) with this exact schema:\n")
	b.WriteString(`{"requirements":[{"id":"REQ-1","text":"..."}],`)
	b.WriteString(`"scope":{"include":["internal/**"],"exclude":[]}}`)
	b.WriteString("\n")
	return b.String()
}
