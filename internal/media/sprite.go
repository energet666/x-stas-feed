package media

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	spriteVersion = "sprite-v1"
	// One sheet holds at most 100 tiles and never samples faster than one tile
	// every two seconds, so short clips stay coarse and long ones stay bounded.
	spriteMaxTiles    = 100
	spriteMinInterval = 2.0
	spriteColumns     = 10
	spriteTileWidth   = 160
	spriteMinDuration = 5.0
	spriteTailMargin  = 0.5
	spriteQuality     = "5"
	// Past this length a full decode costs seconds, while the sampling
	// interval is wide enough that snapping tiles to keyframes is invisible.
	spriteKeyframeOnlySeconds = 5 * 60
)

// ErrSpriteUnavailable reports media that is too short to preview by scrubbing.
var ErrSpriteUnavailable = errors.New("sprite sheet is unavailable for this media")

// SpriteSheet describes the tile grid of one generated scrubbing sheet. Tile i
// holds the frame at intervalSeconds*(i+0.5), so a client maps a timeline
// position to a tile with floor(time/intervalSeconds).
type SpriteSheet struct {
	Version         string  `json:"version"`
	TileWidth       int     `json:"tileWidth"`
	TileHeight      int     `json:"tileHeight"`
	Columns         int     `json:"columns"`
	Rows            int     `json:"rows"`
	Count           int     `json:"count"`
	IntervalSeconds float64 `json:"intervalSeconds"`
	DurationSeconds float64 `json:"durationSeconds"`
}

type spriteLayout struct {
	columns  int
	rows     int
	count    int
	interval float64
}

type spriteResult struct {
	path  string
	sheet SpriteSheet
	err   error
}

type spriteCall struct {
	done   chan struct{}
	result spriteResult
}

// SpriteForID returns the cached scrubbing sheet for one video, generating it
// on first use. Concurrent callers for the same media share one generation.
func (l *Library) SpriteForID(id string) (string, SpriteSheet, error) {
	path, mimeType, err := l.PathForID(id)
	if err != nil {
		return "", SpriteSheet{}, err
	}
	if !strings.HasPrefix(mimeType, "video/") {
		return "", SpriteSheet{}, ErrSpriteUnavailable
	}

	return l.spriteOnce(id, func() (string, SpriteSheet, error) {
		return l.buildSprite(id, path)
	})
}

