package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// runSetupFromDir chạy setup từ một working directory giả lập (thông qua
// override target) và trả về stdout output. Dùng để test end-to-end mà không
// cần phụ thuộc vào os.Getwd() thật.
func runSetupFromDir(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"--target", dir}, args...)
	var buf bytes.Buffer
	// Gọi trực tiếp writeTaskfile thông qua một hàm wrapper để capture output.
	// Vì Run() dùng os.Stdout cứng, ta dùng cách khác: refactor nhẹ bằng cách
	// truyền buffer vào writeTaskfile qua một test-only entry point.
	err := runWithOutput(full, &buf)
	return buf.String(), err
}

// runWithOutput là wrapper test: parse flag giống Run() rồi gọi writeTaskfile
// với stdout có thể điều khiển được.
func runWithOutput(args []string, stdout *bytes.Buffer) error {
	// Phục hồi từ CLI args: --target DIR [--dry-run] [--force]
	target, dryRun, force, err := parseSetupArgs(args)
	if err != nil {
		return err
	}
	return writeTaskfile(target, defaultScripts, dryRun, force, stdout)
}

// parseSetupArgs tách các flag của setup ra khỏi args, không phụ thuộc flag
// package để test dễ hơn.
func parseSetupArgs(args []string) (target string, dryRun, force bool, err error) {
	target, _ = os.Getwd()
	dryRun = false
	force = false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--target":
			if i+1 >= len(args) {
				return "", false, false, errMissingValue("--target")
			}
			target = args[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "--force":
			force = true
		default:
			return "", false, false, errUnknownFlag(args[i])
		}
	}
	return target, dryRun, force, nil
}

type flagError struct{ msg string }

func (e *flagError) Error() string { return e.msg }

func errMissingValue(flag string) error  { return &flagError{"missing value for " + flag} }
func errUnknownFlag(flag string) error   { return &flagError{"unknown flag: " + flag} }

func TestRunWritesNewTaskfile(t *testing.T) {
	dir := t.TempDir()
	out, err := runSetupFromDir(t, dir)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// stdout phải có thông báo ghi file.
	if !strings.Contains(out, "wrote") || !strings.Contains(out, "Taskfile.yml") {
		t.Errorf("stdout should mention writing Taskfile.yml, got: %q", out)
	}

	// File phải tồn tại.
	path := filepath.Join(dir, "Taskfile.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Taskfile.yml was not created: %v", err)
	}

	// Phải chứa tất cả 20 task mặc định.
	body := string(data)
	for _, name := range ScriptNames() {
		if !strings.Contains(body, name+":") {
			t.Errorf("Taskfile.yml is missing task %q", name)
		}
	}
}

func TestRunGeneratesValidYAML(t *testing.T) {
	dir := t.TempDir()
	if _, err := runSetupFromDir(t, dir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Taskfile.yml is not valid YAML: %v\n---\n%s", err, data)
	}

	// Kiểm tra version được set.
	if v, ok := parsed["version"].(string); !ok || v == "" {
		t.Errorf("expected version to be a non-empty string, got %v", parsed["version"])
	}

	// Kiểm tra tasks là map và có đủ số lượng.
	tasks, ok := parsed["tasks"].(map[string]any)
	if !ok {
		t.Fatalf("expected tasks to be a map, got %T", parsed["tasks"])
	}
	if got, want := len(tasks), len(defaultScripts); got != want {
		t.Errorf("expected %d tasks, got %d", want, got)
	}

	// Kiểm tra cấu trúc của một task cụ thể.
	nsStatus, ok := tasks["ns:status"].(map[string]any)
	if !ok {
		t.Fatalf("ns:status task should be a map, got %T", tasks["ns:status"])
	}
	desc, _ := nsStatus["desc"].(string)
	if desc == "" {
		t.Errorf("ns:status should have a desc, got %q", desc)
	}
	cmds, ok := nsStatus["cmds"].([]any)
	if !ok || len(cmds) == 0 {
		t.Fatalf("ns:status should have non-empty cmds, got %v", nsStatus["cmds"])
	}
	cmd0, _ := cmds[0].(string)
	if !strings.Contains(cmd0, "ns-workspace@latest") || !strings.Contains(cmd0, "status") {
		t.Errorf("ns:status cmd should invoke ns-workspace status, got %q", cmd0)
	}
}

