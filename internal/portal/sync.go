package portal

import (
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
)

// completedJobTTL is how long finished sync jobs stay queryable so the
// SSE client can attach after a fast command (status/doctor) finishes.
const completedJobTTL = 2 * time.Minute

// SyncRunner executes agentsync commands and streams report lines.
type SyncRunner struct {
	presets fs.FS

	mu     sync.Mutex
	jobs   map[string]*syncJob
	nextID int
}

type syncJob struct {
	mu      sync.Mutex
	ID      string
	Command string
	Lines   []string
	Done    bool
	Err     error
	cond    *sync.Cond
}

// syncReporter implements agentsync.StatusReporter and forwards lines to a job.
type syncReporter struct {
	job *syncJob
}

func (r syncReporter) Line(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	r.job.append(line)
}

// NewSyncRunner creates a new sync runner.
func NewSyncRunner(presets fs.FS) *SyncRunner {
	return &SyncRunner{
		presets: presets,
		jobs:    map[string]*syncJob{},
	}
}

// Start begins a sync command and returns a job ID.
// tools is a comma-separated provider filter (e.g. "claude" or "claude,codex");
// an empty value means all providers.
func (r *SyncRunner) Start(command string, agentsDir string, tools string) (string, error) {
	switch command {
	case "init", "update", "status", "doctor", "registry":
		// supported
	default:
		return "", fmt.Errorf("unsupported sync command %q", command)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := fmt.Sprintf("%d-%d", time.Now().Unix(), r.nextID)
	job := &syncJob{
		ID:      id,
		Command: command,
	}
	job.cond = sync.NewCond(&job.mu)
	r.jobs[id] = job

	go r.run(job, agentsDir, tools)
	return id, nil
}

func (r *SyncRunner) run(job *syncJob, agentsDir string, tools string) {
	defer func() {
		job.mu.Lock()
		job.Done = true
		job.mu.Unlock()
		job.broadcast()
		// Keep the job long enough for the portal SSE client to connect
		// and drain lines. Fast commands (status/doctor) often finish
		// before the browser opens EventSource; deleting immediately
		// made those streams return 404 with an empty terminal.
		time.AfterFunc(completedJobTTL, func() {
			r.mu.Lock()
			delete(r.jobs, job.ID)
			r.mu.Unlock()
		})
	}()

	manager := agentsync.Manager{Presets: r.presets}
	opt, err := r.buildOptions(job.Command, agentsDir, tools)
	if err != nil {
		job.Err = err
		job.append(fmt.Sprintf("error: %v", err))
		return
	}

	ctx, err := manager.ContextWithReporter(opt, syncReporter{job: job})
	if err != nil {
		job.Err = err
		job.append(fmt.Sprintf("error: %v", err))
		return
	}

	switch job.Command {
	case "init":
		job.Err = manager.ApplyWithContext(ctx, false)
	case "update":
		job.Err = manager.ApplyWithContext(ctx, true)
	case "status":
		job.Err = manager.StatusWithContext(ctx)
	case "doctor":
		job.Err = manager.DoctorWithContext(ctx)
	case "registry":
		job.Err = manager.InstallRegistrySkillsWithContext(ctx)
	}
	if job.Err != nil {
		job.append(fmt.Sprintf("error: %v", job.Err))
	}
}

func (r *SyncRunner) buildOptions(command string, agentsDir string, tools string) (agentsync.Options, error) {
	homeDefault, err := agentsync.DefaultAgentsDir()
	if err != nil {
		return agentsync.Options{}, err
	}
	if agentsDir == "" {
		agentsDir = homeDefault
	}
	configDefault, err := agentsync.DefaultUserConfigPath()
	if err != nil {
		return agentsync.Options{}, err
	}
	toolFilter := agentsync.ParseTools(tools)
	if len(toolFilter) == 0 {
		toolFilter = map[string]bool{"all": true}
	}
	return agentsync.Options{
		Command:    command,
		AgentsDir:  agentsDir,
		ConfigPath: configDefault,
		ToolFilter: toolFilter,
	}, nil
}

// Job returns a snapshot of a job. If id is empty, returns nil.
func (r *SyncRunner) Job(id string) *syncJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jobs[id]
}

// Subscribe streams lines to the provided callback until the job is done.
// It returns immediately if the job is unknown or already done (after
// replaying any buffered lines).
func (job *syncJob) Subscribe(fn func(string)) {
	if job == nil {
		return
	}
	seen := 0
	for {
		job.mu.Lock()
		for !job.Done && seen >= len(job.Lines) {
			job.cond.Wait()
		}
		lines := job.Lines[seen:]
		done := job.Done
		job.mu.Unlock()
		for _, line := range lines {
			fn(line)
			seen++
		}
		if done {
			return
		}
	}
}

func (job *syncJob) append(line string) {
	job.mu.Lock()
	job.Lines = append(job.Lines, line)
	job.mu.Unlock()
	job.broadcast()
}

func (job *syncJob) broadcast() {
	job.cond.Broadcast()
}
