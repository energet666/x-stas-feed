package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
	posterScanSeconds  = 10
	posterScanStep     = 0.5
	posterScanFPS      = 2
	posterScanSize     = 64
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

	if seconds == 0 {
		if bestSeconds, err := bestInitialPosterTime(ffmpeg, path); err == nil {
			seconds = normalizePosterTime(bestSeconds)
		}
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
