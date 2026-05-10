package media

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const coverDirName = ".covers"

func (l *Library) CoverForID(id string) (string, error) {
	path, _, err := l.PathForID(id)
	if err != nil {
		return "", err
	}

	l.mu.RLock()
	item, ok := l.itemsByID[id]
	l.mu.RUnlock()
	if !ok {
		return "", os.ErrNotExist
	}
	if item.Type != "audio" || !item.HasCover {
		return "", os.ErrNotExist
	}

	ffmpeg, err := ffmpegPath()
	if err != nil {
		return "", errors.New("ffmpeg is required to extract audio covers")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	cacheDir := filepath.Join(l.root, coverDirName)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	cachePath := filepath.Join(cacheDir, coverCacheName(id, info.Size(), info.ModTime().UnixNano()))
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	tmpFile, err := os.CreateTemp(cacheDir, ".cover-*.jpg")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", path,
		"-an",
		"-map", "0:v:0",
		"-frames:v", "1",
		"-vf", "scale=w='min(960,iw)':h=-2",
		"-q:v", "3",
		"-y", tmpPath,
	}
	if output, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("extract audio cover: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if info, err := os.Stat(tmpPath); err != nil || info.Size() == 0 {
		return "", os.ErrNotExist
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

func coverCacheName(id string, size int64, modTime int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", id, size, modTime)))
	return hex.EncodeToString(sum[:]) + ".jpg"
}
