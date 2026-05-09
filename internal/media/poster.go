package media

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	posterDirName      = ".posters"
	posterMaxTime      = 24 * 60 * 60
	posterTimeInterval = 0.5
)

func (l *Library) PosterForID(id string, seconds float64) (string, error) {
	path, mimeType, err := l.PathForID(id)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(mimeType, "video/") {
		return "", errors.New("posters are only available for videos")
	}
	ffmpeg, err := ffmpegPath()
	if err != nil {
		return "", errors.New("ffmpeg is required to generate video posters")
	}

	seconds = normalizePosterTime(seconds)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	cacheDir := filepath.Join(l.root, posterDirName)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	cachePath := filepath.Join(cacheDir, posterCacheName(id, info.Size(), info.ModTime().UnixNano(), seconds))
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	tmpFile, err := os.CreateTemp(cacheDir, ".poster-*.jpg")
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
		"-ss", fmt.Sprintf("%.3f", seconds),
		"-i", path,
		"-frames:v", "1",
		"-vf", "scale=w='min(720,iw)':h=-2",
		"-q:v", "3",
		"-y", tmpPath,
	}
	if output, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("generate poster: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

func normalizePosterTime(seconds float64) float64 {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0
	}
	if seconds > posterMaxTime {
		seconds = posterMaxTime
	}
	return math.Round(seconds/posterTimeInterval) * posterTimeInterval
}

func posterCacheName(id string, size int64, modTime int64, seconds float64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%.1f", id, size, modTime, seconds)))
	return hex.EncodeToString(sum[:]) + ".jpg"
}
