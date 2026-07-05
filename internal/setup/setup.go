// Package setup implements the `setup` subcommand of ns-workspace. The
// command materializes a go-task Taskfile.yml listing every script/command
// exposed by ns-workspace (CLI subcommands, npm scripts from package.json and
// Go build/test targets) into the user's working directory so that `task --list`
// surfaces them in one place.
package setup

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// scriptSpec mô tả một task sẽ được sinh ra trong Taskfile.yml.
type scriptSpec struct {
	Name        string   // Tên task (vd "ns:status")
	Description string   // Mô tả hiển thị trong `task --list`
	Commands    []string // Mỗi phần tử là một dòng shell command
}

// nsCLI là tiền tố command để gọi ns-workspace ở chế độ remote (tương đương
// `go run github.com/ngosangns/ns-workspace@latest ...`). Khi user chạy Taskfile
// trong checkout local, họ có thể override biến `NS_WORKSPACE` thành `go run .`
// để dùng code local thay vì tải từ network.
const nsCLI = "go run github.com/ngosangns/ns-workspace@latest"

// defaultScripts là danh sách scripts/commands của ns-workspace sẽ được
// materialize vào Taskfile.yml. Danh sách được giữ hard-coded để đảm bảo
// output ổn định và không phụ thuộc vào filesystem của repo khi user chạy
// setup ở cwd bất kỳ.
//
// Quy ước đặt tên:
//   - ns:*       — subcommand của ns-workspace CLI
//   - lint:*     — npm scripts kiểm tra code style
//   - format:*   — npm scripts format code
//   - build:*    — npm scripts build artifact
//   - go:*       — Go toolchain commands
//
// Task trùng tên với danh sách này sẽ bị rewrite mỗi lần setup. User nên đặt
// tên task riêng (không dùng các prefix trên) để giữ task do mình tự định nghĩa.
var defaultScripts = []scriptSpec{
	// Nhóm ns:* — subcommand của ns-workspace CLI.
	{
		Name:        "ns:status",
		Description: "Hiển thị trạng thái cài đặt agents của ns-workspace",
		Commands:    []string{nsCLI + " status"},
	},
	{
		Name:        "ns:doctor",
		Description: "Validate JSON config và report local agent CLI",
		Commands:    []string{nsCLI + " doctor"},
	},
	{
		Name:        "ns:init",
		Description: "Cài cấu hình shared và link/copy sang adapter native",
		Commands:    []string{nsCLI + " init"},
	},
	{
		Name:        "ns:update",
		Description: "Rewrite các phần config do tool quản lý từ preset embedded",
		Commands:    []string{nsCLI + " update"},
	},
	{
		Name:        "ns:preview",
		Description: "Chạy Quartz dev server cho docs/ của project hiện tại",
		Commands:    []string{nsCLI + " preview --project . --open"},
	},
	{
		Name:        "ns:portal",
		Description: "Chạy portal server quản lý preset/skills/MCP/registry",
		Commands:    []string{nsCLI + " portal --open"},
	},
	{
		Name:        "ns:search",
		Description: "Mở Search/Code Graph standalone cho docs/ của project hiện tại",
		Commands:    []string{nsCLI + " search --project ."},
	},
	{
		Name:        "ns:export",
		Description: "Export docs + graph thành file HTML tĩnh self-contained",
		Commands:    []string{nsCLI + " export --project . --out ./kb.html --open"},
	},
	{
		Name:        "ns:kb:validate",
		Description: "Kiểm tra OKF conformance của docs (frontmatter + type)",
		Commands:    []string{nsCLI + " kb validate --project ."},
	},
	{
		Name:        "ns:kb:index",
		Description: "Sinh lại index.md progressive-disclosure cho từng thư mục docs",
		Commands:    []string{nsCLI + " kb index --project ."},
	},
	{
		Name:        "ns:lsp:list",
		Description: "Liệt kê language server đã cài và trạng thái cho từng ngôn ngữ",
		Commands:    []string{nsCLI + " lsp list"},
	},
	{
		Name:        "ns:lsp:install",
		Description: "Cài language server (vd: task ns:lsp:install -- auto, task ns:lsp:install -- kotlin)",
		Commands:    []string{nsCLI + " lsp install"},
	},
	{
		Name:        "ns:graph",
		Description: "Chạy query Search/Code Graph terminal (mặc định không truyền --query; dùng `task ns:graph -- <args>` để truyền flag)",
		Commands:    []string{nsCLI + " graph --project ."},
	},
	{
		Name:        "ns:mcp",
		Description: "Chạy MCP command trên docs/ hiện tại (vd: task ns:mcp -- list-docs --type module)",
		Commands:    []string{nsCLI + " mcp --project . {{.CLI_ARGS}}"},
	},
	{
		Name:        "ns:harness:list",
		Description: "Liệt kê harness task có sẵn trong .harness/tasks/",
		Commands:    []string{nsCLI + " harness list"},
	},
	{
		Name:        "ns:harness:run",
		Description: "Chạy harness task theo --task ID (mặc định không truyền --task; dùng `task ns:harness:run -- --task <id>`)",
		Commands:    []string{nsCLI + " harness run --project ."},
	},

	// Nhóm lint:* — kiểm tra code style theo module.
	{
		Name:        "lint:portal",
		Description: "Lint TypeScript + Vue trong portal UI",
		Commands:    []string{"npm run lint:portal"},
	},
	{
		Name:        "lint:portal:fix",
		Description: "Auto-fix TypeScript + Vue lint issues",
		Commands:    []string{"npm run lint:portal:fix"},
	},
	{
		Name:        "lint:preview",
		Description: "Lint markdown + html trong docs/ (nội dung preview)",
		Commands:    []string{"npm run lint:preview"},
	},
	{
		Name:        "lint:preview:fix",
		Description: "Auto-fix markdown + html trong docs/ preview",
		Commands:    []string{"npm run lint:preview:fix"},
	},
	{
		Name:        "lint:doc",
		Description: "Lint markdown + html toàn bộ docs (README, docs, presets)",
		Commands:    []string{"npm run lint:doc"},
	},
	{
		Name:        "lint:doc:fix",
		Description: "Auto-fix markdown + html toàn bộ docs",
		Commands:    []string{"npm run lint:doc:fix"},
	},

	// Nhóm go:* — Go toolchain commands.
	{
		Name:        "go:build",
		Description: "Build toàn bộ Go packages trong repo",
		Commands:    []string{"go build ./..."},
	},
	{
		Name:        "go:test",
		Description: "Chạy Go test cho toàn bộ packages",
		Commands:    []string{"go test ./..."},
	},
}

