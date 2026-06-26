package cli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/ngosangns/ns-workspace/internal/harness"
)

// engineAPI là tập các phương thức mà RunHarness gọi. Tách thành interface
// để test có thể inject mock trả về error / result theo ý muốn.
type engineAPI interface {
	ListTasks() ([]*harness.Task, error)
	Run(ctx context.Context, id string, dryRun bool) (*harness.LoopResult, error)
	Eval(id string) ([]harness.EvalResult, error)
	Status(id string) (*harness.State, error)
	Resume(ctx context.Context, id string) (*harness.LoopResult, error)
	Stop(id string) error
}

// filepathAbs là seam test: cho phép thay thế filepath.Abs để mô phỏng error path.
var filepathAbs = filepath.Abs

// newEngine cho phép test override engine creation (vd: inject mock engine).
var newEngine = func(root string, reporter harness.Reporter) engineAPI {
	return harness.NewEngine(root, reporter)
}

// validSubcommands là danh sách subcommand hợp lệ; biến package-level để test
// có thể tạm thời mở rộng nếu cần cover nhánh default.
var validSubcommands = map[string]bool{"list": true, "run": true, "eval": true, "status": true, "resume": true, "stop": true}

func IsHarnessCommand(cmd string) bool {
	switch cmd {
	case "harness":
		return true
	default:
		return false
	}
}

func RunHarness(args []string) error {
	cmd := ""
	cmdIdx := -1
	for i, arg := range args {
		if validSubcommands[arg] {
			cmd = arg
			cmdIdx = i
			break
		}
	}
	if cmd == "" {
		return fmt.Errorf("usage: harness <list|run|eval|status|resume|stop> [flags]")
	}
	flagArgs := append(args[:cmdIdx], args[cmdIdx+1:]...)
	flagSet := flag.NewFlagSet("harness", flag.ContinueOnError)
	project := flagSet.String("project", ".", "project root to inspect")
	taskID := flagSet.String("task", "", "task id to run/eval/status/resume/stop")
	dryRun := flagSet.Bool("dry-run", false, "show planned actions without running")
	if err := flagSet.Parse(flagArgs); err != nil {
		return err
	}
	root, err := filepathAbs(*project)
	if err != nil {
		return err
	}
	engine := newEngine(root, nil)
	ctx := context.Background()
	switch cmd {
	case "list":
		tasks, err := engine.ListTasks()
		if err != nil {
			return err
		}
		for _, task := range tasks {
			fmt.Printf("%s: %s\n", task.ID, task.Description)
		}
		return nil
	case "run":
		if *taskID == "" {
			return fmt.Errorf("--task required")
		}
		res, err := engine.Run(ctx, *taskID, *dryRun)
		if err != nil {
			return err
		}
		fmt.Printf("harness %s: finalized=%v reason=%s iterations=%d\n", *taskID, res.Finalized, res.Reason, res.Iterations)
		if res.State != nil && res.State.Paused {
			fmt.Printf("paused: %s\n", res.State.PausedReason)
		}
		return nil
	case "eval":
		if *taskID == "" {
			return fmt.Errorf("--task required")
		}
		results, err := engine.Eval(*taskID)
		if err != nil {
			return err
		}
		for _, r := range results {
			fmt.Printf("%s: passed=%v exit=%d\n", r.Name, r.Passed, r.ExitCode)
		}
		return nil
	case "status":
		if *taskID == "" {
			return fmt.Errorf("--task required")
		}
		state, err := engine.Status(*taskID)
		if err != nil {
			return err
		}
		fmt.Printf("phase=%s iteration=%d paused=%v\n", state.Phase, state.Iteration, state.Paused)
		if state.Paused {
			fmt.Printf("paused reason: %s\n", state.PausedReason)
		}
		return nil
	case "resume":
		if *taskID == "" {
			return fmt.Errorf("--task required")
		}
		res, err := engine.Resume(ctx, *taskID)
		if err != nil {
			return err
		}
		fmt.Printf("harness %s: finalized=%v reason=%s iterations=%d\n", *taskID, res.Finalized, res.Reason, res.Iterations)
		return nil
	case "stop":
		if *taskID == "" {
			return fmt.Errorf("--task required")
		}
		if err := engine.Stop(*taskID); err != nil {
			return err
		}
		fmt.Printf("stopped %s\n", *taskID)
		return nil
	default:
		return fmt.Errorf("unknown harness subcommand %q", cmd)
	}
}
