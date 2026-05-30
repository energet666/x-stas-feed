package media

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const metadataDirName = ".metadata"

type MetadataStore struct {
	root string
	mu   sync.Mutex
}

type Metadata struct {
	DisplayName string         `json:"displayName"`
	ModifiedAt  time.Time      `json:"modifiedAt,omitempty"`
	LikeCount   int            `json:"likeCount"`
	Audio       *AudioMetadata `json:"audio,omitempty"`
	Video       *VideoMetadata `json:"video,omitempty"`
}

type AudioMetadata struct {
	DurationSeconds       float64   `json:"durationSeconds,omitempty"`
	Tags                  AudioTags `json:"tags,omitempty"`
	HasCover              bool      `json:"hasCover,omitempty"`
	CoverFile             string    `json:"coverFile,omitempty"`
	SourceSize            int64     `json:"sourceSize"`
	SourceModTimeUnixNano int64     `json:"sourceModTimeUnixNano"`
	ProbedAt              time.Time `json:"probedAt,omitempty"`
}

type VideoMetadata struct {
	DurationSeconds       float64   `json:"durationSeconds,omitempty"`
	SourceSize            int64     `json:"sourceSize"`
	SourceModTimeUnixNano int64     `json:"sourceModTimeUnixNano"`
	ProbedAt              time.Time `json:"probedAt,omitempty"`
}

func NewMetadataStore(mediaRoot string) *MetadataStore {
	return &MetadataStore{root: filepath.Join(mediaRoot, metadataDirName)}
}

func (s *MetadataStore) Exists(filename string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.pathForFilename(filename))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func (s *MetadataStore) Get(filename string) (Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.pathForFilename(filename))
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, nil
	}
	if err != nil {
		return Metadata{}, err
	}
	defer file.Close()

	var metadata Metadata
	if err := json.NewDecoder(file).Decode(&metadata); err != nil {
		return Metadata{}, err
	}
	return normalizeMetadata(metadata), nil
}

func (s *MetadataStore) Set(filename string, metadata Metadata) error {
	metadata = normalizeMetadata(metadata)

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeLocked(filename, metadata)
}

func (s *MetadataStore) AddLike(filename string) (Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err := s.readLocked(filename)
	if err != nil {
		return Metadata{}, err
	}
	metadata.LikeCount++
	if err := s.writeLocked(filename, metadata); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func (s *MetadataStore) readLocked(filename string) (Metadata, error) {
	file, err := os.Open(s.pathForFilename(filename))
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, nil
	}
	if err != nil {
		return Metadata{}, err
	}
	defer file.Close()

	var metadata Metadata
	if err := json.NewDecoder(file).Decode(&metadata); err != nil {
		return Metadata{}, err
	}
	return normalizeMetadata(metadata), nil
}

func (s *MetadataStore) writeLocked(filename string, metadata Metadata) error {
	metadata = normalizeMetadata(metadata)

	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(s.root, ".metadata-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	encodeErr := json.NewEncoder(tmpFile).Encode(metadata)
	closeErr := tmpFile.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}

	return os.Rename(tmpPath, s.pathForFilename(filename))
}

func normalizeMetadata(metadata Metadata) Metadata {
	if metadata.LikeCount < 0 {
		metadata.LikeCount = 0
	}
	return metadata
}

func (s *MetadataStore) pathForFilename(filename string) string {
	return filepath.Join(s.root, filename+".json")
}