func TestRunMergesAndRewritesExistingTasks(t *testing.T) {
	dir := t.TempDir()
	// File có sẵn: 1 custom task (giữ lại) + 1 task trùng tên (sẽ bị rewrite).
	existing := `version: "3"
vars:
  CUSTOM_VAR: hello
tasks:
  ns:status:
    desc: USER OVERRIDE - this should be replaced
    cmds:
      - echo user-custom-command
  my:custom:
    desc: A user-defined task
    cmds:
      - echo custom
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runSetupFromDir(t, dir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("re-parsing failed: %v", err)
	}

	tasks := parsed["tasks"].(map[string]any)

	// 1. Custom task phải còn nguyên.
	custom, ok := tasks["my:custom"].(map[string]any)
	if !ok {
		t.Fatalf("user-defined task my:custom was lost: %v", tasks)
	}
	if desc, _ := custom["desc"].(string); !strings.Contains(desc, "user-defined") {
		t.Errorf("my:custom desc was modified: %q", desc)
	}

	// 2. ns:status phải bị rewrite (không còn "USER OVERRIDE").
	nsStatus, ok := tasks["ns:status"].(map[string]any)
	if !ok {
		t.Fatalf("ns:status missing: %v", tasks)
	}
	if desc, _ := nsStatus["desc"].(string); strings.Contains(desc, "USER OVERRIDE") {
		t.Errorf("ns:status should have been rewritten, got desc=%q", desc)
	}
	if desc, _ := nsStatus["desc"].(string); !strings.Contains(desc, "ns-workspace") {
		t.Errorf("ns:status desc should be the preset description, got %q", desc)
	}

	// 3. Top-level var CUSTOM_VAR phải được giữ nguyên.
	vars, ok := parsed["vars"].(map[string]any)
	if !ok {
		t.Fatalf("vars was lost: %v", parsed)
	}
	if cv, _ := vars["CUSTOM_VAR"].(string); cv != "hello" {
		t.Errorf("CUSTOM_VAR was modified: %q", cv)
	}

	// 4. Tất cả default scripts phải có mặt.
	for _, name := range ScriptNames() {
		if _, ok := tasks[name]; !ok {
			t.Errorf("missing task %q after merge", name)
		}
	}
}

func TestRunForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	// File có sẵn với custom task mà --force sẽ xoá.
	existing := `version: "3"
tasks:
  my:custom:
    desc: User task that should disappear
    cmds:
      - echo bye
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runSetupFromDir(t, dir, "--force"); err != nil {
		t.Fatalf("setup --force failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(body, "my:custom") {
		t.Errorf("--force should have removed custom task, body:\n%s", body)
	}

	// Vẫn phải có toàn bộ default scripts.
	for _, name := range ScriptNames() {
		if !strings.Contains(body, name+":") {
			t.Errorf("missing default task %q after --force", name)
		}
	}
}

func TestRunDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	out, err := runSetupFromDir(t, dir, "--dry-run")
	if err != nil {
		t.Fatalf("setup --dry-run failed: %v", err)
	}

	// File không được tạo.
	path := filepath.Join(dir, "Taskfile.yml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write file, but %s exists (err=%v)", path, err)
	}

	// Output phải chứa YAML hợp lệ.
	if !strings.Contains(out, "tasks:") {
		t.Errorf("dry-run output should contain YAML body, got: %q", out)
	}
	if !strings.Contains(out, "ns:status:") {
		t.Errorf("dry-run output should list default tasks, got: %q", out)
	}
}

func TestRunRespectsTargetFlag(t *testing.T) {
	parent := t.TempDir()
	subdir := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := runSetupFromDir(t, parent, "--target", subdir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// File phải ở subdir, không ở parent.
	if _, err := os.Stat(filepath.Join(subdir, "Taskfile.yml")); err != nil {
		t.Fatalf("Taskfile.yml not written to --target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "Taskfile.yml")); !os.IsNotExist(err) {
		t.Errorf("Taskfile.yml should NOT be at parent level")
	}
}

func TestRunContainsExpectedTasks(t *testing.T) {
	dir := t.TempDir()
	if _, err := runSetupFromDir(t, dir); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)

	expectedTasks := []string{
		"ns:status", "ns:doctor", "ns:init", "ns:update",
		"ns:preview", "ns:search", "ns:export",
		"ns:kb:validate", "ns:kb:index", "ns:lsp:list",
		"lint:docs", "lint:docs:fix", "lint:preview", "lint:preview:fix",
		"format:docs", "format:docs:check", "format:preview", "format:preview:check",
		"build:preview",
		"go:build", "go:test",
	}
	for _, name := range expectedTasks {
		if !strings.Contains(body, name+":") {
			t.Errorf("missing expected task %q in Taskfile.yml", name)
		}
	}

	// Đếm tổng số task phải đúng.
	if got, want := len(ScriptNames()), 21; got != want {
		t.Errorf("DefaultScripts length = %d, want %d", got, want)
	}
}

func TestScriptNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range defaultScripts {
		if seen[s.Name] {
			t.Errorf("duplicate task name: %q", s.Name)
		}
		seen[s.Name] = true
		if s.Name == "" {
			t.Error("script has empty Name")
		}
		if s.Description == "" {
			t.Errorf("script %q has empty Description", s.Name)
		}
		if len(s.Commands) == 0 {
			t.Errorf("script %q has no Commands", s.Name)
		}
	}
}

func TestScriptNamesAreSorted(t *testing.T) {
	// ScriptNames() phải trả về alphabet cho documentation.
	got := ScriptNames()
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("ScriptNames() not sorted at index %d: %q >= %q", i, got[i-1], got[i])
		}
	}
}

func TestRunHandlesMissingValueFlag(t *testing.T) {
	dir := t.TempDir()
	_, err := runSetupFromDir(t, dir, "--target")
	if err == nil {
		t.Fatal("expected error for --target without value")
	}
	if !strings.Contains(err.Error(), "missing value") {
		t.Errorf("unexpected error: %v", err)
	}
}