// taskfileHeader là comment đầu file Taskfile.yml để user biết file này do
// tool sinh ra và cách regenerate. Lưu ý: raw string literal (backtick) của Go
// không cho phép ký tự backtick bên trong, nên chuỗi gốc dùng dấu nháy đơn
// cho lệnh trong comment.
const taskfileHeader = "# Generated by ns-workspace setup. Re-run 'go run github.com/ngosangns/ns-workspace@latest setup' to refresh.\n" +
	"# Custom tasks should use names that don't collide with the reserved prefixes\n" +
	"# (ns:, lint:, go:) to survive regeneration.\n" +
	"#\n" +
	"# Run 'task --list' to see all available tasks.\n"

// Run implements the `setup` subcommand. It parses flags, resolves the target
// directory, then either prints the planned Taskfile.yml (--dry-run) or merges
// the default scripts into the existing Taskfile.yml at target/Taskfile.yml.
//
// Flags:
//
//	--target PATH   directory to write Taskfile.yml (default: current directory)
//	--dry-run       show the planned YAML on stdout instead of writing
//	--force         replace the existing Taskfile.yml entirely instead of merging
func Run(args []string) error {
	return runWithWriter(args, os.Stdout)
}

// getwdFunc là seam test: cho phép thay thế os.Getwd để mô phỏng error path.
var getwdFunc = os.Getwd

// runWithWriter là entry point testable: cho phép truyền stdout writer để
// capture output mà không cần gọi os.Stdout trực tiếp.
func runWithWriter(args []string, stdout io.Writer) error {
	cwd, err := getwdFunc()
	if err != nil {
		return fmt.Errorf("setup: cannot determine working directory: %w", err)
	}
	target := cwd
	dryRun := false
	force := false

	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	// Mặc định flag tự in help/parse errors ra fs.Output() (stderr). Ta ghi vào
	// Discard để không spam stderr khi parse lỗi. fs.Usage riêng sẽ ghi vào stdout
	// để user thấy được khi gọi --help/-h.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(stdout, "Usage: setup [--target DIR] [--dry-run] [--force]")
		fs.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(stdout, "  -%s\n    \t%s\n", f.Name, f.Usage)
		})
	}
	fs.StringVar(&target, "target", target, "directory to write Taskfile.yml (default current directory)")
	fs.BoolVar(&dryRun, "dry-run", false, "print planned Taskfile.yml on stdout instead of writing")
	fs.BoolVar(&force, "force", false, "replace existing Taskfile.yml instead of merging")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		return fmt.Errorf("setup: %w", err)
	}

	return writeTaskfile(target, defaultScripts, dryRun, force, stdout)
}

// writeTaskfile thực hiện logic chính: đọc (nếu có) Taskfile.yml hiện tại, merge
// defaultScripts vào, rồi ghi lại hoặc in ra stdout tuỳ chế độ.
//
// Quy tắc merge:
//   - Nếu file chưa tồn tại → tạo mới với toàn bộ default scripts.
//   - Nếu file đã tồn tại và --force → bỏ qua nội dung cũ, tạo mới hoàn toàn.
//   - Nếu file đã tồn tại và không --force → parse YAML hiện tại; với mỗi
//     script trong defaultScripts, task trùng tên sẽ bị rewrite, task khác
//     được giữ nguyên.
//   - Top-level key khác ngoài `version`/`tasks` được giữ nguyên.
func writeTaskfile(target string, scripts []scriptSpec, dryRun, force bool, stdout io.Writer) error {
	return writeTaskfileWithOps(target, scripts, dryRun, force, stdout, defaultOps)
}

