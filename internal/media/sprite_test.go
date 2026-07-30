package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpriteLayoutForDurationRejectsShortMedia(t *testing.T) {
	for _, duration := range []float64{0, 1.5, 4.9} {
		if _, ok := spriteLayoutForDuration(duration); ok {
			t.Fatalf("expected duration %.1f to be rejected", duration)
		}
	}
}

func TestSpriteLayoutForDurationKeepsMinimumInterval(t *testing.T) {
	layout, ok := spriteLayoutForDuration(60)
	if !ok {
		t.Fatal("expected a layout for a 60 second video")
	}
	if layout.interval != spriteMinInterval {
		t.Fatalf("interval = %.2f, want %.2f", layout.interval, spriteMinInterval)
	}
	if layout.count != 29 {
		t.Fatalf("count = %d, want 29", layout.count)
	}
	if layout.columns != spriteColumns || layout.rows != 3 {
		t.Fatalf("grid = %dx%d, want %dx3", layout.columns, layout.rows, spriteColumns)
	}
}

func TestSpriteLayoutForDurationUsesSingleRowForShortMedia(t *testing.T) {
	layout, ok := spriteLayoutForDuration(10)
	if !ok {
		t.Fatal("expected a layout for a 10 second video")
	}
	if layout.count != 4 || layout.columns != 4 || layout.rows != 1 {
		t.Fatalf("layout = %d tiles in %dx%d, want 4 tiles in 4x1", layout.count, layout.columns, layout.rows)
	}
}

func TestSpriteLayoutForDurationCapsTileCount(t *testing.T) {
	for _, duration := range []float64{400, 3600, 24 * 60 * 60} {
		layout, ok := spriteLayoutForDuration(duration)
		if !ok {
			t.Fatalf("expected a layout for duration %.0f", duration)
		}
		if layout.count != spriteMaxTiles {
			t.Fatalf("duration %.0f: count = %d, want %d", duration, layout.count, spriteMaxTiles)
		}
		if layout.columns*layout.rows < layout.count {
			t.Fatalf("duration %.0f: grid %dx%d cannot hold %d tiles", duration, layout.columns, layout.rows, layout.count)
		}
	}
}

// ffmpeg only emits the last sample when the source runs half an interval past
// it, so every tile has to fit inside the shortened timeline. A grid that
// overruns it ends with a black padding tile; one that stops too early wastes
// preview coverage.
func TestSpriteLayoutFillsEveryTile(t *testing.T) {
	for _, duration := range []float64{5, 5.1, 9.9, 10, 10.5, 61, 199, 200, 201, 480.932, 601, 1076.2, 7200} {
		layout, ok := spriteLayoutForDuration(duration)
		if !ok {
			t.Fatalf("expected a layout for duration %.1f", duration)
		}
		if layout.interval < spriteMinInterval {
			t.Fatalf("duration %.1f: interval = %.3f, want at least %.1f", duration, layout.interval, spriteMinInterval)
		}

		usable := duration - spriteTailMargin
		if span := float64(layout.count) * layout.interval; span > usable+1e-9 {
			t.Fatalf("duration %.1f: %d tiles span %.3f, past the usable %.3f", duration, layout.count, span, usable)
		}
		if layout.count < spriteMaxTiles {
			if span := float64(layout.count+1) * layout.interval; span <= usable {
				t.Fatalf("duration %.1f: dropped a usable tile, %d tiles still fit in %.3f", duration, layout.count+1, usable)
			}
		}
	}
}

func TestSpriteSheetRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.mp4.sprite-v1.json")
	sheet := SpriteSheet{
		Version:         spriteVersion,
		TileWidth:       160,
		TileHeight:      90,
		Columns:         10,
		Rows:            3,
		Count:           30,
		IntervalSeconds: 2,
		DurationSeconds: 60,
	}

	if err := writeSpriteSheet(path, sheet); err != nil {
		t.Fatalf("writeSpriteSheet: %v", err)
	}
	loaded, err := readSpriteSheet(path)
	if err != nil {
		t.Fatalf("readSpriteSheet: %v", err)
	}
	if loaded != sheet {
		t.Fatalf("loaded = %+v, want %+v", loaded, sheet)
	}
}

func TestReadSpriteSheetRejectsOtherVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.mp4.sprite-v1.json")
	if err := os.WriteFile(path, []byte(`{"version":"sprite-v0","tileWidth":160,"tileHeight":90,"columns":10,"rows":3,"count":30,"intervalSeconds":2}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	if _, err := readSpriteSheet(path); err == nil {
		t.Fatal("expected a stale sprite version to be rejected")
	}
}
