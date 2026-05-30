package media

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func IsImageFilename(name string) bool {
	kind, ok := kindForPath(name)
	return ok && kind == "image"
}

func IsBoardBackgroundImageFilename(name string) bool {
	return IsImageFilename(name) && strings.ToLower(filepath.Ext(name)) != ".gif"
}

func (l *Library) SaveUpload(originalName string, reader io.Reader) (Item, error) {
	return l.SaveUploadWithModifiedAt(originalName, reader, time.Time{})
}

func (l *Library) SaveUploadWithModifiedAt(originalName string, reader io.Reader, sourceModifiedAt time.Time) (Item, error) {
	started := time.Now()
	if originalName == "" {
		return Item{}, errors.New("filename is required")
	}
	if filepath.Base(originalName) != originalName || strings.ContainsAny(originalName, `/\`) {
		return Item{}, errors.New("filename must not include a path")
	}

	extension := strings.ToLower(filepath.Ext(originalName))
	kind, ok := kindForPath(originalName)
	if !ok {
		return Item{}, fmt.Errorf("unsupported file type %q", extension)
	}
	if err := l.ensureIndex(); err != nil {
		return Item{}, err
	}

	root, err := filepath.Abs(l.root)
	if err != nil {
		return Item{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Item{}, err
	}

	filename, file, err := createUniqueFile(root, originalName)
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
	item.DisplayName = filename
	metadata := Metadata{DisplayName: item.DisplayName, LikeCount: item.LikeCount}
	if updatedMetadata, ok := l.applyAudioMetadata(&item, path, info, metadata); ok {
		metadata.Audio = updatedMetadata.Audio
		metadata.Video = updatedMetadata.Video
	}
	if updatedMetadata, ok := l.applyVideoMetadata(&item, path, info, metadata); ok {
		metadata.Audio = updatedMetadata.Audio
		metadata.Video = updatedMetadata.Video
	}
	if !sourceModifiedAt.IsZero() {
		metadata.ModifiedAt = sourceModifiedAt.UTC()
		item.ModifiedAt = metadata.ModifiedAt
	}
	if err := l.metadata.Set(item.Filename, metadata); err != nil {
		return Item{}, err
	}
	if err := l.insertItem(item, path); err != nil {
		return Item{}, err
	}
	l.logf("upload saved originalName=%q mediaID=%s filename=%q type=%s size=%d duration=%s", originalName, item.ID, item.Filename, item.Type, item.Size, time.Since(started).Round(time.Millisecond))
	return item, nil
}
