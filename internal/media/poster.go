package media

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	posterDirName      = ".posters"
	posterMaxTime      = 24 * 60 * 60
	posterTimeInterval = 0.5
	posterScanSeconds  = 10
	posterScanStep     = 0.5
	posterScanFPS      = 2
	posterScanSize     = 64
	posterSmartVersion = "smart-v1"
)

func (l *Library) PosterForID(id string, seconds float64) (string, error) {
	started := time.Now()
	path, mimeType, err := l.PathForID(id)
	if err != nil {
		return "", err
	}
	item := l.itemSnapshot(id)
	filename := item.Filename
	if filename == "" {
		filename = filepath.Base(path)
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

	if seconds == 0 {
		if bestSeconds, cached, err := initialPosterTime(ffmpeg, path, filename, info.ModTime(), cacheDir); err == nil {
			l.logf(
				"video poster initial time ready mediaID=%s filename=%q seconds=%.1f source=%s duration=%s",
				id,
				filename,
				normalizePosterTime(bestSeconds),
				cacheSource(cached),
				time.Since(started).Round(time.Millisecond),
			)
			seconds = normalizePosterTime(bestSeconds)
		}
	}

	cachePath := filepath.Join(cacheDir, posterCacheName(filename, seconds))
	if cacheFresh(cachePath, info.ModTime()) {
		l.logf(
			"video poster ready mediaID=%s filename=%q seconds=%.1f source=cache path=%q duration=%s",
			id,
			filename,
			seconds,
			filepath.Base(cachePath),
			time.Since(started).Round(time.Millisecond),
		)
		return cachePath, nil
	} else if _, err := os.Stat(cachePath); err != nil && !errors.Is(err, os.ErrNotExist) {
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

	l.logf(
		"video poster ready mediaID=%s filename=%q seconds=%.1f source=generated path=%q duration=%s",
		id,
		filename,
		seconds,
		filepath.Base(cachePath),
		time.Since(started).Round(time.Millisecond),
	)
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

func posterCacheName(filename string, seconds float64) string {
	return fmt.Sprintf("%s.%.1fs.jpg", filename, seconds)
}

func initialPosterTime(ffmpeg, path, filename string, sourceModTime time.Time, cacheDir string) (float64, bool, error) {
	cachePath := filepath.Join(cacheDir, posterSmartTimeCacheName(filename))
	if cacheFresh(cachePath, sourceModTime) {
		if seconds, err := readPosterSmartTime(cachePath); err == nil {
			return seconds, true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(cachePath)
		}
	} else if _, err := os.Stat(cachePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(cachePath)
	}

	seconds, err := bestInitialPosterTime(ffmpeg, path)
	if err != nil {
		return 0, false, err
	}
	if err := writePosterSmartTime(cachePath, seconds); err != nil {
		return 0, false, err
	}
	return seconds, false, nil
}

func cacheSource(cached bool) string {
	if cached {
		return "cache"
	}
	return "generated"
}

func posterSmartTimeCacheName(filename string) string {
	return filename + "." + posterSmartVersion + ".poster-time"
}

func cacheFresh(path string, sourceModTime time.Time) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && !info.ModTime().Before(sourceModTime)
}

func readPosterSmartTime(path string) (float64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(content)), 64)
	if err != nil {
		return 0, err
	}
	return normalizePosterTime(seconds), nil
}

