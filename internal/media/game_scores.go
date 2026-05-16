package media

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	gameScoresDirName  = ".game-scores"
	gameScoresFileName = "leaderboard.json"
	maxGameScores      = 5
	maxGameNameRunes   = 40
)

type GameScore struct {
	Name      string    `json:"name"`
	Score     int       `json:"score"`
	CreatedAt time.Time `json:"createdAt"`
}

type GameScoreStore struct {
	path string
	mu   sync.Mutex
}

func NewGameScoreStore(contentRoot string) *GameScoreStore {
	return &GameScoreStore{path: filepath.Join(contentRoot, gameScoresDirName, gameScoresFileName)}
}

func (s *GameScoreStore) List() ([]GameScore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readLocked()
}

func (s *GameScoreStore) Add(name string, score int) ([]GameScore, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = defaultAuthor
	}
	runes := []rune(name)
	if len(runes) > maxGameNameRunes {
		name = string(runes[:maxGameNameRunes])
	}

	next := GameScore{
		Name:      name,
		Score:     score,
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	scores, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	scores = append(scores, next)
	sortGameScores(scores)
	if len(scores) > maxGameScores {
		scores = scores[:maxGameScores]
	}
	if err := s.writeLocked(scores); err != nil {
		return nil, err
	}
	return scores, nil
}

func (s *GameScoreStore) readLocked() ([]GameScore, error) {
	bytes, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []GameScore{}, nil
	}
	if err != nil {
		return nil, err
	}

	var response struct {
		Scores []GameScore `json:"scores"`
	}
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	scores := response.Scores[:0]
	for _, score := range response.Scores {
		score.Name = strings.Join(strings.Fields(score.Name), " ")
		if score.Name == "" {
			score.Name = defaultAuthor
		}
		runes := []rune(score.Name)
		if len(runes) > maxGameNameRunes {
			score.Name = string(runes[:maxGameNameRunes])
		}
		scores = append(scores, score)
	}
	sortGameScores(scores)
	if len(scores) > maxGameScores {
		scores = scores[:maxGameScores]
	}
	return scores, nil
}

func (s *GameScoreStore) writeLocked(scores []GameScore) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(s.path), ".game-scores-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(map[string][]GameScore{"scores": scores}); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func sortGameScores(scores []GameScore) {
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].Score != scores[j].Score {
			return scores[i].Score > scores[j].Score
		}
		return scores[i].CreatedAt.Before(scores[j].CreatedAt)
	})
}
