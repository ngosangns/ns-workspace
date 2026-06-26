package graphquery

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

type ArchiveSource struct {
	Version  string
	FileName string
	URL      string
	SHA256   string
	Format   string
}

func installArchiveLSP(ctx context.Context, spec InstallSpec, source ArchiveSource, executableCandidates []string) (string, error) {
	cacheDir := filepath.Join(CacheRoot(), spec.ID)
	tmpParent := filepath.Join(cacheDir, "tmp")
	if err := os.MkdirAll(tmpParent, 0o755); err != nil {
		return "", err
	}
	tmpDir, err := mkdirTempFn(tmpParent, "install-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, source.FileName)
	if err := downloadArchive(ctx, source, archivePath); err != nil {
		return "", err
	}
	extractDir := filepath.Join(tmpDir, "extract")
	if err := mkdirAllFn(extractDir, 0o755); err != nil {
		return "", err
	}
	switch source.Format {
	case "zip":
		err = extractZipArchive(archivePath, extractDir)
	case "tar.gz":
		err = extractTarGzArchive(archivePath, extractDir)
	default:
		err = fmt.Errorf("unsupported archive format %q", source.Format)
	}
	if err != nil {
		return "", err
	}

	launcher, err := findArchiveExecutable(extractDir, executableCandidates)
	if err != nil {
		return "", err
	}
	launcherRel, err := relFn(extractDir, launcher)
	if err != nil {
		return "", err
	}

	versionsDir := filepath.Join(cacheDir, "versions")
	installDir := filepath.Join(versionsDir, source.Version)
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		return "", err
	}
	if err := removeAllFn(installDir); err != nil {
		return "", err
	}
	if err := renameFn(extractDir, installDir); err != nil {
		return "", err
	}

	launcherPath := filepath.Join(installDir, launcherRel)
	if err := chmodFn(launcherPath, 0o755); err != nil {
		return "", err
	}
	binDir := filepath.Join(cacheDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	// Keep resolver paths stable when upstream archives change launcher names.
	wrapperPath := filepath.Join(binDir, internalutil.ExecutableNames(spec.Command)[0])
	if err := writeExecutableWrapper(wrapperPath, launcherPath); err != nil {
		return "", err
	}
	return wrapperPath, nil
}

func downloadArchive(ctx context.Context, source ArchiveSource, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hash), resp.Body)
	closeErr := wrapForClose(out).Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(source.SHA256)) {
		return fmt.Errorf("download checksum mismatch for %s: got %s, want %s", source.FileName, got, source.SHA256)
	}
	return nil
}

// closableFile is a thin wrapper around *os.File that lets tests inject close
// errors via the closeFn variable. In production closeFn is nil and Close
// delegates to the underlying file.
type closableFile struct {
	*os.File
}

func (c closableFile) Close() error {
	if closeFn != nil {
		return closeFn()
	}
	return c.File.Close()
}

// closeFn is a test seam to simulate os.File.Close() returning an error.
var closeFn func() error

// mkdirAllFn is a test seam for os.MkdirAll so tests can exercise the
// mkdirAll error branches in installArchiveLSP.
var mkdirAllFn = os.MkdirAll

// relFn is a test seam for filepath.Rel so tests can exercise the Rel error
// branch in installArchiveLSP (only reachable on Windows in practice).
var relFn = filepath.Rel

// renameFn is a test seam for os.Rename so tests can exercise the rename
// error branch in installArchiveLSP.
var renameFn = os.Rename

// chmodFn is a test seam for os.Chmod so tests can exercise the chmod error
// branch in installArchiveLSP.
var chmodFn = os.Chmod

// removeAllFn is a test seam for os.RemoveAll so tests can exercise the
// removeAll error branch in installArchiveLSP.
var removeAllFn = os.RemoveAll

// zipEntryOpenFn is a test seam so tests can exercise the entry.Open error
// branch in extractZipArchive.
var zipEntryOpenFn = func(entry *zip.File) (io.ReadCloser, error) { return entry.Open() }

// readAllFn is a test seam for io.ReadAll used in the symlink branch so tests
// can exercise the symlink-read error branch.
var readAllFn = io.ReadAll

// mkdirTempFn is a test seam for os.MkdirTemp so tests can exercise the
// MkdirTemp error branch in installArchiveLSP.
var mkdirTempFn = os.MkdirTemp

// copyFn is a test seam for io.Copy so tests can exercise the copy error
// branches in extractZipArchive and extractTarGzArchive.
var copyFn = io.Copy

