package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var safeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (l *Library) SaveUpload(originalName string, reader io.Reader) (Item, error) {
	if originalName == "" {
		return Item{}, errors.New("filename is required")
	}
	if filepath.Base(originalName) != originalName || strings.ContainsAny(originalName, `/\`) {
		return Item{}, errors.New("filename must not include a path")
	}

	extension := strings.ToLower(filepath.Ext(originalName))
	kind, ok := supportedExtensions[extension]
	if !ok {
		return Item{}, fmt.Errorf("unsupported media type %q", extension)
	}

	root, err := filepath.Abs(l.root)
	if err != nil {
		return Item{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Item{}, err
	}

	filename, file, err := l.createUploadFile(root, originalName, extension)
	if err != nil {
		return Item{}, err
	}

	path := filepath.Join(root, filename)
	size, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return Item{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return Item{}, closeErr
	}
	if size == 0 {
		_ = os.Remove(path)
		return Item{}, errors.New("uploaded file is empty")
	}

	now := time.Now().UTC()
	if err := os.Chtimes(path, now, now); err != nil {
		return Item{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return Item{}, err
	}
	item := itemFromFile(filename, path, kind, info)
	displayName := normalizeDisplayName(originalName)
	if displayName == "" {
		displayName = item.Filename
	}
	item.DisplayName = displayName
	if err := l.metadata.Set(item.ID, Metadata{DisplayName: displayName, LikeCount: item.LikeCount}); err != nil {
		return Item{}, err
	}
	l.Invalidate()
	return item, nil
}

func (l *Library) createUploadFile(root, originalName, extension string) (string, *os.File, error) {
	base := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	base = sanitizeUploadBase(base)
	if base == "" {
		base = "media"
	}

	for attempt := 0; attempt < 16; attempt++ {
		filename := fmt.Sprintf("%s-%s-%s%s", base, time.Now().UTC().Format("20060102T150405.000000000"), randomUploadSuffix(), extension)
		path := filepath.Join(root, filename)
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", nil, err
		}
		if absPath != root && !strings.HasPrefix(absPath, root+string(os.PathSeparator)) {
			return "", nil, errors.New("upload path escapes root")
		}

		file, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return filename, file, err
	}

	return "", nil, errors.New("could not allocate upload filename")
}

func sanitizeUploadBase(base string) string {
	base = strings.TrimSpace(base)
	base = strings.Trim(base, ".")
	base = safeFilenameChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-_.")
	if len(base) > 80 {
		base = strings.Trim(base[:80], "-_.")
	}
	return base
}

func randomUploadSuffix() string {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(bytes[:])
}