// writeTaskfileWithOps giống writeTaskfile nhưng nhận systemOps để test có thể
// inject hàm OS-level (vd: simulate Abs/MkdirAll/WriteFile fail).
func writeTaskfileWithOps(target string, scripts []scriptSpec, dryRun, force bool, stdout io.Writer, ops systemOps) error {
	absTarget, err := ops.abs(target)
	if err != nil {
		return fmt.Errorf("setup: cannot resolve target %q: %w", target, err)
	}
	taskfilePath := filepath.Join(absTarget, "Taskfile.yml")

	// 1. Đọc file hiện tại (nếu có và không --force).
	var existing map[string]any
	if !force {
		existing, err = readExistingTaskfile(taskfilePath)
		if err != nil {
			return err
		}
	}
	if existing == nil {
		existing = map[string]any{}
	}

	// 2. Đảm bảo version được set.
	if _, ok := existing["version"]; !ok {
		existing["version"] = "3"
	}

	// 3. Lấy/ khởi tạo tasks map.
	tasksAny, ok := existing["tasks"]
	if !ok {
		tasksAny = map[string]any{}
	}
	tasks, ok := tasksAny.(map[string]any)
	if !ok {
		// Trường hợp `tasks` tồn tại nhưng không phải map (ví dụ null) → reset.
		tasks = map[string]any{}
	}

	// 4. Merge từng script. Task trùng tên bị rewrite luôn.
	for _, s := range scripts {
		taskBody := map[string]any{
			"desc": s.Description,
			"cmds": toStringSlice(s.Commands),
		}
		tasks[s.Name] = taskBody
	}
	existing["tasks"] = tasks

	// 5. Marshal YAML.
	out, err := ops.yamlMarshal(existing)
	if err != nil {
		return fmt.Errorf("setup: cannot marshal Taskfile.yml: %w", err)
	}

	// 6. Output: stdout (dry-run) hoặc ghi file.
	if dryRun {
		_, _ = fmt.Fprint(stdout, taskfileHeader)
		_, _ = fmt.Fprint(stdout, string(out))
		return nil
	}

	if err := ops.mkdirAll(absTarget, 0o755); err != nil {
		return fmt.Errorf("setup: cannot create target directory %q: %w", absTarget, err)
	}
	body := taskfileHeader + string(out)
	if err := ops.writeFile(taskfilePath, []byte(body), 0o644); err != nil {
		return fmt.Errorf("setup: cannot write %q: %w", taskfilePath, err)
	}
	fmt.Fprintf(stdout, "setup: wrote %s (%d tasks)\n", taskfilePath, len(scripts))
	return nil
}

// readExistingTaskfile đọc Taskfile.yml hiện tại thành map. Trả về map rỗng
// (không phải nil) nếu file chưa tồn tại.
func readExistingTaskfile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("setup: cannot read existing Taskfile.yml at %q: %w", path, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("setup: cannot parse existing Taskfile.yml at %q: %w", path, err)
	}
	if parsed == nil {
		return map[string]any{}, nil
	}
	return parsed, nil
}

// toStringSlice chuyển []string thành []any để yaml.v3 marshal đúng kiểu chuỗi
// cho `cmds`. Nếu truyền trực tiếp []string vào map[string]any thì yaml.v3 có
// thể render dưới dạng flow style; dùng []any giúp render block style ổn định.
func toStringSlice(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

// DefaultScripts trả về bản copy của danh sách scripts hard-coded (copy cả
// slice lẫn Commands bên trong để caller sửa không ảnh hưởng list gốc). Export
// để test và để command khác (nếu có) có thể tham chiếu.
func DefaultScripts() []scriptSpec {
	out := make([]scriptSpec, len(defaultScripts))
	for i, s := range defaultScripts {
		out[i] = scriptSpec{
			Name:        s.Name,
			Description: s.Description,
			Commands:    append([]string(nil), s.Commands...),
		}
	}
	return out
}

// ScriptNames trả về tên của tất cả scripts theo thứ tự alphabet (dùng cho
// test và cho documentation).
func ScriptNames() []string {
	names := make([]string, 0, len(defaultScripts))
	for _, s := range defaultScripts {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	return names
}

// systemOps là các hàm OS-level mà writeTaskfile dùng. Tách ra biến package-level
// để test có thể inject mock (vd: simulate MkdirAll/WriteFile/Abs fail) mà
// không cần refactor public API.
type systemOps struct {
	abs            func(string) (string, error)
	mkdirAll       func(string, os.FileMode) error
	writeFile      func(string, []byte, os.FileMode) error
	yamlMarshal    func(any) ([]byte, error)
}

var defaultOps = systemOps{
	abs:         filepath.Abs,
	mkdirAll:    os.MkdirAll,
	writeFile:   os.WriteFile,
	yamlMarshal: yaml.Marshal,
}