func writePosterSmartTime(path string, seconds float64) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".poster-time-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	_, writeErr := fmt.Fprintf(tmpFile, "%.1f\n", normalizePosterTime(seconds))
	closeErr := tmpFile.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func bestInitialPosterTime(ffmpeg, path string) (float64, error) {
	if usable, err := exactPosterFrameUsable(ffmpeg, path, 0); err == nil && usable {
		return 0, nil
	}

	frameSize := posterScanSize * posterScanSize * 3
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", path,
		"-ss", fmt.Sprintf("%.3f", posterScanStep),
		"-t", fmt.Sprintf("%d", posterScanSeconds),
		"-vf", fmt.Sprintf("fps=%d,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,format=rgb24", posterScanFPS, posterScanSize, posterScanSize, posterScanSize, posterScanSize),
		"-f", "rawvideo",
		"pipe:1",
	}

	output, err := exec.Command(ffmpeg, args...).Output()
	if err != nil {
		return 0, err
	}

	reader := bytes.NewReader(output)
	frame := make([]byte, frameSize)
	index := 0
	bestFallbackTime := 0.0
	bestFallbackScore := 0.0
	for {
		_, err := io.ReadFull(reader, frame)
		if err == nil {
			if posterFrameUsable(frame) {
				return posterScanStep + float64(index)/posterScanFPS, nil
			}
			if score := posterFrameFallbackScore(frame); score > bestFallbackScore {
				bestFallbackTime = posterScanStep + float64(index)/posterScanFPS
				bestFallbackScore = score
			}
			index++
			continue
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if bestFallbackScore > 0 {
				return bestFallbackTime, nil
			}
			return 0, nil
		}
		return 0, err
	}
}

func exactPosterFrameUsable(ffmpeg, path string, seconds float64) (bool, error) {
	frameSize := posterScanSize * posterScanSize * 3
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", seconds),
		"-i", path,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,format=rgb24", posterScanSize, posterScanSize, posterScanSize, posterScanSize),
		"-f", "rawvideo",
		"pipe:1",
	}

	output, err := exec.Command(ffmpeg, args...).Output()
	if err != nil {
		return false, err
	}
	if len(output) < frameSize {
		return false, io.ErrUnexpectedEOF
	}
	return posterFrameUsable(output[:frameSize]), nil
}

func posterFrameUsable(rgb []byte) bool {
	stats, ok := posterFrameStatsForRGB(rgb)
	if !ok {
		return false
	}
	return stats.averageLuma > 24 && stats.blackRatio < 0.85 && (stats.averageLuma > 55 || stats.brightRatio > 0.04)
}

func posterFrameFallbackScore(rgb []byte) float64 {
	stats, ok := posterFrameStatsForRGB(rgb)
	if !ok {
		return 0
	}
	if stats.averageLuma < 7 || stats.blackRatio > 0.9 || stats.brightRatio < 0.006 || stats.edgeDelta < 2.5 {
		return 0
	}
	return stats.averageLuma + stats.brightRatio*500 + stats.edgeDelta*4 - stats.blackRatio*10
}

type posterFrameStats struct {
	averageLuma float64
	blackRatio  float64
	brightRatio float64
	edgeDelta   float64
}

func posterFrameStatsForRGB(rgb []byte) (posterFrameStats, bool) {
	if len(rgb) < 3 {
		return posterFrameStats{}, false
	}

	pixels := len(rgb) / 3
	lumas := make([]float64, pixels)
	var lumaTotal float64
	blackPixels := 0
	brightPixels := 0
	for i := 0; i+2 < len(rgb); i += 3 {
		luma := 0.2126*float64(rgb[i]) + 0.7152*float64(rgb[i+1]) + 0.0722*float64(rgb[i+2])
		lumas[i/3] = luma
		lumaTotal += luma
		if luma < 16 {
			blackPixels++
		}
		if luma > 120 {
			brightPixels++
		}
	}

	averageLuma := lumaTotal / float64(pixels)
	blackRatio := float64(blackPixels) / float64(pixels)
	brightRatio := float64(brightPixels) / float64(pixels)
	edgeDelta := posterFrameEdgeDelta(lumas, posterScanSize, posterScanSize)
	return posterFrameStats{
		averageLuma: averageLuma,
		blackRatio:  blackRatio,
		brightRatio: brightRatio,
		edgeDelta:   edgeDelta,
	}, true
}

func posterFrameEdgeDelta(lumas []float64, width, height int) float64 {
	if len(lumas) != width*height {
		return 0
	}

	total := 0.0
	edges := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			current := lumas[y*width+x]
			if x+1 < width {
				total += math.Abs(current - lumas[y*width+x+1])
				edges++
			}
			if y+1 < height {
				total += math.Abs(current - lumas[(y+1)*width+x])
				edges++
			}
		}
	}
	if edges == 0 {
		return 0
	}
	return total / float64(edges)
}
