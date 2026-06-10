package media

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBoardStoreInitDoesNotCreatePlaceholderFromBoardMetadata(t *testing.T) {
	dir := t.TempDir()
	boardID := "abc123"
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	if err := os.MkdirAll(filepath.Join(dir, boardsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	metaLine, err := json.Marshal(boardMeta{Name: "Sketch", CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, boardsDirName, boardID+".jsonl"), append(metaLine, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, boardID+".board")); !os.IsNotExist(err) {
		t.Fatalf("expected metadata-only board not to create placeholder, err=%v", err)
	}
	if _, err := store.Get(boardID); err != ErrBoardNotFound {
		t.Fatalf("expected metadata-only board not to be loaded, got %v", err)
	}
}

func TestBoardStoreInitLoadsBoardWhenPlaceholderExists(t *testing.T) {
	dir := t.TempDir()
	boardID := "abc123"
	mediaID := EncodeID(boardID + ".board")
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	writeTestFile(t, dir, boardID+".board", createdAt)
	if err := os.MkdirAll(filepath.Join(dir, boardsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	metaLine, err := json.Marshal(boardMeta{Name: "Sketch", CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, boardsDirName, boardID+".board.jsonl"), append(metaLine, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Get(mediaID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Sketch" || info.ID != mediaID || info.MediaID != mediaID {
		t.Fatalf("expected placeholder-backed board to load, got %#v", info)
	}
}

func TestBoardStoreInitCreatesMissingMetadataForPlaceholder(t *testing.T) {
	dir := t.TempDir()
	boardID := "abc123"
	mediaID := EncodeID(boardID + ".board")
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	writeTestFile(t, dir, boardID+".board", createdAt)

	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Get(mediaID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != defaultBoardName(boardID) || !info.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected fallback board metadata from placeholder, got %#v", info)
	}
	if _, err := os.Stat(filepath.Join(dir, boardsDirName, boardID+".board.jsonl")); err != nil {
		t.Fatalf("expected board metadata file to be created: %v", err)
	}
}

func TestBoardStoreAddStrokeNormalizesCoordinates(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	stroke, err := store.AddStroke(info.ID, "line", [][]float64{
		{-1.234, 40.567},
		{1200.987, 799.949},
	}, "#fff", 4, 0.45, "Tester")
	if err != nil {
		t.Fatal(err)
	}

	expected := [][]float64{{-1.2, 40.6}, {1201, 799.9}}
	if !samePoints(stroke.Points, expected) {
		t.Fatalf("expected normalized points %#v, got %#v", expected, stroke.Points)
	}
	if stroke.Opacity != 0.45 {
		t.Fatalf("expected stroke opacity 0.45, got %v", stroke.Opacity)
	}

	strokes, err := store.Strokes(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(strokes) != 1 || !samePoints(strokes[0].Points, expected) {
		t.Fatalf("expected stored normalized points %#v, got %#v", expected, strokes)
	}
}

func TestBoardStoreCreateUsesBoardFilenameForEmptyInput(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("")
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "board" || info.Filename != "board.board" {
		t.Fatalf("expected default board filename and name, got %#v", info)
	}
	if _, err := os.Stat(filepath.Join(dir, "board.board")); err != nil {
		t.Fatalf("expected board placeholder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, boardsDirName, "board.board.jsonl")); err != nil {
		t.Fatalf("expected board history file: %v", err)
	}
}

func TestBoardStoreCreatePreservesExplicitBoardName(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Board")
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "Board" {
		t.Fatalf("expected explicit board name to be preserved, got %q", info.Name)
	}
	if info.Filename != "Board.board" {
		t.Fatalf("expected explicit board filename, got %#v", info)
	}
}

func TestBoardStoreCreateAddsSuffixForDuplicateName(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	first, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	if first.Filename != "Sketch.board" || first.Name != "Sketch" {
		t.Fatalf("expected first board to use explicit name, got %#v", first)
	}
	if second.Filename != "Sketch (1).board" || second.Name != "Sketch (1)" {
		t.Fatalf("expected duplicate board to use suffix, got %#v", second)
	}
	if _, err := os.Stat(filepath.Join(dir, "Sketch (1).board")); err != nil {
		t.Fatalf("expected suffixed board placeholder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, boardsDirName, "Sketch (1).board.jsonl")); err != nil {
		t.Fatalf("expected suffixed board history file: %v", err)
	}
}

func TestImageCanvasAppliesJPEGEXIFOrientation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotated.jpeg")

	img := image.NewRGBA(image.Rect(0, 0, 3024, 4032))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, nil); err != nil {
		t.Fatal(err)
	}

	bytes := encoded.Bytes()
	withEXIF := append([]byte{0xff, 0xd8}, jpegEXIFSegment(6)...)
	withEXIF = append(withEXIF, bytes[2:]...)
	if err := os.WriteFile(path, withEXIF, 0o644); err != nil {
		t.Fatal(err)
	}

	canvas := imageCanvas(path)
	if canvas.Width != 1131 || canvas.Height != 849 {
		t.Fatalf("expected EXIF-rotated normalized canvas 1131x849, got %#v", canvas)
	}
}

func TestNormalizeImageBoardCanvasPreservesSmallImages(t *testing.T) {
	canvas := normalizeImageBoardCanvas(1200, 800)
	if canvas.Width != 1200 || canvas.Height != 800 {
		t.Fatalf("expected small image canvas to stay unchanged, got %#v", canvas)
	}
}

func TestNormalizeImageBoardCanvasCapsLongSide(t *testing.T) {
	canvas := normalizeImageBoardCanvas(4032, 3024)
	if canvas.Width != 1131 || canvas.Height != 849 {
		t.Fatalf("expected normalized landscape canvas 1131x849, got %#v", canvas)
	}

	canvas = normalizeImageBoardCanvas(3024, 4032)
	if canvas.Width != 849 || canvas.Height != 1131 {
		t.Fatalf("expected normalized portrait canvas 849x1131, got %#v", canvas)
	}
}

func TestBoardStoreAddStrokeAllowsSingleFreeformPoint(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	stroke, err := store.AddStroke(info.ID, "freeform", [][]float64{{10.04, 20.06}}, "#fff", 4, 1, "Tester")
	if err != nil {
		t.Fatal(err)
	}

	expected := [][]float64{{10, 20.1}}
	if !samePoints(stroke.Points, expected) {
		t.Fatalf("expected normalized point %#v, got %#v", expected, stroke.Points)
	}
}

func jpegEXIFSegment(orientation uint16) []byte {
	tiff := make([]byte, 8+2+12+4)
	copy(tiff[0:2], "II")
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)
	binary.LittleEndian.PutUint16(tiff[8:10], 1)
	entry := tiff[10:22]
	binary.LittleEndian.PutUint16(entry[0:2], 0x0112)
	binary.LittleEndian.PutUint16(entry[2:4], 3)
	binary.LittleEndian.PutUint32(entry[4:8], 1)
	binary.LittleEndian.PutUint16(entry[8:10], orientation)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:4], uint16(len(payload)+2))
	return append(segment, payload...)
}

func TestBoardStoreAddStrokeRejectsSingleLinePoint(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.AddStroke(info.ID, "line", [][]float64{{10, 20}}, "#fff", 4, 1, "Tester"); err == nil {
		t.Fatal("expected single-point line stroke to be rejected")
	}
}

func TestBoardStoreAddStrokeRejectsInvalidPoints(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.AddStroke(info.ID, "line", [][]float64{{1, 2, 3}, {4, 5}}, "#fff", 4, 1, "Tester"); err == nil {
		t.Fatal("expected invalid coordinate pair to be rejected")
	}
}

func TestBoardStoreAddStrokeRejectsInvalidOpacity(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.AddStroke(info.ID, "freeform", [][]float64{{10, 20}}, "#fff", 4, 1.1, "Tester"); err == nil {
		t.Fatal("expected opacity above 1 to be rejected")
	}
}

func TestBoardStorePersistsOrderedImageOperation(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	info, err := store.Create("Sketch")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddStroke(info.ID, "freeform", [][]float64{{1, 2}}, "#fff", 4, 1, "Tester"); err != nil {
		t.Fatal(err)
	}
	image, err := store.AddImage(info.ID, "image/png", strings.NewReader("png"), 10, 20, 300, 200, 32.55, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	if image.Rotation != 32.6 {
		t.Fatalf("expected normalized rotation, got %v", image.Rotation)
	}

	reloaded := NewBoardStore(dir)
	if err := reloaded.Init(); err != nil {
		t.Fatal(err)
	}
	operations, err := reloaded.Operations(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 2 || operations[0].Type != "stroke" || operations[1].Type != "image" {
		t.Fatalf("expected ordered stroke and image operations, got %#v", operations)
	}
	path, mimeType, err := reloaded.AssetPath(info.ID, image.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/png" {
		t.Fatalf("expected image/png, got %q", mimeType)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes) != "png" {
		t.Fatalf("expected persisted asset bytes, got %q", bytes)
	}
}

func TestBoardStoreDeduplicatesImageAssetsByContent(t *testing.T) {
	dir := t.TempDir()
	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	firstBoard, err := store.Create("First")
	if err != nil {
		t.Fatal(err)
	}
	secondBoard, err := store.Create("Second")
	if err != nil {
		t.Fatal(err)
	}

	content := "same transparent image bytes"
	first, err := store.AddImage(firstBoard.ID, "image/png", strings.NewReader(content), 10, 20, 300, 200, 0, "One")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.AddImage(secondBoard.ID, "image/webp", strings.NewReader(content), 40, 50, 150, 100, 25, "Two")
	if err != nil {
		t.Fatal(err)
	}

	if first.ID == second.ID {
		t.Fatal("expected placements to keep distinct operation IDs")
	}
	if first.AssetID != second.AssetID || first.Filename != second.Filename {
		t.Fatalf("expected matching content to reuse one asset, first=%#v second=%#v", first, second)
	}
	entries, err := os.ReadDir(filepath.Join(dir, boardsDirName, boardAssetsDirName))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != first.Filename {
		t.Fatalf("expected exactly one content-addressed asset, got %#v", entries)
	}

	assets := store.Assets()
	if len(assets) != 1 || assets[0].ID != first.AssetID || assets[0].UsageCount != 2 {
		t.Fatalf("expected one reusable asset with two usages, got %#v", assets)
	}
	reused, err := store.AddExistingImage(firstBoard.ID, first.AssetID, 60, 70, 120, 80, 15, "Three")
	if err != nil {
		t.Fatal(err)
	}
	if reused.AssetID != first.AssetID || reused.ID == first.ID {
		t.Fatalf("expected a distinct placement reusing the asset, got %#v", reused)
	}
	assets = store.Assets()
	if len(assets) != 1 || assets[0].UsageCount != 3 {
		t.Fatalf("expected reuse count to increase, got %#v", assets)
	}
}

func TestBoardStoreInitImportsStickerPackAssets(t *testing.T) {
	dir := t.TempDir()
	stickerDir := filepath.Join(dir, boardsDirName, boardStickerPackDirName, "openmoji")
	if err := os.MkdirAll(stickerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pngBytes := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	if err := os.WriteFile(filepath.Join(stickerDir, "rocket.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stickerDir, "rocket-copy.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stickerDir, "README.md"), []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewBoardStore(dir)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	assets := store.Assets()
	if len(assets) != 1 {
		t.Fatalf("expected duplicate sticker files to produce one asset, got %#v", assets)
	}
	asset := assets[0]
	if asset.MimeType != "image/png" || asset.UsageCount != 0 {
		t.Fatalf("expected unused PNG sticker asset, got %#v", asset)
	}
	path, mimeType, err := store.GlobalAssetPath(asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/png" {
		t.Fatalf("expected image/png, got %q", mimeType)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, pngBytes) {
		t.Fatalf("expected imported sticker bytes, got %q", stored)
	}

	board, err := store.Create("Sticker board")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddExistingImage(board.ID, asset.ID, 10, 20, 100, 100, 0, "Tester"); err != nil {
		t.Fatal(err)
	}
	assets = store.Assets()
	if len(assets) != 1 || assets[0].UsageCount != 1 {
		t.Fatalf("expected imported sticker usage to increase, got %#v", assets)
	}

	reloaded := NewBoardStore(dir)
	if err := reloaded.Init(); err != nil {
		t.Fatal(err)
	}
	assets = reloaded.Assets()
	if len(assets) != 1 || assets[0].UsageCount != 1 {
		t.Fatalf("expected imported sticker and usage after restart, got %#v", assets)
	}
}

func samePoints(a, b [][]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
