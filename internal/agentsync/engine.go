package agentsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// writeFileManaged writes data to path, honoring dry-run, replace
// semantics, and backup-before-overwrite. Reports the chosen action on
// ctx.Report so users can see what would happen.
func writeFileManaged(ctx Context, path string, data []byte, replace bool) error {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) == string(data) {
			ctx.Report.Line("ok: %s", path)
			return nil
		}
		if !replace {
			ctx.Report.Line("skip existing: %s", path)
			return nil
		}
		if err := backupPath(ctx, path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensureDir(ctx, filepath.Dir(path)); err != nil {
		return err
	}
	ctx.Report.Line("write: %s", path)
	if ctx.DryRun {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

// linkOrCopy creates dst pointing at src (symlink by default, or a
// recursive copy when ctx.CopyMode is true or on Windows). It honors
// replace (back up + remove the existing dst first) and dry-run.
func linkOrCopy(ctx Context, src, dst string, replace bool) error {
	if _, err := os.Lstat(dst); err == nil {
		if sameLink(dst, src) {
			ctx.Report.Line("ok: %s -> %s", dst, src)
			return nil
		}
		if !replace {
			ctx.Report.Line("skip existing: %s", dst)
			return nil
		}
		if err := backupAndRemove(ctx, dst); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if ctx.DryRun {
		if ctx.CopyMode || runtime.GOOS == "windows" {
			ctx.Report.Line("copy: %s -> %s", src, dst)
			return nil
		}
		ctx.Report.Line("link: %s -> %s", dst, src)
		return nil
	}
	if ctx.CopyMode || runtime.GOOS == "windows" {
		return copyAny(ctx, src, dst)
	}
	ctx.Report.Line("link: %s -> %s", dst, src)
	return os.Symlink(src, dst)
}

// copyAny copies src to dst, dispatching to copyDir for directories and
// writeFileManaged for regular files. Used by linkOrCopy when copy mode
// is requested.
func copyAny(ctx Context, src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(ctx, src, dst)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFileManaged(ctx, dst, data, true)
}

// copyDir walks src recursively and writes each file under dst via
// writeFileManaged, preserving the directory layout.
func copyDir(ctx Context, src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return ensureDir(ctx, target)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFileManaged(ctx, target, data, true)
	})
}

// backupAndRemove renames path aside as a timestamped backup, then
// removes the original (directory or file). Used by linkOrCopy when
// replacing an existing dst.
func backupAndRemove(ctx Context, path string) error {
	if err := backupPath(ctx, path); err != nil {
		return err
	}
	ctx.Report.Line("remove: %s", path)
	if ctx.DryRun {
		return nil
	}
	return os.RemoveAll(path)
}

// backupPath renames path aside with a timestamped suffix. If the
// suffix already exists (collision from a previous backup), an
// incrementing counter is appended until a free name is found.
func backupPath(ctx Context, path string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	backup := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
	if !ctx.DryRun {
		backup = uniqueBackupPath(backup)
	}
	ctx.Report.Line("backup: %s -> %s", path, backup)
	if ctx.DryRun {
		return nil
	}
	return os.Rename(path, backup)
}

// uniqueBackupPath returns path with a numeric suffix appended when
// the bare path already exists. Used to avoid clobbering prior backups.
func uniqueBackupPath(path string) string {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

// ensureDir creates path (and parents) with mode 0o755. The call is
// deduplicated per Context so duplicate operations don't print "mkdir"
// lines for the same path twice.
func ensureDir(ctx Context, path string) error {
	if path == "" || path == "." {
		return nil
	}
	if ctx.seenDirs[path] {
		return nil
	}
	ctx.seenDirs[path] = true
	ctx.Report.Line("mkdir: %s", path)
	if ctx.DryRun {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

// printPathStatus emits a one-line summary of path for the `status` and
// `doctor` commands. It distinguishes missing/file/dir/symlink.
func printPathStatus(ctx Context, path string) {
	if path == "" {
		return
	}
	info, err := os.Lstat(path)
	if err != nil {
		ctx.Report.Line("missing: %s", path)
		return
	}
	kind := "file"
	if info.IsDir() {
		kind = "dir"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(path)
		ctx.Report.Line("link: %s -> %s", path, target)
		return
	}
	ctx.Report.Line("ok %-4s %s", kind, path)
}

// checkJSON reads path (if it exists) and reports whether it parses as
// JSON. Used by `doctor` to surface malformed native configs before
// they get overwritten by an update.
func checkJSON(ctx Context, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		ctx.Report.Line("invalid json: %s: %v", path, err)
		return
	}
	ctx.Report.Line("valid json: %s", path)
}

// embeddedRootFor maps a target dir name to the preset root that
// supplies its content. "agents" comes from presets/subagents; anything
// else comes from presets/skills.
func embeddedRootFor(sourceRoot string) string {
	switch filepath.Base(sourceRoot) {
	case "agents":
		return "presets/subagents"
	default:
		return "presets/skills"
	}
}

// embeddedEntryNames returns the sorted child names directly inside
// the embedded FS root. Used by LinkSkillDirs in dry-run mode when the
// source directory does not yet exist on disk.
func embeddedEntryNames(presets fs.FS, root string) ([]string, error) {
	entries, err := fs.ReadDir(presets, root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// artifactList renders a slice of ArtifactKind as a sorted
// comma-separated string. Used by `doctor` and `catalog` to show which
// artifact kinds each adapter produces.
func artifactList(values []ArtifactKind) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, string(value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
