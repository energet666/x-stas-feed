package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGameScoreStoreKeepsTopFive(t *testing.T) {
	store := NewGameScoreStore(t.TempDir())

	for _, entry := range []struct {
		name  string
		score int
	}{
		{name: "A", score: 100},
		{name: "B", score: 500},
		{name: "C", score: -200},
		{name: "D", score: 300},
		{name: "E", score: 700},
		{name: "F", score: 200},
	} {
		if _, err := store.Add(entry.name, entry.score); err != nil {
			t.Fatal(err)
		}
	}

	scores, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 5 {
		t.Fatalf("expected five scores, got %#v", scores)
	}

	want := []int{700, 500, 300, 200, 100}
	for i, score := range scores {
		if score.Score != want[i] {
			t.Fatalf("score %d: expected %d, got %#v", i, want[i], scores)
		}
	}
}

func TestGameScoreStoreNormalizesNames(t *testing.T) {
	store := NewGameScoreStore(t.TempDir())

	scores, err := store.Add("   ", 10)
	if err != nil {
		t.Fatal(err)
	}
	if scores[0].Name != defaultAuthor {
		t.Fatalf("expected default author, got %#v", scores[0])
	}
}

func TestGameScoreStoreWritesLeaderboardInOwnDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewGameScoreStore(dir)

	if _, err := store.Add("Pilot", 100); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, gameScoresDirName, gameScoresFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected leaderboard file at %s: %v", path, err)
	}
}
