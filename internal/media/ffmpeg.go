package media

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func ffmpegPath() (string, error) {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	for _, base := range ffmpegSearchRoots() {
		path := filepath.Join(base, "tools", "ffmpeg", runtime.GOOS+"-"+runtime.GOARCH, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return exec.LookPath(name)
}

func ffmpegSearchRoots() []string {
	roots := make([]string, 0, 2)
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		if len(roots) == 0 || roots[0] != dir {
			roots = append(roots, dir)
		}
	}
	return roots
}
