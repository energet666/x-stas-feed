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

const uploadTempPattern = ".upload-*.tmp"

func IsImageFilename(name string) bool {
	kind, ok := kindForPath(name)
	return ok && kind == "image"
}

func IsBoardBackgroundImageFilename(name string) bool {
	return IsImageFilename(name) && strings.ToLower(filepath.Ext(name)) != ".gif"
}

// CreateUploadTemp opens a scratch file inside the library root so a received
// upload can be moved into place with a rename instead of a second full copy.
// The dot prefix keeps the scratch file out of the media index.
func (l *Library) CreateUploadTemp() (*os.File, error) {
	root, err := l.prepareUploadRoot()
	if err != nil {
		return nil, err
	}
	return os.CreateTemp(root, uploadTempPattern)
}

// RemoveUploadTemps deletes scratch files left behind by interrupted uploads.
func (l *Library) RemoveUploadTemps() error {
	root, err := filepath.Abs(l.root)
	if err != nil {
		return err
	}
	matches, err := filepath.Glob(filepath.Join(root, uploadTempPattern))
	if err != nil {
		return err
	}
	for _, path := range matches {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		l.logf("upload temp removed path=%q", path)
	}
	return nil
}

func (l *Library) SaveUpload(originalName string, reader io.Reader) (Item, error) {
	return l.SaveUploadWithModifiedAt(originalName, reader, time.Time{})
}

func (l *Library) SaveUploadWithModifiedAt(originalName string, reader io.Reader, sourceModifiedAt time.Time) (Item, error) {
	started := time.Now()
	root, kind, err := l.prepareUpload(originalName)
	if err != nil {
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

	return l.finishUpload(started, originalName, filename, path, kind, sourceModifiedAt)
}

// SaveUploadedTempFile takes ownership of a scratch file created by
// CreateUploadTemp and renames it into the library, which keeps a large upload
// from being read and written a second time. The scratch file is left in place
// when the upload is rejected, so the caller stays responsible for removing it.
func (l *Library) SaveUploadedTempFile(originalName, tempPath string, sourceModifiedAt time.Time) (Item, error) {
	started := time.Now()
	root, kind, err := l.prepareUpload(originalName)
	if err != nil {
		return Item{}, err
	}

	tempInfo, err := os.Stat(tempPath)
	if err != nil {
		return Item{}, err
	}
	if tempInfo.Size() == 0 {
		return Item{}, errors.New("uploaded file is empty")
	}

	filename, file, err := createUniqueFile(root, originalName)
	if err != nil {
		return Item{}, err
	}
	path := filepath.Join(root, filename)
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return Item{}, err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(path)
		return Item{}, err
	}
	if err := os.Chmod(path, 0o644); err != nil {
		l.logf("upload permissions not applied filename=%q error=%v", filename, err)
	}

	return l.finishUpload(started, originalName, filename, path, kind, sourceModifiedAt)
}

func (l *Library) prepareUpload(originalName string) (string, string, error) {
	if originalName == "" {
		return "", "", errors.New("filename is required")
	}
	if filepath.Base(originalName) != originalName || strings.ContainsAny(originalName, `/\`) {
		return "", "", errors.New("filename must not include a path")
	}

	kind, ok := kindForPath(originalName)
	if !ok {
		return "", "", fmt.Errorf("unsupported file type %q", strings.ToLower(filepath.Ext(originalName)))
	}
	if err := l.ensureIndex(); err != nil {
		return "", "", err
	}

	root, err := l.prepareUploadRoot()
	if err != nil {
		return "", "", err
	}
	return root, kind, nil
}

func (l *Library) prepareUploadRoot() (string, error) {
	root, err := filepath.Abs(l.root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	return root, nil
}

func (l *Library) finishUpload(started time.Time, originalName, filename, path, kind string, sourceModifiedAt time.Time) (Item, error) {
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
