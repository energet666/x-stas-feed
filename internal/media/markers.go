package media

import (
	"bufio"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	markersDirName      = ".markers"
	maxMarkersPerMedia  = 1000
	maxMarkerTimeSecond = 24 * 60 * 60
)

var ErrMarkerLimitReached = errors.New("marker limit reached for this media")

// MarkerStore keeps timeline markers for one media root, one JSON object per line.
type MarkerStore struct {
	root string
	mu   sync.Mutex
}

type Marker struct {
	TimeSeconds int       `json:"timeSeconds"`
	Author      string    `json:"author"`
	CreatedAt   time.Time `json:"createdAt"`
}

func NewMarkerStore(mediaRoot string) *MarkerStore {
	return &MarkerStore{root: filepath.Join(mediaRoot, markersDirName)}
}

func (s *MarkerStore) List(filename string) ([]Marker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	markers, err := readMarkerFile(s.pathForFilename(filename))
	if errors.Is(err, os.ErrNotExist) {
		return []Marker{}, nil
	}
	if err != nil {
		return nil, err
	}
	return markers, nil
}

// Add stores one marker at a whole second. One author marks one second once, so
// a repeat returns the existing marker and reports that nothing was created.
func (s *MarkerStore) Add(filename string, timeSeconds float64, author string) (Marker, bool, error) {
	if math.IsNaN(timeSeconds) || math.IsInf(timeSeconds, 0) {
		return Marker{}, false, errors.New("marker time is invalid")
	}
	if timeSeconds < 0 || timeSeconds > maxMarkerTimeSecond {
		return Marker{}, false, errors.New("marker time is out of range")
	}

	marker := Marker{
		TimeSeconds: int(math.Round(timeSeconds)),
		Author:      normalizeAuthor(author),
		CreatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := readMarkerFile(s.pathForFilename(filename))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Marker{}, false, err
	}
	for _, current := range existing {
		if current.TimeSeconds == marker.TimeSeconds && current.Author == marker.Author {
			return current, false, nil
		}
	}
	if len(existing) >= maxMarkersPerMedia {
		return Marker{}, false, ErrMarkerLimitReached
	}

	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return Marker{}, false, err
	}

	file, err := os.OpenFile(s.pathForFilename(filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Marker{}, false, err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(marker); err != nil {
		return Marker{}, false, err
	}

	return marker, true, nil
}

func readMarkerFile(path string) ([]Marker, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var markers []Marker
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var marker Marker
		if err := json.Unmarshal([]byte(line), &marker); err != nil {
			return nil, err
		}
		if marker.TimeSeconds < 0 {
			continue
		}
		marker.Author = normalizeAuthor(marker.Author)
		markers = append(markers, marker)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if markers == nil {
		return []Marker{}, nil
	}

	return markers, nil
}

func (s *MarkerStore) pathForFilename(filename string) string {
	return filepath.Join(s.root, filename+".jsonl")
}
