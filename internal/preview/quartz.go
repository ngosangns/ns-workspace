package preview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const quartzGitURL = "https://github.com/jackyzha0/quartz.git"

// quartzCacheDir returns the base cache directory for Quartz artifacts.
func quartzCacheDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "ns-workspace", "quartz"), nil
}

// quartzRepoDir returns the directory where the Quartz repository is cached.
func quartzRepoDir() (string, error) {
	dir, err := quartzCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repo"), nil
}

// quartzWorkspaceDir returns a per-project Quartz workspace directory.
func quartzWorkspaceDir(projectRoot string) (string, error) {
	dir, err := quartzCacheDir()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(projectRoot))
	return filepath.Join(dir, "workspaces", hex.EncodeToString(hash[:8])), nil
}

// ensureQuartzRepo makes sure the Quartz repository is cloned and its npm
// dependencies are installed in the user cache. It returns the repository root.
func ensureQuartzRepo() (string, error) {
	dir, err := quartzRepoDir()
	if err != nil {
		return "", err
	}
	if quartzRepoExists(dir) {
		return dir, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	if err := runCommand("git", []string{"clone", "--depth", "1", quartzGitURL, dir}, "", nil); err != nil {
		return "", fmt.Errorf("clone quartz repository: %w", err)
	}
	if err := runCommand("npm", []string{"install"}, dir, nil); err != nil {
		return "", fmt.Errorf("install quartz dependencies: %w", err)
	}
	// Install plugins referenced by the default config so the build can resolve
	// components like Head.tsx that import from .quartz/plugins.
	if err := runCommand("npx", []string{"quartz", "plugin", "install", "--from-config"}, dir, nil); err != nil {
		return "", fmt.Errorf("install quartz plugins: %w", err)
	}
	return dir, nil
}

func quartzRepoExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "package.json"))
	return err == nil && !info.IsDir()
}

// prepareQuartzWorkspace creates a clean Quartz workspace for the project and
// links the docs directory into it as the content folder. The returned cleanup
// function removes the workspace; the caller should defer it.
func prepareQuartzWorkspace(projectRoot, docsDir string) (string, func(), error) {
	workspace, err := quartzWorkspaceDir(projectRoot)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {}

	if err := os.RemoveAll(workspace); err != nil {
		return "", cleanup, err
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return "", cleanup, err
	}

	docsAbs := docsRoot(projectRoot, docsDir)
	contentDir := filepath.Join(workspace, "content")
	if err := linkOrCopyDir(docsAbs, contentDir); err != nil {
		return "", cleanup, fmt.Errorf("link docs into quartz workspace: %w", err)
	}

	cleanup = func() { _ = os.RemoveAll(workspace) }
	return workspace, cleanup, nil
}

// runQuartzServe runs the Quartz dev server for the given workspace. It blocks
// until the server process exits. wsPort is used for the WebSocket hot-reload
// channel; passing an empty string leaves Quartz's default.
func runQuartzServe(repoDir, workspaceDir, port, wsPort string, stdout, stderr io.Writer) error {
	args := []string{"quartz", "build", "--serve", "--directory", workspaceDir, "--port", port}
	if wsPort != "" {
		args = append(args, "--wsPort", wsPort)
	}
	cmd := exec.Command("npx", args...)
	cmd.Dir = repoDir
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}
	return cmd.Run()
}

// linkOrCopyDir tries to symlink src to dst; if symlinks are not supported it
// falls back to a recursive copy. This keeps the workspace fast on Unix and
// portable on Windows.
func linkOrCopyDir(src, dst string) error {
	if runtime.GOOS != "windows" {
		if err := os.Symlink(src, dst); err == nil {
			return nil
		}
	}
	return copyDir(src, dst)
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// resolveQuartzRepo returns the Quartz repository root to use. If dir is
// non-empty, it must contain a Quartz package.json; otherwise the cached clone
// from ensureQuartzRepo is returned.
func resolveQuartzRepo(dir string) (string, error) {
	if dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", fmt.Errorf("resolve quartz directory: %w", err)
		}
		if !quartzRepoExists(abs) {
			return "", fmt.Errorf("quartz directory %q does not contain package.json", abs)
		}
		return abs, nil
	}
	return ensureQuartzRepoForTest()
}

// ensureQuartzRepoForTest lets tests stub the cached clone path.
var ensureQuartzRepoForTest = ensureQuartzRepo

// resolveQuartzRepoForTest lets tests stub Quartz repo resolution.
var resolveQuartzRepoForTest = resolveQuartzRepo

func runCommand(name string, args []string, dir string, env []string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
