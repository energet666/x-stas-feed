package media

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const coverDirName = ".covers"

func (l *Library) CoverForID(id string) (string, error) {
	started := time.Now()
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
	filename := item.Filename
	if filename == "" {
		filename = filepath.Base(path)
	}
	if item.Type != "audio" || !item.HasCover {
		return "", os.ErrNotExist
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if item.CoverFile != "" && filepath.Base(item.CoverFile) == item.CoverFile {
		coverPath := filepath.Join(l.root, coverDirName, item.CoverFile)
		if cacheFresh(coverPath, info.ModTime()) {
			l.logf(
				"audio cover ready mediaID=%s filename=%q source=cache path=%q duration=%s",
				id,
				filename,
				item.CoverFile,
				time.Since(started).Round(time.Millisecond),
			)
			return coverPath, nil
		}
	}

	coverFile, err := l.extractAudioCover(id, path, item.Filename, info.ModTime())
	if err != nil {
		return "", err
	}
	return filepath.Join(l.root, coverDirName, coverFile), nil
}

func (l *Library) extractAudioCover(id, path string, filename string, sourceModTime time.Time) (string, error) {
	started := time.Now()
	ffmpeg, err := ffmpegPath()
	if err != nil {
		return "", errors.New("ffmpeg is required to extract audio covers")
	}
	if filename == "" {
		filename = filepath.Base(path)
	}

	cacheDir := filepath.Join(l.root, coverDirName)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	cacheFile := coverCacheName(filename)
	cachePath := filepath.Join(cacheDir, cacheFile)
	if cacheFresh(cachePath, sourceModTime) {
		l.logf(
			"audio cover ready mediaID=%s filename=%q source=cache path=%q duration=%s",
			id,
			filename,
			cacheFile,
			time.Since(started).Round(time.Millisecond),
		)
		return cacheFile, nil
	} else if _, err := os.Stat(cachePath); err != nil && !errors.Is(err, os.ErrNotExist) {
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

	l.logf(
		"audio cover ready mediaID=%s filename=%q source=generated path=%q duration=%s",
		id,
		filename,
		cacheFile,
		time.Since(started).Round(time.Millisecond),
	)
	return cacheFile, nil
}

func coverCacheName(filename string) string {
	return filename + ".jpg"
}