func (l *Library) buildSprite(id, path string) (string, SpriteSheet, error) {
	started := time.Now()
	item := l.itemSnapshot(id)
	filename := item.Filename
	if filename == "" {
		filename = filepath.Base(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", SpriteSheet{}, err
	}

	duration := item.DurationSeconds
	if duration <= 0 {
		probed, probeErr := probeMedia(path)
		if probeErr != nil {
			return "", SpriteSheet{}, probeErr
		}
		duration = probed.DurationSeconds
	}

	layout, ok := spriteLayoutForDuration(duration)
	if !ok {
		return "", SpriteSheet{}, ErrSpriteUnavailable
	}

	cacheDir := filepath.Join(l.root, posterDirName)
	imagePath := filepath.Join(cacheDir, spriteImageCacheName(filename))
	metaPath := filepath.Join(cacheDir, spriteMetaCacheName(filename))

	if cacheFresh(imagePath, info.ModTime()) && cacheFresh(metaPath, info.ModTime()) {
		if sheet, err := readSpriteSheet(metaPath); err == nil {
			l.logf(
				"video sprite ready mediaID=%s filename=%q tiles=%d interval=%.2fs source=cache path=%q duration=%s",
				id, filename, sheet.Count, sheet.IntervalSeconds, filepath.Base(imagePath), time.Since(started).Round(time.Millisecond),
			)
			return imagePath, sheet, nil
		}
	}

	ffmpeg, err := ffmpegPath()
	if err != nil {
		return "", SpriteSheet{}, errors.New("ffmpeg is required to generate sprite sheets")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", SpriteSheet{}, err
	}

	tmpFile, err := os.CreateTemp(cacheDir, ".sprite-*.jpg")
	if err != nil {
		return "", SpriteSheet{}, err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	keyframeOnly := duration >= spriteKeyframeOnlySeconds
	if err := runSpriteEncode(ffmpeg, path, tmpPath, layout, keyframeOnly); err != nil {
		if !keyframeOnly {
			return "", SpriteSheet{}, err
		}
		// Keyframe-only decoding is a speed optimization, not a requirement:
		// containers without usable keyframe markers fall back to a full pass.
		l.logf("video sprite keyframe pass failed mediaID=%s filename=%q error=%v", id, filename, err)
		if err := runSpriteEncode(ffmpeg, path, tmpPath, layout, false); err != nil {
			return "", SpriteSheet{}, err
		}
		keyframeOnly = false
	}

	tileWidth, tileHeight, err := spriteTileSize(tmpPath, layout)
	if err != nil {
		return "", SpriteSheet{}, err
	}

	sheet := SpriteSheet{
		Version:         spriteVersion,
		TileWidth:       tileWidth,
		TileHeight:      tileHeight,
		Columns:         layout.columns,
		Rows:            layout.rows,
		Count:           layout.count,
		IntervalSeconds: layout.interval,
		DurationSeconds: duration,
	}
	if err := writeSpriteSheet(metaPath, sheet); err != nil {
		return "", SpriteSheet{}, err
	}
	if err := os.Rename(tmpPath, imagePath); err != nil {
		return "", SpriteSheet{}, err
	}

	l.logf(
		"video sprite ready mediaID=%s filename=%q tiles=%d grid=%dx%d tile=%dx%d interval=%.2fs keyframeOnly=%t source=generated path=%q duration=%s",
		id, filename, sheet.Count, sheet.Columns, sheet.Rows, sheet.TileWidth, sheet.TileHeight,
		sheet.IntervalSeconds, keyframeOnly, filepath.Base(imagePath), time.Since(started).Round(time.Millisecond),
	)
	return imagePath, sheet, nil
}

// spriteLayoutForDuration keeps the sheet within spriteMaxTiles tiles while
// never sampling faster than spriteMinInterval.
func spriteLayoutForDuration(duration float64) (spriteLayout, bool) {
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < spriteMinDuration {
		return spriteLayout{}, false
	}

	// The fps filter only emits its last slot once the source runs half an
	// interval past it, and container duration can overstate the video stream,
	// so the grid is planned against a slightly shortened timeline. Otherwise
	// the sheet ends with a black padding tile.
	usable := duration - spriteTailMargin
	interval := usable / spriteMaxTiles
	count := spriteMaxTiles
	if interval < spriteMinInterval {
		interval = spriteMinInterval
		count = int(math.Floor(usable / interval))
	}
	if count < 2 {
		return spriteLayout{}, false
	}

	columns := spriteColumns
	if count < columns {
		columns = count
	}
	rows := (count + columns - 1) / columns

	return spriteLayout{columns: columns, rows: rows, count: count, interval: interval}, true
}

func runSpriteEncode(ffmpeg, source, target string, layout spriteLayout, keyframeOnly bool) error {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if keyframeOnly {
		args = append(args, "-skip_frame", "nokey")
	}
	args = append(args,
		// Half an interval of lead-in centers every tile inside its own slice
		// of the timeline instead of starting on the frame at zero.
		"-ss", fmt.Sprintf("%.3f", layout.interval/2),
		"-i", source,
		"-an",
		"-vf", fmt.Sprintf(
			"fps=1/%.6f,scale=%d:-2:flags=bicubic,setsar=1,tile=%dx%d",
			layout.interval, spriteTileWidth, layout.columns, layout.rows,
		),
		"-frames:v", "1",
		"-q:v", spriteQuality,
		"-y", target,
	)

	if output, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("generate sprite sheet: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// spriteTileSize reads the produced sheet instead of probing the source, since
// the tile filter always emits a full padded grid.
func spriteTileSize(path string, layout spriteLayout) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, err := jpeg.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	if layout.columns <= 0 || layout.rows <= 0 {
		return 0, 0, errors.New("sprite layout is empty")
	}
	tileWidth := config.Width / layout.columns
	tileHeight := config.Height / layout.rows
	if tileWidth <= 0 || tileHeight <= 0 {
		return 0, 0, errors.New("sprite sheet has no usable tiles")
	}
	return tileWidth, tileHeight, nil
}

func spriteImageCacheName(filename string) string {
	return filename + "." + spriteVersion + ".jpg"
}

func spriteMetaCacheName(filename string) string {
	return filename + "." + spriteVersion + ".json"
}

func readSpriteSheet(path string) (SpriteSheet, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return SpriteSheet{}, err
	}

	var sheet SpriteSheet
	if err := json.Unmarshal(content, &sheet); err != nil {
		return SpriteSheet{}, err
	}
	if sheet.Version != spriteVersion || !sheet.usable() {
		return SpriteSheet{}, errors.New("sprite sheet metadata is unusable")
	}
	return sheet, nil
}

func (s SpriteSheet) usable() bool {
	return s.TileWidth > 0 && s.TileHeight > 0 && s.Columns > 0 && s.Rows > 0 && s.Count > 0 && s.IntervalSeconds > 0
}

func writeSpriteSheet(path string, sheet SpriteSheet) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".sprite-meta-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	writeErr := json.NewEncoder(tmpFile).Encode(sheet)
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

// spriteOnce collapses concurrent generations of one media into a single
// ffmpeg pass. A scrubbing hover can otherwise fan out into many identical
// requests before the first one finishes writing its cache.
func (l *Library) spriteOnce(id string, generate func() (string, SpriteSheet, error)) (string, SpriteSheet, error) {
	l.spriteMu.Lock()
	if call, ok := l.spriteCalls[id]; ok {
		l.spriteMu.Unlock()
		<-call.done
		return call.result.path, call.result.sheet, call.result.err
	}
	if l.spriteCalls == nil {
		l.spriteCalls = make(map[string]*spriteCall)
	}
	call := &spriteCall{done: make(chan struct{})}
	l.spriteCalls[id] = call
	l.spriteMu.Unlock()

	path, sheet, err := generate()
	call.result = spriteResult{path: path, sheet: sheet, err: err}
	close(call.done)

	l.spriteMu.Lock()
	delete(l.spriteCalls, id)
	l.spriteMu.Unlock()

	return path, sheet, err
}