// ioCopyWriter is a helper interface for tests that want to simulate io.Copy
// failures. The default copyFn honours it.
type ioCopyWriter interface {
	io.Writer
}

// File-open seam for tests so the file-close error branches can be exercised
// without resorting to platform-specific filesystem quirks.
var openArchiveFile = os.OpenFile

// createFile opens dest with the requested flags/perm, returning *os.File or
// an error. Tests can replace this variable to simulate I/O failures.
func createFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := openArchiveFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// wrapForClose wraps an *os.File so its Close method honours the closeFn seam.
func wrapForClose(f *os.File) io.Closer {
	return closableFile{File: f}
}

// wrapForCloser wraps any io.Closer so its Close method honours the closeFn seam.
func wrapForCloser(c io.Closer) io.Closer {
	return closerFunc(func() error {
		if closeFn != nil {
			return closeFn()
		}
		return c.Close()
	})
}

type closerFunc func() error

func (c closerFunc) Close() error { return c() }

func extractZipArchive(archivePath, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		target, err := safeArchiveTarget(dest, entry.Name)
		if err != nil {
			return err
		}
		mode := entry.Mode()
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := zipEntryOpenFn(entry)
		if err != nil {
			return err
		}
		if mode&os.ModeSymlink != 0 {
			linkTarget, readErr := readAllFn(rc)
			closeErr := wrapForCloser(rc).Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			if err := createSafeArchiveSymlink(dest, string(linkTarget), target); err != nil {
				return err
			}
			continue
		}
		perm := mode.Perm()
		if perm == 0 {
			perm = 0o644
		}
		out, err := createFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := copyFn(out, rc)
		closeOutErr := wrapForClose(out).Close()
		closeReadErr := wrapForCloser(rc).Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		if closeReadErr != nil {
			return closeReadErr
		}
	}
	return nil
}

func extractTarGzArchive(archivePath, dest string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeArchiveTarget(dest, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			perm := os.FileMode(header.Mode).Perm()
			if perm == 0 {
				perm = 0o644
			}
			out, err := createFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
			if err != nil {
				return err
			}
			_, copyErr := copyFn(out, tarReader)
			closeErr := wrapForClose(out).Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := createSafeArchiveSymlink(dest, header.Linkname, target); err != nil {
				return err
			}
		}
	}
}

func safeArchiveTarget(dest, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("archive entry uses absolute path: %s", name)
	}
	clean := filepath.Clean(name)
	if clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", fmt.Errorf("archive entry escapes target directory: %s", name)
	}
	target := filepath.Join(dest, clean)
	destClean := filepath.Clean(dest)
	if !strings.HasPrefix(filepath.Clean(target), destClean+string(os.PathSeparator)) && filepath.Clean(target) != destClean {
		return "", fmt.Errorf("archive entry escapes target directory: %s", name)
	}
	return target, nil
}

func createSafeArchiveSymlink(dest, linkTarget, target string) error {
	if filepath.IsAbs(linkTarget) {
		return fmt.Errorf("archive symlink uses absolute target: %s", linkTarget)
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(target), linkTarget))
	destClean := filepath.Clean(dest)
	if !strings.HasPrefix(resolved, destClean+string(os.PathSeparator)) && resolved != destClean {
		return fmt.Errorf("archive symlink escapes target directory: %s", linkTarget)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Symlink(linkTarget, target)
}

func findArchiveExecutable(root string, candidates []string) (string, error) {
	for _, candidate := range candidates {
		path := filepath.Join(root, candidate)
		if internalutil.ExecutableFile(path) {
			return path, nil
		}
	}
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || found != "" || entry.IsDir() {
			return err
		}
		name := entry.Name()
		for _, candidate := range candidates {
			if name == filepath.Base(candidate) && internalutil.ExecutableFile(path) {
				found = path
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("archive does not contain executable candidate %s", strings.Join(candidates, ", "))
	}
	return found, nil
}

func writeExecutableWrapper(wrapperPath, launcherPath string) error {
	if err := os.MkdirAll(filepath.Dir(wrapperPath), 0o755); err != nil {
		return err
	}
	content := "#!/bin/sh\nexec " + shellQuote(launcherPath) + " \"$@\"\n"
	if err := os.WriteFile(wrapperPath, []byte(content), 0o755); err != nil {
		return err
	}
	return os.Chmod(wrapperPath, 0o755)
}
